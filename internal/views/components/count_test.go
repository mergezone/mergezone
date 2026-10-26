package components

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"merge/internal/model"
)

func TestRepositorySectionHeadingsOutweighTheirContent(t *testing.T) {
	var output bytes.Buffer
	if err := PullRequestCounts(model.ExpiryCounts{}, false, model.RepositoryFilters{}, 1).Render(context.Background(), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `class="my-2 px-1 text-sm font-bold text-base-content/70`) {
		t.Error("Overview heading should be heavier than the status content")
	}

	output.Reset()
	if err := PullRequestScrollSection(nil, model.RepositoryFilters{}, 0, false).Render(context.Background(), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `class="mt-2 px-1 text-sm font-bold text-base-content/70"`) {
		t.Error("Pull Requests heading should be heavier than pull request card titles")
	}
}

func TestPullRequestStatusCountRendersSolidIcons(t *testing.T) {
	var output bytes.Buffer
	if err := PullRequestStatusCount("fa-seedling text-success", 3, "Fresh", "/example", false).Render(context.Background(), &output); err != nil {
		t.Fatal(err)
	}

	rendered := output.String()
	for _, expected := range []string{
		"status-icon fa-solid",
		"fa-seedling",
		"text-success",
		`class="status-tile"`,
		`class="status-tile-label"`,
		`class="status-tile-count"`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("status count output does not contain %q", expected)
		}
	}
}

func TestInlinePullRequestCountRendersSolidIcons(t *testing.T) {
	var output bytes.Buffer
	if err := InlinePullRequestCount("fa-seedling text-success", 3).Render(context.Background(), &output); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output.String(), "status-icon fa-solid") {
		t.Error("inline count output does not contain the Font Awesome solid style")
	}
}
