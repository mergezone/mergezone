package handler

import (
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"merge/internal/model"
	"merge/internal/provider"
)

// sidebarStamped builds a stamped pull request with one scope and one
// contributor, used to exercise the unfiltered sidebar summaries.
func sidebarStamped(number int, author string, created time.Time, scope string) model.StampedPullRequest {
	return model.StampedPullRequest{
		PullRequest: model.PullRequest{
			Number:    number,
			Author:    model.User{Name: author},
			CreatedAt: &created,
			UpdatedAt: &created,
		},
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
		sidebarStamped(1, "alice", base.Add(3*time.Hour), "backend"),
		sidebarStamped(2, "bob", base.Add(2*time.Hour), "frontend"),
		sidebarStamped(3, "carol", base.Add(time.Hour), "docs"),
	}
}

func TestSidebarSummariesKeepUnselectedValues(t *testing.T) {
	all := sidebarTestPullRequests()

	t.Run("a selected scope does not hide the other scopes", func(t *testing.T) {
		scopes, _ := sidebarSummaries(all, provider.RepositoryQuery{Scope: []string{"backend"}})
		want := []string{"backend", "frontend", "docs"}
		if got := summaryScopeNames(scopes); !slices.Equal(got, want) {
			t.Errorf("scope summaries = %v, want %v", got, want)
		}
	})

	t.Run("a selected contributor does not hide the other contributors", func(t *testing.T) {
		_, contributors := sidebarSummaries(all, provider.RepositoryQuery{Contributor: []string{"alice"}})
		want := []string{"alice", "bob", "carol"}
		if got := summaryContributorNames(contributors); !slices.Equal(got, want) {
			t.Errorf("contributor summaries = %v, want %v", got, want)
		}
	})

	t.Run("other dimensions still narrow the list", func(t *testing.T) {
		scopes, contributors := sidebarSummaries(all, provider.RepositoryQuery{
			Scope:       []string{"backend"},
			Contributor: []string{"alice"},
		})
		if got, want := summaryScopeNames(scopes), []string{"backend"}; !slices.Equal(got, want) {
			t.Errorf("scope summaries = %v, want %v", got, want)
		}
		if got, want := summaryContributorNames(contributors), []string{"alice"}; !slices.Equal(got, want) {
			t.Errorf("contributor summaries = %v, want %v", got, want)
		}
	})
}

func TestRepositorySidebarShowsUnselectedScopes(t *testing.T) {
	stub := &stubProvider{pullRequests: sidebarTestPullRequests()}
	server := &Server{
		Provider: stub,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	request := &RepositoryRequest{
		Server: server,
		Query:  provider.RepositoryQuery{Owner: "ghostty-org", Repository: "ghostty", Scope: []string{"backend"}},
		ListOptions: provider.PullRequestListOptions{
			PerPage: 20,
			Page:    1,
		},
	}
	handler := requestTimingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleRepository(request, w, r)
	}))

	body := serveHTMX(t, handler, "/ghostty-org/ghostty?scope=backend").Body.String()

	selected := strings.Index(body, "Clear scope filter backend")
	frontend := strings.Index(body, "Filter pull requests by scope frontend")
	docs := strings.Index(body, "Filter pull requests by scope docs")
	if selected < 0 || frontend < 0 || docs < 0 {
		t.Fatalf("sidebar missing scope cards (selected=%d frontend=%d docs=%d): %q", selected, frontend, docs, body)
	}
	if !(selected < frontend && frontend < docs) {
		t.Errorf("expected selected scope first, then frontend then docs; got positions selected=%d frontend=%d docs=%d", selected, frontend, docs)
	}
}
