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

func ParseRepositoryQuery(routeVariables map[string]string, queryValues url.Values) (RepositoryQuery, PullRequestListOptions) {
	repositoryQuery := RepositoryQuery{
		Owner:      routeVariables["owner"],
		Repository: routeVariables["repo"],
		// TODO(hayden): Move this to options for historical slicing?
		AsOf: time.Now(),
	}
	if scope := queryValues.Get("scope"); scope != "" {
		repositoryQuery.Scope = strings.Split(scope, ",")
	}
	if contributor := queryValues.Get("contributor"); contributor != "" {
		repositoryQuery.Contributor = strings.Split(contributor, ",")
	}
	if status := queryValues.Get("status"); status != "" {
		repositoryQuery.Status = strings.Split(status, ",")
	}

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
	GetPullRequests(ctx context.Context, query RepositoryQuery, options PullRequestListOptions) ([]model.StampedPullRequest, PullRequestPagination, PullRequestTimings, error)
	GetPullRequestDiff(ctx context.Context, owner, repository string, pullRequestNumber int) (string, error)
	GetPullRequestComments(ctx context.Context, owner, repository string, pullRequestNumber int) ([]model.PullRequestComment, error)
}
