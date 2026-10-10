package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"merge/internal/model"

	"github.com/gorilla/mux"
)

func (s *Server) HandlePullRequestComments(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	number, err := strconv.Atoi(vars["number"])
	if err != nil {
		http.Error(w, "invalid pull request number", http.StatusBadRequest)
		return
	}

	comments, err := s.getPullRequestComments(vars["owner"], vars["repo"], number)
	if err != nil {
		s.Logger.Warn("could not fetch pull request comments", "error", err)
		http.Error(w, "could not fetch pull request comments", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(comments); err != nil {
		s.Logger.Error("could not encode pull request comments", "error", err)
	}
}

func (s *Server) githubToken() string {
	if tokenProvider, ok := s.Provider.(interface{ GitHubToken() string }); ok {
		return tokenProvider.GitHubToken()
	}
	return ""
}

func (s *Server) getPullRequestComments(owner, repository string, number int) ([]model.PullRequestComment, error) {
	var comments []model.PullRequestComment
	for page := 1; ; page++ {
		requestURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/comments?per_page=100&page=%d", owner, repository, number, page)
		request, err := http.NewRequest(http.MethodGet, requestURL, nil)
		if err != nil {
			return nil, err
		}
		request.Header.Set("Accept", "application/vnd.github+json")
		request.Header.Set("Authorization", "Bearer "+s.githubToken())
		request.Header.Set("X-GitHub-Api-Version", "2026-03-10")

		response, err := http.DefaultClient.Do(request)
		if err != nil {
			return nil, err
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			return nil, fmt.Errorf("GitHub comments request failed with status: %s", response.Status)
		}

		var pageComments []model.PullRequestComment
		if err := json.NewDecoder(io.LimitReader(response.Body, 5<<20)).Decode(&pageComments); err != nil {
			response.Body.Close()
			return nil, err
		}
		response.Body.Close()
		comments = append(comments, pageComments...)
		if len(pageComments) < 100 || !strings.Contains(response.Header.Get("Link"), `rel="next"`) {
			return comments, nil
		}
	}
}
