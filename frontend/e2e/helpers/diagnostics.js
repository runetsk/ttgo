// Forwards browser console + pageerror output to the test runner's stdout.
// No-op unless E2E_DEBUG is set, so it can run as an always-on fixture without
// spamming normal runs (replaces the per-spec console-logging beforeEach).
export function attachConsoleLogging(page) {
    if (!process.env.E2E_DEBUG) return;
    page.on('console', (msg) => console.log(`[browser:${msg.type()}] ${msg.text()}`));
    page.on('pageerror', (err) => console.log(`[browser:error] ${err.message}`));
}

// Collects pageerror messages into a live array the test can assert on (for
// specs that verify a page renders with zero uncaught errors).
export function collectPageErrors(page) {
    const errors = [];
    page.on('pageerror', (err) => errors.push(err.message));
    return errors;
}

// Counts requests whose URL matches `pattern` (substring or RegExp) into a live
// { count } object — used to prove a deep-link reload triggers no navigation loop.
export function countRequests(page, pattern) {
    const state = { count: 0 };
    const matches = (url) => (typeof pattern === 'string' ? url.includes(pattern) : pattern.test(url));
    page.on('request', (req) => { if (matches(req.url())) state.count += 1; });
    return state;
}
