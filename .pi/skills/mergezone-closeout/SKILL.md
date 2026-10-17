---
name: mergezone-closeout
description: Run MergeZone pull-request readiness checks: inspect the branch diff against its merge base, validate formatting, generated artifacts, build, tests, CI workflow and repository state, then summarize actionable findings. Use before opening or updating a MergeZone pull request.
---

# MergeZone PR Close-out

Perform a read-only PR-readiness review unless the user explicitly requests fixes. Do not stage, commit, push, rewrite history, or edit files as part of the check. If checks produce generated-file changes, report them and restore only files that this run changed, taking care not to discard pre-existing user changes.

## 1. Establish scope and repository state

From the repository root:

```sh
git status --short --branch
git branch --show-current
git log -1 --oneline
```

Determine the comparison base in this order:

1. Explicit target branch supplied by the user.
2. Current branch's upstream, if configured.
3. `origin/main` (fallback to local `main`).

Find the merge base, then inspect commits and diff:

```sh
git merge-base HEAD <base>
git log --oneline <merge-base>..HEAD
git diff --stat <merge-base>...HEAD
git diff --check <merge-base>...HEAD
```

Summarize only branch changes relative to the merge base. Separately note dirty/untracked files; never accidentally treat them as committed branch changes. If no usable base exists, say so and compare against the explicitly identified fallback rather than guessing silently.

## 2. Review changes

Read the relevant changed files and assess:

- Correctness, regressions, error handling, security-sensitive changes, and compatibility.
- Tests added/updated where behavior changed.
- Documentation/configuration consistency.
- Generated artifacts and whether source changes regenerate them reproducibly.
- Scope creep, unrelated formatting, dependency/lockfile drift, and accidental secrets/build output.

Prioritize actionable findings by severity. Cite paths and line numbers when possible. Do not claim a finding is a bug without supporting evidence.

## 3. Run repository checks

Inspect the current `Makefile`, `package.json`, `.github/workflows/*.yml`, `.gitignore`, and `.prettierignore` first; do not assume old commands or workflow structure. Current expected project commands are:

```sh
make fmt
make build
go test ./...
```

`make ci` runs `act` locally and may require Docker, network access, an event payload, or special local options. Run it only when practical and safe; otherwise report that it was not run and why. Avoid duplicating expensive checks if the CI target already runs all of them. The checked-in workflow is authoritative for CI behavior; compare its steps with actual Makefile targets and the package manager/lockfile.

### Protect user changes and generated files

Before running format/build commands, record `git status --short` and identify pre-existing modifications. Commands may rewrite tracked generated outputs (`public/main.css`, `*_templ.go`) and format source files. After each check:

- Capture exit status and relevant output.
- Inspect `git status` / `git diff` for changes caused by the check.
- Do not revert any path that was already modified before the check.
- Restore only newly modified generated artifacts if they are merely check side effects; explain this cleanup.
- If formatting changes source files, do not silently discard them. Report that the branch is not formatted, or only apply requested fixes after approval.

Formatting is configured through `make fmt`: `go fmt ./...`, `go tool templ fmt .`, and `pnpm run format` (invoked via the Makefile's overridable `NPM` variable). `.prettierignore` excludes generated artifacts such as `public/main.css` and `*_templ.go`; retain this separation because Sass emits compressed CSS that Prettier would expand, causing the generated diff check to fail.

Build is expected to regenerate Templ Go and Sass CSS via `make build` (currently depends on `templ css`). Confirm these generated outputs are clean after build if CI checks `git diff`.

## 4. Check GitHub Actions/CI alignment

Inspect every relevant workflow under `.github/workflows/`. Verify that:

- Setup actions provide required Go and Node versions.
- pnpm is installed before `setup-node` when using `cache: pnpm`.
- Dependency installation uses the committed `pnpm-lock.yaml` and frozen install (the repo's `make deps` target).
- CI invokes current targets (e.g. `make fmt`, `make build`) rather than obsolete `make generate` or npm commands.
- Formatting/build-generated changes are caught with `git diff --exit-code` at the right point.
- Test commands cover the Go packages.

For local `act`, recognize its container may need root to write under `/opt/hostedtoolcache`. The Makefile's local `ci` target may use `act --pull=false --container-options "-u 0:0"`; this is a local workaround and does not imply GitHub-hosted runners need root. Report Docker/image/tool-cache failures separately from actual project test failures.

## 5. Report

Provide a concise close-out with:

1. **Verdict:** ready, ready with caveats, or not ready.
2. **Branch summary:** changes since merge base.
3. **Findings:** highest priority first, with file/line references and rationale; explicitly say when none found.
4. **Checks:** each command run and pass/fail/not run, including whether it changed the working tree.
5. **Remaining actions:** specific fixes or PR notes, if any.

Distinguish verified facts from assumptions. Do not call a branch PR-ready merely because local tests pass if CI was not run or important workflow gaps remain.