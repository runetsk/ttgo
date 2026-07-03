// S1 — CI ingest storm (spec §5). Finds the write-path ceiling.
//
// MODE=vus (default): closed-loop ramp of concurrent simulated CI pipelines.
//   Each VU is one pipeline: create run -> RESULTS_PER_RUN result POSTs ->
//   complete. Ramp until latency/error thresholds break; the summary +
//   telemetry map WHERE it broke.
// MODE=rate: open-loop arrivals — PIPELINES_PER_MIN new pipelines start per
//   minute regardless of how slow the server gets (avoids coordinated
//   omission at the pipeline level). Use to measure sustained throughput at
//   a fixed load level once the ramp has bracketed the ceiling.
//
// Thresholds are observational: they mark the run failed but never abort it —
// the whole point is to keep ramping past the breaking point and map it.
import { ciPipeline } from '../lib/api.js';

const RESULTS_PER_RUN = Number(__ENV.RESULTS_PER_RUN || 200);
const MODE = __ENV.MODE || 'vus';

const scenarios =
  MODE === 'rate'
    ? {
        pipeline_arrivals: {
          executor: 'constant-arrival-rate',
          rate: Number(__ENV.PIPELINES_PER_MIN || 6),
          timeUnit: '1m',
          duration: __ENV.RATE_DURATION || '10m',
          preAllocatedVUs: 50,
          maxVUs: 200,
        },
      }
    : {
        ci_pipelines: {
          executor: 'ramping-vus',
          startVUs: 0,
          stages: JSON.parse(
            __ENV.STAGES ||
              '[{"duration":"2m","target":10},{"duration":"3m","target":40},{"duration":"4m","target":100},{"duration":"1m","target":0}]'
          ),
          gracefulStop: '90s',
        },
      };

export const options = {
  scenarios,
  thresholds: {
    // Breaking-point definition from the spec: add-result p95 > 500ms or
    // error rate > 1% marks the run as over the line.
    'http_req_duration{op:add_result}': ['p(95)<500'],
    'http_req_duration{op:create_run}': ['p(95)<1000'],
    'http_req_duration{op:complete_run}': ['p(95)<1000'],
    http_req_failed: ['rate<0.01'],
  },
  summaryTrendStats: ['avg', 'p(50)', 'p(90)', 'p(95)', 'p(99)', 'max'],
};

export default function () {
  ciPipeline(RESULTS_PER_RUN, 'perf-storm');
}
