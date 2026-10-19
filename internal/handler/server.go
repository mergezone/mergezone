package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"time"

	"merge/internal/model"
	"merge/internal/provider"
	"merge/internal/telemetry"
	"merge/internal/views/components"
	"merge/internal/views/pages"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Server struct {
	BaseURL  string
	Commit   string
	Port     int
	Router   *mux.Router
	Logger   *slog.Logger
	Provider provider.Provider
}

func (s *Server) HandleHomePage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	markRequestWorkComplete(r)
	err := pages.HomePage(s.BaseURL, "", "", s.Commit).Render(r.Context(), w)
	if err != nil {
		s.Logger.Error(err.Error())
		recordRequestError(r, err)
	}
}

type RepositoryRequest struct {
	*Server
	Query       provider.RepositoryQuery
	ListOptions provider.PullRequestListOptions
}

type RepositoryHandler func(*RepositoryRequest, http.ResponseWriter, *http.Request)

func (s *Server) WithParsedRepositoryRequest(next RepositoryHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.Logger.Info("handling HTTP request", "path", r.URL.Path)

		repositoryQuery, listOptions := provider.ParseRepositoryQuery(mux.Vars(r), r.URL.Query())
		request := &RepositoryRequest{
			Server:      s,
			Query:       repositoryQuery,
			ListOptions: listOptions,
		}

		next(request, w, r)
	}
}

