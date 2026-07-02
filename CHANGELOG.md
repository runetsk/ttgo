# Changelog

All notable changes to TTGO are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-07-01

### Added
- **E2E result reporting (dogfooding).** An opt-in Playwright reporter pushes the
  e2e suite's own results into a running TTGO instance as a test run — auto-provisioning
  a `Playwright E2E` folder, category, and a test case per Playwright test, then recording
  per-test pass/fail, duration, and failure details. Enabled by setting a write-scoped
  `TTGO_REPORT_TOKEN`; a no-op otherwise, so normal runs are unaffected.

### Changed
- Replaced the Jira-centric defect system with native, tracker-owned defects (title/description/status/severity) that work with no external configuration. Defects are global and link to many results/test cases; an optional reference-only external link (Jira/GitHub/any URL) can be attached. Added a Defects triage page. Existing Jira defect links are migrated to native defects on first startup.
- **Audit-log action string change:** the test-case reverification audit-log action changed from `defect_link:reverification_dismissed:<id>` to `defect:reverification_dismissed:<id>`. Any audit-log consumer keyed on the old prefix should update.
- Unified the test-runs sidebar with the library sidebar: shared SVG folder/chevron
  icons (replacing the emoji `📁/📂` and text `▾/›`), the same `--sidebar-*` color
  tokens, and matching row styling (left accent border on the selected folder, hover,
  spacing, and indentation). The three inline folder action buttons (`+ ✎ 🗑`) collapse
  to the library's `+` / `⋮` pattern, where `⋮` opens the existing context menu.
- **Quality workspace.** Requirements, Traceability, Defects, Categories, and Analytics are
  now grouped under a single **Quality** top-nav section with a collapsible left sidebar
  (icon-only rail when collapsed; the expanded/collapsed state is remembered).
- **Editable defects.** The Defects page gained full create/edit support — a modal for
  title, description, severity, and status; an inline status toggle; search by title or
  external key; status/severity filters; a per-row description preview; and a deep link from
  a run result's linked-defect title straight to that defect.
- **View affected tests.** A defect row on the Defects page expands to list the test cases it
  affects — each linking to its test-case detail — backed by a new `GET /api/defects/{id}/tests` endpoint.
- Native `<select>` menus now theme their option popups to match the app — a dark-mode-friendly
  background and a brand-indigo selected row instead of the browser's default white list.

### Fixed
- **Custom fields:** creating a `SELECT` custom field failed with HTTP 400 ("options array
  of strings") because the frontend sent the options JSON-stringified while the API expects
  a real array. `createCustomField` now sends the array, and the settings list renders the
  options whether they come back as an array or a legacy string.
- Run comparison: cancel in-flight fetches when switching the compared run, so rapidly
  changing the selection can no longer briefly render a stale comparison.
- Defect create/edit modal readability: the dialog used a translucent panel that left its
  labels and inputs hard to read over the dark overlay; it now uses a solid, opaque
  background that reads cleanly in both light and dark themes.

## [0.1.0] - 2026-06-29

### Added
- **Run-to-run comparison.** A new **Compare** tab on the test-run page compares the
  open run against any other run: an analytics-style summary (per-run pass/fail/skip
  bars + a metric-diff table) and an outcome-grouped per-test diff — regressions,
  fixes, still-failing, other changes, unchanged, and tests that ran in only one run —
  with rows that expand to a side-by-side detail of both runs. Shareable via a
  `?compareWith=<runId>` deep link.

[Unreleased]: https://github.com/runetsk/ttgo/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/runetsk/ttgo/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/runetsk/ttgo/releases/tag/v0.1.0
