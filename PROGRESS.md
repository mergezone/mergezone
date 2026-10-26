# Design system migration: Bulma + Sass → DaisyUI + Tailwind

Status: **complete**. The app now renders Tailwind utility classes directly in
Templ components and builds its stylesheet with the Tailwind CLI. Bulma, Sass,
and the old SCSS partials have been removed.

## Why

- Component styles live next to the markup they describe.
- One CSS build step instead of a Sass pipeline plus a Bulma bridge layer.
- DaisyUI supplies accessible component primitives and semantic color tokens.
- The existing light/dark palette is preserved, so the app looks familiar.

## What changed

### Build

| Before | After |
| --- | --- |
| `sass` + Bulma | `@tailwindcss/cli` + `daisyui` |
| `public/main.scss` + `_*.scss` partials | `internal/views/styles/app.css` |
| Sass watch/build scripts | `pnpm run build:css` / `pnpm run watch:css` |
| CSS generated separately | `make build` runs `templ` + CSS first |

The generated stylesheet is committed at `public/main.css`. Tailwind scans the
Templ files, the Go helper package, and `public/alpine.js` for class names.

### Themes

DaisyUI themes named `light` and `dark` redefine the original palette through
semantic slots (`base-100`, `base-200`, `base-300`, `base-content`, `primary`,
`success`, `warning`, `error`, `accent`, …). Dark-mode accent colors were
brightened slightly for contrast.

The app toggles `data-theme` on `<html>` instead of swapping a class. Tailwind's
`dark:` variant is wired to `[data-theme="dark"]`, and the inline bootstrap in
`layout/base.templ` still resolves the saved/system theme before first paint.
Theme and density are icon buttons; each icon reflects the currently selected
state and activates the other state when pressed.

### Typography

- Manrope is the heading family; system UI remains the body family.
- Heading font family and role weights are centralized in `app.css` under
  `--type-heading-font` and the `type-*` classes. Use `type-primary-heading`,
  `type-section-heading`, or `type-card-heading` in templates instead of adding
  local font-family/weight utilities. Sidebar entries use `type-sidebar-entry`.

### Component styling

- Reusable layouts are expressed as short component classes in Templ; their
  structure, responsive breakpoints, and compact-density variants live in
  `internal/views/styles/app.css`.
- Add or adjust styles on the existing hooks (`repository-main`, `home-main`,
  `pull-request-card`, `context-pane-*`, `sidebar-panel-*`) before adding long
  utility bundles to templates. Key density-specific rules to
  `[data-density="compact"]`; use media queries only for viewport behavior.
- Markdown wrappers keep the `prose` roles in markup; shared media sizing and
  spacing belong to `.markdown-content` in the component stylesheet.

### Markup conventions

| Before | After |
| --- | --- |
| `.ok` / `.warn` / `.error` / `.special` | `text-success` / `text-warning` / `text-error` / `text-accent` |
| `.box`, `.panel`, `.card` | `rounded-lg border border-base-300 bg-base-100` |
| `.level`, `.stat` | Tailwind grid/flex + `status-tile` hook |
| `.content` for Markdown | `prose prose-sm max-w-none dark:prose-invert` |
| `.open`, `.selected`, `.is-current` | `data-open`, `data-selected`, `data-current` with `data-[…]` variants |
| Font Awesome spinners | `<span class="loading loading-spinner">` |
| SCSS scrollbar mixins | `scrollbar-slim` utility |

Runtime hooks used by Alpine/HTMX and tests were kept (`pull-request`,
`title-input`, `search-input-row`, `sidebar-panel-item`, IDs, `data-*` payloads).

### Layout and mobile behavior

- Desktop keeps the resizable filter sidebar, main list, and context pane.
- Mobile keeps the bottom dock. The filter sheet and context pane open as
  fixed bottom sheets above the dock, with a dimmed tap-to-dismiss backdrop.
- Markdown images, htmx indicators, diff theming, pane resizing, and zoom
  layering remain in the small custom CSS layer in `app.css`.

### List density

- The density button is grouped with the theme button. On mobile they stay
  with the Merge Zone brand in the navigation; on desktop the brand and toggles
  move to a bordered footer anchored to the bottom of the aside, below the
  independently scrolling filters. Repository search stays at the top. Both
  buttons use DaisyUI's square button variant with centered icons, base-100 fill,
  and base-300 border. Their list and sun/moon icons indicate the current state;
  accessible labels and tooltips describe the action to switch state. The
  pull-request header no longer duplicates the controls.
- The choice is stored in `localStorage` (`list-density`) and applied as
  `data-density` on `<html>` before first paint.
- Density-specific component rules are scoped to `<html data-density="compact">` in `app.css`.
  Htmx swaps and whole-page navigation inherit the same density without
  re-rendering.
- Compact rows are about 52px tall with a 4px gap, a single-line truncated
  title with a full-title tooltip, and a one-line metadata summary. Narrow
  layouts drop discussion count and scope first, keeping number, status,
  updated age, and author. Comfortable rows keep the opened age.

### Context pane metadata

- The former seven-row metadata table is now a compact table for Author, Age,
  Scope, and Discussion. The author row includes the existing profile image;
  every scope renders as a filter chip. The description renders first as a card
  and is hidden when empty; the old collapsible Details and Description sections
  are gone.
- The pane body is a flex column, so its cards use `shrink-0`; without it a long
  description is clipped by the card's `overflow-hidden` and the pane cannot
  scroll.
- Header chips follow a fixed priority: requested reviews, then PR status, then
  freshness. CI/checks and merge-readiness chips are omitted because the GitHub
  provider does not expose those states, and missing data never renders a
  placeholder.
- Freshness is suppressed when it duplicates a merged or closed state. Scope and
  discussion count stay in the metadata table as secondary values, and
  supersession is explained in a short note under the chips.
- Compact age suffixes use `m` for minutes and `mo` for months to avoid the
  ambiguous `21m` display. Long age labels spell both units out.
- The requested-reviewer count is seeded from the existing `requested_reviewers`
  field in `pr_card.templ`; no new API or backend data is required.
- The JSON card seed is the pane's primary data source. Dataset fields remain a
  fallback for seedless cards; scopes use an ordered plural JSON payload plus
  the original first-scope fields for compatibility.

## Verification

- `go tool templ generate`
- `pnpm run build:css`
- `go build ./...`
- `go test ./...` (includes handler and component snapshot/response tests)
- Manual Firefox checks at 1440×900 and 390×844 in light and dark themes:
  homepage, repository view, open context pane, open filter sheet, no
  horizontal overflow, no runtime console errors.

## Follow-ups

- Screenshot-based visual regression tests would catch future style drift.
- The `scopes` / `contributors` template files are marked deprecated in favor of
  `sidebar_card.templ`; consolidating them is separate cleanup.
- Diff rendering still depends on `@pierre/diffs` and keeps a few `!important`
  overrides for its shadow DOM styles.
