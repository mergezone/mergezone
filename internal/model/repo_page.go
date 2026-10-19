package model

import (
	"net/url"
	"strconv"
	"strings"
)

// RepositoryFilters contains the URL-addressable filters for a repository view.
type RepositoryFilters struct {
	Owner       string
	Repository  string
	Scope       []string
	Contributor []string
	Status      []string
}

func (f RepositoryFilters) URL() string {
	u := url.URL{Path: f.basePath()}
	if q := f.queryValues(); len(q) > 0 {
		u.RawQuery = q.Encode()
	}
	return u.String()
}

func (f RepositoryFilters) FragmentURL(fragment, sortOrder string) string {
	u := url.URL{Path: f.basePath()}
	q := f.queryValues()
	q.Set("fragment", fragment)
	q.Set("sort", sortOrder)
	u.RawQuery = q.Encode()
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
	if len(f.Scope) > 0 {
		q.Set("scope", strings.Join(f.Scope, ","))
	}
	if len(f.Contributor) > 0 {
		q.Set("contributor", strings.Join(f.Contributor, ","))
	}
	if len(f.Status) > 0 {
		q.Set("status", strings.Join(f.Status, ","))
	}
	return q
}

func toggleString(slice []string, value string) []string {
	for i, item := range slice {
		if item == value {
			return append(slice[:i], slice[i+1:]...)
		}
	}

	return append(slice, value)
}

func (f RepositoryFilters) WithScope(scope string) RepositoryFilters {
	f.Scope = toggleString(f.Scope, scope)
	return f
}

func (f RepositoryFilters) WithContributor(contributor string) RepositoryFilters {
	f.Contributor = toggleString(f.Contributor, contributor)
	return f
}

func (f RepositoryFilters) WithStatus(status string) RepositoryFilters {
	f.Status = toggleString(f.Status, status)
	return f
}

// RepositoryViewData is the shared data model used to render the full document,
// main-view response, and repository fragments.
type RepositoryViewData struct {
	BaseURL           string
	Owner             string
	Repository        string
	Scope             []string
	Contributor       []string
	Status            []string
	PullRequests      []StampedPullRequest
	OverallCounts     ExpiryCounts
	ScopeCounts       []ScopeInfo
	ScopeSort         string
	ContributorCounts []ContributorInfo
	ContributorSort   string
	CurrentPage       int
	HasMore           bool
}
