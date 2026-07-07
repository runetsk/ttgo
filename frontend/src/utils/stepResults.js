// Shared shape for manual per-step verdicts stored in RunResult.steps.
// Each entry: { order_index, action, expected_result, status, note }
// where status is 'PASS' | 'FAIL' | 'SKIP' | '' (unmarked). Consumed by the
// execution page (capture) and RunResultDetail (read-only render).

export function isManualStepResults(steps) {
    return Array.isArray(steps) && steps.length > 0 &&
        steps.every(s => s && typeof s === 'object' && 'order_index' in s && 'status' in s);
}

export function parseStepVerdicts(steps) {
    if (!isManualStepResults(steps)) return {};
    const map = {};
    for (const s of steps) {
        map[s.order_index] = { status: s.status || '', note: s.note || '' };
    }
    return map;
}

export function buildStepResults(authoredSteps, verdictMap) {
    return [...(authoredSteps || [])]
        .sort((a, b) => (a.order_index || 0) - (b.order_index || 0))
        .map(s => ({
            order_index: s.order_index || 0,
            action: s.action || '',
            expected_result: s.expected_result || '',
            status: verdictMap[s.order_index]?.status || '',
            note: verdictMap[s.order_index]?.note || '',
        }));
}
