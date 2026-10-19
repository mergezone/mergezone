package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"merge/internal/model"
	"merge/internal/provider"
	"merge/internal/telemetry"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type GitHub struct {
	Token  string
	Logger *slog.Logger
}

// githubHTTPClient is shared by every GitHub API call so that the
// transport-level response cache and connection pool are reused across
// requests. The cache transport wraps otelhttp so cache hits are visible in
// traces without a corresponding network span.
var githubHTTPClient = &http.Client{
	Transport: newCachingTransport(otelhttp.NewTransport(http.DefaultTransport)),
}

func (g GitHub) configureAPIRequest(request *http.Request) {
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+g.Token)
	request.Header.Set("X-GitHub-Api-Version", "2026-03-10")
}

func responseHasNextPage(response *http.Response) bool {
	linkHeader := response.Header.Get("Link")
	return strings.Contains(linkHeader, `rel="next"`)
}

func (g GitHub) getPullRequestsJSON(ctx context.Context, query provider.RepositoryQuery, options provider.PullRequestListOptions, gatherWaits *gatherWaitTracker, crawlSpan trace.Span) ([]byte, provider.PullRequestPagination, error) {
	requestURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?state=all&per_page=%d&page=%d", query.Owner, query.Repository, options.PerPage, options.Page)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	pagination := provider.PullRequestPagination{}
	if err != nil {
		return nil, pagination, err
	}

	g.configureAPIRequest(req)

	waitStarted := time.Now()
	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		gatherWaits.add(waitStarted, time.Now())
		return nil, pagination, err
	}
	defer resp.Body.Close()
	crawlSpan.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))

	if resp.StatusCode != http.StatusOK {
		gatherWaits.add(waitStarted, time.Now())
		provider.LogRateLimitExceeded(g.Logger, resp)
		return nil, pagination, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	gatherWaits.add(waitStarted, time.Now())
	if err != nil {
		return nil, pagination, err
	}
	pagination.HasNext = responseHasNextPage(resp)

	return body, pagination, nil
}

func (g GitHub) GetPullRequests(ctx context.Context, query provider.RepositoryQuery, options provider.PullRequestListOptions) ([]model.StampedPullRequest, provider.PullRequestPagination, provider.PullRequestTimings, error) {
	ctx, crawlSpan := telemetry.Tracer("merge/github").Start(ctx, "github.crawl",
		trace.WithAttributes(
			attribute.String("github.owner", query.Owner),
			attribute.String("github.repository", query.Repository),
			attribute.Int("github.page", options.Page),
		),
	)
	defer crawlSpan.End()

	operationStarted := time.Now()
	gatherWaits := &gatherWaitTracker{}
	pullRequestJSON, hasNext, err := g.getPullRequestsJSON(ctx, query, options, gatherWaits, crawlSpan)
	if err != nil {
		telemetry.RecordError(crawlSpan, err)
		return nil, hasNext, pullRequestTimings(operationStarted, gatherWaits), err
	}

	var pullRequests []model.PullRequest
	_, parseSpan := telemetry.Tracer("merge/github").Start(ctx, "github.parse_pull_requests")
	if err := json.Unmarshal(pullRequestJSON, &pullRequests); err != nil {
		telemetry.RecordError(parseSpan, err)
		parseSpan.End()
		telemetry.RecordError(crawlSpan, err)
		return nil, hasNext, pullRequestTimings(operationStarted, gatherWaits), err
	}
	parseSpan.SetAttributes(attribute.Int("github.pull_request_count", len(pullRequests)))
	parseSpan.End()

	if err := g.populatePullRequestCommentCounts(ctx, query.Owner, query.Repository, pullRequests, gatherWaits); err != nil {
		telemetry.RecordError(crawlSpan, err)
		return nil, hasNext, pullRequestTimings(operationStarted, gatherWaits), err
	}

	_, logicSpan := telemetry.Tracer("merge/github").Start(ctx, "github.filter_and_categorize",
		trace.WithAttributes(attribute.Int("github.pull_request_count", len(pullRequests))),
	)
	// TODO(hayden): Implement `Mappable` interface
	stampedPullRequests := model.StampNow(pullRequests)

	filteredPullRequests := filterPullRequests(stampedPullRequests, query)
	logicSpan.SetAttributes(attribute.Int("github.filtered_pull_request_count", len(filteredPullRequests)))
	logicSpan.End()
	return filteredPullRequests, hasNext, pullRequestTimings(operationStarted, gatherWaits), nil
}

func (g GitHub) GetPullRequestDiff(ctx context.Context, owner, repository string, pullRequestNumber int) (string, error) {
	ctx, span := telemetry.Tracer("merge/github").Start(ctx, "github.pull_request_diff",
		trace.WithAttributes(
			attribute.String("github.owner", owner),
			attribute.String("github.repository", repository),
			attribute.Int("github.pull_request.number", pullRequestNumber),
		),
	)
	defer span.End()

	requestURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repository, pullRequestNumber)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/vnd.github.v3.diff")
	req.Header.Set("Authorization", "Bearer "+g.Token)
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")

	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		telemetry.RecordError(span, err)
		return "", err
	}
	defer resp.Body.Close()
	span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))

	if resp.StatusCode != http.StatusOK {
		provider.LogRateLimitExceeded(g.Logger, resp)
		err := fmt.Errorf("request failed with status: %s", resp.Status)
		telemetry.RecordError(span, err)
		return "", err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		telemetry.RecordError(span, err)
		return "", err
	}

	return string(body), nil
}