func HandlePullRequestListJSON(request *RepositoryRequest, w http.ResponseWriter, r *http.Request) {
	pullRequests, _, _, err := request.Provider.GetPullRequests(r.Context(), request.Query, request.ListOptions)
	if err != nil {
		request.Logger.Warn(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	encodedPullRequests, err := json.Marshal(pullRequests)
	if err != nil {
		request.Logger.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(encodedPullRequests)
}

func fetchPullRequestPages(ctx context.Context, request *RepositoryRequest, upToPage int) (allPullRequests, currentPagePullRequests []model.StampedPullRequest, hasNext bool, timings provider.PullRequestTimings, err error) {
	for page := 1; page <= upToPage; page++ {
		listOptions := request.ListOptions.WithPage(page)
		pullRequests, pagination, pageTimings, fetchErr := request.Provider.GetPullRequests(ctx, request.Query, listOptions)
		if fetchErr != nil {
			return nil, nil, false, timings, fetchErr
		}
		timings.Gather += pageTimings.Gather
		allPullRequests = append(allPullRequests, pullRequests...)
		if page == upToPage {
			currentPagePullRequests = pullRequests
			hasNext = pagination.HasNext
		}
	}
	return allPullRequests, currentPagePullRequests, hasNext, timings, nil
}

// RepositoryFragment identifies a rendered repository fragment requested through
// the fragment query parameter. An empty value selects a document or main-view response.
type RepositoryFragment string

const (
	PullRequestDetailFragment     RepositoryFragment = "pr-detail"
	PullRequestDiscussionFragment RepositoryFragment = "pr-discussion"
	ContributorsFragment          RepositoryFragment = "contributors"
	ScopesFragment                RepositoryFragment = "scopes"
)

func isHTMXRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// ShouldPushURL reports whether a repository response is a page-level navigation
// whose filters should be reflected in the browser URL. Fragment responses and
// paginated load-more responses are rendered in place and must not push a URL,
// because fragment, sort, and page are request-local rather than page state.
func ShouldPushURL(fragment RepositoryFragment, page int) bool {
	return fragment == "" && page <= 1
}

func HandleRepository(request *RepositoryRequest, w http.ResponseWriter, r *http.Request) {
	// Response contract: `fragment` selects a named repository fragment and takes
	// precedence; otherwise HX-Request=true returns the repository main view, while
	// direct navigation returns the complete HTML document.
	filters := model.RepositoryFilters{
		Owner:       request.Query.Owner,
		Repository:  request.Query.Repository,
		Scope:       request.Query.Scope,
		Contributor: request.Query.Contributor,
		Status:      request.Query.Status,
	}

	// Load-more requests return the requested page of pull requests and updated aggregate summaries.
	fragment := RepositoryFragment(r.URL.Query().Get("fragment"))
	if fragment == "" && request.ListOptions.Page > 1 && isHTMXRequest(r) {
		allPullRequests, currentPagePullRequests, hasNext, timings, err := fetchPullRequestPages(r.Context(), request, request.ListOptions.Page)
		if err != nil {
			request.Logger.Warn(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		recordProviderTimings(r, timings)

		nextPullRequestPage := request.ListOptions.Page + 1

		scopes := make([]model.ScopeInfo, 0)
		for _, scope := range model.ScopeAges(allPullRequests) {
			scopes = append(scopes, scope)
		}

		markRequestWorkComplete(r)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		err = pages.AppendPullRequestsResponse(
			currentPagePullRequests,
			filters,
			nextPullRequestPage,
			hasNext,
			model.PullRequestExpiryCounts(allPullRequests),
			scopes,
			"recent",
			model.ContributorActivity(allPullRequests),
			"recent",
		).Render(r.Context(), w)
		if err != nil {
			request.Logger.Error(err.Error())
		}
		return
	}

	pullRequests, pagination, timings, err := request.Provider.GetPullRequests(r.Context(), request.Query, request.ListOptions)
	if err != nil {
		request.Logger.Warn(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	recordProviderTimings(r, timings)
	_, logicSpan := telemetry.Tracer("merge/handler").Start(r.Context(), "repository.categorize",
		trace.WithAttributes(attribute.Int("repository.pull_request_count", len(pullRequests))),
	)

	scopeSummaries := model.ScopeAges(pullRequests)

	viewData := model.RepositoryViewData{
		BaseURL:           request.BaseURL,
		Owner:             request.Query.Owner,
		Repository:        request.Query.Repository,
		Scope:             filters.Scope,
		Contributor:       filters.Contributor,
		Status:            filters.Status,
		PullRequests:      pullRequests,
		OverallCounts:     model.PullRequestExpiryCounts(pullRequests),
		ScopeCounts:       scopeSummaries,
		ScopeSort:         "recent",
		ContributorCounts: model.ContributorActivity(pullRequests),
		ContributorSort:   "recent",
		CurrentPage:       1,
		HasMore:           pagination.HasNext,
	}
	logicSpan.SetAttributes(attribute.Int("repository.scope_count", len(viewData.ScopeCounts)))
	logicSpan.End()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	switch fragment {
	case PullRequestDiscussionFragment:
		pullRequestNumber, err := strconv.Atoi(r.URL.Query().Get("number"))
		if err != nil {
			http.Error(w, "Invalid pull request number", http.StatusBadRequest)
			return
		}
		for _, pullRequest := range viewData.PullRequests {
			if pullRequest.Number == pullRequestNumber {
				comments, commentErr := request.getPullRequestComments(r.Context(), request.Query.Owner, request.Query.Repository, pullRequestNumber)
				if commentErr != nil {
					request.Logger.Warn("could not load pull request discussion", "error", commentErr)
					http.Error(w, "Could not load pull request discussion", http.StatusBadGateway)
					return
				}
				markRequestWorkComplete(r)
				err = components.PullRequestDiscussion(comments).Render(r.Context(), w)
				if err != nil {
					request.Logger.Error(err.Error())
				}
				return
			}
		}
		http.Error(w, "Pull request not found", http.StatusNotFound)
		return
	case PullRequestDetailFragment:
		pullRequestNumber, err := strconv.Atoi(r.URL.Query().Get("number"))
		if err != nil {
			http.Error(w, "Invalid pull request number", http.StatusBadRequest)
			return
		}
		for _, pullRequest := range viewData.PullRequests {
			if pullRequest.Number == pullRequestNumber {
				markRequestWorkComplete(r)
				err = components.PullRequestDetail(pullRequest).Render(r.Context(), w)
				if err != nil {
					request.Logger.Error(err.Error())
				}
				return
			}
		}
		http.Error(w, "Pull request not found", http.StatusNotFound)
		return
	case ContributorsFragment:
		sortOrder := r.URL.Query().Get("sort")
		contributorSummaries := viewData.ContributorCounts
		if sortOrder == "top" {
			sort.SliceStable(contributorSummaries, func(i, j int) bool {
				return contributorSummaries[i].TotalPullRequests() > contributorSummaries[j].TotalPullRequests()
			})
		}
		markRequestWorkComplete(r)
		err = components.ContributorSummaries(contributorSummaries, sortOrder, filters).Render(r.Context(), w)
	case ScopesFragment:
		sortOrder := r.URL.Query().Get("sort")
		scopeSummaries := viewData.ScopeCounts
		if sortOrder == "top" {
			sort.SliceStable(scopeSummaries, func(i, j int) bool {
				return scopeSummaries[i].TotalPullRequests() > scopeSummaries[j].TotalPullRequests()
			})
		}
		markRequestWorkComplete(r)
		err = components.ScopeSummaries(scopeSummaries, sortOrder, filters).Render(r.Context(), w)
	default:
		if fragment != "" {
			http.Error(w, "Unknown fragment", http.StatusBadRequest)
			return
		}
		markRequestWorkComplete(r)
		if isHTMXRequest(r) {
			if ShouldPushURL(fragment, request.ListOptions.Page) {
				w.Header().Set("HX-Push-Url", filters.URL())
			}
			err = pages.RepositoryMainView(viewData, request.Commit).Render(r.Context(), w)
		} else {
			err = pages.RepositoryDocument(viewData, r.URL.Path, request.Commit).Render(r.Context(), w)
		}
	}
	if err != nil {
		request.Logger.Error(err.Error())
		recordRequestError(r, err)
	}
}

func (s *Server) HandlePullRequestDiff(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.Tracer("merge/handler").Start(r.Context(), "pull_request.diff")
	defer span.End()
	vars := mux.Vars(r)
	owner := vars["owner"]
	repository := vars["repo"]
	pullRequestNumber, err := strconv.Atoi(vars["number"])
	if err != nil {
		span.SetAttributes(attribute.Int("http.response.status_code", http.StatusBadRequest))
		span.SetStatus(codes.Error, "invalid pull request number")
		http.Error(w, "invalid pull request number", http.StatusBadRequest)
		return
	}

	diff, err := s.Provider.GetPullRequestDiff(ctx, owner, repository, pullRequestNumber)
	if err != nil {
		span.SetAttributes(attribute.Int("http.response.status_code", http.StatusInternalServerError))
		telemetry.RecordError(span, err)
		s.Logger.Warn(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	span.SetAttributes(attribute.Int("http.response.status_code", http.StatusOK))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(diff))
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	s.Router.PathPrefix("/public/").Handler(http.StripPrefix("/public/", http.FileServer(http.Dir("public"))))
	s.Router.Handle("/", requestTimingMiddleware(http.HandlerFunc(s.HandleHomePage)))

	s.Router.HandleFunc("/{owner}/{repo}/pull/{number}/diff", s.HandlePullRequestDiff)
	repositoryHandler := s.WithParsedRepositoryRequest(HandleRepository)
	s.Router.Handle("/{owner}/{repo}", requestTimingMiddleware(repositoryHandler))
	s.Router.Handle("/{owner}/{repo}/", requestTimingMiddleware(repositoryHandler))
	s.Router.Handle("/{owner}/{repo}/{scope:.*}", requestTimingMiddleware(repositoryHandler))

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.Port),
		Handler: s.Router,
	}
	serverErrors := make(chan error, 1)
	go func() {
		s.Logger.Info("starting HTTP server", "port", s.Port)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return err
		}
		if err := <-serverErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
