import test from 'node:test';
import assert from 'node:assert/strict';
import { BULK_DEFECT_TYPE_OPTIONS, buildBulkDefectTypePayload, summarizeBulkTriage } from './bulkTriage.js';

test('payload carries the ids and defect_type', () => {
    const payload = buildBulkDefectTypePayload(new Set(['r1', 'r2']), 'product_bug');
    assert.deepEqual(payload, { result_ids: ['r1', 'r2'], defect_type: 'product_bug' });
});

test('payload NEVER carries a status key — that is what selects triage mode', () => {
    for (const opt of BULK_DEFECT_TYPE_OPTIONS) {
        const payload = buildBulkDefectTypePayload(['r1'], opt.value);
        assert.deepEqual(Object.keys(payload).sort(), ['defect_type', 'result_ids']);
        assert.equal('status' in payload, false, `${opt.value} must not send a status`);
    }
});

test('accepts any iterable of ids (Set from the component, array from tests)', () => {
    assert.deepEqual(buildBulkDefectTypePayload(['r1'], 'system_issue').result_ids, ['r1']);
    assert.deepEqual(buildBulkDefectTypePayload(new Set(['r1']), 'system_issue').result_ids, ['r1']);
});

test('no selection => null (the action is disabled, no request is made)', () => {
    assert.equal(buildBulkDefectTypePayload(new Set(), 'product_bug'), null);
    assert.equal(buildBulkDefectTypePayload([], 'product_bug'), null);
    assert.equal(buildBulkDefectTypePayload(null, 'product_bug'), null);
    assert.equal(buildBulkDefectTypePayload(undefined, 'product_bug'), null);
});

test('unknown or empty defect type => null', () => {
    assert.equal(buildBulkDefectTypePayload(['r1'], ''), null);
    assert.equal(buildBulkDefectTypePayload(['r1'], undefined), null);
    assert.equal(buildBulkDefectTypePayload(['r1'], 'Product Bug'), null);
    assert.equal(buildBulkDefectTypePayload(['r1'], 'flaky_test'), null);
});

test('summary reports both counts on a mixed selection', () => {
    assert.deepEqual(summarizeBulkTriage({ updated: 12, skipped: 3 }), {
        tone: 'warning', message: '12 updated, 3 skipped (not failures)',
    });
});

test('summary is a plain success when nothing was skipped', () => {
    assert.deepEqual(summarizeBulkTriage({ updated: 5, skipped: 0 }), {
        tone: 'success', message: '5 updated',
    });
});

test('all skipped => nothing updated, and it says so instead of looking like a success', () => {
    const summary = summarizeBulkTriage({ updated: 0, skipped: 4 });
    assert.deepEqual(summary, { tone: 'warning', message: 'Nothing updated — 4 skipped (not failures)' });
    assert.notEqual(summary.tone, 'success');
});

test('summary tolerates a missing or malformed response body', () => {
    assert.deepEqual(summarizeBulkTriage({}), { tone: 'info', message: 'Nothing updated' });
    assert.deepEqual(summarizeBulkTriage(null), { tone: 'info', message: 'Nothing updated' });
    assert.deepEqual(summarizeBulkTriage(undefined), { tone: 'info', message: 'Nothing updated' });
    assert.deepEqual(summarizeBulkTriage({ updated: 'x', skipped: 'y' }), { tone: 'info', message: 'Nothing updated' });
});
