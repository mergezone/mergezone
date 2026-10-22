package provider

import (
	"net/url"
	"slices"
	"testing"
)

func TestParseRepositoryQueryFilters(t *testing.T) {
	tests := []struct {
		name       string
		route      map[string]string
		query      url.Values
		wantScope  []string
		wantAuthor []string
		wantStatus []string
		wantOwner  string
		wantRepo   string
		wantPage   int
	}{
		{
			name:      "route identifies the repository",
			route:     map[string]string{"owner": "octocat", "repo": "hello-world"},
			query:     url.Values{},
			wantOwner: "octocat",
			wantRepo:  "hello-world",
			wantPage:  1,
		},
		{
			name:       "comma-separated values become selections",
			route:      map[string]string{"owner": "octocat", "repo": "hello-world"},
			query:      url.Values{"scope": {"backend,frontend"}, "contributor": {"alice,bob"}, "status": {"fresh,stale"}},
			wantScope:  []string{"backend", "frontend"},
			wantAuthor: []string{"alice", "bob"},
			wantStatus: []string{"fresh", "stale"},
			wantOwner:  "octocat",
			wantRepo:   "hello-world",
			wantPage:   1,
		},
		{
			name:       "values are trimmed",
			route:      map[string]string{"owner": "octocat", "repo": "hello-world"},
			query:      url.Values{"scope": {" backend , frontend "}, "contributor": {" alice "}, "status": {" fresh "}},
			wantScope:  []string{"backend", "frontend"},
			wantAuthor: []string{"alice"},
			wantStatus: []string{"fresh"},
			wantOwner:  "octocat",
			wantRepo:   "hello-world",
			wantPage:   1,
		},
		{
			name:       "empty segments are dropped",
			route:      map[string]string{"owner": "octocat", "repo": "hello-world"},
			query:      url.Values{"scope": {",backend,,"}, "contributor": {"alice,"}, "status": {",fresh"}},
			wantScope:  []string{"backend"},
			wantAuthor: []string{"alice"},
			wantStatus: []string{"fresh"},
			wantOwner:  "octocat",
			wantRepo:   "hello-world",
			wantPage:   1,
		},
		{
			name:      "an all-empty selection is ignored",
			route:     map[string]string{"owner": "octocat", "repo": "hello-world"},
			query:     url.Values{"scope": {",,"}, "contributor": {" "}, "status": {" , "}},
			wantOwner: "octocat",
			wantRepo:  "hello-world",
			wantPage:  1,
		},
		{
			name:      "a path segment is not a filter",
			route:     map[string]string{"owner": "octocat", "repo": "hello-world", "scope": "backend"},
			query:     url.Values{},
			wantOwner: "octocat",
			wantRepo:  "hello-world",
			wantPage:  1,
		},
		{
			name:      "the page is parsed",
			route:     map[string]string{"owner": "octocat", "repo": "hello-world"},
			query:     url.Values{"page": {"4"}},
			wantOwner: "octocat",
			wantRepo:  "hello-world",
			wantPage:  4,
		},
		{
			name:      "an invalid page falls back to the first page",
			route:     map[string]string{"owner": "octocat", "repo": "hello-world"},
			query:     url.Values{"page": {"nope"}},
			wantOwner: "octocat",
			wantRepo:  "hello-world",
			wantPage:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, options := ParseRepositoryQuery(tt.route, tt.query)
			if !slices.Equal(query.Scope, tt.wantScope) {
				t.Errorf("Scope = %v, want %v", query.Scope, tt.wantScope)
			}
			if !slices.Equal(query.Contributor, tt.wantAuthor) {
				t.Errorf("Contributor = %v, want %v", query.Contributor, tt.wantAuthor)
			}
			if !slices.Equal(query.Status, tt.wantStatus) {
				t.Errorf("Status = %v, want %v", query.Status, tt.wantStatus)
			}
			if query.Owner != tt.wantOwner {
				t.Errorf("Owner = %q, want %q", query.Owner, tt.wantOwner)
			}
			if query.Repository != tt.wantRepo {
				t.Errorf("Repository = %q, want %q", query.Repository, tt.wantRepo)
			}
			if options.PerPage != 20 {
				t.Errorf("PerPage = %d, want 20", options.PerPage)
			}
			if options.Page != tt.wantPage {
				t.Errorf("Page = %d, want %d", options.Page, tt.wantPage)
			}
		})
	}
}
