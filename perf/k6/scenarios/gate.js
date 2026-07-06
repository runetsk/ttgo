// S6 — regression gate (spec §5). A short, FIXED, moderate mixed profile on
// a freshly-reseeded small tier, with hard per-op thresholds derived from a
// committed baseline: breach => k6 exit 99 => `make gate` fails. "Did this
// change make it slower?" as a command.
//
//   make -C perf gate-baseline   # (re)measure, write perf/baselines/gate-small.json — commit it
//   make -C perf gate            # enforce: per-op p95 < baseline x 1.3 (floor 25 ms)
//
// Re-baseline (and commit) after intentional perf changes or hardware
// changes. Baselines are machine-specific: the gate refuses a baseline
// recorded on different hardware unless GATE_IGNORE_MACHINE=1.
import { sleep } from 'k6';
import { ciPipeline, resultsPerRun, assertTokenPool, tokenPoolSize, intKnob } from '../lib/api.js';
import { browseOnce } from '../lib/reads.js';
import { summaryFiles } from '../lib/summary.js';

const GATE_OPS = [
  'create_run', 'add_result', 'complete_run',
  'runs_list', 'run_detail', 'analytics_summary',
  'analytics_trend', 'analytics_flaky', 'search',
];
const HEADROOM = 1.3;
const FLOOR_MS = 25;

const WRITE_BASELINE = __ENV.WRITE_BASELINE === '1';
const BASELINE_PATH = __ENV.TTGO_BASELINE_PATH;
if (!BASELINE_PATH) {
  throw new Error('TTGO_BASELINE_PATH not set — run via `make -C perf gate` / `make -C perf gate-baseline`');
}

// Failsafes apply in BOTH modes: a baseline measured over errors or dropped
// load would gate future runs against garbage.
const thresholds = {
  http_req_failed: ['rate<0.01'],
  checks: ['rate>0.99'],
  dropped_iterations: ['count==0'],
};
// Anti-vacuous guard (Codex review 2026-07-06, verified on k6 v2.1.0): a
// tag-scoped duration threshold over ZERO samples reports p(95)=0 and
// passes — the gate could "pass" an op it never exercised, and a baseline
// could record 0. Requiring per-op request counts makes silence a failure.
for (const op of GATE_OPS) {
  thresholds[`http_reqs{op:${op}}`] = ['count>0'];
}

if (!WRITE_BASELINE) {
  let baseline;
  try {
    baseline = JSON.parse(open(BASELINE_PATH));
  } catch (_) {
    throw new Error(
      `no readable baseline at ${BASELINE_PATH} — run \`make -C perf gate-baseline\` first and commit the file`
    );
  }
  const machine = __ENV.TTGO_MACHINE || '';
  if (baseline.machine !== machine && __ENV.GATE_IGNORE_MACHINE !== '1') {
    throw new Error(
      `baseline machine "${baseline.machine}" != this machine "${machine}" — re-baseline, or GATE_IGNORE_MACHINE=1 to override`
    );
  }
  for (const op of GATE_OPS) {
    const p95 = baseline.ops && baseline.ops[op];
    if (typeof p95 !== 'number') {
      throw new Error(`baseline missing op "${op}" — re-run \`make -C perf gate-baseline\``);
    }
    thresholds[`http_req_duration{op:${op}}`] = [`p(95)<${Math.max(Math.ceil(p95 * HEADROOM), FLOOR_MS)}`];
  }
} else {
  // k6 only materializes a tag-scoped submetric (e.g. http_req_duration{op:x})
  // in data.metrics if something references it — normally a real threshold.
  // A baseline run has no breach bound to enforce yet, so register each op
  // with an always-true threshold purely to make handleSummary's per-op
  // p(95) readout exist; it can never fail a run.
  for (const op of GATE_OPS) {
    thresholds[`http_req_duration{op:${op}}`] = ['p(95)>=0'];
  }
}

const READ_VUS = intKnob('READ_VUS', 10);
const PIPELINES_PER_MIN = intKnob('PIPELINES_PER_MIN', 25);
const RESULTS_PER_RUN = resultsPerRun(200);
const MAX_VUS = intKnob('MAX_VUS', Math.max(1, tokenPoolSize() - READ_VUS));
assertTokenPool(READ_VUS + MAX_VUS);

export const options = {
  scenarios: {
    browsing: { executor: 'constant-vus', exec: 'browse', vus: READ_VUS, duration: '3m' },
    ingest: {
      executor: 'constant-arrival-rate', exec: 'ingest',
      rate: PIPELINES_PER_MIN, timeUnit: '1m', duration: '3m',
      preAllocatedVUs: Math.min(20, MAX_VUS), maxVUs: MAX_VUS, gracefulStop: '90s',
    },
  },
  thresholds,
  summaryTrendStats: ['avg', 'p(50)', 'p(90)', 'p(95)', 'p(99)', 'max'],
};

export function browse() {
  browseOnce(__VU * 7919 + __ITER);
  sleep(1);
}

export function ingest() {
  ciPipeline(RESULTS_PER_RUN, 'perf-gate');
}

export function handleSummary(data) {
  const files = summaryFiles(data);
  if (WRITE_BASELINE) {
    const ops = {};
    for (const op of GATE_OPS) {
      const m = data.metrics[`http_req_duration{op:${op}}`];
      const reqs = data.metrics[`http_reqs{op:${op}}`];
      const count = reqs && reqs.values && reqs.values.count;
      if (!m || !count) {
        throw new Error(`baseline run produced no samples for op "${op}" — profile too short or mix broken`);
      }
      ops[op] = m.values['p(95)'];
    }
    files[BASELINE_PATH] = JSON.stringify(
      {
        machine: __ENV.TTGO_MACHINE || '',
        tier: __ENV.TTGO_TIER || 'small',
        derivation: `gate thresholds = p95 x ${HEADROOM}, floor ${FLOOR_MS}ms`,
        ops,
      },
      null, 2
    );
  }
  return files;
}
