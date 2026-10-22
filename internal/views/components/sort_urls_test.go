package components

import (
	"context"
	"html"
	"io"
	"strings"
	"testing"

	"merge/internal/model"
)

func renderSortFragmentURLs(t *testing.T, component interface {
	Render(context.Context, io.Writer) error
}) string {
	t.Helper()
	var output strings.Builder
	if err := component.Render(context.Background(), &output); err != nil {
		t.Fatal(err)
	}
	return html.UnescapeString(output.String())
}

// A sort control requests a sidebar fragment in place, but carries the current
// filters (and the other list's sort) in both the fragment URL and the pushed
// page URL, so a later navigation cannot drop them.
func TestScopeSortButtonsPreserveCurrentFilters(t *testing.T) {
	filters := model.RepositoryFilters{
		Owner: "octocat", Repository: "hello-world", Scope: []string{"backend"}, Contributor: []string{"alice"}, Status: []string{"stale"},
		ScopeSort: model.SortTop, ContributorSort: model.SortTop,
	}
	output := renderSortFragmentURLs(t, ScopeSummaries(nil, model.SortTop, filters, 1))
	for _, sortOrder := range []string{model.SortRecent, model.SortTop} {
		fragment := filters.WithScopeSort(sortOrder).FragmentURL("scopes", 1)
		if !strings.Contains(output, `hx-get="`+fragment+`"`) {
			t.Errorf("scope sort %q fragment URL missing; want hx-get=%q", sortOrder, fragment)
		}
		pushed := filters.WithScopeSort(sortOrder).URL()
		if !strings.Contains(output, `hx-push-url="`+pushed+`"`) {
			t.Errorf("scope sort %q push URL missing; want hx-push-url=%q", sortOrder, pushed)
		}
	}
	if !strings.Contains(output, `hx-target="#scopes-section"`) {
		t.Errorf("scope sort buttons do not target #scopes-section: %q", output)
	}
}

func TestContributorSortButtonsPreserveCurrentFilters(t *testing.T) {
	filters := model.RepositoryFilters{
		Owner: "octocat", Repository: "hello-world", Scope: []string{"backend"}, Contributor: []string{"alice"}, Status: []string{"stale"},
		ScopeSort: model.SortTop, ContributorSort: model.SortTop,
	}
	output := renderSortFragmentURLs(t, ContributorSummaries(nil, model.SortTop, filters, 1))
	for _, sortOrder := range []string{model.SortRecent, model.SortTop} {
		fragment := filters.WithContributorSort(sortOrder).FragmentURL("contributors", 1)
		if !strings.Contains(output, `hx-get="`+fragment+`"`) {
			t.Errorf("contributor sort %q fragment URL missing; want hx-get=%q", sortOrder, fragment)
		}
		pushed := filters.WithContributorSort(sortOrder).URL()
		if !strings.Contains(output, `hx-push-url="`+pushed+`"`) {
			t.Errorf("contributor sort %q push URL missing; want hx-push-url=%q", sortOrder, pushed)
		}
	}
	if !strings.Contains(output, `hx-target="#contributors-section"`) {
		t.Errorf("contributor sort buttons do not target #contributors-section: %q", output)
	}
}

// Sorting after loading more pages must rebuild the same summaries, so the sort
// fragment URL carries the loaded page instead of resetting to the first one.
func TestScopeSortButtonsCarryTheLoadedPage(t *testing.T) {
	filters := model.RepositoryFilters{Owner: "octocat", Repository: "hello-world"}
	output := renderSortFragmentURLs(t, ScopeSummaries([]model.ScopeInfo{{Name: "backend"}}, model.SortTop, filters, 3))
	for _, sortOrder := range []string{model.SortRecent, model.SortTop} {
		fragment := filters.WithScopeSort(sortOrder).FragmentURL("scopes", 3)
		if !strings.Contains(output, `hx-get="`+fragment+`"`) {
			t.Errorf("scope sort %q does not carry page 3; want hx-get=%q", sortOrder, fragment)
		}
	}
}

func TestContributorSortButtonsCarryTheLoadedPage(t *testing.T) {
	filters := model.RepositoryFilters{Owner: "octocat", Repository: "hello-world"}
	output := renderSortFragmentURLs(t, ContributorSummaries([]model.ContributorInfo{{Name: "alice"}}, model.SortTop, filters, 3))
	for _, sortOrder := range []string{model.SortRecent, model.SortTop} {
		fragment := filters.WithContributorSort(sortOrder).FragmentURL("contributors", 3)
		if !strings.Contains(output, `hx-get="`+fragment+`"`) {
			t.Errorf("contributor sort %q does not carry page 3; want hx-get=%q", sortOrder, fragment)
		}
	}
}
