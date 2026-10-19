package components

import (
	"slices"
	"strings"
	"testing"

	"merge/internal/model"
)

func scopeNames(summaries []model.ScopeInfo) []string {
	names := make([]string, len(summaries))
	for i, summary := range summaries {
		names[i] = summary.Name
	}
	return names
}

func contributorNames(summaries []model.ContributorInfo) []string {
	names := make([]string, len(summaries))
	for i, summary := range summaries {
		names[i] = summary.Name
	}
	return names
}

func TestSelectedScopesFirst(t *testing.T) {
	summaries := []model.ScopeInfo{{Name: "frontend"}, {Name: "backend"}, {Name: "docs"}}

	tests := []struct {
		name     string
		selected []string
		want     []string
	}{
		{
			name: "no selection keeps the list unchanged",
			want: []string{"frontend", "backend", "docs"},
		},
		{
			name:     "selected scopes move to the front in their original order",
			selected: []string{"backend", "docs"},
			want:     []string{"backend", "docs", "frontend"},
		},
		{
			name:     "selections missing from the list are ignored",
			selected: []string{"infra"},
			want:     []string{"frontend", "backend", "docs"},
		},
		{
			name:     "selecting every scope keeps the list unchanged",
			selected: []string{"frontend", "backend", "docs"},
			want:     []string{"frontend", "backend", "docs"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scopeNames(selectedScopesFirst(summaries, tt.selected))
			if !slices.Equal(got, tt.want) {
				t.Errorf("selectedScopesFirst() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelectedContributorsFirst(t *testing.T) {
	summaries := []model.ContributorInfo{{Name: "carol"}, {Name: "alice"}, {Name: "bob"}}

	tests := []struct {
		name     string
		selected []string
		want     []string
	}{
		{
			name: "no selection keeps the list unchanged",
			want: []string{"carol", "alice", "bob"},
		},
		{
			name:     "selected contributors move to the front in their original order",
			selected: []string{"bob"},
			want:     []string{"bob", "carol", "alice"},
		},
		{
			name:     "selections missing from the list are ignored",
			selected: []string{"dave"},
			want:     []string{"carol", "alice", "bob"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contributorNames(selectedContributorsFirst(summaries, tt.selected))
			if !slices.Equal(got, tt.want) {
				t.Errorf("selectedContributorsFirst() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScopeSummariesRenderSelectedScopesFirst(t *testing.T) {
	filters := model.RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"docs"}}
	summaries := []model.ScopeInfo{{Name: "frontend"}, {Name: "docs"}}

	output := renderSortFragmentURLs(t, ScopeSummaries(summaries, "recent", filters))

	selected := strings.Index(output, "Clear scope filter docs")
	unselected := strings.Index(output, "Filter pull requests by scope frontend")
	if selected < 0 || unselected < 0 {
		t.Fatalf("expected both scope cards in output: %q", output)
	}
	if selected > unselected {
		t.Errorf("selected scope rendered after unselected scope: selected=%d unselected=%d", selected, unselected)
	}
}

func TestContributorSummariesRenderSelectedContributorsFirst(t *testing.T) {
	filters := model.RepositoryFilters{Owner: "octocat", Repository: "hello-world", Contributor: []string{"bob"}}
	summaries := []model.ContributorInfo{{Name: "alice"}, {Name: "bob"}}

	output := renderSortFragmentURLs(t, ContributorSummaries(summaries, "recent", filters))

	selected := strings.Index(output, "Clear contributor filter bob")
	unselected := strings.Index(output, "Filter pull requests by contributor alice")
	if selected < 0 || unselected < 0 {
		t.Fatalf("expected both contributor cards in output: %q", output)
	}
	if selected > unselected {
		t.Errorf("selected contributor rendered after unselected contributor: selected=%d unselected=%d", selected, unselected)
	}
}
