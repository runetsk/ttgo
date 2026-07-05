# TTGO Performance Tests

Load and capacity tests for the TTGO tracking API, per
`specs/2026-07-03-performance-testing-design.md` (local-only). Phase 1 covers
the harness, the seeder, a smoke check, and S1 "CI ingest storm". Phase 2 adds
the read-path scenarios — S2 "dashboards under ingest" (`mixed`) and S3
dataset scaling (`dataset`) — plus medium/large tiers and `scripts/analyze.sh`.
Phases 3–4 add S4 WebSocket fan-out (`ws`) and the S6 regression gate
(`gate` / `gate-baseline`).

## Prerequisites

- Go toolchain (same as the backend; run `make setup` in `backend/` once).
- [k6](https://grafana.com/docs/k6/latest/set-up/install-k6/) v1.x or later on PATH
  (`brew install k6` on macOS).
- `bash`, `curl` (used by the runner script).

## Quick start

```bash
make -C perf seed       # provision perf/.scratch/perf-small.db + token manifest
make -C perf smoke      # ~1 min sanity check of the whole harness
make -C perf capacity   # S1: ramp 0→100 concurrent CI pipelines (~10 min)
```

Every scenario run gets a fresh directory under `perf/results/<timestamp>-<scenario>/`
(`-<mode>` is appended when `MODE` is set, so closed-loop `capacity` and
open-loop `capacity-rate` results stay distinguishable):

| File | Contents |
|---|---|
| `summary.json` | k6 end-of-run summary via `handleSummary` (metric values nested under `.values`, e.g. `.metrics["http_req_duration{op:add_result}"].values["p(95)"]`) |
| `telemetry.csv` | server RSS (KB), %CPU, and SQLite WAL size (bytes), sampled every 2 s |
| `server.log` | server stdout/stderr for the run |
| `run-config.json` | provenance: git SHA (+dirty flag), k6/Go versions, machine specs, and every knob value (`null` = unset, i.e. the scenario default at that SHA) |

## Targets and knobs

| Target | Scenario | Purpose |
|---|---|---|
| `seed` | — | (Re)provision the scratch DB and `.seed-manifest.json` (wipes first) |
| `smoke` | `smoke.js` | Fail-fast harness sanity check (2 VUs, strict thresholds) |
| `capacity` | `ingest-storm.js` | Closed-loop VU ramp to find the write-path ceiling |
| `capacity-rate` | `ingest-storm.js` (MODE=rate) | Open-loop fixed pipeline arrival rate |
| `mixed` | `mixed-workload.js` | S2: browsing reads, ingest storm joins at half-time (read-latency delta) |
| `dataset` | `dataset-scaling.js` | S3: fixed read mix per tier; compare p95 across small/medium/large |
| `ws` | `ws-fanout.js` | S4: WebSocket fan-out — CLIENTS subscribed sockets + a paced ingest probe (lag/loss/caps) |
| `gate` | `gate.js` | S6: 3-min fixed mixed profile; **fails (exit ≠ 0) if any op's p95 exceeds its committed baseline × 1.3** |
| `gate-baseline` | `gate.js` | Re-measure and rewrite `baselines/gate-<tier>.json` (commit the result) |
| `clean` | — | Delete scratch DB, manifest, and results |

Environment knobs (pass as `VAR=value make -C perf <target>`):

| Var | Default | Meaning |
|---|---|---|
| `TIER` | `small` | Dataset tier: `small` ≈ 10k results, `medium` ≈ 100k, `large` ≈ 1M (switching tiers auto-reseeds — the manifest is checked against the target DB) |
| `PORT` | `8877` | Server port (avoid 8080 to not collide with a dev server) |
| `TOKENS` | `100` | Write tokens minted at seed (needs `RESEED=1` to change); scenarios abort at init if the pool < peak VUs (see methodology) |
| `MAX_VUS` | token pool size | Rate-mode VU ceiling for `capacity-rate`; also caps ingest VUs for `mixed`, where it defaults to the token pool minus `READ_VUS`; must be ≤ the seeded token pool |
| `RESEED` | `0` | `1` forces wipe + reseed before the scenario |
| `RESULTS_PER_RUN` | `200` capacity/mixed / `20` smoke | Results per simulated pipeline (max 500 = ingest pool size) |
| `STAGES` | 10-min ramp to 100 VUs | JSON stages for closed-loop mode |
| `PIPELINES_PER_MIN` | `6` capacity-rate / `75` mixed | Arrival rate for `capacity-rate`; also the ingest-storm rate for `mixed` (S2) |
| `RATE_DURATION` | `10m` | Duration for `capacity-rate` |
| `READ_VUS` | `5` dataset / `20` mixed | Browsing VUs for the read-path scenarios |
| `ITERATIONS` | `2000` | Total read operations for `dataset` |
| `PHASE_MINUTES` | `4` | `mixed` phase length: reads-only for one phase, reads+ingest for one more |
| `USERS` | `10` | Perf users minted at seed (needs `RESEED=1`); WS clients need `ceil(CLIENTS/20)` users |
| `CLIENTS` | `200` | Concurrent WS clients for `ws` (cap-probing at 1000 needs `USERS=50 RESEED=1`) |
| `HOLD_MINUTES` | `3` | `ws` hold phase at full client count |
| `RESULTS_PER_SEC` | `20` | `ws` probe post rate |
| `K6_ARGS` | — | Extra flags appended to `k6 run` |

## Methodology

The local design spec holds the full rationale; this section is self-contained
for operators: what a run measures, what it deliberately excludes, and how to
read the output.

### What a simulated pipeline does

Each VU emulates one CI pipeline making the same calls in the same order as the
real Playwright reporter (`frontend/e2e/reporters/ttgo-client.js`):
`POST /api/runs` → N × `POST /api/runs/{id}/results` (one result per request —
TTGO has no batch-ingest endpoint) → `POST /api/runs/{id}/complete`.

That is call-sequence parity on the ingest hot path, not full reporter parity:
the real reporter can also create folders/test cases on first sight, attach
categories, and send attempt/OS/defect metadata. None of that happens in the
load path here, so S1 measures steady-state ingest cost, not first-run setup.

Payloads are deterministic by loop index (no `Math.random`): ~12 % FAIL with a
realistic multi-line stack trace, ~3 % SKIP, durations spread 50–5000 ms. Two
consequences: runs are directly comparable with each other, and every result
POST carries `test_name_snapshot` exactly as the real reporter does — omitting
it would make the server backfill the name with an extra `test_cases` SELECT
per result, i.e. measure a heavier path than production clients hit.

### Closed loop vs open loop

- `capacity` (MODE=vus, ramping VUs) is **closed-loop**: each VU waits for its
  previous request before sending the next, like a real reporter process. Under
  overload a closed-loop client slows down *with* the server (coordinated
  omission), so the ramp answers "how many concurrent pipelines before
  degradation", not "how many results/sec".
- `capacity-rate` (MODE=rate, constant arrival rate) is **open-loop**: new
  pipelines start on schedule no matter how slow the server gets. Use it after
  the ramp has bracketed the ceiling, to measure sustained results/sec at a
  fixed load level.
- In rate mode `dropped_iterations` has an observational `count==0` threshold:
  nonzero means k6 ran out of VUs (`MAX_VUS`, default = the token pool size)
  and offered *less* load than requested — the run would otherwise look
  healthier than it is.

### Breaking point, and how to read a run

**Breaking point** = add-result p95 > 500 ms sustained, or error rate > 1 %.
Thresholds mark the run failed but never abort it: capacity runs are meant to
ramp *past* the ceiling and map it. Accordingly the capacity Make targets treat
k6's threshold-failure exit code (99) as the expected outcome; `smoke` still
hard-fails on it. Read the three outputs together:

- `summary.json` — per-op percentiles (`op:create_run` / `op:add_result` /
  `op:complete_run`) and `http_req_failed`. The per-op split matters:
  create/complete are 1/N of requests, so blended latency hides the add-result
  signal.
- `telemetry.csv` — server RSS, %CPU, WAL size every 2 s. Sustained WAL growth
  means checkpointing is not keeping up with the write rate; that usually gives
  out before CPU does.
- `server.log` — the first busy-timeout / `database is locked` 5xx marks when
  SQLite's single-writer limit became the bottleneck. Expect this file to be
  large: one log line per request, so a full capacity run writes 10⁵–10⁶ lines
  (tens of MB).

*What breaks first* is the finding; the VU level and results/sec where it
breaks are the capacity numbers. The runner records machine specs, git SHA
(with a dirty flag), k6/Go versions, tier, and knob values in each run's
`run-config.json`, so a kept result carries its own provenance — a `null` knob
means it was unset and the scenario default at that SHA applied. `results/` is
never pruned automatically — `make clean` removes it, but also deletes the DB
and manifest.

### Read-path scenarios (S2/S3) and windowed analysis

`scripts/analyze.sh results/<run-dir> [window-seconds]` turns `server.log`
into a per-window, per-operation p95/error table. Use it instead of the k6
aggregate whenever the question is *when* latency changed: the summary blends
a run's quiet and loaded stretches into one number (both 2026-07-04 capacity
runs passed aggregate thresholds that their peak windows violated 2×).

