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
