package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"merge/internal/model"
	"merge/internal/provider"
)

type GitHub struct {
	Token string
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

func (g GitHub) getPullRequestsJSON(query provider.RepositoryQuery, options provider.PullRequestListOptions) ([]byte, provider.PullRequestPagination, error) {
	requestURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?state=all&per_page=%d&page=%d", query.Owner, query.Repository, options.PerPage, options.Page)

	req, err := http.NewRequest("GET", requestURL, nil)
	pagination := provider.PullRequestPagination{}
	if err != nil {
		return nil, pagination, err
	}

	g.configureAPIRequest(req)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, pagination, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, pagination, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, pagination, err
	}
	pagination.HasNext = responseHasNextPage(resp)

	return body, pagination, nil
}

func (g GitHub) GetPullRequests(query provider.RepositoryQuery, options provider.PullRequestListOptions) ([]model.StampedPullRequest, provider.PullRequestPagination, error) {
	bytes, hasNext, err := g.getPullRequestsJSON(query, options)
	if err != nil {
		return nil, hasNext, err
	}

	var pullRequests []model.PullRequest

	if err := json.Unmarshal(bytes, &pullRequests); err != nil {
		return nil, hasNext, err
	}

	// TODO(hayden): Implement `Mappable` interface
	stampedPullRequests := model.StampNow(pullRequests)

	filteredPullRequests := stampedPullRequests
	if query.Scope != nil {
		inScope := make([]model.StampedPullRequest, 0, len(stampedPullRequests))
		for _, pr := range stampedPullRequests {
			for _, scope := range pr.Scopes {
				if scope == *query.Scope {
					inScope = append(inScope, pr)
				}
			}
		}
		filteredPullRequests = inScope
	}

	if query.Contributor != nil {
		byContributor := make([]model.StampedPullRequest, 0, len(filteredPullRequests))
		for _, pr := range filteredPullRequests {
			if pr.Author.Name == *query.Contributor {
				byContributor = append(byContributor, pr)
			}
		}
		filteredPullRequests = byContributor
	}

	if query.Status != nil {
		byStatus := make([]model.StampedPullRequest, 0, len(filteredPullRequests))
		for _, pr := range filteredPullRequests {
			switch *query.Status {
			case "merged":
				if pr.State == model.Merged {
					byStatus = append(byStatus, pr)
				}
			default:
				if pr.State != model.Merged {
					switch *query.Status {
					case "fresh":
						if pr.ExpiryStatus == model.Fresh {
							byStatus = append(byStatus, pr)
						}
					case "stale":
						if pr.ExpiryStatus == model.Stale {
							byStatus = append(byStatus, pr)
						}
					case "expired":
						if pr.ExpiryStatus == model.Expired {
							byStatus = append(byStatus, pr)
						}
					}
				}
			}
		}
		filteredPullRequests = byStatus
	}

	return filteredPullRequests, hasNext, nil
}

func (g GitHub) GetPullRequestDiff(owner, repository string, pullRequestNumber int) (string, error) {
	requestURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repository, pullRequestNumber)
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/vnd.github.v3.diff")
	req.Header.Set("Authorization", "Bearer "+g.Token)
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request failed with status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
