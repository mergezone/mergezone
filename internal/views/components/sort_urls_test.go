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
	output := renderSortFragmentURLs(t, ScopeSummaries(nil, model.SortTop, filters))
	for _, sortOrder := range []string{model.SortRecent, model.SortTop} {
		fragment := filters.WithScopeSort(sortOrder).FragmentURL("scopes")
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
	output := renderSortFragmentURLs(t, ContributorSummaries(nil, model.SortTop, filters))
	for _, sortOrder := range []string{model.SortRecent, model.SortTop} {
		fragment := filters.WithContributorSort(sortOrder).FragmentURL("contributors")
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
