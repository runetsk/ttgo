// Shared k6 helpers for TTGO perf scenarios. Auth comes from the seed
// manifest written by `perfseed` (run `make -C perf seed`). These files run
// in k6's embedded JS runtime: k6 modules only, no npm imports.
import http from 'k6/http';
import { check, fail } from 'k6';

export const BASE_URL = __ENV.TTGO_BASE_URL || 'http://localhost:8877';

// open() only works in the init context. The default path works whether k6
// resolves it against this lib file or the main scenario script — both live
// two levels below perf/. The runner script always passes TTGO_MANIFEST as an
// absolute path anyway.
const manifest = JSON.parse(open(__ENV.TTGO_MANIFEST || '../../.seed-manifest.json'));

export const ingestCaseIDs = manifest.ingest_test_case_ids || [];

if (!manifest.tokens || manifest.tokens.length === 0) {
  fail('seed manifest has no tokens — run `make -C perf seed` first');
}

// Stable token per VU: spreads the per-request last_used_at update the server
// performs in ValidateToken across distinct token rows.
export function tokenForVU() {
  return manifest.tokens[(__VU - 1) % manifest.tokens.length];
}

function request(method, path, body, op, token) {
  const res = http.request(method, `${BASE_URL}/api${path}`, body == null ? null : JSON.stringify(body), {
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
  return res.json();
}

export function addResult(token, runId, body) {
  return request('POST', `/runs/${runId}/results`, body, 'add_result', token);
}

export function completeRun(token, runId) {
  return request('POST', `/runs/${runId}/complete`, null, 'complete_run', token);
}

const BROWSERS = ['chromium', 'firefox', 'webkit'];

// Mirrors what the Playwright reporter sends per result. Deterministic by
// index (~12% FAIL with a realistic stack trace, ~3% SKIP) — no Math.random,
// so runs are comparable.
export function resultBody(testCaseId, i) {
  const roll = (i * 2654435761) % 100;
  let status = 'PASS';
  const body = {
    test_case_id: testCaseId,
    status,
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

// One simulated CI pipeline, exactly as the real reporter behaves:
// create run -> n sequential result POSTs -> complete run.
export function ciPipeline(nResults, label) {
  if (nResults > ingestCaseIDs.length) {
    fail(`RESULTS_PER_RUN=${nResults} exceeds the ingest pool of ${ingestCaseIDs.length} test cases`);
  }
  const token = tokenForVU();
  const run = createRun(token, `${label} VU${__VU} it${__ITER}`);
  if (!run || !run.id) {
    return; // create_run failed; the check already recorded it
  }
  for (let i = 0; i < nResults; i++) {
    addResult(token, run.id, resultBody(ingestCaseIDs[i], i));
  }
  completeRun(token, run.id);
}
