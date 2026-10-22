package components

import (
	"strings"
	"testing"

	"merge/internal/model"
)

// The sort response is confined to the sidebar: it must not re-render the pull
// request list, the overview tiles, or the load-more control.
func TestSortScopesResponseOnlyRefreshesTheSidebar(t *testing.T) {
	scopes := []model.ScopeInfo{{Name: "backend"}, {Name: "frontend"}}
	contributors := []model.ContributorInfo{{Name: "alice"}}
	filters := model.RepositoryFilters{Owner: "octocat", Repository: "hello-world"}

	output := renderSortFragmentURLs(t, SortScopesResponse(scopes, model.SortTop, contributors, model.SortRecent, filters, 1))

	if !strings.Contains(output, "Filter pull requests by scope backend") {
		t.Errorf("scopes response does not render the scope cards: %q", output)
	}
	if !strings.Contains(output, `hx-swap-oob="innerHTML:#contributors-section"`) {
		t.Errorf("scopes response does not refresh the contributors section out of band: %q", output)
	}
	if !strings.Contains(output, "Filter pull requests by contributor alice") {
		t.Errorf("scopes response does not render the contributor cards: %q", output)
	}
	for _, unexpected := range []string{`id="overview-stats"`, `id="load-more-card"`, `data-pull-request-number`, `id="scroll-content"`} {
		if strings.Contains(output, unexpected) {
			t.Errorf("scopes response touched the main content area: found %q in %q", unexpected, output)
		}
	}
}

func TestSortContributorsResponseOnlyRefreshesTheSidebar(t *testing.T) {
	scopes := []model.ScopeInfo{{Name: "backend"}}
	contributors := []model.ContributorInfo{{Name: "alice"}, {Name: "bob"}}
	filters := model.RepositoryFilters{Owner: "octocat", Repository: "hello-world"}

	output := renderSortFragmentURLs(t, SortContributorsResponse(scopes, model.SortRecent, contributors, model.SortTop, filters, 1))

	if !strings.Contains(output, "Filter pull requests by contributor alice") {
		t.Errorf("contributors response does not render the contributor cards: %q", output)
	}
	if !strings.Contains(output, `hx-swap-oob="innerHTML:#scopes-section"`) {
		t.Errorf("contributors response does not refresh the scopes section out of band: %q", output)
	}
	if !strings.Contains(output, "Filter pull requests by scope backend") {
		t.Errorf("contributors response does not render the scope cards: %q", output)
	}
	for _, unexpected := range []string{`id="overview-stats"`, `id="load-more-card"`, `data-pull-request-number`, `id="scroll-content"`} {
		if strings.Contains(output, unexpected) {
			t.Errorf("contributors response touched the main content area: found %q in %q", unexpected, output)
		}
	}
}

// Both sections carry the loaded page so a later sort of either one rebuilds
// the same summaries.
func TestSortResponseSortControlsCarryTheLoadedPage(t *testing.T) {
	filters := model.RepositoryFilters{Owner: "octocat", Repository: "hello-world"}
	scopes := []model.ScopeInfo{{Name: "backend"}}
	contributors := []model.ContributorInfo{{Name: "alice"}}

	outputs := map[string]string{
		"scopes":       renderSortFragmentURLs(t, SortScopesResponse(scopes, model.SortTop, contributors, model.SortRecent, filters, 3)),
		"contributors": renderSortFragmentURLs(t, SortContributorsResponse(scopes, model.SortRecent, contributors, model.SortTop, filters, 3)),
	}
	wantURLs := []string{
		filters.WithScopeSort(model.SortRecent).FragmentURL("scopes", 3),
		filters.WithScopeSort(model.SortTop).FragmentURL("scopes", 3),
		filters.WithContributorSort(model.SortRecent).FragmentURL("contributors", 3),
		filters.WithContributorSort(model.SortTop).FragmentURL("contributors", 3),
	}
	for name, output := range outputs {
		for _, wantURL := range wantURLs {
			if !strings.Contains(output, `hx-get="`+wantURL+`"`) {
				t.Errorf("%s response missing sort URL %q", name, wantURL)
			}
		}
	}
}
