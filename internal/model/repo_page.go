package model

import (
	"net/url"
	"strconv"
	"strings"
)

// Sidebar sort orders for the scope and contributor lists. The sort is part of
// the URL-addressable view state so it survives the main-view navigations that
// selecting a filter triggers.
const (
	SortRecent = "recent"
	SortTop    = "top"
)

// NormalizeSortOrder maps an empty or unknown sort order to the default
// recency ordering.
func NormalizeSortOrder(sortOrder string) string {
	if sortOrder == SortTop {
		return SortTop
	}
	return SortRecent
}

// RepositoryFilters contains the URL-addressable filters and sidebar sort
// orders for a repository view.
type RepositoryFilters struct {
	Owner           string
	Repository      string
	Scope           []string
	Contributor     []string
	Status          []string
	ScopeSort       string
	ContributorSort string
}

func (f RepositoryFilters) URL() string {
	u := url.URL{Path: f.basePath()}
	if q := f.queryValues(); len(q) > 0 {
		u.RawQuery = encodeQueryValues(q)
	}
	return u.String()
}

// FragmentURL builds the URL for an in-place fragment response. It keeps the
// current filters and sort orders and, once the view has loaded past the first
// page, the page range, so the fragment response can rebuild the same
// summaries. The fragment name selects the response.
func (f RepositoryFilters) FragmentURL(fragment string, page int) string {
	u := url.URL{Path: f.basePath()}
	q := f.queryValues()
	if page > 1 {
		q.Set("page", strconv.Itoa(page))
	}
	q.Set("fragment", fragment)
	u.RawQuery = encodeQueryValues(q)
	return u.String()
}

func (f RepositoryFilters) PageURL(page int) string {
	if page <= 1 {
		return f.URL()
	}
	u := url.URL{Path: f.basePath()}
	q := f.queryValues()
	q.Set("page", strconv.Itoa(page))
	u.RawQuery = encodeQueryValues(q)
	return u.String()
}

func (f RepositoryFilters) basePath() string {
	p := "/" + f.Owner + "/" + f.Repository
	return p
}

// encodeQueryValues encodes q as a URL query string, keeping the commas that
// join multi-selection values literal instead of percent-escaping them as %2C.
// Commas are the only separator this URL scheme uses, and no separator
// character survives url.Values.Encode unescaped — only the unreserved
// characters A–Z a–z 0–9 and - _ . ~ pass through literally — so the encoding
// is corrected for commas rather than substituting a different separator.
func encodeQueryValues(q url.Values) string {
	return strings.ReplaceAll(q.Encode(), "%2C", ",")
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
	// Recency is the default sort, so it is omitted to keep the common URL clean.
	if f.ScopeSort == SortTop {
		q.Set("scope_sort", f.ScopeSort)
	}
	if f.ContributorSort == SortTop {
		q.Set("contributor_sort", f.ContributorSort)
	}
	return q
}

func toggleString(slice []string, value string) []string {
	// An empty filter value is an invalid state; ignore it.
	if value == "" {
		return slice
	}

	for i, item := range slice {
		if item == value {
			// Copy rather than append(slice[:i], slice[i+1:]...) so the removal
			// cannot overwrite elements visible to other slices sharing the
			// backing array (RepositoryFilters copies share them).
			toggled := make([]string, 0, len(slice)-1)
			toggled = append(toggled, slice[:i]...)
			toggled = append(toggled, slice[i+1:]...)
			return toggled
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

func (f RepositoryFilters) WithScopeSort(sortOrder string) RepositoryFilters {
	f.ScopeSort = NormalizeSortOrder(sortOrder)
	return f
}

func (f RepositoryFilters) WithContributorSort(sortOrder string) RepositoryFilters {
	f.ContributorSort = NormalizeSortOrder(sortOrder)
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
