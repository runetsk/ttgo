// Maps a Playwright test-result status to a TTGO ExecutionStatus.
//
// Playwright result.status ∈ 'passed' | 'failed' | 'timedOut' | 'skipped' | 'interrupted'
// TTGO result status       ∈ 'PASS' | 'FAIL' | 'SKIP' | 'PENDING' | 'RUNNING' | 'ERROR'
const STATUS_MAP = {
    passed: 'PASS',
    failed: 'FAIL',
    timedOut: 'FAIL',
    // 'interrupted' = the run was aborted before this test finished. Map to ERROR
    // (not SKIP): the backend completes a run as FAIL only when the latest results
    // include FAIL or ERROR, so an all-SKIP (aborted) run would otherwise go green.
    interrupted: 'ERROR',
    skipped: 'SKIP',
};

export function mapStatus(playwrightStatus) {
    return STATUS_MAP[playwrightStatus] ?? 'FAIL';
}
