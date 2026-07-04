// Weighted read-path operations for S2/S3 (spec §5): what a user browsing the
// dashboard does while CI reports — heavy on runs list/detail, lighter on
// analytics and FTS search. Every request is tagged op:<name> so thresholds
// and summaries stay per-endpoint, never blended.
import http from 'k6/http';
import { check } from 'k6';
import { SharedArray } from 'k6/data';
import { BASE_URL, MANIFEST_PATH, tokenForVU } from './api.js';

// Detail views need real run IDs. They come from the manifest (deterministic,
// newest-first) rather than parsing the runs-list response every iteration —
// per-iteration JSON.parse of list bodies would burn load-generator CPU on
// the machine the measured server shares.
const runIDs = new SharedArray('perf historical run ids', () => {
  const ids = JSON.parse(open(MANIFEST_PATH)).historical_run_ids || [];
  if (ids.length === 0) {
    throw new Error(
      'seed manifest has no historical_run_ids — reseed with RESEED=1 (older manifests predate this field)'
    );
  }
  return ids;
});

// Terms chosen to hit the seeded corpus: error messages contain "Assertion",
// failure types are "assertion"/"timeout", descriptions contain
// "workflow"/"boundary"/"regression".
const SEARCH_TERMS = ['assertion', 'timeout', 'regression', 'boundary', 'workflow'];

function get(path, op, token) {
  const res = http.get(`${BASE_URL}/api${path}`, {
    headers: { Authorization: `Bearer ${token}` },
    tags: { op },
  });
  check(res, { [`${op} 2xx`]: (r) => r.status >= 200 && r.status < 300 }, { op });
  return res;
}

// Weights sum to 100. Deterministic by index (same multiplier as the ingest
// payloads in api.js) — no Math.random, so runs stay comparable.
const MIX = [
  [30, (t, i) => get('/runs?limit=20&offset=0', 'runs_list', t)],
  [30, (t, i) => get(`/runs/${runIDs[i % runIDs.length]}`, 'run_detail', t)],
  [15, (t, i) => get('/analytics/summary', 'analytics_summary', t)],
  [10, (t, i) => get('/analytics/trend?days=30', 'analytics_trend', t)],
  [10, (t, i) => get(`/search?q=${SEARCH_TERMS[i % SEARCH_TERMS.length]}&limit=20`, 'search', t)],
  [5, (t, i) => get('/analytics/flaky?lookback=10&limit=20', 'analytics_flaky', t)],
];

// One browse action, weight-dispatched from a deterministic hash of i.
export function browseOnce(i) {
  const token = tokenForVU();
  const roll = (i * 2654435761) % 100;
  let acc = 0;
  for (const [weight, fn] of MIX) {
    acc += weight;
    if (roll < acc) {
      fn(token, i);
      return;
    }
  }
}
