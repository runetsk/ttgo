// S3 — dataset scaling (spec §5). Fixed read work at low concurrency, run
// once per seeded tier; the per-endpoint p95 curve across tiers is the
// finding. Super-linear growth flags missing indexes or full scans.
//
//   make -C perf dataset                 # current TIER (default small)
//   TIER=medium make -C perf dataset     # auto-reseeds if the tier switched
//   TIER=large make -C perf dataset     # first large seed: ~1-3 min, ~2 GB disk
//
// No think time: this measures endpoint latency at fixed low concurrency,
// not user behavior. Thresholds are observational; the Make target tolerates
// k6 exit 99 because a slow tier is a valid measurement, not a failure.
import { assertTokenPool, intKnob } from '../lib/api.js';
import { browseOnce } from '../lib/reads.js';
import { READ_THRESHOLDS } from '../config/thresholds.js';
import { SCALING_DEFAULTS } from '../config/workloads.js';

const READ_VUS = intKnob('READ_VUS', SCALING_DEFAULTS.vus);
const ITERATIONS = intKnob('ITERATIONS', SCALING_DEFAULTS.iterations);
assertTokenPool(READ_VUS);

export const options = {
  scenarios: {
    reads: {
      executor: 'shared-iterations',
      vus: READ_VUS,
      iterations: ITERATIONS,
      maxDuration: SCALING_DEFAULTS.maxDuration,
    },
  },
  thresholds: READ_THRESHOLDS,
  summaryTrendStats: ['avg', 'p(50)', 'p(90)', 'p(95)', 'p(99)', 'max'],
};

export default function () {
  // Spread deterministic indices across VUs so the mix does not correlate
  // with VU identity (7919 is prime, coprime with the 100-bucket roll).
  browseOnce(__VU * 7919 + __ITER);
}
