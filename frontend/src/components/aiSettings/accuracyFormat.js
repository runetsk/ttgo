// Pure derivation + formatting for the AI failure-analysis accuracy panel.
// No React, no network — the panel JSX stays dumb and the logic stays testable
// (this repo has no jsdom/testing-library, so pure helpers are where coverage lives).
// Consumer: frontend/src/components/AIFailureAnalysisSettings.jsx

// Shown wherever there is no honest number to print. A bucket with zero samples
// must NOT render as "0%" — that reads as "the AI got everything wrong" when it
// actually means "nobody has triaged one of these yet".
export const NO_VALUE = '—';

// The ladder is always these three rows, in this order, even when a bucket is
// still empty. Reading the descent high -> medium -> low is the entire point of
// the panel; a silently missing row would make a flat ladder look like a clean one.
export const CONFIDENCE_LEVELS = ['high', 'medium', 'low'];

const CONFIDENCE_LABELS = {
    high: 'High',
    medium: 'Medium',
    low: 'Low',
};

// toCount coerces a backend count to a non-negative integer. Anything missing or
// malformed becomes 0, which the callers then treat as "no samples".
function toCount(value) {
    return Number.isFinite(value) && value > 0 ? Math.trunc(value) : 0;
}

// formatPercent renders a 0..1 rate as a whole-percent string. Clamped so a
// malformed rate cannot print something like "1400%".
export function formatPercent(rate) {
    if (!Number.isFinite(rate)) return NO_VALUE;
    return `${Math.round(Math.min(Math.max(rate, 0), 1) * 100)}%`;
}

// sampleLabel pluralises the calibration-set size that a rate is based on. The
// count is what makes a rate readable — 100% of 2 results is not a signal.
export function sampleLabel(total) {
    const n = toCount(total);
    return `${n} triaged result${n === 1 ? '' : 's'}`;
}

// summarizeAccuracy shapes the headline. `hasData` is the empty-state switch: a
// missing report, a failed fetch and a report with zero calibration rows are all
// the same thing to the panel — there is nothing honest to show yet. This is the
// common case on day one, before anything has been genuinely triaged.
//
// The headline is direct-prediction agreement (spec B2): a dedup clone repeats its group
// representative's answer, so counting clones would weight one decision by its group size.
// When no row has a recorded provenance (decisions from before the split), it falls back to
// every triaged result and says so through `basis`.
export function summarizeAccuracy(report) {
    const total = toCount(report?.total);
    const agreed = toCount(report?.agreed);
    if (total <= 0) {
        return { hasData: false, basis: 'all', total: 0, agreed: 0, rateLabel: NO_VALUE, samples: sampleLabel(0) };
    }
    const directTotal = toCount(report?.direct_total);
    if (directTotal > 0) {
        const directAgreed = toCount(report?.direct_agreed);
        const rate = Number.isFinite(report?.direct_rate) ? report.direct_rate : directAgreed / directTotal;
        return { hasData: true, basis: 'direct', total, agreed, rateLabel: formatPercent(rate), samples: `${sampleLabel(directTotal)}, direct predictions` };
    }
    // Prefer the server's rate; fall back to the ratio only when it is absent or
    // malformed. total > 0 is guaranteed above, so this cannot divide by zero.
    const rate = Number.isFinite(report?.agreement_rate) ? report.agreement_rate : agreed / total;
    return { hasData: true, basis: 'all', total, agreed, rateLabel: formatPercent(rate), samples: sampleLabel(total) };
}

// The headline's name and its caveat (spec B2). Accepting a suggestion is recorded as agreeing
// with it, so the number measures how often people keep the AI's call, not ground truth.
export const AGREEMENT_LABEL = 'Human agreement';
export const AGREEMENT_TOOLTIP =
    "How often the person triaging kept the AI's suggested defect type. Accepting a suggestion counts as agreement, " +
    "so this measures how often people keep the AI's call, not whether the call was right. " +
    "Counted over direct predictions; results that copied their group's answer (clones) are shown separately.";

// splitLabel shapes the direct/clone split of one bucket (the totals or an engine rung). It is ''
// when nothing in the bucket has a recorded provenance, so legacy data shows no split rather
// than a misleading "0 direct".
export function splitLabel(bucket) {
    const direct = toCount(bucket?.direct_total);
    const clones = toCount(bucket?.clone_total);
    if (direct + clones === 0) return '';
    const parts = [];
    if (direct > 0) {
        const rate = Number.isFinite(bucket?.direct_rate) ? bucket.direct_rate : toCount(bucket?.direct_agreed) / direct;
        parts.push(`direct ${formatPercent(rate)} of ${direct}`);
    } else {
        parts.push('no direct predictions');
    }
    if (clones > 0) parts.push(`clones ${formatPercent(toCount(bucket?.clone_agreed) / clones)} of ${clones}`);
    return parts.join(' · ');
}

// unknownNote explains rows decided before the split existed: they are in the totals only.
export function unknownNote(report) {
    const n = toCount(report?.unknown_provenance);
    if (n === 0) return '';
    return `${n} decision${n === 1 ? '' : 's'} recorded before direct predictions and clones were told apart ${n === 1 ? 'is' : 'are'} counted in the totals only.`;
}

