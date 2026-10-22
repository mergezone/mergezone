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
