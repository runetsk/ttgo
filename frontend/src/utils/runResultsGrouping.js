// Pure client-side grouping for run results. No React, no network.
// Consumers: frontend/src/pages/TestRunDetail.jsx

const STATUS_ORDER      = ['FAIL', 'ERROR', 'PASS', 'SKIP', 'PENDING'];
const AI_VERDICT_ORDER  = ['product_bug', 'flaky_test', 'environment', 'test_data', 'infrastructure', 'unknown'];
const DEFECT_TYPE_ORDER = ['product_bug', 'automation_bug', 'system_issue', 'to_investigate'];

const STATUS_LABELS = {
    FAIL: 'FAIL', ERROR: 'ERROR', PASS: 'PASS', SKIP: 'SKIP', PENDING: 'PENDING',
};
const STATUS_COLORS = {
    FAIL:    { bg: '#fef2f2', pill: '#dc2626' },
    ERROR:   { bg: '#fef2f2', pill: '#b91c1c' },
    PASS:    { bg: '#f0fdf4', pill: '#16a34a' },
    SKIP:    { bg: '#f9fafb', pill: '#6b7280' },
    PENDING: { bg: '#fffbeb', pill: '#d97706' },
};

const AI_VERDICT_LABELS = {
    product_bug: 'Product bug', flaky_test: 'Flaky', environment: 'Environment',
    test_data: 'Test data', infrastructure: 'Infrastructure', unknown: 'Unknown',
};

const DEFECT_TYPE_LABELS = {
    product_bug: 'Product bug', automation_bug: 'Automation bug',
    system_issue: 'System issue', to_investigate: 'To investigate',
};

const NEUTRAL_COLOR = { bg: '#f9fafb', pill: '#4b5563' };

export function errorSignature(errorMessage) {
    if (!errorMessage || typeof errorMessage !== 'string') return null;
    const firstLine = errorMessage.split('\n').map(l => l.trim()).find(l => l.length > 0);
    if (!firstLine) return null;
    let sig = firstLine.replace(/^(Error|Exception|AssertionError):\s*/i, '');
    sig = sig.replace(/\s+/g, ' ').trim().toLowerCase();
    if (sig.length > 120) sig = sig.slice(0, 120);
    return sig || null;
}

function keyFor(dimension, result, currentAnalyses) {
    switch (dimension) {
        case 'status':        return result.status || null;
        case 'ai_verdict':    return currentAnalyses?.[result.id]?.verdict || null;
        case 'defect_type':   return result.defect_type || null;
        case 'error_signature': return errorSignature(result.error_message);
        case 'failure_type':  return result.failure_type || null;
        case 'environment':   return result.environment || null;
        default:              return null;
    }
}

function labelFor(dimension, key) {
    if (key === null) return getGroupConfig(dimension).emptyLabel;
    switch (dimension) {
        case 'status':      return STATUS_LABELS[key] || key;
        case 'ai_verdict':  return AI_VERDICT_LABELS[key] || key;
        case 'defect_type': return DEFECT_TYPE_LABELS[key] || key;
        default:            return key;
    }
}

function colorFor(dimension, key) {
    if (key === null) return NEUTRAL_COLOR;
    if (dimension === 'status') return STATUS_COLORS[key] || NEUTRAL_COLOR;
    return NEUTRAL_COLOR;
}

function orderedKeys(dimension, presentKeys) {
    const present = new Set(presentKeys);
    const hasEmpty = present.has(null);
    present.delete(null);
    let ordered = [];
    const fixed = {
        status: STATUS_ORDER,
        ai_verdict: AI_VERDICT_ORDER,
        defect_type: DEFECT_TYPE_ORDER,
    }[dimension];
    if (fixed) {
        for (const k of fixed) if (present.has(k)) { ordered.push(k); present.delete(k); }
        ordered = ordered.concat([...present].sort());
    } else {
        ordered = [...present].sort();
    }
    if (hasEmpty) ordered.push(null);
    return ordered;
}

function groupSummary(dimension, rows, currentAnalyses) {
    const durations = rows.map(r => Number(r.duration_ms || 0)).filter(n => n > 0);
    const avg = durations.length ? Math.round(durations.reduce((a, b) => a + b, 0) / durations.length) : 0;
    const avgStr = avg ? (avg < 1000 ? `${avg}ms` : `${(avg / 1000).toFixed(1)}s`) : null;
    const parts = [];
    if (avgStr) parts.push(`avg ${avgStr}`);
    if (dimension !== 'ai_verdict' && currentAnalyses) {
        const withVerdict = rows.filter(r => currentAnalyses[r.id]?.verdict).length;
        if (withVerdict > 0) parts.push(`${withVerdict} with AI verdict`);
    }
    return parts.join(' · ');
}

export function getGroupConfig(dimension) {
    const configs = {
        status:          { emptyLabel: '— Unknown —' },
        ai_verdict:      { emptyLabel: '— Unanalyzed —' },
        defect_type:     { emptyLabel: '— None —' },
        error_signature: { emptyLabel: '— No error —' },
        failure_type:    { emptyLabel: '— None —' },
        environment:     { emptyLabel: '— None —' },
    };
    return configs[dimension] || { emptyLabel: '— None —' };
}

export function groupResults(results, dimension, currentAnalyses) {
    if (!Array.isArray(results) || results.length === 0) return [];
    const buckets = new Map();
    for (const r of results) {
        const k = keyFor(dimension, r, currentAnalyses);
        if (!buckets.has(k)) buckets.set(k, []);
        buckets.get(k).push(r);
    }
    const keys = orderedKeys(dimension, [...buckets.keys()]);
    return keys.map(key => ({
        key: key === null ? '__empty__' : `${dimension}:${key}`,
        rawKey: key,
        label: labelFor(dimension, key),
        color: colorFor(dimension, key),
        summary: groupSummary(dimension, buckets.get(key), currentAnalyses),
        rows: buckets.get(key),
    }));
}

export const GROUP_DIMENSIONS = [
    { value: 'status',          label: 'Status' },
    { value: 'ai_verdict',      label: 'AI Verdict' },
    { value: 'defect_type',     label: 'Defect Type' },
    { value: 'error_signature', label: 'Error signature' },
    { value: 'failure_type',    label: 'Failure Type' },
    { value: 'environment',     label: 'Environment' },
];
