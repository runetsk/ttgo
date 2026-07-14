# Changelog

All notable changes to TTGO are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- First-run setup: a fresh instance (no users) shows a "Create admin account" screen at login and bootstraps the admin from the browser (`GET /api/auth/needs-setup`, `POST /api/auth/setup`). `ADMIN_EMAIL`/`ADMIN_PASSWORD` are now optional — still supported for automated seeding — and the server no longer refuses to start without them.
- Test case page shows linked active (open) bugs, each navigable to its tracker (external) or the Defects register (native); linked requirement chips now open the requirement detail page.
- Create-run modal can build a run from hand-picked tests (searchable multi-select) instead of a whole category.
- Complete Run / Reopen buttons on the run detail header (derive PASS/FAIL from results).
- Execution mode on run detail: step-by-step manual execution with Pass/Fail/Skip, defect classification, auto-advance, and duration capture.
- Attach screenshots to a run result as manual evidence — from the execution-mode fail panel and the expandable result detail. Uploads now append to a result's gallery instead of replacing it.
- Per-step Pass/Fail/Skip verdicts with optional notes during manual execution; marking a step fail derives the result to FAIL, and the result detail shows the captured step checklist.
- Assign a test run to a user from the run detail header, and filter the runs list by assignee — including "assigned to me" and "unassigned".
- AI Generate page: live prompt preview showing the exact prompt Generate will send — color-coded by source (requirement, children, coverage, detail, instructions), updates as options change, and flags when no requirement is linked.
- LLM provider settings now group by type — **OpenAI-compatible** (OpenAI, Google Gemini, **OpenRouter**, or any **Custom** OpenAI-compatible host), **Claude**, and **Local / Ollama** — with presets that prefill the endpoint and model. OpenRouter ships the correct base URL, and provider cards show the recognized preset. Existing providers are unaffected.
- AI generation quality engine: deterministic coverage targets derived from acceptance-criteria bullets and child requirements (covered / uncovered / over-represented, with `source_refs` matching), an explainable per-draft quality rubric (test-data specificity, action clarity, expected-result observability, uniqueness, traceability — warnings only, never blocking), and duplicate candidates within the batch and against existing tests via SQLite FTS5. Exposed as `coverage`, `quality`, and `duplicates` on the `/api/ai-generations` responses; draft edits recompute them live. Includes a versioned deterministic benchmark suite (CI) and an opt-in real-provider evaluation harness.
- AI reviewer workflow: drafts are now editable in the detail pane (name, category, description, source refs, steps with add/remove/reorder) with debounced autosave and inline validation; structured rejection reasons with one-click restore; quality-aware filters (invalid, warnings, duplicates, no-refs, edited); a requirement-coverage matrix beside the draft list; `j/k/a/r` keyboard review shortcuts; "Accept all clean drafts" with a pre-flight exclusion list and commit summary; and resumable sessions — refresh recovery plus per-requirement run history with read-only reopen, one-click clone, and a stale-requirement warning.
- Targeted AI draft regeneration: regenerate one draft with quick actions (make more specific, add a negative case, repair selected findings) or a free-form instruction; the result arrives as a new pending version alongside the original with a word-level field/step diff, and choosing one marks the other superseded (versions and their token usage are retained). Optional LLM critic pass for thorough/comprehensive runs appends explainable semantic findings (never overrides structural validation). `POST /api/ai-generations/{id}/cancel` cancels an in-flight run via a new in-process registry (or stamps a stale one cancelled), and the studio gains cancel buttons for the active generation and running history entries.
- AI cost & learning analytics: optional per-provider prompt/completion pricing ($/1M tokens) drives a configured `estimated_cost` on every generation run and per-attempt token/cost breakdown (`attempts` on run payloads); soft per-request and monthly budgets warn with an explicit confirm instead of degrading requests; new `GET /api/ai-generations/reports/summary` powers an Analytics "AI Generation" section — run outcomes, acceptance-vs-edit rates, rejection reasons, token/cost totals, and provider/model comparisons.

