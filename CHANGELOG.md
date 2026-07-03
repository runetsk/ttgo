# Changelog

All notable changes to TTGO are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Performance test harness (phase 1): a `perf/` k6 suite for the result-ingest
  write path — smoke check plus the S1 "CI ingest storm" capacity scenario —
  and a `perfseed` CLI that provisions scratch databases with a deterministic
  dataset, perf users, and write-scoped API tokens. The CLI refuses to touch any
  non-`perf-*.db` database (or a symlinked one) and writes its token manifest at
  mode 0600. Load pipelines send `test_name_snapshot` and spread across a
  `TOKENS`-sized (default 100) token pool so the measured write path matches
  real clients rather than harness artifacts.
- **Webhook signing-secret rotation.** `POST /api/webhooks/{id}/rotate-secret` generates
  a new HMAC signing secret and invalidates the old one; a matching action in Settings →
  Webhooks lets you rotate a secret from the UI. The signing secret is also now revealed
  once on webhook creation (previously returned by the API but never shown in the UI).
- CI now runs a frontend job — ESLint (`--max-warnings 0`), a production build, and the
  reporter unit tests — alongside the existing backend test job, which now runs with
  coverage. Node is pinned to 22 (`.nvmrc` + `package.json` `engines`).
- Docker health checks for the backend and nginx services.
- Handler test coverage for authentication, webhooks, and backups.
- `.env.example` templates for Docker (repo root) and local backend development.

### Fixed
- WebSocket "connected" acknowledgements no longer race the hub closing a slow client's
  channel (the client send channel now has a single owner).
- The failure-analysis worker, defect migration, and webhook dispatch no longer silently
  swallow logging errors.
- Screenshot serving now strictly validates the result ID (UUID) and filename
  (path-traversal hardening).
- `GetTestRun` propagates database errors from its defect-count and retry-stat queries
  instead of ignoring them; retry stats are now computed in a single query.
- Jira child-requirement imports report per-child failures (`child_import_errors`)
  instead of silently skipping them.
- The AI-generation context no longer rewrites `sessionStorage` on every render.

### Changed
- Frontend lint baseline cleared to zero; CI enforces it going forward.
- Large components split into focused files, behavior-preserving: `TestRunDetail`,
  `AIGenSettings`, and `AIGenerateStudio`.
- The backend Docker image now runs as a non-root user (uid 1000).
- Documentation refresh: updated `CLAUDE.md` project structure, fixed the README
  quick-start (`make setup`, `.env.example`), and added an `AGENTS.md` pointer.

### Upgrade notes
- **Non-root container, existing deployments:** the backend now runs as uid 1000. If your
  `db_data` volume was created by an older, root-run container, it's root-owned and the
  new container can't write to it. One-time fix:
  `docker compose run --rm --user root backend chown -R 1000:1000 /data`, then
  `docker compose up -d`. Fresh installs are unaffected.
- **Known limitation (pre-existing):** `secret.key` (the at-rest encryption key) and the
  `backups/` directory aren't yet on named volumes, so they don't survive container
  recreation. Back up `secret.key` before recreating/upgrading the container — losing it
  makes previously-encrypted integration/LLM credentials undecryptable. Tracked as a
  follow-up.

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
