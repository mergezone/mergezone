package pages

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestHomePageHasNoSidebarAndProminentlyPlacesInteractiveSearchAfterCTA(t *testing.T) {
	var output bytes.Buffer
	if err := HomePage("https://example.test", "", "", "abc123").Render(context.Background(), &output); err != nil {
		t.Fatal(err)
	}
	rendered := output.String()
	if strings.Contains(rendered, `class="aside-column`) {
		t.Error("home page should not render the sidebar")
	}
	if strings.Contains(rendered, `id="mobile-dock"`) {
		t.Error("home page should keep the prominent repository search in the main content, not a mobile dock")
	}
	cta := strings.Index(rendered, "and contributor patterns at a glance.")
	search := strings.Index(rendered, `class="home-search`)
	if cta < 0 || search < 0 || search < cta {
		t.Fatal("interactive repository search should follow the call-to-action")
	}
	if !strings.Contains(rendered[search:], `@click="startOwnerEdit()"`) {
		t.Error("home page repository search should remain interactive")
	}
	if !strings.Contains(rendered, `class="home-search rounded-xl border border-base-300 bg-base-100 px-5 py-4 compact:px-3 compact:py-3"`) {
		t.Error("home page search should have prominent sizing and surface styling")
	}
	if got := strings.Count(rendered, `aria-labelledby="`); got < 3 {
		t.Errorf("home page should use labelled semantic sections, found %d", got)
	}
	for _, expected := range []string{
		"fonts.googleapis.com/css2?family=Manrope",
		`<h1 id="home-title" class="font-heading text-4xl leading-[1.1] font-extrabold tracking-tight`,
		`<h3 class="font-heading">`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("home page semantic/max-width structure is missing %q", expected)
		}
	}
}
