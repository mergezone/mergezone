package model

import (
	"fmt"
	"net/url"
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

// parseURL splits a URL string into path and sorted query params for order-agnostic comparison.
func parseURL(raw string) (path string, params url.Values) {
	params = url.Values{}
	if idx := strings.Index(raw, "?"); idx != -1 {
		path = raw[:idx]
		params, _ = url.ParseQuery(raw[idx+1:])
	} else {
		path = raw
	}
	return path, params
}

func urlsEqual(a, b string) bool {
	aPath, aParams := parseURL(a)
	bPath, bParams := parseURL(b)
	if aPath != bPath {
		return false
	}
	if len(aParams) != len(bParams) {
		return false
	}
	for k := range aParams {
		if aParams.Get(k) != bParams.Get(k) {
			return false
		}
	}
	return true
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

	t.Run("set different status", func(t *testing.T) {
		got := filters.WithStatus("stale")
		if !slices.Equal(got.Status, []string{"stale"}) {
			t.Errorf("expected status %q, got %q", "stale", got.Status)
		}
		if !urlsEqual(got.URL(), "/octocat/hello-world?scope=backend&contributor=alice&status=stale") {
			t.Errorf("URL = %q", got.URL())
		}
	})

	t.Run("clear status with empty string", func(t *testing.T) {
		got := filters.WithStatus("")
		if len(got.Status) != 0 {
			t.Errorf("expected empty status, got %q", got.Status)
		}
		if !urlsEqual(got.URL(), "/octocat/hello-world?scope=backend&contributor=alice") {
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
	statuses := []string{"", "fresh", "stale", "expired", "merged"}
	filters := RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend"}, Contributor: []string{"alice"}, Status: []string{"fresh"}}

	for _, target := range statuses {
		name := fmt.Sprintf("target=%q", target)
		t.Run(name, func(t *testing.T) {
			// Replicate the old toggle semantics from counts.templ statusURL:
			//   if currentStatus == targetStatus → clear status
			//   else → set to targetStatus
			var got string
			if slices.Contains(filters.Status, target) {
				got = filters.WithStatus("").URL()
			} else {
				got = filters.WithStatus(target).URL()
			}

			// Replicate old buildStatusURL logic.
			// Note: old code emitted "status=" for empty target when current was set.
			// That was a bug. We skip that case and verify the corrected behavior.
			if target == "" {
				// Old code would produce "...&status=" which is a degenerate query param.
				// New code correctly omits it. Verify the new output is clean.
				want := "/octocat/hello-world?scope=backend&contributor=alice"
				if !urlsEqual(got, want) {
					t.Errorf("clearing status: got %q, want %q", got, want)
				}
				return
			}

			want := oldBuildStatusURL(filters.Owner, filters.Repository, strings.Join(filters.Scope, ","), strings.Join(filters.Contributor, ","), strings.Join(filters.Status, ","), target)
			if !urlsEqual(got, want) {
				t.Errorf("WithStatus(%q).URL() = %q, old buildStatusURL = %q", target, got, want)
			}
		})
	}
}

func TestRepositoryFilters_WithScope(t *testing.T) {
	filters := RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend"}, Contributor: []string{"alice"}, Status: []string{"fresh"}}

	t.Run("clear scope", func(t *testing.T) {
		got := filters.WithScope("")
		if len(got.Scope) != 0 {
			t.Errorf("expected empty scope, got %q", got.Scope)
		}
		if !urlsEqual(got.URL(), "/octocat/hello-world?contributor=alice&status=fresh") {
			t.Errorf("URL = %q", got.URL())
		}
	})

	t.Run("change scope", func(t *testing.T) {
		got := filters.WithScope("frontend")
		if !slices.Equal(got.Scope, []string{"frontend"}) {
			t.Errorf("expected scope %q, got %q", "frontend", got.Scope)
		}
		if !urlsEqual(got.URL(), "/octocat/hello-world?scope=frontend&contributor=alice&status=fresh") {
			t.Errorf("URL = %q", got.URL())
		}
	})

	t.Run("does not mutate original", func(t *testing.T) {
		_ = filters.WithScope("frontend")
		if !slices.Equal(filters.Scope, []string{"backend"}) {
			t.Errorf("original was mutated: scope = %q", filters.Scope)
		}
	})
}

func TestRepositoryFilters_WithContributor(t *testing.T) {
	filters := RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend"}, Contributor: []string{"alice"}, Status: []string{"fresh"}}

	t.Run("clear contributor", func(t *testing.T) {
		got := filters.WithContributor("")
		if len(got.Contributor) != 0 {
			t.Errorf("expected empty contributor, got %q", got.Contributor)
		}
		if !urlsEqual(got.URL(), "/octocat/hello-world?scope=backend&status=fresh") {
			t.Errorf("URL = %q", got.URL())
		}
	})

	t.Run("change contributor", func(t *testing.T) {
		got := filters.WithContributor("bob")
		if !slices.Equal(got.Contributor, []string{"bob"}) {
			t.Errorf("expected contributor %q, got %q", "bob", got.Contributor)
		}
		if !urlsEqual(got.URL(), "/octocat/hello-world?scope=backend&contributor=bob&status=fresh") {
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

	got := filters.WithScope("").URL()
	want := "/octocat/hello-world?contributor=alice&status=fresh"
	if !urlsEqual(got, want) {
		t.Errorf("clear scope URL = %q, want %q", got, want)
	}
}

func TestRepositoryFilters_ClearContributor_MatchesOldContributorsClearFilter(t *testing.T) {
	filters := RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend"}, Contributor: []string{"alice"}, Status: []string{"fresh"}}

	got := filters.WithContributor("").URL()
	want := "/octocat/hello-world?scope=backend&status=fresh"
	if !urlsEqual(got, want) {
		t.Errorf("clear contributor URL = %q, want %q", got, want)
	}
}

func TestRepositoryFilters_ScopeLink_MatchesOldSidebarCard(t *testing.T) {
	filters := RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"old-scope"}, Contributor: []string{"alice"}, Status: []string{"fresh"}}

	got := filters.WithScope("new-scope").URL()
	want := "/octocat/hello-world?scope=new-scope&contributor=alice&status=fresh"
	if !urlsEqual(got, want) {
		t.Errorf("scope link URL = %q, want %q", got, want)
	}
}

func TestRepositoryFilters_ContributorLink_MatchesOldSidebarCard(t *testing.T) {
	filters := RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend"}, Contributor: []string{"alice"}, Status: []string{"fresh"}}

	got := filters.WithContributor("bob").URL()
	want := "/octocat/hello-world?scope=backend&contributor=bob&status=fresh"
	if !urlsEqual(got, want) {
		t.Errorf("contributor link URL = %q, want %q", got, want)
	}
}
