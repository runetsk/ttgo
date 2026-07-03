// Harness sanity check: tiny closed-loop load proving the seed manifest,
// token auth, the run lifecycle, and thresholds all work end to end.
// This is NOT a load test — it exists to fail fast when the harness breaks.
import { ciPipeline } from '../lib/api.js';

export const options = {
  scenarios: {
    smoke: { executor: 'shared-iterations', vus: 2, iterations: 4, maxDuration: '60s' },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    checks: ['rate>0.99'],
    'http_req_duration{op:add_result}': ['p(95)<1000'],
  },
};

export default function () {
  ciPipeline(Number(__ENV.RESULTS_PER_RUN || 20), 'perf-smoke');
}
