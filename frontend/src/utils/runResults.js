// latestAttempts reduces a run's results to one entry per test case (highest
// attempt_number), keeping orphan results (no test_case_id) as-is. Mirrors the
// derivation in TestRunDetail so the run page and the comparison agree.
export function latestAttempts(runResults) {
    const list = Array.isArray(runResults) ? runResults : [];
    const byTestCase = {};
    const orphans = [];
    for (const rr of list) {
        if (!rr.test_case_id) { orphans.push(rr); continue; }
        (byTestCase[rr.test_case_id] = byTestCase[rr.test_case_id] || []).push(rr);
    }
    const latest = [...orphans];
    for (const id in byTestCase) {
        byTestCase[id].sort((a, b) => (b.attempt_number || 0) - (a.attempt_number || 0));
        latest.push(byTestCase[id][0]);
    }
    return latest;
}
