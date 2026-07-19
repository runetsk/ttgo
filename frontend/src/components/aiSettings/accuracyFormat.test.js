import test from 'node:test';
import assert from 'node:assert/strict';
import {
    NO_VALUE,
    CONFIDENCE_LEVELS,
    formatPercent,
    sampleLabel,
    summarizeAccuracy,
    confidenceRows,
    verdictRows,
} from './accuracyFormat.js';

// ── formatPercent ────────────────────────────────────────────────────────

test('formatPercent renders a 0..1 rate as whole percent', () => {
    assert.equal(formatPercent(0), '0%');
    assert.equal(formatPercent(1), '100%');
    assert.equal(formatPercent(0.6923), '69%');
    assert.equal(formatPercent(0.435), '44%');
});

test('formatPercent fails safe on non-numeric input', () => {
    assert.equal(formatPercent(undefined), NO_VALUE);
    assert.equal(formatPercent(null), NO_VALUE);
    assert.equal(formatPercent('0.5'), NO_VALUE);
    assert.equal(formatPercent(NaN), NO_VALUE);
    assert.equal(formatPercent(Infinity), NO_VALUE);
});

test('formatPercent clamps an out-of-range rate', () => {
    assert.equal(formatPercent(14), '100%');
    assert.equal(formatPercent(-2), '0%');
});

// ── sampleLabel ──────────────────────────────────────────────────────────

test('sampleLabel pluralises and floors at zero', () => {
    assert.equal(sampleLabel(0), '0 triaged results');
    assert.equal(sampleLabel(1), '1 triaged result');
    assert.equal(sampleLabel(24), '24 triaged results');
    assert.equal(sampleLabel(undefined), '0 triaged results');
    assert.equal(sampleLabel(-3), '0 triaged results');
});

// ── summarizeAccuracy: the empty state (the day-one case) ────────────────

test('summarizeAccuracy reports no data for a missing or failed report', () => {
    for (const input of [undefined, null, {}]) {
        const got = summarizeAccuracy(input);
        assert.equal(got.hasData, false);
        assert.equal(got.total, 0);
        assert.equal(got.rateLabel, NO_VALUE, 'never renders 0% when there is nothing to measure');
    }
});

test('summarizeAccuracy reports no data for a zero-sample report', () => {
    const got = summarizeAccuracy({ total: 0, agreed: 0, agreement_rate: 0, by_confidence: [] });
    assert.equal(got.hasData, false);
    assert.equal(got.rateLabel, NO_VALUE);
    assert.equal(got.samples, '0 triaged results');
});

// ── summarizeAccuracy: populated ─────────────────────────────────────────

test('summarizeAccuracy formats the headline from a populated report', () => {
    const got = summarizeAccuracy({ total: 26, agreed: 18, agreement_rate: 18 / 26 });
    assert.equal(got.hasData, true);
    assert.equal(got.total, 26);
    assert.equal(got.agreed, 18);
    assert.equal(got.rateLabel, '69%');
    assert.equal(got.samples, '26 triaged results');
});

test('summarizeAccuracy derives the rate when the server omits it', () => {
    const got = summarizeAccuracy({ total: 4, agreed: 1 });
    assert.equal(got.hasData, true);
    assert.equal(got.rateLabel, '25%');
});

test('summarizeAccuracy handles a total with zero agreements', () => {
    const got = summarizeAccuracy({ total: 3, agreed: 0, agreement_rate: 0 });
    assert.equal(got.hasData, true, '0% agreement over real samples is still data');
    assert.equal(got.rateLabel, '0%');
});

// ── confidenceRows: the ladder ───────────────────────────────────────────

test('confidenceRows always returns the three levels in high -> medium -> low order', () => {
    for (const input of [undefined, null, {}, { by_confidence: [] }, { by_confidence: 'nope' }]) {
        const rows = confidenceRows(input);
        assert.deepEqual(rows.map((r) => r.key), CONFIDENCE_LEVELS);
        assert.deepEqual(rows.map((r) => r.rateLabel), [NO_VALUE, NO_VALUE, NO_VALUE]);
        assert.deepEqual(rows.map((r) => r.hasSamples), [false, false, false]);
        assert.deepEqual(rows.map((r) => r.total), [0, 0, 0]);
    }
});

test('confidenceRows shapes every bucket and pins each one to its own level', () => {
    // Deliberately NOT in high -> medium -> low order: shaping must follow the level, not the
    // array position, so a bucket cannot pick up the rate or sample count of its neighbour.
    const rows = confidenceRows({
        total: 26, agreed: 18,
        by_confidence: [
            { confidence: 'medium', total: 13, agreed: 9, rate: 9 / 13 },
            { confidence: 'low', total: 3, agreed: 0, rate: 0 },
            { confidence: 'high', total: 10, agreed: 9, rate: 0.9 },
        ],
    });
    assert.deepEqual(rows.map((r) => r.label), ['High', 'Medium', 'Low']);
    assert.deepEqual(rows.map((r) => r.rateLabel), ['90%', '69%', '0%']);
    assert.deepEqual(rows.map((r) => r.total), [10, 13, 3]);
    assert.deepEqual(rows.map((r) => r.samples), ['10 triaged results', '13 triaged results', '3 triaged results']);
    assert.ok(rows.every((r) => r.hasSamples));
});

