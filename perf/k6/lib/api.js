// Shared k6 helpers for TTGO perf scenarios. Auth comes from the seed
// manifest written by `perfseed` (run `make -C perf seed`). These files run
// in k6's embedded JS runtime: k6 modules only, no npm imports.
import http from 'k6/http';
import { check, fail } from 'k6';
import { SharedArray } from 'k6/data';

export const BASE_URL = __ENV.TTGO_BASE_URL || 'http://localhost:8877';

// open() only works in the init context. The default path works whether k6
// resolves it against this lib file or the main scenario script — both live
// two levels below perf/. The runner script always passes TTGO_MANIFEST as an
// absolute path anyway.
const MANIFEST_PATH = __ENV.TTGO_MANIFEST || '../../.seed-manifest.json';

// SharedArray: the loader runs once and every VU reads the same frozen copy.
// Plain init-context parsing would give each VU its own manifest copy (k6
// runs the init context once per VU) — wasted RSS on the machine the server
// shares.
const tokens = new SharedArray('perf tokens', () => {
  const t = JSON.parse(open(MANIFEST_PATH)).tokens || [];
  if (t.length === 0) {
    throw new Error('seed manifest has no tokens — run `make -C perf seed` first');
  }
  return t;
});

const BROWSERS = ['chromium', 'firefox', 'webkit'];

// Mirrors what the Playwright reporter sends per result. Deterministic by
// index (~12% FAIL with a realistic stack trace, ~3% SKIP) — no Math.random,
// so runs are comparable.
//
// test_name_snapshot is sent on purpose: the real reporter always sends it
// (frontend/e2e/reporters/reporter-helpers.js), and when it is empty the server
// backfills the name with an extra `SELECT` on test_cases per result POST
// (store AddRunResult). Omitting it would make every POST measure a heavier
// path than production clients hit, skewing the capacity numbers. The name
// comes straight from the manifest, so it always matches the seeded case row.
function resultBody(tc, i) {
  const roll = (i * 2654435761) % 100;
  const body = {
    test_case_id: tc.id,
    test_name_snapshot: tc.name,
    status: 'PASS',
    duration_ms: 50 + ((i * 97) % 4950),
    browser: BROWSERS[i % BROWSERS.length],
    environment: 'perf',
  };
  if (roll < 12) {
    body.status = 'FAIL';
    body.error_message = 'Assertion failed: expected element to be visible';
    body.stack_trace =
      'Error: expect(locator).toBeVisible()\n    at perf/scenario.spec.js:42:11\n    at runStep (lib/runner.js:118:9)\n'.repeat(4);
  } else if (roll < 15) {
    body.status = 'SKIP';
  }
  return body;
}

// Result payloads are byte-identical across iterations and VUs, so they are
// serialized once here instead of per request — per-request object building +
// JSON.stringify would burn load-generator CPU exactly at the ramp peak, on
// the machine the measured server shares.
const resultBodies = new SharedArray('perf result bodies', () => {
  const pool = JSON.parse(open(MANIFEST_PATH)).ingest_test_cases || [];
  if (pool.length === 0) {
    throw new Error(
      'seed manifest has no ingest_test_cases — reseed with RESEED=1 (older manifests predate the {id,name} format)'
    );
  }
  return pool.map((tc, i) => JSON.stringify(resultBody(tc, i)));
});

// Stable token per VU: spreads the per-request last_used_at update the server
// performs in ValidateToken across distinct token rows.
export function tokenForVU() {
  return tokens[(__VU - 1) % tokens.length];
}

export function tokenPoolSize() {
  return tokens.length;
}

// Init-context guard for the documented invariant: every concurrently active
// VU needs its own token row, or the last_used_at UPDATE contention the pool
// exists to spread comes back silently — exactly at the load level being
// measured. Throwing here aborts the run before any load is generated.
// Returns peakVUs so callers can use it inline.
export function assertTokenPool(peakVUs) {
  if (!Number.isInteger(peakVUs) || peakVUs < 1) {
    throw new Error(`peak VU count must be a positive integer, got ${peakVUs}`);
  }
  if (tokens.length < peakVUs) {
    throw new Error(
      `token pool (${tokens.length}) < peak VUs (${peakVUs}) — reseed with TOKENS=${peakVUs} RESEED=1`
    );
  }
  return peakVUs;
}

// Validates RESULTS_PER_RUN in the init context, where a throw aborts the run
// immediately. A per-iteration fail() would burn the whole scenario duration
// with zero requests and still exit 0 — thresholds pass vacuously over empty
// metrics. Number.isInteger also rejects NaN (e.g. RESULTS_PER_RUN=1k), which
// sails past `n > pool` comparisons.
export function resultsPerRun(defaultN) {
  const raw = __ENV.RESULTS_PER_RUN;
  const n = raw === undefined || raw === '' ? defaultN : Number(raw);
  if (!Number.isInteger(n) || n < 1 || n > resultBodies.length) {
    throw new Error(
      `RESULTS_PER_RUN=${raw} must be an integer between 1 and ${resultBodies.length} (the ingest pool size)`
    );
  }
  return n;
}

function request(method, path, body, op, token) {
  const payload = body == null ? null : typeof body === 'string' ? body : JSON.stringify(body);
  const res = http.request(method, `${BASE_URL}/api${path}`, payload, {
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    tags: { op },
  });
  check(res, { [`${op} 2xx`]: (r) => r.status >= 200 && r.status < 300 }, { op });
  return res;
}

export function createRun(token, name) {
  // category_id stays null on purpose: creating a run WITH a category makes
  // the backend snapshot every test case in it into PENDING results (see the
  // note in frontend/e2e/reporters/ttgo-client.js).
  const res = request('POST', '/runs', { name, category_id: null, run_folder_id: null }, 'create_run', token);
  if (res.status < 200 || res.status >= 300) {
    return null; // the 2xx check above already recorded the failure
  }
  // res.json() throws on a null/unparseable body (timeouts, connection
  // resets, --discard-response-bodies), which would abort the iteration and
  // bypass the caller's guard — degrade to null instead, and surface a
  // missing body as a failed check rather than an exception.
  let run = null;
  try {
    run = res.json();
  } catch (_) {
    run = null;
  }
  check(run, { 'create_run returned id': (r) => Boolean(r && r.id) }, { op: 'create_run' });
  return run;
}

export function addResult(token, runId, serializedBody) {
  return request('POST', `/runs/${runId}/results`, serializedBody, 'add_result', token);
}

export function completeRun(token, runId) {
  return request('POST', `/runs/${runId}/complete`, null, 'complete_run', token);
}

// One simulated CI pipeline, exactly as the real reporter behaves:
// create run -> n sequential result POSTs -> complete run.
export function ciPipeline(nResults, label) {
  if (nResults > resultBodies.length) {
    // Backstop for direct callers; scenarios validate via resultsPerRun().
    fail(`nResults=${nResults} exceeds the ingest pool of ${resultBodies.length} test cases`);
  }
  const token = tokenForVU();
  const run = createRun(token, `${label} VU${__VU} it${__ITER}`);
  if (!run || !run.id) {
    return; // create_run failed; the checks already recorded it
  }
  for (let i = 0; i < nResults; i++) {
    addResult(token, run.id, resultBodies[i]);
  }
  completeRun(token, run.id);
}
