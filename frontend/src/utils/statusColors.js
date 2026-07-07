// Canonical execution-status → color map. Single source of truth for run and
// run-result status dots, bars, and chart cells across the app. Values are the
// majority choice from the per-component maps this replaced; PASS/FAIL/ERROR
// match --accent-green/--accent-red. Raw hex (not CSS vars) so it renders
// identically in recharts SVG fills and inline styles, in both themes.
export const STATUS_COLORS = {
    PASS: '#22c55e',
    FAIL: '#ef4444',
    ERROR: '#ef4444',
    SKIP: '#9ca3af',
    PENDING: '#f59e0b',
    RUNNING: '#3b82f6',
};
