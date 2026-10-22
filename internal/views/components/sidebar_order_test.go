package components

import (
	"strings"
	"testing"

	"merge/internal/model"
)

// Selecting a value must not move it (or anything else) in the sidebar: the
// card under the cursor has to stay put. The summaries already arrive in their
// sorted order, so selection only changes the card's styling and filter URL.
func TestScopeSummariesKeepSelectionInSortedOrder(t *testing.T) {
	filters := model.RepositoryFilters{Owner: "octocat", Repository: "hello-world", Scope: []string{"backend"}}
	summaries := []model.ScopeInfo{{Name: "docs"}, {Name: "frontend"}, {Name: "backend"}}

	output := renderSortFragmentURLs(t, ScopeSummaries(summaries, "recent", filters, 1))

	docs := strings.Index(output, "Filter pull requests by scope docs")
	frontend := strings.Index(output, "Filter pull requests by scope frontend")
	backend := strings.Index(output, "Clear scope filter backend")
	if docs < 0 || frontend < 0 || backend < 0 {
		t.Fatalf("expected all scope cards in output: %q", output)
	}
	if !(docs < frontend && frontend < backend) {
		t.Errorf("selection reordered scopes; got docs=%d frontend=%d backend=%d", docs, frontend, backend)
	}
}

func TestContributorSummariesKeepSelectionInSortedOrder(t *testing.T) {
	filters := model.RepositoryFilters{Owner: "octocat", Repository: "hello-world", Contributor: []string{"alice"}}
	summaries := []model.ContributorInfo{{Name: "carol"}, {Name: "bob"}, {Name: "alice"}}

	output := renderSortFragmentURLs(t, ContributorSummaries(summaries, "recent", filters, 1))

	carol := strings.Index(output, "Filter pull requests by contributor carol")
	bob := strings.Index(output, "Filter pull requests by contributor bob")
	alice := strings.Index(output, "Clear contributor filter alice")
	if carol < 0 || bob < 0 || alice < 0 {
		t.Fatalf("expected all contributor cards in output: %q", output)
	}
	if !(carol < bob && bob < alice) {
		t.Errorf("selection reordered contributors; got carol=%d bob=%d alice=%d", carol, bob, alice)
	}
}