// coverageLabel renders one engine's coverage (spec B3) as
// "n decided of m · k abstained · j without explanation", plus failed attempts when there are any.
export function coverageLabel(c) {
    const analyses = toCount(c?.analyses);
    if (analyses === 0) return '';
    const parts = [
        `${toCount(c?.decided)} decided of ${analyses}`,
        `${toCount(c?.abstained)} abstained`,
        `${toCount(c?.no_explanation)} without explanation`,
    ];
    const failed = toCount(c?.failed);
    if (failed > 0) parts.push(`${failed} failed`);
    return parts.join(' · ');
}

// coverageFor finds the coverage of one analysis engine. Snapshot rungs such as
// 'typesafe-derived' are not analysis engines and have none of their own.
function coverageFor(report, engine) {
    const list = Array.isArray(report?.coverage) ? report.coverage : [];
    return coverageLabel(list.find((c) => c && c.engine === engine));
}

// coverageRows lists every engine that analysed in the window, for engines with no triaged rung.
export function coverageRows(report) {
    const list = Array.isArray(report?.coverage) ? report.coverage : [];
    return list
        .filter((c) => c && typeof c.engine === 'string' && toCount(c.analyses) > 0)
        .map((c) => ({ key: c.engine, label: ENGINE_LABELS[c.engine] || c.engine, text: coverageLabel(c) }));
}

// The policy-version dropdown (spec B4). '' is "All versions". The selected version stays listed
// even when the window no longer contains it, so the select never shows a value it cannot name.
export const ALL_VERSIONS = '';

export function policyOptions(report, selected = ALL_VERSIONS) {
    const versions = (Array.isArray(report?.policy_versions) ? report.policy_versions : [])
        .filter((v) => typeof v === 'string' && v !== '');
    if (selected && !versions.includes(selected)) versions.unshift(selected);
    return [{ value: ALL_VERSIONS, label: 'All versions' }, ...versions.map((v) => ({ value: v, label: v }))];
}

// shapeRow turns one (possibly absent) confidence bucket into a render-ready row.
function shapeRow(key, bucket) {
    const total = toCount(bucket?.total);
    const agreed = toCount(bucket?.agreed);
    const hasSamples = total > 0;
    const rate = hasSamples && Number.isFinite(bucket?.rate) ? bucket.rate : (hasSamples ? agreed / total : 0);
    return {
        key,
        label: CONFIDENCE_LABELS[key] || key || 'Unspecified',
        total,
        agreed,
        rate,
        rateLabel: hasSamples ? formatPercent(rate) : NO_VALUE,
        hasSamples,
        samples: sampleLabel(total),
    };
}

// confidenceRows shapes the calibration ladder: the three known levels in
// high -> medium -> low order, plus any unrecognised bucket the backend returned
// appended after them, so the rows always account for every result in the
// headline total rather than silently dropping some.
export function confidenceRows(report) {
    const buckets = Array.isArray(report?.by_confidence) ? report.by_confidence : [];
    const byKey = new Map();
    for (const b of buckets) {
        if (b && typeof b.confidence === 'string') byKey.set(b.confidence, b);
    }
    const extras = [...byKey.keys()].filter((k) => !CONFIDENCE_LEVELS.includes(k));
    return [...CONFIDENCE_LEVELS, ...extras].map((key) => shapeRow(key, byKey.get(key)));
}

// Verdicts the analyzer emits, rendered as human text. The mapping to defect_type is
// lossy (flaky_test and test_data both suggest automation_bug), which is exactly why
// the backend groups on the verdict — labelling by the mapped value would merge two
// distinct verdicts into one meaningless row.
const VERDICT_LABELS = {
    product_bug: 'Product bug',
    flaky_test: 'Flaky test',
    test_data: 'Test data',
    environment: 'Environment',
    infrastructure: 'Infrastructure',
    unknown: 'Unknown',
};

// verdictRows shapes the per-verdict breakdown. Unlike the confidence ladder there is no
// fixed row set: only verdicts the AI actually produced are shown, in the server's order
// (most samples first), because an empty row per unseen verdict would be noise rather than
// the missing rung a blank confidence level represents. Buckets with no samples are dropped
// for the same reason.
export function verdictRows(report) {
    const buckets = Array.isArray(report?.by_verdict) ? report.by_verdict : [];
    return buckets
        .filter((b) => b && typeof b.verdict === 'string' && toCount(b.total) > 0)
        .map((b) => ({
            ...shapeRow(b.verdict, b),
            label: VERDICT_LABELS[b.verdict] || b.verdict,
        }));
}

const ENGINE_LABELS = { typesafe: 'TypeSafe', 'typesafe-derived': 'TypeSafe (from verdict)', generative: 'LLM' };
// engineRows shapes one confidence ladder per engine. Ladders are per engine because generative
// confidence is self-reported by the LLM while TypeSafe's is calibrated on the defect-type
// question; one mixed ladder would make both unreadable. Engines with no samples are dropped.
// Each rung carries its direct/clone split and its engine's coverage.
export function engineRows(report) {
    const buckets = Array.isArray(report?.by_engine) ? report.by_engine : [];
    return buckets
        .filter((b) => b && typeof b.engine === 'string' && toCount(b.total) > 0)
        .map((b) => ({
            ...shapeRow(b.engine, b),
            label: ENGINE_LABELS[b.engine] || b.engine,
            rows: confidenceRows({ by_confidence: b.by_confidence }),
            split: splitLabel(b),
            coverage: coverageFor(report, b.engine),
        }));
}
