package model

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// Old BuildLoadMoreURL, copied verbatim from helpers.go before removal.
func oldBuildLoadMoreURL(owner, repo, scope, contributor, status string, page int) string {
	base := "/" + owner + "/" + repo
	parts := []string{}
	if scope != "" {
		parts = append(parts, "scope="+scope)
	}
	if page > 1 {
		parts = append(parts, fmt.Sprintf("page=%d", page))
	}
	if contributor != "" {
		parts = append(parts, "contributor="+contributor)
	}
	if status != "" {
		parts = append(parts, "status="+status)
	}
	if len(parts) > 0 {
		base += "?" + strings.Join(parts, "&")
	}
	return base
}

// oldBuildStatusURL is the old buildStatusURL from counts.templ.
func oldBuildStatusURL(owner, repo, scope, contributor, currentStatus, targetStatus string) string {
	base := "/" + owner + "/" + repo
	parts := []string{}
	if scope != "" {
		parts = append(parts, "scope="+scope)
	}
	if contributor != "" {
		parts = append(parts, "contributor="+contributor)
	}
	if targetStatus != currentStatus {
		parts = append(parts, "status="+targetStatus)
	}
	if len(parts) > 0 {
		base += "?" + strings.Join(parts, "&")
	}
	return base
}

// parseURL splits a URL string into its path and its raw query components.
// It deliberately does not percent-decode: a want value written with a
// literal "," must only match a URL that actually contains a literal ",".
func parseURL(raw string) (path string, components []string) {
	idx := strings.Index(raw, "?")
	if idx == -1 {
		return raw, nil
	}
	path = raw[:idx]
	if q := raw[idx+1:]; q != "" {
		components = strings.Split(q, "&")
	}
	return path, components
}

// urlsEqual reports whether two URLs have the same path and the same raw
// query components. Component order is ignored because url.Values.Encode
// sorts keys while want strings are written in readability order.
func urlsEqual(a, b string) bool {
	aPath, aComponents := parseURL(a)
	bPath, bComponents := parseURL(b)
	if aPath != bPath || len(aComponents) != len(bComponents) {
		return false
	}
	slices.Sort(aComponents)
	slices.Sort(bComponents)
	return slices.Equal(aComponents, bComponents)
}

