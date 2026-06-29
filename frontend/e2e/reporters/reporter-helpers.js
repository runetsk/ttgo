import { mapStatus } from './map-status.js';

// Builds a stable, human-readable TTGO test-case name from a Playwright test.
// titlePath() = ['', '<project>', '<file rel to rootDir>', ...describes, '<title>'].
// Dropping the leading '' (filter) and the project segment (slice) yields
// '<file> › <describe…> › <title>' — unique across the suite and readable in TTGO.
export function testCaseName(test) {
    return test.titlePath().filter(Boolean).slice(1).join(' › ');
}

function truncate(s, max = 4000) {
    if (!s) return '';
    return s.length > max ? `${s.slice(0, max)}\n…[truncated]` : s;
}

// Builds the POST /runs/:id/results body for one finished Playwright test.
export function buildResultBody({ result, testCaseId, name, environment = 'e2e', browser = 'chromium' }) {
    const status = mapStatus(result.status);
    const durationMs = Math.round(result.duration ?? 0);

    const body = {
        test_case_id: testCaseId,
        status,
        test_name_snapshot: name,
        attempt_number: (result.retry ?? 0) + 1,
        duration_ms: durationMs,
        environment,
        browser,
        os: process.platform,
    };

    if (result.startTime) {
        const start = new Date(result.startTime);
        body.start_time = start.toISOString();
        body.end_time = new Date(start.getTime() + durationMs).toISOString();
    }

    // Attach failure detail for FAIL and ERROR (ERROR = an interrupted/aborted test).
    if (status === 'FAIL' || status === 'ERROR') {
        const err = result.error ?? (result.errors && result.errors[0]) ?? {};
        body.error_message = truncate(err.message ?? '');
        body.stack_trace = truncate(err.stack ?? '');
        body.failure_type = result.status; // 'failed' | 'timedOut' | 'interrupted'
    }

    // The create endpoint copies defect_type verbatim — it does NOT apply the
    // backend's FAIL→'to_investigate' default (that lives only on update/bulk-update).
    // Mirror it here so failed results are classified consistently with the UI.
    if (status === 'FAIL') {
        body.defect_type = 'to_investigate';
    }

    return body;
}
