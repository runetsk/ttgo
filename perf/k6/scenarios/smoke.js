// Harness sanity check: tiny closed-loop load proving the seed manifest,
// token auth, the run lifecycle, and thresholds all work end to end.
// This is NOT a load test — it exists to fail fast when the harness breaks.
import { ciPipeline, resultsPerRun, assertTokenPool } from '../lib/api.js';

const VUS = 2;
const RESULTS_PER_RUN = resultsPerRun(20);
assertTokenPool(VUS);

export const options = {
  scenarios: {
    smoke: { executor: 'shared-iterations', vus: VUS, iterations: 4, maxDuration: '60s' },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    checks: ['rate>0.99'],
    'http_req_duration{op:add_result}': ['p(95)<1000'],
  },
};

export default function () {
  ciPipeline(RESULTS_PER_RUN, 'perf-smoke');
}
