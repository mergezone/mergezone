package components

import (
	"strings"
	"testing"

	"merge/internal/model"
)

// Scope chips are filters, so they link through the query string like the
// sidebar does. The URL also carries the current selection and sort, which
// keeps a chip click from dropping either.
func TestPullRequestScopeChipsUseTheQueryFilter(t *testing.T) {
	filters := model.RepositoryFilters{
		Owner: "octocat", Repository: "hello-world",
		Scope: []string{"terminal"}, ScopeSort: model.SortTop,
	}
	pullRequest := model.StampedPullRequest{
		PullRequest: model.PullRequest{Number: 1, Title: "feat(backend): add cache"},
		Scopes:      []string{"backend", "frontend"},
	}

	output := renderSortFragmentURLs(t, PullRequestScrollItems([]model.StampedPullRequest{pullRequest}, filters))

	for _, scope := range []string{"backend", "frontend"} {
		want := filters.WithScope(scope).URL()
		if !strings.Contains(output, `href="`+want+`"`) {
			t.Errorf("scope %q chip href missing %q: %q", scope, want, output)
		}
		if !strings.Contains(output, `hx-get="`+want+`"`) {
			t.Errorf("scope %q chip hx-get missing %q: %q", scope, want, output)
		}
		if strings.Contains(output, `href="/octocat/hello-world/`+scope+`"`) {
			t.Errorf("scope %q chip still uses a path URL: %q", scope, output)
		}
	}
}
