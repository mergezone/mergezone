package handler

import (
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"merge/internal/model"
	"merge/internal/provider"
)

// sidebarStamped builds a stamped pull request with one scope and one
// contributor, used to exercise the sidebar summaries.
func sidebarStamped(number int, author string, created time.Time, scope string) model.StampedPullRequest {
	return model.StampedPullRequest{
		PullRequest: model.PullRequest{
			Number:    number,
			Author:    model.User{Name: author},
			CreatedAt: &created,
			UpdatedAt: &created,
		},
		ExpiryStatus: model.Fresh,
		Scopes:       []string{scope},
		Contributors: []string{author},
	}
}

func summaryScopeNames(summaries []model.ScopeInfo) []string {
	names := make([]string, len(summaries))
	for i, summary := range summaries {
		names[i] = summary.Name
	}
	return names
}

func summaryContributorNames(summaries []model.ContributorInfo) []string {
	names := make([]string, len(summaries))
	for i, summary := range summaries {
		names[i] = summary.Name
	}
	return names
}

func sidebarTestPullRequests() []model.StampedPullRequest {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return []model.StampedPullRequest{
		sidebarStamped(1, "alice", base.Add(time.Hour), "backend"),
		sidebarStamped(2, "bob", base.Add(2*time.Hour), "frontend"),
		sidebarStamped(3, "carol", base.Add(3*time.Hour), "docs"),
	}
}

// The sidebar derives its values from the unfiltered pull requests, so every
// scope and contributor is listed regardless of the active query.
func TestSidebarSummariesListEveryValue(t *testing.T) {
	scopes, contributors := sidebarSummaries(sidebarTestPullRequests(), model.SortRecent, model.SortRecent)

	if got, want := summaryScopeNames(scopes), []string{"docs", "frontend", "backend"}; !slices.Equal(got, want) {
		t.Errorf("scope summaries = %v, want %v", got, want)
	}
	if got, want := summaryContributorNames(contributors), []string{"carol", "bob", "alice"}; !slices.Equal(got, want) {
		t.Errorf("contributor summaries = %v, want %v", got, want)
	}
}

// The top sort orders each list by total pull request count, which differs from
// the recency order when a scope has more pull requests than a newer one.
func TestSidebarSummariesSortByTopCount(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	pullRequests := []model.StampedPullRequest{
		sidebarStamped(1, "alice", base.Add(time.Hour), "backend"),
		sidebarStamped(2, "alice", base.Add(2*time.Hour), "backend"),
		sidebarStamped(3, "bob", base.Add(3*time.Hour), "frontend"),
	}

	recentScopes, _ := sidebarSummaries(pullRequests, model.SortRecent, model.SortRecent)
	if got, want := summaryScopeNames(recentScopes), []string{"frontend", "backend"}; !slices.Equal(got, want) {
		t.Errorf("recent scope summaries = %v, want %v", got, want)
	}

	topScopes, _ := sidebarSummaries(pullRequests, model.SortTop, model.SortTop)
	if got, want := summaryScopeNames(topScopes), []string{"backend", "frontend"}; !slices.Equal(got, want) {
		t.Errorf("top scope summaries = %v, want %v", got, want)
	}
}

// statusCountPattern captures the value shown on an overview status tile, e.g.
// `<p ...>Fresh</p> ... <b ...>3</b>`.
var statusCountPattern = regexp.MustCompile(`(?s)<p[^>]*>([A-Za-z]+)</p>.*?<b[^>]*>(\d+)</b>`)

func overviewStatusCounts(body string) map[string]string {
	counts := map[string]string{}
	for _, match := range statusCountPattern.FindAllStringSubmatch(body, -1) {
		counts[match[1]] = match[2]
	}
	return counts
}

func renderSidebar(t *testing.T, query provider.RepositoryQuery) string {
	t.Helper()
	return renderSidebarRequest(t, sidebarTestPullRequests(), query, "/ghostty-org/ghostty")
}

