// Workload profiles, separated from scenario logic. Values are defaults —
// every scenario still honors its env knobs (STAGES, READ_VUS, ...).

// S1 closed-loop ramp: 10 minutes to 100 VUs (first mapped 2026-07-04:
// p95 knee ~55 pipelines, plateau ~510-525 results/s, first 5xx ~135).
export const DEFAULT_CAPACITY_STAGES = [
  { duration: '2m', target: 10 },
  { duration: '3m', target: 40 },
  { duration: '4m', target: 100 },
  { duration: '1m', target: 0 },
];

// S2: background ingest at ~50% of the measured S1 ceiling —
// 75 pipelines/min x 200 results = 250 results/s — under 20 browsing VUs.
export const MIXED_DEFAULTS = { readVUs: 20, pipelinesPerMin: 75, phaseMinutes: 4 };

// S3: low, fixed read work per tier; the cross-tier p95 curve is the finding.
export const SCALING_DEFAULTS = { vus: 5, iterations: 2000, maxDuration: '20m' };
