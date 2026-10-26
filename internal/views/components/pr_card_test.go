package components

import (
	"strings"
	"testing"
	"time"

	"merge/internal/model"
)

func TestPullRequestStatusIconSpansTheHeaderRows(t *testing.T) {
	pullRequest := model.StampedPullRequest{
		PullRequest: model.PullRequest{Number: 1, Title: "feat(backend): add cache"},
	}
	output := renderSortFragmentURLs(t, PullRequestCard(pullRequest, model.RepositoryFilters{}))

	if !strings.Contains(output, `class="pull-request-status"`) {
		t.Error("status icon container should span both header rows so it stays centered")
	}
}

func TestPullRequestCardSupportsCompactDensity(t *testing.T) {
	pullRequest := model.StampedPullRequest{
		PullRequest: model.PullRequest{
			Number: 42,
			Title:  "feat(backend): a deliberately long pull request title that needs truncating",
			Author: model.User{Name: "octocat", URL: "https://example.com/octocat"},
		},
		Scopes: []string{"backend", "frontend"},
	}
	output := renderSortFragmentURLs(t, PullRequestCard(pullRequest, model.RepositoryFilters{}))

	for _, expected := range []string{
		`class="pull-request-card"`,
		`class="pull-request-header"`,
		`class="pull-request-title-wrap"`,
		`class="item-title font-heading font-semibold pull-request-title"`,
		`class="pull-request-meta-list"`,
		`class="pull-request-meta-updated"`,
		`title="feat(backend): a deliberately long pull request title that needs truncating"`,
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("compact card output does not contain %q", expected)
		}
	}
}

func TestCompactCardShowsRelativeUpdateTime(t *testing.T) {
	now := time.Now()
	created := now.Add(-72 * time.Hour)
	updated := now.Add(-2 * time.Hour)
	base := model.PullRequest{
		Number:    7,
		Title:     "fix(web): keep timestamps in sync",
		CreatedAt: &created,
		UpdatedAt: &updated,
	}
	pullRequest := base.Stamp(now)
	output := renderSortFragmentURLs(t, PullRequestCard(pullRequest, model.RepositoryFilters{}))

	if !strings.Contains(output, `title="Updated 2 hours"`) {
		t.Error("compact timestamp should describe the last update")
	}
	if !strings.Contains(output, `title="Opened 3 days"`) {
		t.Error("comfortable timestamp should still describe when the PR opened")
	}
}

func TestPullRequestCardSeedsRequestedReviewerCount(t *testing.T) {
	pullRequest := model.StampedPullRequest{
		PullRequest: model.PullRequest{
			Number: 9,
			Title:  "fix(api): wait for reviewers",
			Author: model.User{Name: "author", AvatarURL: "https://example.test/avatar.png"},
			Reviewers: []model.User{
				{Name: "reviewer-one"},
				{Name: "reviewer-two"},
			},
		},
	}
	output := renderSortFragmentURLs(t, PullRequestCard(pullRequest, model.RepositoryFilters{}))

	if !strings.Contains(output, `"reviewerCount":2`) {
		t.Error("pull request seed should include the requested reviewer count")
	}
	if !strings.Contains(output, `data-pull-request-reviewer-count="2"`) {
		t.Error("pull request dataset should include the requested reviewer count")
	}
	if !strings.Contains(output, `"authorAvatarURL":"https://example.test/avatar.png"`) {
		t.Error("pull request seed should include the author's profile picture URL")
	}
}

func TestPullRequestCardSeedsEveryScopeForContextPane(t *testing.T) {
	pullRequest := model.StampedPullRequest{
		PullRequest: model.PullRequest{Number: 42, Title: "feat(api,web): add feature"},
		Scopes:      []string{"api", "web"},
	}
	output := renderSortFragmentURLs(t, PullRequestCard(pullRequest, model.RepositoryFilters{Owner: "octocat", Repository: "hello"}))

	if !strings.Contains(output, `"name":"api","url":"/octocat/hello?scope=api"`) {
		t.Error("pull request seed should include the first scope and its filter URL")
	}
	if !strings.Contains(output, `"name":"web","url":"/octocat/hello?scope=web"`) {
		t.Error("pull request seed should include the second scope and its filter URL")
	}
	fallback := `data-pull-request-scopes="[{"name":"api","url":"/octocat/hello?scope=api"},{"name":"web","url":"/octocat/hello?scope=web"}]"`
	if !strings.Contains(output, fallback) {
		t.Errorf("scope dataset fallback should include every scope and URL: %q", output)
	}
	if !strings.Contains(output, `data-pull-request-scope="api" data-pull-request-scope-url="/octocat/hello?scope=api"`) {
		t.Error("scope dataset should retain the first-scope fallback fields")
	}
}

func TestPullRequestListDoesNotOwnDensityToggle(t *testing.T) {
	output := renderSortFragmentURLs(t, PullRequestScrollSection(nil, model.RepositoryFilters{}, 0, false))

	if !strings.Contains(output, "Pull Requests") {
		t.Error("pull request list header is missing")
	}
	if strings.Contains(output, "density-toggle") {
		t.Error("density toggle should live in the navigation, not the pull request list")
	}
}

// Scope chips are filters, so they link through the query string like the
// sidebar does. The URL also carries the current selection and sort, which
// keeps a chip click from dropping either.
func TestPullRequestScopeChipsUseTheQueryFilter(t *testing.T) {
	filters := model.RepositoryFilters{
		Owner: "octocat", Repository: "hello-world",
		Scope: []string{"terminal"}, ScopeSort: model.SortTop,
	}
	pullRequest := model.StampedPullRequest{
		PullRequest: model.PullRequest{Number: 1, Title: "feat(backend): add cache"},
		Scopes:      []string{"backend", "frontend"},
	}

	output := renderSortFragmentURLs(t, PullRequestScrollItems([]model.StampedPullRequest{pullRequest}, filters))

	for _, scope := range []string{"backend", "frontend"} {
		want := filters.WithScope(scope).URL()
		if !strings.Contains(output, `href="`+want+`"`) {
			t.Errorf("scope %q chip href missing %q: %q", scope, want, output)
		}
		if !strings.Contains(output, `hx-get="`+want+`"`) {
			t.Errorf("scope %q chip hx-get missing %q: %q", scope, want, output)
		}
		if strings.Contains(output, `href="/octocat/hello-world/`+scope+`"`) {
			t.Errorf("scope %q chip still uses a path URL: %q", scope, output)
		}
	}
}
