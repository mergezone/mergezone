package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"merge/internal/model"
	"merge/internal/provider"
	"merge/internal/telemetry"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type pullRequestCommentCounts struct {
	Comments       int `json:"comments"`
	ReviewComments int `json:"review_comments"`
}

func (g GitHub) populatePullRequestCommentCounts(ctx context.Context, owner, repository string, pullRequests []model.PullRequest, gatherWaits *gatherWaitTracker) error {
	ctx, span := telemetry.Tracer("merge/github").Start(ctx, "github.scrape_comment_counts",
		trace.WithAttributes(
			attribute.String("github.owner", owner),
			attribute.String("github.repository", repository),
			attribute.Int("github.pull_request_count", len(pullRequests)),
		),
	)
	defer span.End()
	if len(pullRequests) == 0 {
		return nil
	}

	semaphore := make(chan struct{}, 8)
	var waitGroup sync.WaitGroup
	errCh := make(chan error, len(pullRequests))
	for i := range pullRequests {
		waitGroup.Add(1)
		go func(pr *model.PullRequest) {
			defer waitGroup.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			counts, err := g.getPullRequestCommentCounts(ctx, owner, repository, pr.Number, gatherWaits)
			if err != nil {
				errCh <- err
				return
			}
			pr.Comments = counts.Comments
			pr.ReviewComments = counts.ReviewComments
		}(&pullRequests[i])
	}
	waitGroup.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			telemetry.RecordError(span, err)
			return err
		}
	}
	return nil
}

func (g GitHub) getPullRequestCommentCounts(ctx context.Context, owner, repository string, number int, gatherWaits *gatherWaitTracker) (pullRequestCommentCounts, error) {
	requestURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repository, number)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return pullRequestCommentCounts{}, err
	}
	g.configureAPIRequest(request)

	ctx, span := telemetry.Tracer("merge/github").Start(ctx, "github.pull_request_comment_counts",
		trace.WithAttributes(attribute.Int("github.pull_request.number", number)),
	)
	defer span.End()

	waitStarted := time.Now()
	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}
	response, err := client.Do(request.WithContext(ctx))
	if err != nil {
		gatherWaits.add(waitStarted, time.Now())
		telemetry.RecordError(span, err)
		return pullRequestCommentCounts{}, err
	}
	defer response.Body.Close()
	span.SetAttributes(attribute.Int("http.response.status_code", response.StatusCode))
	if response.StatusCode != http.StatusOK {
		gatherWaits.add(waitStarted, time.Now())
		provider.LogRateLimitExceeded(g.Logger, response)
		err := fmt.Errorf("request for pull request #%d comment counts failed with status: %s", number, response.Status)
		telemetry.RecordError(span, err)
		return pullRequestCommentCounts{}, err
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	gatherWaits.add(waitStarted, time.Now())
	if err != nil {
		telemetry.RecordError(span, err)
		return pullRequestCommentCounts{}, err
	}
	var counts pullRequestCommentCounts
	if err := json.Unmarshal(body, &counts); err != nil {
		telemetry.RecordError(span, err)
		return pullRequestCommentCounts{}, err
	}
	return counts, nil
}
