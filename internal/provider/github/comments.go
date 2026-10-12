package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"merge/internal/model"
	"merge/internal/provider"
)

type pullRequestCommentCounts struct {
	Comments       int `json:"comments"`
	ReviewComments int `json:"review_comments"`
}

func (g GitHub) populatePullRequestCommentCounts(owner, repository string, pullRequests []model.PullRequest) error {
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
			counts, err := g.getPullRequestCommentCounts(owner, repository, pr.Number)
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
			return err
		}
	}
	return nil
}

func (g GitHub) getPullRequestCommentCounts(owner, repository string, number int) (pullRequestCommentCounts, error) {
	requestURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repository, number)
	request, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return pullRequestCommentCounts{}, err
	}
	g.configureAPIRequest(request)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return pullRequestCommentCounts{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		provider.LogRateLimitExceeded(g.Logger, response)
		return pullRequestCommentCounts{}, fmt.Errorf("request for pull request #%d comment counts failed with status: %s", number, response.Status)
	}

	var counts pullRequestCommentCounts
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&counts); err != nil {
		return pullRequestCommentCounts{}, err
	}
	return counts, nil
}
