// Central e2e target.
//
// One env var (TTGO_E2E_BASE_URL) drives both the browser baseURL and the REST
// base, so the same suite runs against:
//   - the Docker stack  → nginx on :80 serves the SPA and proxies /api → backend
//   - a local dev server → vite on :5173 serves the SPA and proxies /api → backend
// In both cases the API is reached on the SAME origin as the app, via that
// origin's /api proxy — so no per-file host/port is hardcoded.
//
// Default: the Docker stack on :80. For local dev:
//   TTGO_E2E_BASE_URL=http://localhost:5173 npx playwright test
export const BASE_URL = process.env.TTGO_E2E_BASE_URL || 'http://localhost';
export const API_URL = `${BASE_URL}/api`;

// ── Routes ────────────────────────────────────────────────────────────────────
// SPA paths, resolved by the browser against BASE_URL. Static entries plus
// builders for the parameterized routes (param names mirror App.jsx: runId,
// testId, reqId).
export const ROUTES = {
    HOME: '/',
    RUNS: '/runs',
    CATEGORIES: '/categories',
    REQUIREMENTS: '/requirements',
    SETTINGS: '/settings',
    ANALYTICS: '/analytics',
    DEFECTS: '/defects',
};

export const runDetail = (runId) => `/runs/run/${runId}`;
export const runExecute = (runId) => `/runs/run/${runId}/execute`;
export const runCompare = (runId, compareWithId) => `/runs/run/${runId}?compareWith=${compareWithId}`;
export const testCase = (testId) => `/library/tests/${testId}`;
export const requirement = (reqId) => `/requirements/${reqId}`;
export const runFolder = (runFolderId) => `/runs/folders/${runFolderId}`;
export const libraryFolder = (folderId) => `/library/folders/${folderId}`;
// The register's only URL param — the deep link a test case's linked-bugs panel
// builds (utils/bugs.js bugHref). It is a landing instruction, not a filter, and
// the page deliberately leaves it in the URL.
export const defectsFocus = (defectId) => `/defects?focus=${defectId}`;

// ── Timeouts ──────────────────────────────────────────────────────────────────
// Named buckets so a spec picks intent, not a magic number. The suite-wide
// per-test and expect timeouts live in playwright.config.js; these are the
// per-assertion overrides for surfaces that render slower than the default.
export const TIMEOUTS = {
    UI_SETTLE: 5000,      // toggle on an already-rendered list, negative assertion, inline widget
    ELEMENT: 10000,       // default element / tree-node visibility after a nav or action
    APP_RENDER: 15000,    // app + sidebar first paint; list reconcile after a bulk mutation
    HEAVY_GRID: 30000,    // run-detail results / compare grids after navigating in
    AI_LIFECYCLE: 45000,  // test.setTimeout budget for the AI generate/regenerate lifecycle
};

// Hard sleeps. Prefer a real wait (waitForResponse / toBeVisible) over these;
// the ones that remain are deliberate observation windows, not settle hacks.
export const SLEEPS = {
    SEARCH_DEBOUNCE: 400,   // mirrors the app's search-input debounce before results settle
    PAGEERROR_OBSERVE: 500, // window to collect pageerror events and assert none fired
    LOOP_OBSERVE: 2000,     // window to confirm a deep-link reload triggers no navigation loop
};