- **S2 (`mixed`)** runs browsing VUs for `2 × PHASE_MINUTES`; the ingest
  storm (default 75 pipelines/min × 200 results ≈ 250 results/s, ~50 % of the
  measured S1 ceiling) joins at half-time. The finding is the read-op p95
  delta between the two halves in the analyze.sh output — not the blended
  summary. `dropped_iterations` must be 0, or the background write load was
  lighter than configured.
- **S3 (`dataset`)** runs the same weighted read mix (30 % runs list, 30 %
  run detail, 15 % analytics summary, 10 % trend, 10 % FTS search, 5 % flaky)
  at low fixed concurrency, once per tier: `RESEED=1 make dataset`, then
  `TIER=medium make dataset`, then `TIER=large make dataset`. Reseed is
  automatic on tier switch. Compare per-op p95 across the three summaries —
  super-linear growth with dataset size flags missing indexes or full scans.
- Read scenarios hit `GET /api/runs`, `/api/runs/{id}` (IDs come from the
  manifest's `historical_run_ids`), `/api/analytics/{summary,trend,flaky}`,
  and `/api/search` — all with the same write-scoped bearer tokens (write
  scope satisfies read-scoped endpoints).

### Regression gate (S6)

`make -C perf gate` answers "did this change make it slower?" in ~4 minutes:
a fixed 3-minute blended profile (10 browsing VUs with think time + 25
pipelines/min ≈ 83 results/s) on a freshly-reseeded small tier, with hard
per-op thresholds derived at init from `baselines/gate-small.json`:
`p95 < baseline × 1.3` (floor 25 ms), plus failsafes (`http_req_failed
< 1 %`, checks > 99 %, `dropped_iterations == 0`). A breach exits non-zero.

- Baselines are **committed and machine-stamped** — the gate refuses a
  baseline recorded on other hardware (`GATE_IGNORE_MACHINE=1` overrides,
  at your own risk). Re-run `make -C perf gate-baseline` and commit the new
  file after intentional performance changes, dependency bumps that touch
  the write path, or a hardware change.
- Both targets force `RESEED=1`: gate numbers are only comparable against a
  fresh small tier.
- CI wiring is a documented option in the design spec, not wired here — the
  gate is local-first by design.

### WebSocket fan-out (S4)

WS connections authenticate with **session cookies** — Bearer tokens are
rejected before the upgrade — so `ws` logs in `ceil(CLIENTS/20)` seeded perf
users during k6 setup, paced under the per-IP login limiter (burst 10, then
one login per ~2 s; 50 users ≈ 90 s of setup before any load). Each client
subscribes to `runs:*`; a single probe VU posts results at `RESULTS_PER_SEC`
during the hold phase only, so every connected client should receive every
probe event:

- **Lag** — `ws_lag_ms` (client receive time − server `timestamp`; same
  machine ⇒ no clock skew). Events arrive as `result_updated`.
- **Loss** — `1 − ws_events_received / (ws_probe_results_posted × CLIENTS)`,
  computed from the summary. Attribute loss via `server.log`: hub-ingress
  drops (256-slot broadcast channel full — logged warning) and per-client
  egress drops (256-slot send buffer overflow — client force-disconnected)
  are different failure modes.
- **Caps** — global 1000 clients / 20 per user. `ws_connect_errors` counts
  rejected upgrades; probing past the caps is the point, so the `ws` target
  tolerates k6 exit 99.

### Auth and write amplification

Load traffic authenticates with pre-minted write-scoped API tokens from the
seed manifest — the login endpoint is rate-limited (~30/min per IP) and must
never appear in a load path. Token validation updates `last_used_at` on every
request, so each authenticated request carries one extra SQLite write. That
amplification is part of the real write path and is deliberately measured.

To keep that update from concentrating on a few hot rows, the seed mints
`TOKENS` (default 100) tokens and each VU takes its own (`(__VU-1) % pool`).
The invariant is **tokens ≥ peak VUs** — including rate mode, which can scale
VUs up exactly when the server slows down. The scenarios assert this at init
(a run aborts immediately, before any load, if the manifest pool is smaller
than the configured peak), and rate mode's `MAX_VUS` defaults to the pool
size. Raising a VU ceiling therefore means reseeding with a bigger `TOKENS`
(see the reseed note below).

### Reseeding and run comparability

The runner reseeds only when `RESEED=1` or when the DB/manifest is missing.
Two consequences:

- Seed-time knobs (`TOKENS`, tier contents) **no-op** against an existing
  scratch DB — changing them requires `RESEED=1`. The runner warns when the
  reused manifest's token count differs from `TOKENS`.
- Without reseeding, each run's rows stay in the DB, so run N+1 executes
  against a bigger database than run N. Reseed before every run you intend to
  keep; reuse the DB only for quick iteration.

The seeder is deterministic (`-seed`, default 1): same seed + tier ⇒ identical
dataset with stable IDs.

### What is deliberately excluded

- **No LLM provider, no webhooks** in the scratch DB — AI failure analysis and
  webhook dispatch stay out of the measurements.
- WebSocket broadcasts *do* fire on every result (the hub is part of the write
  path), but with zero subscribers connected the fan-out is a no-op; S4
  (phase 3) measures it under real subscriber load.
- No read traffic in S1 — the dashboards-under-ingest mix is S2's `mixed`
  scenario (see the read-path subsection above).

### Same-machine caveat

Load generator and server share this machine: k6's own CPU (payload building,
HTTP, metric collection) competes with the server exactly at the ramp peak, so
absolute numbers are conservative. Relative comparisons, scaling curves, and
regression deltas are what count. The k6 scripts honor `TTGO_BASE_URL`, so for
publishable absolute numbers run k6 directly against a remote instance — the
runner script always provisions a local server.

## Caveats / safety

- `perfseed` refuses any DB not named `perf-*.db`, and refuses a `perf-*.db`
  path that is a symlink. Treat that as a seatbelt against local mistakes, not
  a security boundary — it does not detect hard links or a symlinked parent
  directory. Everything lives under `perf/.scratch/` (gitignored). `perf/.seed-manifest.json`
  contains raw bearer tokens — it is gitignored and written (and re-chmod'd) to
  mode 0600 even if it already existed; don't move it.
- The `large` tier seeds ~1M results: expect ~1–3 min of seeding, ~1.5–2 GB
  on disk under `perf/.scratch/`, and a few hundred MB of transient seeder
  RAM (rows are generated in memory before batch insert).
- Remaining design-doc options (S5 soak, CI wiring for the gate, the
  webhook-overflow probe) land separately.
