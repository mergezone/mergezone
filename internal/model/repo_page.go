package model

import (
	"net/url"
	"strconv"
)

// RepositoryFilters contains the URL-addressable filters for a repository view.
type RepositoryFilters struct {
	Owner       string
	Repository  string
	Scope       string
	Contributor string
	Status      string
}

func (f RepositoryFilters) URL() string {
	u := url.URL{Path: f.basePath()}
	if q := f.queryValues(); len(q) > 0 {
		u.RawQuery = q.Encode()
	}
	return u.String()
}

func (f RepositoryFilters) PageURL(page int) string {
	if page <= 1 {
		return f.URL()
	}
	u := url.URL{Path: f.basePath()}
	q := f.queryValues()
	q.Set("page", strconv.Itoa(page))
	u.RawQuery = q.Encode()
	return u.String()
}

func (f RepositoryFilters) basePath() string {
	p := "/" + f.Owner + "/" + f.Repository
	return p
}

func (f RepositoryFilters) queryValues() url.Values {
	q := url.Values{}
	if f.Scope != "" {
		q.Set("scope", f.Scope)
	}
	if f.Contributor != "" {
		q.Set("contributor", f.Contributor)
	}
	if f.Status != "" {
		q.Set("status", f.Status)
	}
	return q
}

func (f RepositoryFilters) WithScope(scope string) RepositoryFilters {
	f.Scope = scope
	return f
}

func (f RepositoryFilters) WithContributor(contributor string) RepositoryFilters {
	f.Contributor = contributor
	return f
}

func (f RepositoryFilters) WithStatus(status string) RepositoryFilters {
	f.Status = status
	return f
}

// RepositoryViewData is the shared data model used to render the full document,
// main-view response, and repository fragments.
type RepositoryViewData struct {
	BaseURL           string
	Owner             string
	Repository        string
	Scope             string
	Contributor       string
	Status            string
	PullRequests      []StampedPullRequest
	OverallCounts     ExpiryCounts
	ScopeCounts       []ScopeInfo
	ScopeSort         string
	ContributorCounts []ContributorInfo
	ContributorSort   string
	CurrentPage       int
	HasMore           bool
}
