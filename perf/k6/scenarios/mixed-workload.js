// S2 — dashboards under ingest (spec §5). Browsing VUs run the whole time;
// the ingest storm (constant arrival rate, default 75 pipelines/min x 200
// results = 250 results/s ≈ 50% of the measured S1 ceiling) joins at
// half-time. Compare read-op p95 between the quiet first half and the loaded
// second half — scripts/analyze.sh gives the per-window view; the delta is
// the finding.
//
// Browsing VUs pace themselves with 1s think time, so READ_VUS ≈ browse
// requests/sec. Ingest workers take tokens above the browser range; the pool
// must cover both (asserted at init).
import { sleep } from 'k6';
import { ciPipeline, resultsPerRun, assertTokenPool, tokenPoolSize, intKnob } from '../lib/api.js';
import { browseOnce } from '../lib/reads.js';
import { INGEST_THRESHOLDS, READ_THRESHOLDS } from '../config/thresholds.js';
import { MIXED_DEFAULTS } from '../config/workloads.js';
export { handleSummary } from '../lib/summary.js';

const RESULTS_PER_RUN = resultsPerRun(200);
const READ_VUS = intKnob('READ_VUS', MIXED_DEFAULTS.readVUs);
const PIPELINES_PER_MIN = intKnob('PIPELINES_PER_MIN', MIXED_DEFAULTS.pipelinesPerMin);
const PHASE_MINUTES = intKnob('PHASE_MINUTES', MIXED_DEFAULTS.phaseMinutes);
const MAX_VUS = intKnob('MAX_VUS', Math.max(1, tokenPoolSize() - READ_VUS));
assertTokenPool(READ_VUS + MAX_VUS);

export const options = {
  scenarios: {
    browsing: {
      executor: 'constant-vus',
      exec: 'browse',
      vus: READ_VUS,
      duration: `${2 * PHASE_MINUTES}m`,
    },
    ingest: {
      executor: 'constant-arrival-rate',
      exec: 'ingest',
      rate: PIPELINES_PER_MIN,
      timeUnit: '1m',
      startTime: `${PHASE_MINUTES}m`,
      duration: `${PHASE_MINUTES}m`,
      preAllocatedVUs: Math.min(50, MAX_VUS),
      maxVUs: MAX_VUS,
      gracefulStop: '90s',
    },
  },
  thresholds: {
    ...READ_THRESHOLDS,
    ...INGEST_THRESHOLDS,
    // Nonzero = k6 ran out of ingest VUs and offered LESS write load than
    // configured — the read-latency delta would be measured against a
    // lighter background than reported.
    dropped_iterations: ['count==0'],
  },
  summaryTrendStats: ['avg', 'p(50)', 'p(90)', 'p(95)', 'p(99)', 'max'],
};

export function browse() {
  browseOnce(__VU * 7919 + __ITER);
  sleep(1);
}

export function ingest() {
  ciPipeline(RESULTS_PER_RUN, 'perf-mixed');
}
