// Shared threshold sets. Thresholds are configuration, not per-scenario
// copy-paste — scenarios import the set matching their scope and may extend
// it (e.g. rate mode adds dropped_iterations). The 500ms/1% values are the
// breaking-point definition from the design spec §6; capacity-style runs
// treat a breach as the expected mapping outcome (k6 exit 99), smoke does not.

export const INGEST_THRESHOLDS = {
  'http_req_duration{op:add_result}': ['p(95)<500'],
  'http_req_duration{op:create_run}': ['p(95)<1000'],
  'http_req_duration{op:complete_run}': ['p(95)<1000'],
  http_req_failed: ['rate<0.01'],
};

export const READ_THRESHOLDS = {
  'http_req_duration{op:runs_list}': ['p(95)<500'],
  'http_req_duration{op:run_detail}': ['p(95)<500'],
  'http_req_duration{op:analytics_summary}': ['p(95)<500'],
  'http_req_duration{op:analytics_trend}': ['p(95)<500'],
  'http_req_duration{op:analytics_flaky}': ['p(95)<500'],
  'http_req_duration{op:search}': ['p(95)<500'],
  http_req_failed: ['rate<0.01'],
};

export const SMOKE_THRESHOLDS = {
  http_req_failed: ['rate<0.01'],
  checks: ['rate>0.99'],
  'http_req_duration{op:add_result}': ['p(95)<1000'],
};
