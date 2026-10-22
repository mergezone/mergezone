package handler

import (
	"context"
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

type pagedStubProvider struct {
	pages     map[int][]model.StampedPullRequest
	requested []int
}

func (p *pagedStubProvider) GetPullRequests(_ context.Context, _ provider.RepositoryQuery, options provider.PullRequestListOptions) ([]model.StampedPullRequest, provider.PullRequestPagination, provider.PullRequestTimings, error) {
	p.requested = append(p.requested, options.Page)
	return p.pages[options.Page], provider.PullRequestPagination{HasNext: options.Page < 2}, provider.PullRequestTimings{Gather: time.Millisecond}, nil
}

func (p *pagedStubProvider) GetPullRequestDiff(context.Context, string, string, int) (string, error) {
	return "", nil
}

func (p *pagedStubProvider) GetPullRequestComments(context.Context, string, string, int) ([]model.PullRequestComment, error) {
	return nil, nil
}

func sortFragmentHandler(p provider.Provider, page int) http.Handler {
	server := &Server{Provider: p, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	return requestTimingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := &RepositoryRequest{
			Server:      server,
			Query:       provider.RepositoryQuery{Owner: "octocat", Repository: "hello-world"},
			ListOptions: provider.PullRequestListOptions{PerPage: 20, Page: page},
		}
		HandleRepository(request, w, r)
	}))
}

func pullRequestInScope(number int, scope string) model.StampedPullRequest {
	return model.StampedPullRequest{PullRequest: model.PullRequest{Number: number}, Scopes: []string{scope}}
}

// Sorting after loading more pages must rebuild the sidebar from every loaded
// page, not just the first one, and must leave the main content alone.
func TestRepositorySortFragmentRebuildsEveryLoadedPage(t *testing.T) {
	stub := &pagedStubProvider{pages: map[int][]model.StampedPullRequest{
		1: {pullRequestInScope(1, "backend")},
		2: {pullRequestInScope(2, "frontend")},
	}}
	handler := sortFragmentHandler(stub, 2)

	response := serveHTMX(t, handler, "/octocat/hello-world?fragment=scopes&scope_sort=top&page=2")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %q", response.Code, http.StatusOK, response.Body.String())
	}
	if !slices.Equal(stub.requested, []int{1, 2}) {
		t.Errorf("provider pages = %v, want [1 2]", stub.requested)
	}

	body := response.Body.String()
	for _, scope := range []string{"backend", "frontend"} {
		if !strings.Contains(body, "Filter pull requests by scope "+scope) {
			t.Errorf("response is missing scope %q from a loaded page: %q", scope, body)
		}
	}
	for _, unexpected := range []string{`id="overview-stats"`, `id="load-more-card"`, `id="scroll-content"`, `class="item pull-request box"`} {
		if strings.Contains(body, unexpected) {
			t.Errorf("sort response touched the main content area: found %q in %q", unexpected, body)
		}
	}
	if !strings.Contains(body, "page=2") {
		t.Errorf("sort controls do not carry the loaded page: %q", body)
	}
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", contentType)
	}
}

func TestRepositorySortFragmentHandlesContributors(t *testing.T) {
	stub := &pagedStubProvider{pages: map[int][]model.StampedPullRequest{
		1: {{PullRequest: model.PullRequest{Number: 1}, Contributors: []string{"alice"}}},
	}}
	handler := sortFragmentHandler(stub, 1)

	response := serveHTMX(t, handler, "/octocat/hello-world?fragment=contributors&contributor_sort=top")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %q", response.Code, http.StatusOK, response.Body.String())
	}
	body := response.Body.String()
	if !strings.Contains(body, "Filter pull requests by contributor alice") {
		t.Errorf("response is missing the contributor cards: %q", body)
	}
	if !strings.Contains(body, `hx-swap-oob="innerHTML:#scopes-section"`) {
		t.Errorf("response does not refresh the scopes section out of band: %q", body)
	}
}
