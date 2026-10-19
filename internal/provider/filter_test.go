package provider

import (
	"slices"
	"testing"

	"merge/internal/model"
)

// stamped builds a minimal StampedPullRequest for filter tests.
func stamped(number int, author string, state model.State, expiry model.ExpiryStatus, scopes ...string) model.StampedPullRequest {
	return model.StampedPullRequest{
		PullRequest:  model.PullRequest{Number: number, Author: model.User{Name: author}},
		State:        state,
		ExpiryStatus: expiry,
		Scopes:       scopes,
	}
}

func numbers(prs []model.StampedPullRequest) []int {
	result := make([]int, len(prs))
	for i, pr := range prs {
		result[i] = pr.Number
	}
	return result
}

func TestFilterPullRequests_MultipleSelections(t *testing.T) {
	all := []model.StampedPullRequest{
		stamped(1, "alice", model.Open, model.Fresh, "backend"),
		stamped(2, "bob", model.Open, model.Stale, "frontend"),
		stamped(3, "alice", model.Open, model.Expired, "backend", "frontend"),
		stamped(4, "carol", model.Merged, model.Fresh, "docs"),
		stamped(5, "bob", model.Merged, model.Stale, "backend"),
	}

	tests := []struct {
		name  string
		query RepositoryQuery
		want  []int
	}{
		{
			name:  "nil query keeps every pull request",
			query: RepositoryQuery{},
			want:  []int{1, 2, 3, 4, 5},
		},
		{
			name:  "multiple scopes keep any pull request matching a selected scope",
			query: RepositoryQuery{Scope: []string{"backend", "frontend"}},
			want:  []int{1, 2, 3, 5},
		},
		{
			name:  "single scope excludes the other scopes",
			query: RepositoryQuery{Scope: []string{"docs"}},
			want:  []int{4},
		},
		{
			name:  "scope with no matches yields an empty result",
			query: RepositoryQuery{Scope: []string{"infra"}},
			want:  []int{},
		},
		{
			name:  "multiple contributors keep any pull request by a selected author",
			query: RepositoryQuery{Contributor: []string{"alice", "bob"}},
			want:  []int{1, 2, 3, 5},
		},
		{
			name:  "single contributor excludes the other authors",
			query: RepositoryQuery{Contributor: []string{"carol"}},
			want:  []int{4},
		},
		{
			name:  "multiple statuses keep any pull request with a selected status",
			query: RepositoryQuery{Status: []string{"fresh", "expired"}},
			want:  []int{1, 4, 3},
		},
		{
			name:  "merged status matches merged pull request state",
			query: RepositoryQuery{Status: []string{"merged"}},
			want:  []int{4, 5},
		},
		{
			name:  "merged and stale statuses keep a merged stale pull request once",
			query: RepositoryQuery{Status: []string{"merged", "stale"}},
			want:  []int{4, 5, 2},
		},
		{
			name: "dimensions combine with AND",
			query: RepositoryQuery{
				Scope:       []string{"backend", "frontend"},
				Contributor: []string{"alice"},
				Status:      []string{"expired"},
			},
			want: []int{3},
		},
		{
			name: "multi-selection in every dimension keeps a pull request once",
			query: RepositoryQuery{
				Scope:       []string{"backend"},
				Contributor: []string{"bob"},
				Status:      []string{"merged", "stale"},
			},
			want: []int{5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := numbers(FilterPullRequests(all, tt.query))
			if !slices.Equal(got, tt.want) {
				t.Errorf("FilterPullRequests() = %v, want %v", got, tt.want)
			}
		})
	}
}
