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

func TestSidebarUsesSemanticComponentClasses(t *testing.T) {
	summaries := []model.ScopeInfo{{Name: "backend"}}
	output := renderSortFragmentURLs(t, ScopeSummaries(summaries, "recent", model.RepositoryFilters{}, 1))

	for _, expected := range []string{
		"compact:px-2",
		"compact:py-1.5",
		"compact:pl-6",
		"compact:gap-0.5",
		"compact:size-5",
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("sidebar output does not contain %q", expected)
		}
	}
}

func TestScopeAndContributorSectionHeadingsOutweighTheirCards(t *testing.T) {
	filters := model.RepositoryFilters{}
	scopeOutput := renderSortFragmentURLs(t, ScopeSummaries(nil, "recent", filters, 1))
	contributorOutput := renderSortFragmentURLs(t, ContributorSummaries(nil, "recent", filters, 1))
	for name, output := range map[string]string{
		"scope":       scopeOutput,
		"contributor": contributorOutput,
	} {
		if !strings.Contains(output, `class="section-heading-label font-heading flex min-w-0 items-center gap-2 text-sm font-extrabold`) {
			t.Errorf("%s section heading should be heavier than its cards", name)
		}
	}
}

func TestSidebarEntryTitlesUseSidebarTypographyRole(t *testing.T) {
	for name, output := range map[string]string{
		"scope":       renderSortFragmentURLs(t, ScopeCard(model.ScopeInfo{Name: "backend"}, model.RepositoryFilters{})),
		"contributor": renderSortFragmentURLs(t, ContributorCard(model.ContributorInfo{Name: "octocat"}, model.RepositoryFilters{})),
	} {
		if !strings.Contains(output, `class="sidebar-panel-title flex-1 truncate text-sm font-normal text-base-content"`) {
			t.Errorf("%s sidebar entry should use regular weight", name)
		}
	}
}

func TestSidebarPinsToggleFooterAfterScrollableFilters(t *testing.T) {
	output := renderSortFragmentURLs(t, RepositorySidebar(model.RepositoryFilters{}, nil, "", nil, "", 1))
	filtersIndex := strings.Index(output, "overflow-y-auto")
	footerIndex := strings.Index(output, "mt-auto hidden shrink-0 items-center justify-between")
	if filtersIndex < 0 || footerIndex < 0 || filtersIndex > footerIndex {
		t.Fatal("toggle footer should follow the scrollable filters in the sidebar")
	}
	if !strings.Contains(output[footerIndex:], `id="density-toggle-desktop"`) || !strings.Contains(output[footerIndex:], `id="theme-toggle-desktop"`) {
		t.Error("sidebar bottom footer should contain both display toggles")
	}
	if !strings.Contains(output[footerIndex:], ">Merge Zone</a>") {
		t.Error("sidebar bottom footer should contain the Merge Zone brand")
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
