# Changelog

All notable changes to TTGO are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- WebSocket result-level events (`result_updated`, `result_bulk_updated`,
  `result_retried`, `result_deleted`) now carry a delta payload
  (`{run_id, run, results | result_ids+patch | deleted_result_ids}` where
  `run` is a summary with latest-attempt counters and no `run_results`)
  instead of the full re-fetched test run. External WS consumers must merge
  deltas or re-fetch the run. Fan-out frame size is now O(changed rows)
  (~100× smaller by the end of a 200-result run) and result ingestion no
  longer re-reads the whole run per posted result (S4 finding: full-run
  frames overflowed per-client egress buffers at 1000 subscribers and
  throttled the probe to ~5 results/s of the configured 20).

### Fixed
- Perf harness `run-config.json` now records the S4 `ws` knobs (`CLIENTS`,
  `HOLD_MINUTES`, `RESULTS_PER_SEC`, `USERS`); previously omitted, so a kept
  `ws` run's provenance did not capture its client count, hold length, or
  probe rate.
- Perf harness measurement-validity fixes (external Codex review): per-op
  `count>0` thresholds everywhere a duration threshold exists (a tag-scoped
  p95 over zero samples evaluates to 0 and passes — the gate could pass an op
  it never exercised); the S4 probe starts after a 10 s subscribe-settle
  window so late handshakes are not misattributed as fan-out loss; gate
  baselines are stamped with CPU/cores/RAM/arch instead of CPU model alone
  (baseline re-measured, capturing the delta-payload write-path gain:
  add-result baseline 4.3 → 1.3 ms); `analyze.sh` uses exact nearest-rank
  p95 and survives midnight rollover; the perfseed caveat in `perf/README.md`
  now matches the implemented hard-link/parent-symlink guards.
- Run detail page crashed on load ("Something went wrong") since the effect
  dependency-array lint cleanup: the mount effect read `loadRun` /
  `loadCurrentAnalyses` from its dependency array before their `const`
  declarations (TDZ `ReferenceError`). Declarations now precede the effect.
- Read-path query scaling on large databases (S3 finding, ~1M results): the
  runs list computed status and defect-type counts in two passes, and the
  defect pass's `status IN ('FAIL','ERROR')` predicate baited SQLite into
  scanning the whole table via the status index (~2s per page); both counts
  now come from one grouped query pinned to the `test_run_id` index.
  Analytics summary/trend/flaky-candidate queries are now answered entirely
  from a new covering index `run_results(start_time, status, test_case_id)`
  instead of fetching every wide row in the date window, and the flaky
  detector's per-test-case history reads are index-only ordered scans on
  `run_results(test_case_id, start_time, status)` (names are resolved only
  for the returned page). Superseded single-column/duplicate indexes on
  `run_results` are dropped at startup (`test_case_id` ×2, `start_time`, and
  the old `(test_case_id, status, start_time)` composite), so ingest writes
  fewer index entries than before. At the large perf tier this takes
  `GET /api/runs` from ~2.1s p95 to ~35ms, `/api/analytics/summary` from
  ~740ms to ~50ms, and `/api/analytics/trend` / `flaky` from ~2.3–2.4s to
  ~220–300ms.

### Added
- Performance test harness (phase 1): a `perf/` k6 suite for the result-ingest
  write path — smoke check plus the S1 "CI ingest storm" capacity scenario —
  and a `perfseed` CLI that provisions scratch databases with a deterministic
  dataset, perf users, and write-scoped API tokens. The CLI refuses to touch any
  non-`perf-*.db` database (or a symlinked one) and writes its token manifest at
  mode 0600. Load pipelines send `test_name_snapshot` and spread across a
  `TOKENS`-sized (default 100) token pool so the measured write path matches
  real clients rather than harness artifacts.
- Perf harness hardening (post-review): the runner refuses a busy port and ties
  the readiness probe to the server it started (a stale listener can no longer
  be measured as the current build); the server is a direct child with the
  cleanup trap installed first, so teardown genuinely waits for graceful
  shutdown; capacity targets treat k6's threshold-failure exit (99) as the
  expected mapping-run outcome while `smoke` still hard-fails; `RESULTS_PER_RUN`
  is validated in the init context (a typo aborts the run instead of producing
  a vacuously green summary); scenarios assert `tokens ≥ peak VUs` at init and
  rate mode's new `MAX_VUS` knob defaults to the token-pool size, with an
  observational `dropped_iterations` threshold; `perfseed` additionally refuses
  hard links, symlinked parent directories, and a symlinked manifest path; the
  seed manifest now carries `{id,name}` pairs so k6 no longer re-derives the
  seeded case-name format (reseed with `RESEED=1` after upgrading); the k6 lib
  loads the manifest via `SharedArray` and pre-serializes result payloads to
  cut load-generator overhead; seeding is owned by a single `perf/scripts/seed.sh`
  used by both `make seed` and the runner's auto-reseed.
- Perf run provenance: every results directory now gets a `run-config.json`
  recording git SHA (+dirty flag), k6/Go versions, machine specs, and all knob
  values (`null` = scenario default), replacing the "record these manually"
  instruction; rate-mode runs append `-rate` to the results directory name so
  closed- and open-loop `ingest-storm` results stay distinguishable.
- Perf phase 2 (read paths): `medium` (~100k results) and `large` (~1M)
  seeder tiers; S2 `mixed` scenario (browsing reads with an ingest storm
  joining at half-time — the read-latency delta is the finding); S3 `dataset`
  scenario (fixed read mix per tier for a dataset-scaling curve); a weighted
  read-mix k6 lib driving runs list/detail, analytics, and FTS search with
  per-op thresholds; `perf/scripts/analyze.sh` for windowed per-op
  p95/error tables from server logs; shared k6 config (`perf/k6/config/`)
  for thresholds and workload profiles; the seed manifest carries
  `historical_run_ids`; the runner reseeds automatically when the manifest
  and target tier DB disagree; the seeder rejects non-positive counts and
  results not divisible by runs.
- Perf phase 4: S6 regression gate — `make -C perf gate` runs a fixed 3-minute
  mixed profile against a freshly-seeded small tier and fails if any
  operation's p95 exceeds its committed, machine-stamped baseline
  (`perf/baselines/gate-<tier>.json`, refreshed via `make -C perf
  gate-baseline`) by more than 30 %; all perf scenarios moved from the
  deprecated `--summary-export` to a shared `handleSummary` (vendored
  k6-summary keeps the terminal output; `summary.json` now uses the
  handleSummary shape with values under `.values`).
- Perf phase 3: S4 WebSocket fan-out scenario (`make -C perf ws`) — session-
  cookie authenticated clients (Bearer tokens cannot open WS connections)
  subscribe to `runs:*` while a paced probe posts results; measures broadcast
  lag from server timestamps, count-based delivery loss, and behavior at the
  1000-global/20-per-user connection caps. The seed manifest now carries the
  perf users' shared password, and `seed.sh` exposes a `USERS` knob.
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
