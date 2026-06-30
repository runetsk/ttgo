# Changelog

All notable changes to TTGO are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **E2E result reporting (dogfooding).** An opt-in Playwright reporter pushes the
  e2e suite's own results into a running TTGO instance as a test run — auto-provisioning
  a `Playwright E2E` folder, category, and a test case per Playwright test, then recording
  per-test pass/fail, duration, and failure details. Enabled by setting a write-scoped
  `TTGO_REPORT_TOKEN`; a no-op otherwise, so normal runs are unaffected.

### Changed
- Unified the test-runs sidebar with the library sidebar: shared SVG folder/chevron
  icons (replacing the emoji `📁/📂` and text `▾/›`), the same `--sidebar-*` color
  tokens, and matching row styling (left accent border on the selected folder, hover,
  spacing, and indentation). The three inline folder action buttons (`+ ✎ 🗑`) collapse
  to the library's `+` / `⋮` pattern, where `⋮` opens the existing context menu.

### Fixed
- **Custom fields:** creating a `SELECT` custom field failed with HTTP 400 ("options array
  of strings") because the frontend sent the options JSON-stringified while the API expects
  a real array. `createCustomField` now sends the array, and the settings list renders the
  options whether they come back as an array or a legacy string.
- Run comparison: cancel in-flight fetches when switching the compared run, so rapidly
  changing the selection can no longer briefly render a stale comparison.

## [0.1.0] - 2026-06-29

### Added
- **Run-to-run comparison.** A new **Compare** tab on the test-run page compares the
  open run against any other run: an analytics-style summary (per-run pass/fail/skip
  bars + a metric-diff table) and an outcome-grouped per-test diff — regressions,
  fixes, still-failing, other changes, unchanged, and tests that ran in only one run —
  with rows that expand to a side-by-side detail of both runs. Shareable via a
  `?compareWith=<runId>` deep link.

[Unreleased]: https://github.com/runetsk/ttgo/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/runetsk/ttgo/releases/tag/v0.1.0
