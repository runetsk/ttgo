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

// applyResultDelta merges a result-level WS delta payload (run_id + run
// summary + affected rows / patch / deleted ids) into the previous run
// state from GET /runs/{id}. Returns prev untouched when the delta doesn't
// apply (no state yet, different run) — the initial fetch or the next
// event covers it.
export function applyResultDelta(prev, delta) {
    if (!prev || !delta || !delta.run || prev.id !== delta.run_id) return prev;

    let results = prev.run_results || [];
    if (Array.isArray(delta.results) && delta.results.length > 0) {
        const byId = new Map(results.map(r => [r.id, r]));
        for (const row of delta.results) {
            byId.set(row.id, { ...byId.get(row.id), ...row });
        }
        results = Array.from(byId.values());
    }
    if (Array.isArray(delta.result_ids) && delta.patch) {
        const ids = new Set(delta.result_ids);
        results = results.map(r => (ids.has(r.id) ? { ...r, ...delta.patch } : r));
    }
    if (Array.isArray(delta.deleted_result_ids) && delta.deleted_result_ids.length > 0) {
        const gone = new Set(delta.deleted_result_ids);
        results = results.filter(r => !gone.has(r.id));
    }
    return { ...prev, ...delta.run, run_results: results };
}
