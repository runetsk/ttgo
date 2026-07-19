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
export function summarizeAccuracy(report) {
    const total = toCount(report?.total);
    const agreed = toCount(report?.agreed);
    if (total <= 0) {
        return { hasData: false, total: 0, agreed: 0, rateLabel: NO_VALUE, samples: sampleLabel(0) };
    }
    // Prefer the server's rate; fall back to the ratio only when it is absent or
    // malformed. total > 0 is guaranteed above, so this cannot divide by zero.
    const rate = Number.isFinite(report?.agreement_rate) ? report.agreement_rate : agreed / total;
    return { hasData: true, total, agreed, rateLabel: formatPercent(rate), samples: sampleLabel(total) };
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
