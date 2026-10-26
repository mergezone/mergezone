package components

import (
	"strings"
	"testing"
)

func TestDisplayTogglesAreStatefulDaisyUIButtons(t *testing.T) {
	output := renderSortFragmentURLs(t, DisplayToggles("desktop"))

	for _, expected := range []string{
		`id="theme-toggle-desktop"`,
		`id="density-toggle-desktop"`,
		`class="display-toggles flex items-center gap-2"`,
		`class="btn btn-ghost btn-square size-10 min-h-0 border border-base-300 bg-base-100 p-0 text-base-content`,
		`$store.app.density === 'compact' ? 'fa-list' : 'fa-bars-staggered'`,
		`$store.app.theme === 'dark' ? 'fa-moon' : 'fa-sun'`,
		`@click="$store.app.setDensity($store.app.density === 'compact' ? 'comfortable' : 'compact')"`,
		`@click="$store.app.setTheme($store.app.theme === 'dark' ? 'light' : 'dark')"`,
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("navigation output does not contain %q", expected)
		}
	}

	if strings.Contains(output, `role="switch"`) || strings.Contains(output, "toggle-xl") || strings.Contains(output, "display-toggle-switches") {
		t.Error("display controls should always render as buttons, not switches")
	}
	assertToggleButtonContains(t, output, `id="theme-toggle-desktop"`, `$store.app.theme === 'dark' ? 'fa-moon' : 'fa-sun'`)
	assertToggleButtonContains(t, output, `id="density-toggle-desktop"`, `$store.app.density === 'compact' ? 'fa-list' : 'fa-bars-staggered'`)
}

func TestRepositoryNavigationKeepsTogglesOnMobile(t *testing.T) {
	output := renderSortFragmentURLs(t, RepositoryNavigation("owner", "repository"))
	for _, expected := range []string{`id="density-toggle-mobile"`, `id="theme-toggle-mobile"`, `lg:hidden`} {
		if !strings.Contains(output, expected) {
			t.Errorf("mobile navigation is missing %q", expected)
		}
	}
}

func TestAsideFooterGroupsBrandWithDesktopToggles(t *testing.T) {
	output := renderSortFragmentURLs(t, AsideFooter())
	for _, expected := range []string{
		"mt-auto hidden shrink-0 items-center justify-between",
		"border-t border-base-300",
		"lg:flex",
		">Merge Zone</a>",
		`id="density-toggle-desktop"`,
		`id="theme-toggle-desktop"`,
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("aside footer is missing %q", expected)
		}
	}
}

func assertToggleButtonContains(t *testing.T, output, marker string, expected ...string) {
	t.Helper()

	markerIndex := strings.Index(output, marker)
	if markerIndex < 0 {
		t.Fatalf("button marker %q not found", marker)
	}
	start := strings.LastIndex(output[:markerIndex], "<button")
	if start < 0 {
		t.Fatalf("button opening tag for %q not found", marker)
	}
	end := strings.Index(output[markerIndex:], "</button>")
	if end < 0 {
		t.Fatalf("button for %q is not closed", marker)
	}
	markup := output[start : markerIndex+end]
	for _, value := range expected {
		if !strings.Contains(markup, value) {
			t.Errorf("button %q does not contain %q", marker, value)
		}
	}
}
