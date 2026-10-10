package handler

import "testing"

func TestShouldPushURL(t *testing.T) {
	tests := []struct {
		name     string
		fragment RepositoryFragment
		page     int
		want     bool
	}{
		{name: "main view first page", fragment: "", page: 1, want: true},
		{name: "main view zero page", fragment: "", page: 0, want: true},
		{name: "load more page", fragment: "", page: 2, want: false},
		{name: "scopes fragment", fragment: ScopesFragment, page: 1, want: false},
		{name: "contributors fragment", fragment: ContributorsFragment, page: 1, want: false},
		{name: "pull request detail fragment", fragment: PullRequestDetailFragment, page: 1, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldPushURL(tt.fragment, tt.page); got != tt.want {
				t.Errorf("ShouldPushURL(%q, %d) = %t, want %t", tt.fragment, tt.page, got, tt.want)
			}
		})
	}
}