func renderSidebarRequest(t *testing.T, pullRequests []model.StampedPullRequest, query provider.RepositoryQuery, target string) string {
	t.Helper()
	request := &RepositoryRequest{
		Server: &Server{
			Provider: &stubProvider{pullRequests: pullRequests},
			Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		},
		Query:       query,
		ListOptions: provider.PullRequestListOptions{PerPage: 20, Page: 1},
	}
	handler := requestTimingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleRepository(request, w, r)
	}))
	return serveHTMX(t, handler, target).Body.String()
}

// The sidebar sort is page state: it is read from the URL, orders the cards,
// and is carried into every card's link so selecting a filter keeps the sort.
func TestRepositorySidebarSortComesFromTheURLAndSticksToTheCards(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	pullRequests := []model.StampedPullRequest{
		sidebarStamped(1, "alice", base.Add(time.Hour), "backend"),
		sidebarStamped(2, "alice", base.Add(2*time.Hour), "backend"),
		sidebarStamped(3, "bob", base.Add(3*time.Hour), "frontend"),
	}
	query := provider.RepositoryQuery{Owner: "ghostty-org", Repository: "ghostty"}

	body := renderSidebarRequest(t, pullRequests, query, "/ghostty-org/ghostty?scope_sort=top&contributor_sort=top")

	backend := strings.Index(body, "Filter pull requests by scope backend")
	frontend := strings.Index(body, "Filter pull requests by scope frontend")
	if backend < 0 || frontend < 0 {
		t.Fatalf("expected both scope cards: %q", body)
	}
	if backend > frontend {
		t.Errorf("scope_sort=top did not move the busiest scope first; backend=%d frontend=%d", backend, frontend)
	}

	alice := strings.Index(body, "Filter pull requests by contributor alice")
	bob := strings.Index(body, "Filter pull requests by contributor bob")
	if alice < 0 || bob < 0 {
		t.Fatalf("expected both contributor cards: %q", body)
	}
	if alice > bob {
		t.Errorf("contributor_sort=top did not move the busiest contributor first; alice=%d bob=%d", alice, bob)
	}

	// Card links carry the active sort, so selecting a filter keeps it.
	if !strings.Contains(body, "scope=backend&amp;scope_sort=top") {
		t.Errorf("scope card link dropped scope_sort: %q", body)
	}
	if !strings.Contains(body, "contributor=alice&amp;contributor_sort=top") {
		t.Errorf("contributor card link dropped contributor_sort: %q", body)
	}
}

// Selecting one filter must not trim another facet: the scopes, contributors,
// and overview counts stay fixed so the menu never shifts under the cursor.
func TestRepositoryFiltersDoNotTrimTheSidebar(t *testing.T) {
	cases := []struct {
		name  string
		query provider.RepositoryQuery
	}{
		{"no filters", provider.RepositoryQuery{Owner: "ghostty-org", Repository: "ghostty"}},
		{"scope selected", provider.RepositoryQuery{Owner: "ghostty-org", Repository: "ghostty", Scope: []string{"backend"}}},
		{"contributor selected", provider.RepositoryQuery{Owner: "ghostty-org", Repository: "ghostty", Contributor: []string{"alice"}}},
		{"status selected", provider.RepositoryQuery{Owner: "ghostty-org", Repository: "ghostty", Status: []string{"fresh"}}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			body := renderSidebar(t, tt.query)

			if got := strings.Count(body, "sidebar-panel-item scope"); got != 3 {
				t.Errorf("scope cards = %d, want 3 (all scopes stay visible)", got)
			}
			if got := strings.Count(body, "sidebar-panel-item contributor"); got != 3 {
				t.Errorf("contributor cards = %d, want 3 (all contributors stay visible)", got)
			}
			if got := overviewStatusCounts(body)["Fresh"]; got != "3" {
				t.Errorf("Fresh overview count = %q, want 3 (counted across every pull request)", got)
			}
		})
	}
}
