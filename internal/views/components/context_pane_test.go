package components

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestContextPaneTitleIsPlainTextAndGitHubButtonUsesTheURL(t *testing.T) {
	var output bytes.Buffer
	if err := ContextPane().Render(context.Background(), &output); err != nil {
		t.Fatal(err)
	}

	rendered := output.String()
	if !strings.Contains(rendered, "$store.contextPane.pullRequestURL") {
		t.Error("context pane does not bind the pull request URL")
	}
	if strings.Contains(rendered, "$store.contextPane.pullRequestUrl") {
		t.Error("context pane uses the wrong-cased pullRequestUrl property")
	}
	if strings.Contains(rendered, `<a class="context-pane-title`) {
		t.Error("context pane title should be plain text, not a link")
	}
	if !strings.Contains(rendered, `class="context-pane-title font-heading flex min-w-0 flex-1 items-start gap-2 text-xl leading-tight font-extrabold`) {
		t.Error("pull request title should use a stronger heading weight")
	}
	if !strings.Contains(rendered, `context-pane-discussion-count badge badge-sm badge-ghost px-1.5 text-sm leading-none font-bold`) {
		t.Error("discussion count should match its section heading typography")
	}
	if !strings.Contains(rendered, `aria-label="Open pull request on GitHub"`) {
		t.Error("context pane is missing the GitHub action")
	}
	titleStart := strings.Index(rendered, `class="context-pane-title`)
	if titleStart < 0 {
		t.Fatal("context pane title not found")
	}
	titleEnd := strings.Index(rendered[titleStart:], "</div>")
	numberIndex := strings.Index(rendered, `class="context-pane-number`)
	githubIndex := strings.Index(rendered, `aria-label="Open pull request on GitHub"`)
	if titleEnd < 0 || numberIndex < titleStart+titleEnd || githubIndex < numberIndex {
		t.Error("pull request number should be grouped in the toolbar before the GitHub button")
	}
}

func TestContextPaneGitHubButtonDoesNotWrapTheCloseButton(t *testing.T) {
	var output bytes.Buffer
	if err := ContextPane().Render(context.Background(), &output); err != nil {
		t.Fatal(err)
	}

	rendered := output.String()
	buttonStart := strings.Index(rendered, `aria-label="Open pull request on GitHub"`)
	if buttonStart < 0 {
		t.Fatal("GitHub button not found")
	}
	anchorEnd := strings.Index(rendered[buttonStart:], "</a>")
	if anchorEnd < 0 {
		t.Fatal("GitHub button anchor is not closed")
	}
	if strings.Contains(rendered[buttonStart:buttonStart+anchorEnd], "icon-close-button") {
		t.Error("close button should not be nested inside the GitHub link")
	}
}

func TestContextPaneUsesSemanticComponentClasses(t *testing.T) {
	var output bytes.Buffer
	if err := ContextPane().Render(context.Background(), &output); err != nil {
		t.Fatal(err)
	}

	rendered := output.String()
	for _, expected := range []string{
		`class="context-pane-header"`,
		`class="context-pane-body"`,
		"context-pane-card",
		`class="context-pane-section-heading `,
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("context pane output does not contain %q", expected)
		}
	}
}

func TestContextPaneUsesChipsInPriorityOrder(t *testing.T) {
	var output bytes.Buffer
	if err := ContextPane().Render(context.Background(), &output); err != nil {
		t.Fatal(err)
	}

	rendered := output.String()
	order := []string{
		"context-pane-review-chip",
		"context-pane-state-chip",
		"context-pane-freshness-chip",
		"context-pane-scope-field",
		"context-pane-discussion-field",
	}
	lastIndex := -1
	for _, marker := range order {
		index := strings.Index(rendered, marker)
		if index < 0 {
			t.Fatalf("context pane is missing %q", marker)
		}
		if index < lastIndex {
			t.Errorf("%q is out of priority order", marker)
		}
		lastIndex = index
	}
}

func TestContextPaneUsesMetadataTableWithAuthorAvatarAndScopeChips(t *testing.T) {
	var output bytes.Buffer
	if err := ContextPane().Render(context.Background(), &output); err != nil {
		t.Fatal(err)
	}

	rendered := output.String()
	if !strings.Contains(rendered, `<table class="context-pane-metadata-table`) {
		t.Error("context pane should render a metadata table")
	}
	for _, expected := range []string{
		"context-pane-description-card",
		"context-pane-meta-card",
		"Author",
		"Age",
		"Scope",
		"Discussion",
		"context-pane-discussion-count badge badge-sm badge-ghost px-1.5 text-sm leading-none font-bold",
		`x-text="$store.contextPane.pullRequestComments"`,
		"pullRequestAuthorAvatarURL",
		`x-for="scope in $store.contextPane.pullRequestScopes"`,
		"badge badge-ghost badge-sm",
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("context pane output does not contain %q", expected)
		}
	}

	descriptionIndex := strings.Index(rendered, "context-pane-description-card")
	metadataIndex := strings.Index(rendered, "context-pane-meta-card")
	if descriptionIndex < 0 || metadataIndex < 0 || descriptionIndex > metadataIndex {
		t.Error("description should appear before secondary metadata")
	}
}

func TestContextPaneScrollChildrenDoNotShrink(t *testing.T) {
	var output bytes.Buffer
	if err := ContextPane().Render(context.Background(), &output); err != nil {
		t.Fatal(err)
	}

	rendered := output.String()
	for _, expected := range []string{
		"context-pane-description-card context-pane-card",
		"context-pane-meta-card context-pane-card",
		"context-pane-section context-pane-card",
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("context pane scroll child %q should not shrink", expected)
		}
	}
}

func TestContextPaneClosesWhenTappingOutsideOnMobile(t *testing.T) {
	var output bytes.Buffer
	if err := ContextPane().Render(context.Background(), &output); err != nil {
		t.Fatal(err)
	}

	rendered := output.String()
	for _, expected := range []string{
		"@click.away=",
		"window.innerWidth < 1024",
		"$store.contextPane.closePane()",
		"closest('.pull-request')",
		"closest('.filter-panel')",
		"closest('.mobile-dock')",
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("context pane output does not contain %q", expected)
		}
	}
}