### Changed
- AI Generate composer simplified: the category preset buttons (Happy paths, Edge cases, Negative, …) are gone — the live prompt preview already shows what the options inject — and the instructions box is now clearly labeled as optional additional instructions appended to the prompt.
- Default AI generation prompt fixed and aligned: the Detail Level definitions now use the exact values the UI sends ("Simplified" / "Standard" / "Detailed" — previously two of three never matched), and the coverage bullet uses the "Functional" category name from the enum. Existing installs that never customized the template are upgraded automatically on startup; customized templates are untouched (Reset to Default picks up the new text).
- Library sidebar shows test cases in the folder tree by default (the "Show tests in tree" toggle now defaults on); anyone who explicitly turned it off keeps that choice.
- Linked-requirements picker on the test case page now shows richer options — an identifier pill, the title with search-match highlighting, a source badge (Jira/Confluence), and a one-line description preview — plus keyboard navigation (↑/↓ to move, Enter to link, Esc to close).
- New Test Run modal is now a larger folder/test tree picker (tri-state folder checkboxes), and newly created runs appear in the run sidebar live.
- Run detail's Execute button now runs only the checked results — showing the count (e.g. "Execute (3)") and scoping the execution queue to that subset. It is disabled until at least one result is checked (use the header select-all checkbox to run the whole run).
- Creating a test run now opens its detail page immediately.
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
- A PENDING run switches to RUNNING automatically when its first result is updated manually.
- Execution-status colors (dots, bars, chart cells) now come from a single shared palette, so PASS/RUNNING/SKIP render consistently across the run pages, timeline, folder sidebar, and analytics.
- Add tests to an open run using the same folder tree-picker as run creation (multi-select, whole-folder select), replacing the single-select dropdown; tests already in the run appear locked (checked + disabled). Backed by a new `POST /runs/{id}/results/bulk` endpoint.

### Deprecated
- `POST /api/requirements/{id}/generate-tests` and `POST /api/requirements/{id}/accept-generated-tests`: both now delegate to the durable `/api/ai-generations` lifecycle (runs persist, `temp_id` carries the persisted draft id, legacy acceptance is finally transactional). As delegated endpoints, the legacy routes now also honor configured soft cost budgets (a configured, exceeded budget returns `409`, since the legacy request has no acknowledge flag) and the additional-instructions length limit. Migrate to `/api/ai-generations`; the legacy routes will be removed in a future major release.

### Fixed
- "Erase All Data" (Settings → Demo Data) did nothing after typing `ERASE`: the confirm request omitted the body the backend requires, so `DELETE /api/admin/reset` was rejected with 400 and nothing was erased. The frontend now sends `{"confirm":"CONFIRM RESET"}`; added a backend test pinning the reset confirmation contract.
- Removed the in-place sign-in modal: anyone who needs to authenticate is now redirected to the `/login` page and returned to where they were after signing in. `/login` shows the normal sign-in form, or the first-run "Create admin account" form on a brand-new instance (no users). Previously that first-run screen was unreachable — an unauthenticated visitor only ever got the modal, which offered no way to bootstrap an admin nobody had credentials for. The `.env.example` files also no longer ship populated `ADMIN_EMAIL`/`ADMIN_PASSWORD` (copying them verbatim silently seeded an admin and skipped first-run setup), so both default to blank now.
- Logged-out visits to /login no longer fire unauthenticated app-level API calls (folder tree, LLM providers, AI features) — they 401'd noisily and popped the session-expired modal over the login form; these fetches now wait for sign-in and refetch on each login. Also fixed a React "setState in render" error from the login page's already-signed-in redirect.
- Test Runs list: "Passed"/"Failed" status filters matched nothing (sent PASSED/FAILED instead of PASS/FAIL).
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

### Added
- Durable AI generation lifecycle: runs, drafts, and events persist across refreshes; new `/api/ai-generations` endpoints (create/read/list, draft edit, structured draft rejection, atomic accept).
- Provider-native structured output: OpenAI-compatible cloud providers receive a strict JSON schema (`json_schema`) with automatic `json_object` downgrade; canonical `{"test_cases": [...]}` response envelope with `source_refs`.
- Bounded retry with exponential backoff, jitter, and `Retry-After` support for transient LLM failures; normalized error categories in API responses.
- Deterministic draft validation with field-level findings (blocks acceptance of invalid drafts).

### Changed
- AI Generate studio now creates idempotent generation runs and accepts drafts atomically — a failed batch acceptance no longer leaves partially created test cases.
- The legacy `generate-tests`/`accept-generated-tests` endpoints remain as compatibility adapters and are slated for deprecation after the migration window.

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
