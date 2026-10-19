package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"merge/internal/model"
	"merge/internal/views/helpers"

	"github.com/gorilla/mux"
)

func (s *Server) HandlePullRequestComments(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	number, err := strconv.Atoi(vars["number"])
	if err != nil {
		http.Error(w, "invalid pull request number", http.StatusBadRequest)
		return
	}

	comments, err := s.getPullRequestComments(r.Context(), vars["owner"], vars["repo"], number)
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

// getPullRequestComments fetches raw comments from the provider and renders
// their markdown bodies for display.
func (s *Server) getPullRequestComments(ctx context.Context, owner, repository string, number int) ([]model.PullRequestComment, error) {
	comments, err := s.Provider.GetPullRequestComments(ctx, owner, repository, number)
	if err != nil {
		return nil, err
	}
	for i := range comments {
		comments[i].BodyHTML = helpers.RenderDiscussionMarkdown(comments[i].Body)
	}
	return comments, nil
}
