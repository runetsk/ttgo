// Harness sanity check: tiny closed-loop load proving the seed manifest,
// token auth, the run lifecycle, and thresholds all work end to end.
// This is NOT a load test — it exists to fail fast when the harness breaks.
import { ciPipeline, resultsPerRun, assertTokenPool } from '../lib/api.js';
import { SMOKE_THRESHOLDS } from '../config/thresholds.js';
export { handleSummary } from '../lib/summary.js';

const VUS = 2;
const RESULTS_PER_RUN = resultsPerRun(20);
assertTokenPool(VUS);

export const options = {
  scenarios: {
    smoke: { executor: 'shared-iterations', vus: VUS, iterations: 4, maxDuration: '60s' },
  },
  thresholds: SMOKE_THRESHOLDS,
};

export default function () {
  ciPipeline(RESULTS_PER_RUN, 'perf-smoke');
}
