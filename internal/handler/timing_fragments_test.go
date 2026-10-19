package handler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"merge/internal/model"
	"merge/internal/provider"
)

type stubProvider struct {
	pullRequests []model.StampedPullRequest
	calls        int
}

func (s *stubProvider) GetPullRequests(context.Context, provider.RepositoryQuery, provider.PullRequestListOptions) ([]model.StampedPullRequest, provider.PullRequestPagination, provider.PullRequestTimings, error) {
	s.calls++
	return s.pullRequests, provider.PullRequestPagination{}, provider.PullRequestTimings{Gather: 2 * time.Millisecond}, nil
}

func (s *stubProvider) GetPullRequestDiff(context.Context, string, string, int) (string, error) {
	return "", nil
}

func (s *stubProvider) GetPullRequestComments(context.Context, string, string, int) ([]model.PullRequestComment, error) {
	return nil, nil
}

func repositoryTimingHandler(stub *stubProvider, page int) http.Handler {
	server := &Server{
		Provider: stub,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	return requestTimingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := &RepositoryRequest{
			Server:      server,
			Query:       provider.RepositoryQuery{Owner: "ghostty-org", Repository: "ghostty"},
			ListOptions: provider.PullRequestListOptions{PerPage: 20, Page: page},
		}
		HandleRepository(request, w, r)
	}))
}

func serveHTMX(t *testing.T, handler http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.Header.Set("HX-Request", "true")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func assertTimingPayload(t *testing.T, body string) {
	t.Helper()
	if !strings.Contains(body, "data-status-timings") {
		t.Fatalf("response missing timing payload: %q", body)
	}
	values := map[string]float64{}
	for _, match := range timingAttribute.FindAllStringSubmatch(body, -1) {
		milliseconds, err := strconv.ParseFloat(match[2], 64)
		if err != nil || milliseconds < 0 {
			t.Errorf("%s duration = %q, want non-negative milliseconds", match[1], match[2])
		}
		values[match[1]] = milliseconds
	}
	for _, name := range []string{"gather", "logic", "template"} {
		if _, ok := values[name]; !ok {
			t.Fatalf("missing %s timing in payload: %q", name, body)
		}
	}
	if values["gather"] <= 0 {
		t.Fatalf("gather duration = %v, want provider timings to be recorded", values["gather"])
	}
}

func TestRepositorySummaryFragmentsEmitTimings(t *testing.T) {
	for _, fragment := range []string{"contributors", "scopes"} {
		t.Run(fragment, func(t *testing.T) {
			handler := repositoryTimingHandler(&stubProvider{}, 1)
			response := serveHTMX(t, handler, "/ghostty-org/ghostty?fragment="+fragment)
			assertTimingPayload(t, response.Body.String())
		})
	}
}

func TestRepositoryDetailFragmentsEmitTimings(t *testing.T) {
	for _, fragment := range []string{"pr-detail", "pr-discussion"} {
		t.Run(fragment, func(t *testing.T) {
			stub := &stubProvider{pullRequests: []model.StampedPullRequest{{PullRequest: model.PullRequest{Number: 1}}}}
			handler := repositoryTimingHandler(stub, 1)
			response := serveHTMX(t, handler, "/ghostty-org/ghostty?fragment="+fragment+"&number=1")
			assertTimingPayload(t, response.Body.String())
		})
	}
}

func TestRepositoryLoadMoreEmitsTimings(t *testing.T) {
	stub := &stubProvider{}
	handler := repositoryTimingHandler(stub, 2)

	response := serveHTMX(t, handler, "/ghostty-org/ghostty?page=2")

	assertTimingPayload(t, response.Body.String())
	if stub.calls != 2 {
		t.Fatalf("provider calls = %d, want 2 (one per requested page)", stub.calls)
	}
}
