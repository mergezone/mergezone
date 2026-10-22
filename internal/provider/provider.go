package provider

import (
	"context"
	"strings"

	"merge/internal/model"
	"net/url"
	"strconv"
	"time"
)

type RepositoryQuery struct {
	Owner       string
	Repository  string
	AsOf        time.Time
	Scope       []string
	Contributor []string
	// TODO Permit only values of Status enum
	Status []string
}

type PullRequestListOptions struct {
	PerPage int
	Page    int
}

func (options PullRequestListOptions) WithPage(page int) PullRequestListOptions {
	options.Page = page
	return options
}

func repositoryQueryIntOrDefault(key string, queryValues url.Values, fallback int) int {
	if rawValue := queryValues.Get(key); rawValue != "" {
		if parsed, err := strconv.Atoi(rawValue); err == nil {
			return parsed
		}
	}

	return fallback
}

// parseFilterValues splits a comma-separated filter value, trimming each entry
// and dropping empty ones. An empty or all-empty value yields a nil selection,
// so a stray comma cannot filter every pull request out of the list.
func parseFilterValues(rawValue string) []string {
	if rawValue == "" {
		return nil
	}

	values := make([]string, 0)
	for _, value := range strings.Split(rawValue, ",") {
		value = strings.TrimSpace(value)
		if value != "" {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func ParseRepositoryQuery(routeVariables map[string]string, queryValues url.Values) (RepositoryQuery, PullRequestListOptions) {
	repositoryQuery := RepositoryQuery{
		Owner:      routeVariables["owner"],
		Repository: routeVariables["repo"],
		// TODO(hayden): Move this to options for historical slicing?
		AsOf: time.Now(),
	}
	// A scope can arrive as a query selection or as the path segment used by
	// the pull request card's scope chips; an explicit query selection wins.
	if scope := parseFilterValues(queryValues.Get("scope")); len(scope) > 0 {
		repositoryQuery.Scope = scope
	} else if pathScope := strings.Trim(routeVariables["scope"], "/"); pathScope != "" {
		repositoryQuery.Scope = parseFilterValues(pathScope)
	}
	repositoryQuery.Contributor = parseFilterValues(queryValues.Get("contributor"))
	repositoryQuery.Status = parseFilterValues(queryValues.Get("status"))

	options := PullRequestListOptions{
		PerPage: 20,
		Page:    repositoryQueryIntOrDefault("page", queryValues, 1),
	}

	return repositoryQuery, options
}

type PullRequestPagination struct {
	HasNext bool
}

type PullRequestTimings struct {
	Gather time.Duration
}

type Provider interface {
	// GetPullRequests returns a page of the repository's pull requests without
	// applying the query's scope, contributor, or status selections. The query
	// still identifies the repository and is the natural place to carry the
	// selections, so callers pick the pull requests they want with
	// FilterPullRequests once the unfiltered set is available.
	GetPullRequests(ctx context.Context, query RepositoryQuery, options PullRequestListOptions) ([]model.StampedPullRequest, PullRequestPagination, PullRequestTimings, error)
	GetPullRequestDiff(ctx context.Context, owner, repository string, pullRequestNumber int) (string, error)
	GetPullRequestComments(ctx context.Context, owner, repository string, pullRequestNumber int) ([]model.PullRequestComment, error)
}