func TestRepositoryFilters_URL(t *testing.T) {
	tests := []struct {
		name    string
		filters RepositoryFilters
		want    string
	}{
		{
			name:    "owner and repo only",
			filters: RepositoryFilters{Owner: "octocat", Repository: "hello-world"},
			want:    "/octocat/hello-world",
		},
		{
			name:    "with scope",
			filters: RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend"}},
			want:    "/octocat/hello-world?scope=backend",
		},
		{
			name:    "with contributor",
			filters: RepositoryFilters{Owner: "octocat", Repository: "hello-world", Contributor: []string{"alice"}},
			want:    "/octocat/hello-world?contributor=alice",
		},
		{
			name:    "with status",
			filters: RepositoryFilters{Owner: "octocat", Repository: "hello-world", Status: []string{"fresh"}},
			want:    "/octocat/hello-world?status=fresh",
		},
		{
			name:    "with scope and contributor",
			filters: RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend"}, Contributor: []string{"alice"}},
			want:    "/octocat/hello-world?scope=backend&contributor=alice",
		},
		{
			name:    "with scope and status",
			filters: RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend"}, Status: []string{"stale"}},
			want:    "/octocat/hello-world?scope=backend&status=stale",
		},
		{
			name:    "with contributor and status",
			filters: RepositoryFilters{Owner: "octocat", Repository: "hello-world", Contributor: []string{"alice"}, Status: []string{"fresh"}},
			want:    "/octocat/hello-world?contributor=alice&status=fresh",
		},
		{
			name:    "with all filters",
			filters: RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend"}, Contributor: []string{"alice"}, Status: []string{"fresh"}},
			want:    "/octocat/hello-world?scope=backend&contributor=alice&status=fresh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filters.URL()
			if !urlsEqual(got, tt.want) {
				t.Errorf("RepositoryFilters.URL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRepositoryFilters_PageURL(t *testing.T) {
	tests := []struct {
		name    string
		filters RepositoryFilters
		page    int
		want    string
	}{
		{
			name:    "page 1 omits page param",
			filters: RepositoryFilters{Owner: "octocat", Repository: "hello-world"},
			page:    1,
			want:    "/octocat/hello-world",
		},
		{
			name:    "page 0 omits page param",
			filters: RepositoryFilters{Owner: "octocat", Repository: "hello-world"},
			page:    0,
			want:    "/octocat/hello-world",
		},
		{
			name:    "page 2 without other filters",
			filters: RepositoryFilters{Owner: "octocat", Repository: "hello-world"},
			page:    2,
			want:    "/octocat/hello-world?page=2",
		},
		{
			name:    "page 5 with scope",
			filters: RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend"}},
			page:    5,
			want:    "/octocat/hello-world?scope=backend&page=5",
		},
		{
			name:    "page 3 with contributor and status",
			filters: RepositoryFilters{Owner: "octocat", Repository: "hello-world", Contributor: []string{"alice"}, Status: []string{"fresh"}},
			page:    3,
			want:    "/octocat/hello-world?contributor=alice&status=fresh&page=3",
		},
		{
			name:    "page 10 with all filters",
			filters: RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend"}, Contributor: []string{"alice"}, Status: []string{"stale"}},
			page:    10,
			want:    "/octocat/hello-world?scope=backend&contributor=alice&status=stale&page=10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filters.PageURL(tt.page)
			if !urlsEqual(got, tt.want) {
				t.Errorf("RepositoryFilters.PageURL(%d) = %q, want %q", tt.page, got, tt.want)
			}
		})
	}
}

func TestRepositoryFilters_PageURL_MatchesOldBuildLoadMoreURL(t *testing.T) {
	filters := []RepositoryFilters{
		{Owner: "octocat", Repository: "hello-world"},
		{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend"}},
		{Owner: "octocat", Repository: "hello-world", Contributor: []string{"alice"}},
		{Owner: "octocat", Repository: "hello-world", Status: []string{"fresh"}},
		{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend"}, Contributor: []string{"alice"}, Status: []string{"stale"}},
	}
	pages := []int{1, 2, 5, 100}

	for _, f := range filters {
		for _, p := range pages {
			scope, contributor, status := strings.Join(f.Scope, ","), strings.Join(f.Contributor, ","), strings.Join(f.Status, ",")
			name := fmt.Sprintf("owner=%s/repo=%s/scope=%s/contrib=%s/status=%s/page=%d",
				f.Owner, f.Repository, scope, contributor, status, p)
			t.Run(name, func(t *testing.T) {
				want := oldBuildLoadMoreURL(f.Owner, f.Repository, scope, contributor, status, p)
				got := f.PageURL(p)
				if !urlsEqual(got, want) {
					t.Errorf("PageURL(%d) = %q, old = %q", p, got, want)
				}
			})
		}
	}
}

func TestRepositoryFilters_WithStatus(t *testing.T) {
	filters := RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend"}, Contributor: []string{"alice"}, Status: []string{"fresh"}}

	t.Run("toggle adds status not currently selected", func(t *testing.T) {
		got := filters.WithStatus("stale")
		if !slices.Equal(got.Status, []string{"fresh", "stale"}) {
			t.Errorf("expected status [fresh stale], got %q", got.Status)
		}
		if !urlsEqual(got.URL(), "/octocat/hello-world?scope=backend&contributor=alice&status=fresh,stale") {
			t.Errorf("URL = %q", got.URL())
		}
	})

	t.Run("empty value is ignored", func(t *testing.T) {
		got := filters.WithStatus("")
		if !slices.Equal(got.Status, []string{"fresh"}) {
			t.Errorf("expected status [fresh], got %q", got.Status)
		}
		if !urlsEqual(got.URL(), "/octocat/hello-world?scope=backend&contributor=alice&status=fresh") {
			t.Errorf("URL = %q", got.URL())
		}
	})

	t.Run("does not mutate original", func(t *testing.T) {
		_ = filters.WithStatus("stale")
		if !slices.Equal(filters.Status, []string{"fresh"}) {
			t.Errorf("original was mutated: status = %q", filters.Status)
		}
	})
}

func TestRepositoryFilters_StatusToggle_MatchesOldBuildStatusURL(t *testing.T) {
	statuses := []string{"fresh", "stale", "expired", "merged"}
	filters := RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend"}, Contributor: []string{"alice"}}

	for _, target := range statuses {
		name := fmt.Sprintf("target=%q", target)
		t.Run(name, func(t *testing.T) {
			// repositoryStatusURL from counts.templ toggles the target status.
			got := filters.WithStatus(target).URL()

			// With no status currently selected, adding a status matches the old
			// single-select buildStatusURL output.
			want := oldBuildStatusURL(filters.Owner, filters.Repository, strings.Join(filters.Scope, ","), strings.Join(filters.Contributor, ","), strings.Join(filters.Status, ","), target)
			if !urlsEqual(got, want) {
				t.Errorf("WithStatus(%q).URL() = %q, old buildStatusURL = %q", target, got, want)
			}
		})
	}

	t.Run("toggling off the selected status clears the filter", func(t *testing.T) {
		got := filters.WithStatus("fresh").WithStatus("fresh")
		if len(got.Status) != 0 {
			t.Errorf("expected empty status, got %q", got.Status)
		}
		if !urlsEqual(got.URL(), "/octocat/hello-world?scope=backend&contributor=alice") {
			t.Errorf("URL = %q", got.URL())
		}
	})

	t.Run("empty value is ignored", func(t *testing.T) {
		got := filters.WithStatus("fresh").WithStatus("stale").WithStatus("")
		if !slices.Equal(got.Status, []string{"fresh", "stale"}) {
			t.Errorf("expected status [fresh stale], got %q", got.Status)
		}
		if !urlsEqual(got.URL(), "/octocat/hello-world?scope=backend&contributor=alice&status=fresh,stale") {
			t.Errorf("URL = %q", got.URL())
		}
	})
}

func TestRepositoryFilters_WithScope(t *testing.T) {
	filters := RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend"}, Contributor: []string{"alice"}, Status: []string{"fresh"}}

	t.Run("empty value is ignored", func(t *testing.T) {
		got := filters.WithScope("")
		if !slices.Equal(got.Scope, []string{"backend"}) {
			t.Errorf("expected scope [backend], got %q", got.Scope)
		}
		if !urlsEqual(got.URL(), "/octocat/hello-world?scope=backend&contributor=alice&status=fresh") {
			t.Errorf("URL = %q", got.URL())
		}
	})

	t.Run("toggle adds scope not currently selected", func(t *testing.T) {
		got := filters.WithScope("frontend")
		if !slices.Equal(got.Scope, []string{"backend", "frontend"}) {
			t.Errorf("expected scope [backend frontend], got %q", got.Scope)
		}
		if !urlsEqual(got.URL(), "/octocat/hello-world?scope=backend,frontend&contributor=alice&status=fresh") {
			t.Errorf("URL = %q", got.URL())
		}
	})

	t.Run("does not mutate original", func(t *testing.T) {
		_ = filters.WithScope("frontend")
		if !slices.Equal(filters.Scope, []string{"backend"}) {
			t.Errorf("original was mutated: scope = %q", filters.Scope)
		}
	})

	t.Run("toggling off does not mutate original", func(t *testing.T) {
		multi := RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend", "frontend"}}
		got := multi.WithScope("backend")
		if !slices.Equal(got.Scope, []string{"frontend"}) {
			t.Errorf("expected scope [frontend], got %q", got.Scope)
		}
		if !slices.Equal(multi.Scope, []string{"backend", "frontend"}) {
			t.Errorf("original was mutated: scope = %q", multi.Scope)
		}
	})
}

func TestRepositoryFilters_WithContributor(t *testing.T) {
	filters := RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend"}, Contributor: []string{"alice"}, Status: []string{"fresh"}}

	t.Run("empty value is ignored", func(t *testing.T) {
		got := filters.WithContributor("")
		if !slices.Equal(got.Contributor, []string{"alice"}) {
			t.Errorf("expected contributor [alice], got %q", got.Contributor)
		}
		if !urlsEqual(got.URL(), "/octocat/hello-world?scope=backend&contributor=alice&status=fresh") {
			t.Errorf("URL = %q", got.URL())
		}
	})

	t.Run("toggle adds contributor not currently selected", func(t *testing.T) {
		got := filters.WithContributor("bob")
		if !slices.Equal(got.Contributor, []string{"alice", "bob"}) {
			t.Errorf("expected contributor [alice bob], got %q", got.Contributor)
		}
		if !urlsEqual(got.URL(), "/octocat/hello-world?scope=backend&contributor=alice,bob&status=fresh") {
			t.Errorf("URL = %q", got.URL())
		}
	})

	t.Run("does not mutate original", func(t *testing.T) {
		_ = filters.WithContributor("bob")
		if !slices.Equal(filters.Contributor, []string{"alice"}) {
			t.Errorf("original was mutated: contributor = %q", filters.Contributor)
		}
	})
}

func TestRepositoryFilters_ClearScope_MatchesOldScopesClearFilter(t *testing.T) {
	filters := RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend"}, Contributor: []string{"alice"}, Status: []string{"fresh"}}

	got := filters.WithScope("backend").URL()
	want := "/octocat/hello-world?contributor=alice&status=fresh"
	if !urlsEqual(got, want) {
		t.Errorf("clear scope URL = %q, want %q", got, want)
	}
}

func TestRepositoryFilters_ClearScope_RemovesOnlyCurrentScope(t *testing.T) {
	filters := RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend", "frontend"}}

	got := filters.WithScope("backend").URL()
	want := "/octocat/hello-world?scope=frontend"
	if !urlsEqual(got, want) {
		t.Errorf("clear scope URL = %q, want %q", got, want)
	}
}

func TestRepositoryFilters_ClearContributor_MatchesOldContributorsClearFilter(t *testing.T) {
	filters := RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend"}, Contributor: []string{"alice"}, Status: []string{"fresh"}}

	got := filters.WithContributor("alice").URL()
	want := "/octocat/hello-world?scope=backend&status=fresh"
	if !urlsEqual(got, want) {
		t.Errorf("clear contributor URL = %q, want %q", got, want)
	}
}

func TestRepositoryFilters_ClearContributor_RemovesOnlyCurrentContributor(t *testing.T) {
	filters := RepositoryFilters{Owner: "octocat", Repository: "hello-world", Contributor: []string{"alice", "bob"}}

	got := filters.WithContributor("alice").URL()
	want := "/octocat/hello-world?contributor=bob"
	if !urlsEqual(got, want) {
		t.Errorf("clear contributor URL = %q, want %q", got, want)
	}
}

func TestRepositoryFilters_ScopeLink_AddsToCurrentSelection(t *testing.T) {
	filters := RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"old-scope"}, Contributor: []string{"alice"}, Status: []string{"fresh"}}

	got := filters.WithScope("new-scope").URL()
	want := "/octocat/hello-world?scope=old-scope,new-scope&contributor=alice&status=fresh"
	if !urlsEqual(got, want) {
		t.Errorf("scope link URL = %q, want %q", got, want)
	}
}

func TestRepositoryFilters_ContributorLink_AddsToCurrentSelection(t *testing.T) {
	filters := RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend"}, Contributor: []string{"alice"}, Status: []string{"fresh"}}

	got := filters.WithContributor("bob").URL()
	want := "/octocat/hello-world?scope=backend&contributor=alice,bob&status=fresh"
	if !urlsEqual(got, want) {
		t.Errorf("contributor link URL = %q, want %q", got, want)
	}
}

func TestRepositoryFilters_URL_MultipleSelections(t *testing.T) {
	tests := []struct {
		name    string
		filters RepositoryFilters
		want    string
	}{
		{
			name:    "multiple scopes",
			filters: RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend", "frontend"}},
			want:    "/octocat/hello-world?scope=backend,frontend",
		},
		{
			name:    "multiple contributors",
			filters: RepositoryFilters{Owner: "octocat", Repository: "hello-world", Contributor: []string{"alice", "bob"}},
			want:    "/octocat/hello-world?contributor=alice,bob",
		},
		{
			name:    "multiple statuses",
			filters: RepositoryFilters{Owner: "octocat", Repository: "hello-world", Status: []string{"fresh", "stale"}},
			want:    "/octocat/hello-world?status=fresh,stale",
		},
		{
			name:    "multiple selections in one filter with a single selection in another",
			filters: RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend", "frontend"}, Status: []string{"stale"}},
			want:    "/octocat/hello-world?scope=backend,frontend&status=stale",
		},
		{
			name: "multiple selections across every filter",
			filters: RepositoryFilters{
				Owner: "octocat", Repository: "hello-world",
				Scope:       []string{"backend", "frontend", "infra"},
				Contributor: []string{"alice", "bob"},
				Status:      []string{"fresh", "stale", "expired"},
			},
			want: "/octocat/hello-world?scope=backend,frontend,infra&contributor=alice,bob&status=fresh,stale,expired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filters.URL()
			if strings.Contains(got, "%2C") {
				t.Errorf("RepositoryFilters.URL() percent-escaped the separator: %q", got)
			}
			if !urlsEqual(got, tt.want) {
				t.Errorf("RepositoryFilters.URL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRepositoryFilters_PageURL_MultipleSelections(t *testing.T) {
	filters := RepositoryFilters{
		Owner: "octocat", Repository: "hello-world",
		Scope:       []string{"backend", "frontend"},
		Contributor: []string{"alice", "bob"},
		Status:      []string{"fresh", "stale"},
	}

	tests := []struct {
		name string
		page int
		want string
	}{
		{
			name: "page 1 keeps multi-selection filters without a page param",
			page: 1,
			want: "/octocat/hello-world?scope=backend,frontend&contributor=alice,bob&status=fresh,stale",
		},
		{
			name: "page 5 adds the page param alongside multi-selection filters",
			page: 5,
			want: "/octocat/hello-world?scope=backend,frontend&contributor=alice,bob&status=fresh,stale&page=5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filters.PageURL(tt.page)
			if !urlsEqual(got, tt.want) {
				t.Errorf("RepositoryFilters.PageURL(%d) = %q, want %q", tt.page, got, tt.want)
			}
		})
	}
}

func TestRepositoryFilters_SortsRoundTripThroughTheURL(t *testing.T) {
	filters := RepositoryFilters{
		Owner: "octocat", Repository: "hello-world",
		Scope: []string{"backend"}, Contributor: []string{"alice"}, Status: []string{"stale"},
	}

	tests := []struct {
		name   string
		filter RepositoryFilters
		want   string
	}{
		{
			name:   "recency is the default and stays out of the URL",
			filter: filters.WithScopeSort("top").WithScopeSort("recent").WithContributorSort("nonsense"),
			want:   "/octocat/hello-world?scope=backend&contributor=alice&status=stale",
		},
		{
			name:   "top scope sort is kept alongside the filters",
			filter: filters.WithScopeSort("top"),
			want:   "/octocat/hello-world?scope=backend&contributor=alice&status=stale&scope_sort=top",
		},
		{
			name:   "top contributor sort is kept alongside the filters",
			filter: filters.WithContributorSort("top"),
			want:   "/octocat/hello-world?scope=backend&contributor=alice&status=stale&contributor_sort=top",
		},
		{
			name:   "both sorts are kept alongside the filters",
			filter: filters.WithScopeSort("top").WithContributorSort("top"),
			want:   "/octocat/hello-world?scope=backend&contributor=alice&status=stale&scope_sort=top&contributor_sort=top",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.URL(); !urlsEqual(got, tt.want) {
				t.Errorf("RepositoryFilters.URL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRepositoryFilters_FragmentURLCarriesFiltersAndSorts(t *testing.T) {
	filters := RepositoryFilters{
		Owner: "octocat", Repository: "hello-world",
		Scope: []string{"backend"}, Contributor: []string{"alice"}, Status: []string{"stale"},
		ScopeSort: SortTop, ContributorSort: SortTop,
	}

	tests := []struct {
		name     string
		fragment string
		page     int
		want     string
	}{
		{
			name:     "scopes fragment keeps both sorts and adds the fragment name",
			fragment: "scopes",
			page:     1,
			want:     "/octocat/hello-world?scope=backend&contributor=alice&status=stale&scope_sort=top&contributor_sort=top&fragment=scopes",
		},
		{
			name:     "contributors fragment keeps both sorts",
			fragment: "contributors",
			page:     1,
			want:     "/octocat/hello-world?scope=backend&contributor=alice&status=stale&scope_sort=top&contributor_sort=top&fragment=contributors",
		},
		{
			name:     "a loaded page is kept so the fragment rebuilds the same summaries",
			fragment: "scopes",
			page:     3,
			want:     "/octocat/hello-world?scope=backend&contributor=alice&status=stale&scope_sort=top&contributor_sort=top&page=3&fragment=scopes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filters.FragmentURL(tt.fragment, tt.page); !urlsEqual(got, tt.want) {
				t.Errorf("RepositoryFilters.FragmentURL(%q, %d) = %q, want %q", tt.fragment, tt.page, got, tt.want)
			}
		})
	}
}

func TestRepositoryFilters_Label(t *testing.T) {
	tests := []struct {
		name    string
		filters RepositoryFilters
		want    string
	}{
		{name: "no filters", filters: RepositoryFilters{}, want: ""},
		{
			name:    "every selected value is listed in filter order",
			filters: RepositoryFilters{Scope: []string{"backend", "frontend"}, Contributor: []string{"alice"}, Status: []string{"stale"}},
			want:    "backend, frontend, alice, stale",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filters.Label(); got != tt.want {
				t.Errorf("RepositoryFilters.Label() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeSortOrder(t *testing.T) {
	tests := map[string]string{
		"":        SortRecent,
		"recent":  SortRecent,
		"top":     SortTop,
		"unknown": SortRecent,
	}
	for input, want := range tests {
		if got := NormalizeSortOrder(input); got != want {
			t.Errorf("NormalizeSortOrder(%q) = %q, want %q", input, got, want)
		}
	}
}
