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

func TestScopeSortButtonsPreserveCurrentFilters(t *testing.T) {
	filters := model.RepositoryFilters{
		Owner: "octocat", Repository: "hello-world", Scope: "backend", Contributor: "alice", Status: "stale",
	}
	output := renderSortFragmentURLs(t, ScopeSummaries(nil, "recent", filters))
	for _, sortOrder := range []string{"recent", "top"} {
		want := filters.FragmentURL("scopes", sortOrder)
		if !strings.Contains(output, `hx-get="`+want+`"`) {
			t.Errorf("scope sort %q URL missing; want hx-get=%q", sortOrder, want)
		}
	}
}

func TestContributorSortButtonsPreserveCurrentFilters(t *testing.T) {
	filters := model.RepositoryFilters{
		Owner: "octocat", Repository: "hello-world", Scope: "backend", Contributor: "alice", Status: "stale",
	}
	output := renderSortFragmentURLs(t, ContributorSummaries(nil, "recent", filters))
	for _, sortOrder := range []string{"recent", "top"} {
		want := filters.FragmentURL("contributors", sortOrder)
		if !strings.Contains(output, `hx-get="`+want+`"`) {
			t.Errorf("contributor sort %q URL missing; want hx-get=%q", sortOrder, want)
		}
	}
}
