import { latestAttempts } from './runResults.js';

const isFail = (s) => s === 'FAIL' || s === 'ERROR';
const isPass = (s) => s === 'PASS';
const matchKey = (r) => (r.test_case_id ? `tc:${r.test_case_id}` : `nm:${r.test_name_snapshot || ''}`);

function summarize(latest) {
    let passed = 0, failed = 0, skipped = 0, durationMs = 0;
    for (const r of latest) {
        if (isPass(r.status)) passed++;
        else if (isFail(r.status)) failed++;
        else skipped++; // SKIP, PENDING, and any other non-pass/non-fail status
        durationMs += Number(r.duration_ms || 0);
    }
    const total = latest.length;
    return { total, passed, failed, skipped, passRate: total ? (passed / total) * 100 : 0, durationMs };
}

// compared -> this transition. stillFailing is checked before unchanged so a
// both-failing pair lands in stillFailing (not unchanged).
function classify(thisStatus, comparedStatus) {
    if (isPass(comparedStatus) && isFail(thisStatus)) return 'regressions';
    if (isFail(comparedStatus) && isPass(thisStatus)) return 'fixed';
    if (isFail(comparedStatus) && isFail(thisStatus)) return 'stillFailing';
    if (thisStatus === comparedStatus) return 'unchanged';
    return 'otherChanges';
}

const GROUP_ORDER = [
    ['regressions', 'Regressions'],
    ['fixed', 'Fixed'],
    ['stillFailing', 'Still failing'],
    ['otherChanges', 'Other changes'],
    ['unchanged', 'Unchanged'],
    ['onlyThis', 'Only in this run'],
    ['onlyCompared', 'Only in compared run'],
];

export function diffRuns(thisRun, comparedRun) {
    const thisLatest = latestAttempts(thisRun && thisRun.run_results);
    const comparedLatest = latestAttempts(comparedRun && comparedRun.run_results);

    const thisByKey = new Map(thisLatest.map((r) => [matchKey(r), r]));
    const comparedByKey = new Map(comparedLatest.map((r) => [matchKey(r), r]));

    const buckets = {
        regressions: [], fixed: [], stillFailing: [], otherChanges: [],
        unchanged: [], onlyThis: [], onlyCompared: [],
    };

    for (const [key, thisResult] of thisByKey) {
        const comparedResult = comparedByKey.get(key) || null;
        const name = thisResult.test_name_snapshot || (comparedResult && comparedResult.test_name_snapshot) || '(unnamed)';
        const row = { testCaseId: thisResult.test_case_id || null, name, thisResult, comparedResult };
        if (!comparedResult) buckets.onlyThis.push(row);
        else buckets[classify(thisResult.status, comparedResult.status)].push(row);
    }
    for (const [key, comparedResult] of comparedByKey) {
        if (thisByKey.has(key)) continue;
        buckets.onlyCompared.push({
            testCaseId: comparedResult.test_case_id || null,
            name: comparedResult.test_name_snapshot || '(unnamed)',
            thisResult: null,
            comparedResult,
        });
    }

    const groups = GROUP_ORDER.map(([key, label]) => ({ key, label, rows: buckets[key] }));
    const counts = {
        regressions: buckets.regressions.length,
        fixed: buckets.fixed.length,
        stillFailing: buckets.stillFailing.length,
        otherChanges: buckets.otherChanges.length,
        unchanged: buckets.unchanged.length,
        onlyThis: buckets.onlyThis.length,
        onlyCompared: buckets.onlyCompared.length,
        shared: thisLatest.length - buckets.onlyThis.length,
    };

    return {
        summary: {
            this: { ...summarize(thisLatest), name: (thisRun && thisRun.name) || 'This run', createdAt: thisRun && thisRun.created_at },
            compared: { ...summarize(comparedLatest), name: (comparedRun && comparedRun.name) || 'Compared run', createdAt: comparedRun && comparedRun.created_at },
            counts,
        },
        groups,
    };
}
