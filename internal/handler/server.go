package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"

	"merge/internal/model"
	"merge/internal/provider"
	"merge/internal/views/components"
	"merge/internal/views/pages"

	"github.com/gorilla/mux"
)

type Server struct {
	BaseURL  string
	Port     int
	Router   *mux.Router
	Logger   *slog.Logger
	Provider provider.Provider
}

func (s *Server) HandleHomePage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := pages.HomePage(s.BaseURL, "", "").Render(r.Context(), w)
	if err != nil {
		s.Logger.Error(err.Error())
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
	pullRequests, _, err := request.Provider.GetPullRequests(request.Query, request.ListOptions)
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

func fetchPullRequestPages(request *RepositoryRequest, upToPage int) (allPullRequests, currentPagePullRequests []model.StampedPullRequest, hasNext bool, err error) {
	for page := 1; page <= upToPage; page++ {
		listOptions := request.ListOptions.WithPage(page)
		pullRequests, pagination, fetchErr := request.Provider.GetPullRequests(request.Query, listOptions)
		if fetchErr != nil {
			return nil, nil, false, fetchErr
		}
		allPullRequests = append(allPullRequests, pullRequests...)
		if page == upToPage {
			currentPagePullRequests = pullRequests
			hasNext = pagination.HasNext
		}
	}
	return allPullRequests, currentPagePullRequests, hasNext, nil
}

func optionalRepositoryFilterValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// RepositoryFragment identifies a rendered repository fragment requested through
// the fragment query parameter. An empty value selects a document or main-view response.
type RepositoryFragment string

const (
	PullRequestDetailFragment RepositoryFragment = "pr-detail"
	ContributorsFragment      RepositoryFragment = "contributors"
	ScopesFragment            RepositoryFragment = "scopes"
)

func isHTMXRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func HandleRepository(request *RepositoryRequest, w http.ResponseWriter, r *http.Request) {
	// Response contract: `fragment` selects a named repository fragment and takes
	// precedence; otherwise HX-Request=true returns the repository main view, while
	// direct navigation returns the complete HTML document.
	filters := model.RepositoryFilters{
		Owner:       request.Query.Owner,
		Repository:  request.Query.Repository,
		Scope:       optionalRepositoryFilterValue(request.Query.Scope),
		Contributor: optionalRepositoryFilterValue(request.Query.Contributor),
		Status:      optionalRepositoryFilterValue(request.Query.Status),
	}

	// Load-more requests return the requested page of pull requests and updated aggregate summaries.
	fragment := RepositoryFragment(r.URL.Query().Get("fragment"))
	if fragment == "" && request.ListOptions.Page > 1 && isHTMXRequest(r) {
		allPullRequests, currentPagePullRequests, hasNext, err := fetchPullRequestPages(request, request.ListOptions.Page)
		if err != nil {
			request.Logger.Warn(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		nextPullRequestPage := request.ListOptions.Page + 1

		scopes := make([]model.ScopeInfo, 0)
		for _, scope := range model.ScopeAges(allPullRequests) {
			scopes = append(scopes, scope)
		}

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

	pullRequests, pagination, err := request.Provider.GetPullRequests(request.Query, request.ListOptions)
	if err != nil {
		request.Logger.Warn(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	switch fragment {
	case PullRequestDetailFragment:
		pullRequestNumber, err := strconv.Atoi(r.URL.Query().Get("number"))
		if err != nil {
			http.Error(w, "Invalid pull request number", http.StatusBadRequest)
			return
		}
		for _, pullRequest := range viewData.PullRequests {
			if pullRequest.Number == pullRequestNumber {
				err = components.PullRequestDetail(pullRequest, request.Query.Owner, request.Query.Repository, r.URL.Path+r.URL.RawQuery).Render(r.Context(), w)
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
		err = components.ContributorSummaries(contributorSummaries, sortOrder, filters).Render(r.Context(), w)
	case ScopesFragment:
		sortOrder := r.URL.Query().Get("sort")
		scopeSummaries := viewData.ScopeCounts
		if sortOrder == "top" {
			sort.SliceStable(scopeSummaries, func(i, j int) bool {
				return scopeSummaries[i].TotalPullRequests() > scopeSummaries[j].TotalPullRequests()
			})
		}
		err = components.ScopeSummaries(scopeSummaries, sortOrder, filters).Render(r.Context(), w)
	default:
		if fragment != "" {
			http.Error(w, "Unknown fragment", http.StatusBadRequest)
			return
		}
		if isHTMXRequest(r) {
			w.Header().Set("HX-Push-Url", filters.URL())
			err = pages.RepositoryMainView(viewData).Render(r.Context(), w)
		} else {
			err = pages.RepositoryDocument(viewData, r.URL.Path).Render(r.Context(), w)
		}
	}
	if err != nil {
		request.Logger.Error(err.Error())
	}
}

func (s *Server) HandlePullRequestDiff(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	owner := vars["owner"]
	repository := vars["repo"]
	pullRequestNumber, err := strconv.Atoi(vars["number"])
	if err != nil {
		http.Error(w, "invalid pull request number", http.StatusBadRequest)
		return
	}

	diff, err := s.Provider.GetPullRequestDiff(owner, repository, pullRequestNumber)
	if err != nil {
		s.Logger.Warn(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(diff))
}

func (s *Server) ListenAndServe() error {
	s.Router.PathPrefix("/public/").Handler(http.StripPrefix("/public/", http.FileServer(http.Dir("public"))))
	s.Router.HandleFunc("/", s.HandleHomePage)

	s.Router.HandleFunc("/{owner}/{repo}/pull/{number}/diff", s.HandlePullRequestDiff)
	s.Router.HandleFunc("/{owner}/{repo}", s.WithParsedRepositoryRequest(HandleRepository))
	s.Router.HandleFunc("/{owner}/{repo}/", s.WithParsedRepositoryRequest(HandleRepository))
	s.Router.HandleFunc("/{owner}/{repo}/{scope:.*}", s.WithParsedRepositoryRequest(HandleRepository))

	s.Logger.Info("starting HTTP server", "port", s.Port)
	return http.ListenAndServe(fmt.Sprintf(":%d", s.Port), s.Router)
}
