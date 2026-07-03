# TTGO Performance Tests

Load and capacity tests for the TTGO tracking API, per
`specs/2026-07-03-performance-testing-design.md` (local-only). Phase 1 covers
the harness, the seeder, a smoke check, and S1 "CI ingest storm".

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

Every scenario run gets a fresh directory under `perf/results/<timestamp>-<scenario>/`:

| File | Contents |
|---|---|
| `summary.json` | k6 metrics summary (per-op latency percentiles, error rates) |
| `telemetry.csv` | server RSS (KB), %CPU, and SQLite WAL size (bytes), sampled every 2 s |
| `server.log` | server stdout/stderr for the run |

## Targets and knobs

| Target | Scenario | Purpose |
|---|---|---|
| `seed` | — | (Re)provision the scratch DB and `.seed-manifest.json` (wipes first) |
| `smoke` | `smoke.js` | Fail-fast harness sanity check (2 VUs, strict thresholds) |
| `capacity` | `ingest-storm.js` | Closed-loop VU ramp to find the write-path ceiling |
| `capacity-rate` | `ingest-storm.js` (MODE=rate) | Open-loop fixed pipeline arrival rate |
| `clean` | — | Delete scratch DB, manifest, and results |

Environment knobs (pass as `VAR=value make -C perf <target>`):

| Var | Default | Meaning |
|---|---|---|
| `TIER` | `small` | Dataset tier (phase 1: small ≈ 10k historical results) |
| `PORT` | `8877` | Server port (avoid 8080 to not collide with a dev server) |
| `TOKENS` | `100` | Write tokens minted at seed; keep ≥ peak VUs so ingest spreads across token rows (see methodology) |
| `RESEED` | `0` | `1` forces wipe + reseed before the scenario |
| `RESULTS_PER_RUN` | `200` | Results per simulated pipeline (max 500 = ingest pool size) |
| `STAGES` | 10-min ramp to 100 VUs | JSON stages for closed-loop mode |
| `PIPELINES_PER_MIN` | `6` | Arrival rate for `capacity-rate` |
| `RATE_DURATION` | `10m` | Duration for `capacity-rate` |
| `K6_ARGS` | — | Extra flags appended to `k6 run` |

## Methodology (short version — the spec has the full one)

- **Breaking point** = add-result p95 > 500 ms sustained, or error rate > 1 %.
  Thresholds mark a run failed but never abort it: capacity runs are meant to
  ramp *past* the ceiling and map it. Read `summary.json` percentiles together
  with `telemetry.csv` (WAL growth, RSS) to see what gave out first.
- Each simulated pipeline behaves exactly like the real Playwright reporter:
  `POST /api/runs` → N × `POST /api/runs/{id}/results` (one result per
  request — TTGO has no batch-ingest endpoint) → `POST /api/runs/{id}/complete`.
- Auth uses pre-minted write-scoped API tokens from the seed manifest — the
  login endpoint is rate-limited (~30/min per IP) and must not appear in load
  paths. The server updates `last_used_at` on every token validation, so each
  authenticated request carries one extra SQLite write; the seed mints `TOKENS`
  (default 100) of them so that at peak VUs each pipeline gets its own token row
  instead of concentrating those writes onto a few. Bump `TOKENS` if you raise
  the VU ceiling above 100.
- Each result POST sends `test_name_snapshot`, exactly as the Playwright
  reporter does. Omitting it makes the server backfill the name with an extra
  `test_cases` lookup per result, which would measure a heavier path than real
  clients hit.
- The scratch DB has **no LLM provider and no webhooks configured**, so AI
  failure analysis and webhook dispatch stay out of the measurements.
- Load generator and server share this machine: absolute numbers are
  conservative; relative comparisons and regressions are what count. Record
  machine specs when you keep a result.

## Caveats / safety

- `perfseed` refuses any DB not named `perf-*.db`, and refuses a `perf-*.db`
  path that is a symlink (so it can't be aimed at a real database indirectly);
  everything lives under `perf/.scratch/` (gitignored). `perf/.seed-manifest.json`
  contains raw bearer tokens — it is gitignored and written (and re-chmod'd) to
  mode 0600 even if it already existed; don't move it.
- Later phases (read-path scenarios, WebSocket fan-out, regression gate) are
  specified in the design doc and land separately.