test('confidenceRows reorders buckets the server returned out of order', () => {
    const rows = confidenceRows({
        by_confidence: [
            { confidence: 'low', total: 2, agreed: 1, rate: 0.5 },
            { confidence: 'high', total: 4, agreed: 4, rate: 1 },
        ],
    });
    assert.deepEqual(rows.map((r) => r.key), ['high', 'medium', 'low']);
    assert.deepEqual(rows.map((r) => r.rateLabel), ['100%', NO_VALUE, '50%']);
});

test('confidenceRows keeps a partly-filled ladder distinguishable from a flat one', () => {
    const rows = confidenceRows({ by_confidence: [{ confidence: 'high', total: 5, agreed: 4, rate: 0.8 }] });
    assert.deepEqual(rows.map((r) => r.rateLabel), ['80%', NO_VALUE, NO_VALUE]);
    assert.equal(rows[1].samples, '0 triaged results');
});

test('confidenceRows derives a missing rate without dividing by zero', () => {
    const rows = confidenceRows({
        by_confidence: [
            { confidence: 'high', total: 8, agreed: 6 },
            { confidence: 'medium', total: 0, agreed: 0 },
        ],
    });
    assert.equal(rows[0].rateLabel, '75%');
    assert.equal(rows[1].rateLabel, NO_VALUE);
    assert.equal(rows[1].rate, 0);
});

test('confidenceRows appends unrecognised buckets instead of dropping them', () => {
    const rows = confidenceRows({
        by_confidence: [
            { confidence: 'high', total: 2, agreed: 2, rate: 1 },
            { confidence: '', total: 3, agreed: 1, rate: 1 / 3 },
        ],
    });
    assert.equal(rows.length, 4, 'three known levels plus the unrecognised one');
    assert.deepEqual(rows.slice(0, 3).map((r) => r.key), CONFIDENCE_LEVELS);
    assert.equal(rows[3].label, 'Unspecified');
    assert.equal(rows[3].rateLabel, '33%');
});

test('confidenceRows ignores malformed bucket entries', () => {
    const rows = confidenceRows({ by_confidence: [null, 'x', { total: 5 }, { confidence: 'high', total: 1, agreed: 1, rate: 1 }] });
    assert.deepEqual(rows.map((r) => r.key), CONFIDENCE_LEVELS);
    assert.equal(rows[0].rateLabel, '100%');
});

// ── verdictRows: the per-verdict breakdown ───────────────────────────────

test('verdictRows keeps verdicts that share a defect_type apart', () => {
    // The whole reason the backend snapshots the verdict as well as the mapped defect_type:
    // flaky_test and test_data both suggest automation_bug, so a breakdown keyed on the mapped
    // value would merge a perfect verdict with a useless one into one meaningless row.
    const rows = verdictRows({
        by_verdict: [
            { verdict: 'flaky_test', total: 6, agreed: 6, rate: 1 },
            { verdict: 'test_data', total: 4, agreed: 0, rate: 0 },
        ],
    });
    assert.deepEqual(rows.map((r) => r.label), ['Flaky test', 'Test data']);
    assert.deepEqual(rows.map((r) => r.rateLabel), ['100%', '0%']);
    assert.deepEqual(rows.map((r) => r.samples), ['6 triaged results', '4 triaged results']);
});

test('verdictRows preserves the server order rather than re-sorting', () => {
    const rows = verdictRows({
        by_verdict: [
            { verdict: 'environment', total: 9, agreed: 3, rate: 1 / 3 },
            { verdict: 'product_bug', total: 5, agreed: 5, rate: 1 },
        ],
    });
    assert.deepEqual(rows.map((r) => r.key), ['environment', 'product_bug'], 'most samples first, as the server sent it');
});

test('verdictRows labels an unrecognised verdict with its raw key', () => {
    const rows = verdictRows({ by_verdict: [{ verdict: 'brand_new_verdict', total: 2, agreed: 1, rate: 0.5 }] });
    assert.deepEqual(rows.map((r) => r.label), ['brand_new_verdict']);
    assert.equal(rows[0].rateLabel, '50%');
});

test('verdictRows drops empty and malformed buckets', () => {
    const rows = verdictRows({
        by_verdict: [null, 'x', { total: 3 }, { verdict: 'unknown', total: 0, agreed: 0 }, { verdict: 'product_bug', total: 1, agreed: 1, rate: 1 }],
    });
    assert.deepEqual(rows.map((r) => r.key), ['product_bug'], 'a zero-sample verdict is noise, not a missing rung');
});

test('verdictRows tolerates a missing or malformed report', () => {
    for (const input of [undefined, null, {}, { by_verdict: [] }, { by_verdict: 'nope' }]) {
        assert.deepEqual(verdictRows(input), []);
    }
});
