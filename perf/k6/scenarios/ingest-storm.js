// S1 — CI ingest storm (spec §5). Finds the write-path ceiling.
//
// MODE=vus (default): closed-loop ramp of concurrent simulated CI pipelines.
//   Each VU is one pipeline: create run -> RESULTS_PER_RUN result POSTs ->
//   complete. Ramp until latency/error thresholds break; the summary +
//   telemetry map WHERE it broke.
// MODE=rate: open-loop arrivals — PIPELINES_PER_MIN new pipelines start per
//   minute regardless of how slow the server gets (avoids coordinated
//   omission at the pipeline level). Use to measure sustained throughput at
//   a fixed load level once the ramp has bracketed the ceiling. MAX_VUS
//   defaults to the seeded token-pool size so VU scale-up can never break
//   the one-token-per-pipeline invariant; the dropped_iterations threshold
//   flags when k6 could not offer the requested load.
//
// Thresholds are observational: they mark the run failed but never abort it —
// the whole point is to keep ramping past the breaking point and map it. The
// capacity Make targets treat k6's threshold-failure exit (99) as expected.
import { ciPipeline, resultsPerRun, assertTokenPool, tokenPoolSize } from '../lib/api.js';

const RESULTS_PER_RUN = resultsPerRun(200);
const MODE = __ENV.MODE || 'vus';

const thresholds = {
  // Breaking-point definition from the spec: add-result p95 > 500ms or
  // error rate > 1% marks the run as over the line.
  'http_req_duration{op:add_result}': ['p(95)<500'],
  'http_req_duration{op:create_run}': ['p(95)<1000'],
  'http_req_duration{op:complete_run}': ['p(95)<1000'],
  http_req_failed: ['rate<0.01'],
};

let scenarios;
if (MODE === 'rate') {
  const maxVUs = assertTokenPool(__ENV.MAX_VUS ? Number(__ENV.MAX_VUS) : tokenPoolSize());
  scenarios = {
    pipeline_arrivals: {
      executor: 'constant-arrival-rate',
      rate: Number(__ENV.PIPELINES_PER_MIN || 6),
      timeUnit: '1m',
      duration: __ENV.RATE_DURATION || '10m',
      preAllocatedVUs: Math.min(50, maxVUs),
      maxVUs,
    },
  };
  // Nonzero = k6 ran out of VUs and offered LESS load than requested — the
  // summary would otherwise read healthier than the configured arrival rate.
  thresholds.dropped_iterations = ['count==0'];
} else {
  const stages = JSON.parse(
    __ENV.STAGES ||
      '[{"duration":"2m","target":10},{"duration":"3m","target":40},{"duration":"4m","target":100},{"duration":"1m","target":0}]'
  );
  assertTokenPool(Math.max(...stages.map((s) => s.target)));
  scenarios = {
    ci_pipelines: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages,
      gracefulStop: '90s',
    },
  };
}

export const options = {
  scenarios,
  thresholds,
  summaryTrendStats: ['avg', 'p(50)', 'p(90)', 'p(95)', 'p(99)', 'max'],
};

export default function () {
  ciPipeline(RESULTS_PER_RUN, 'perf-storm');
}
