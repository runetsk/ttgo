import test from 'node:test';
import assert from 'node:assert/strict';
import {
    RETEST_TITLE_MAX,
    SELECTION_SAFE_CHANGES,
    UNASSIGNED,
    buildBulkPayload,
    buildRetestRun,
    nextSelectionOnFilterChange,
} from './defectActions.js';

// ── buildBulkPayload ──

test('payload carries the ids and every field it was given', () => {
    const payload = buildBulkPayload(new Set(['d1', 'd2']), {
        status: 'closed', severity: 'major', assignee_id: 'u7',
    });
    assert.deepEqual(payload, {
        ids: ['d1', 'd2'], status: 'closed', severity: 'major', assignee_id: 'u7',
    });
});

test('accepts any iterable of ids (Set from the page, array from tests)', () => {
    assert.deepEqual(buildBulkPayload(['d1'], { status: 'fixed' }).ids, ['d1']);
    assert.deepEqual(buildBulkPayload(new Set(['d1']), { status: 'fixed' }).ids, ['d1']);
});

test('ids are deduped and blanks dropped, so the 500 cap counts real defects', () => {
    assert.deepEqual(buildBulkPayload(['d1', 'd2', 'd1', '', null, 'd2'], {}).ids, ['d1', 'd2']);
});

test('no selection => null (the action is disabled, no request is made)', () => {
    assert.equal(buildBulkPayload(new Set(), { status: 'closed' }), null);
    assert.equal(buildBulkPayload([], { status: 'closed' }), null);
    assert.equal(buildBulkPayload(null, { status: 'closed' }), null);
    assert.equal(buildBulkPayload(undefined, { status: 'closed' }), null);
    assert.equal(buildBulkPayload(['', null], { status: 'closed' }), null);
});

test('empty status/severity are dropped — the endpoint cannot clear them', () => {
    for (const empty of ['', null, undefined]) {
        assert.deepEqual(buildBulkPayload(['d1'], { status: empty, severity: empty }), { ids: ['d1'] });
    }
});

test('unknown keys never reach the wire', () => {
    const payload = buildBulkPayload(['d1'], {
        assigneeId: 'u7', defect_type: 'product_bug', ids: ['nope'], title: 'x',
    });
    assert.deepEqual(payload, { ids: ['d1'] });
});

test('no fields at all => a bare read-back, not a broken body', () => {
    assert.deepEqual(buildBulkPayload(['d1', 'd2'], {}), { ids: ['d1', 'd2'] });
    assert.deepEqual(buildBulkPayload(['d1', 'd2']), { ids: ['d1', 'd2'] });
    assert.deepEqual(buildBulkPayload(['d1'], null), { ids: ['d1'] });
});

test('unassign is sent as "" — null would decode to nil and change nothing', () => {
    assert.equal(UNASSIGNED, '');
    for (const empty of ['', null]) {
        const payload = buildBulkPayload(['d1'], { assignee_id: empty });
        assert.equal('assignee_id' in payload, true, `${String(empty)} must still send the key`);
        assert.equal(payload.assignee_id, '');
    }
});

test('an omitted assignee leaves the key out entirely (= unchanged)', () => {
    assert.equal('assignee_id' in buildBulkPayload(['d1'], { status: 'closed' }), false);
    assert.equal('assignee_id' in buildBulkPayload(['d1'], { assignee_id: undefined }), false);
});

// ── nextSelectionOnFilterChange ──

test('every filter-bar dimension clears the selection', () => {
    for (const changed of ['query', 'status', 'sevs', 'severities', 'sort']) {
        const next = nextSelectionOnFilterChange(new Set(['d1', 'd2']), changed);
        assert.equal(next.size, 0, `${changed} must clear the selection`);
    }
});

test('an unrecognised dimension clears too — guessing wrong must not enable a wrong write', () => {
    assert.equal(nextSelectionOnFilterChange(new Set(['d1']), 'sevrity').size, 0);
    assert.equal(nextSelectionOnFilterChange(new Set(['d1']), undefined).size, 0);
});

test('changes that do not touch the visible rows keep the selection, by identity', () => {
    const selection = new Set(['d1', 'd2']);
    for (const changed of SELECTION_SAFE_CHANGES) {
        assert.equal(nextSelectionOnFilterChange(selection, changed), selection,
            `${changed} must return the same Set, not a copy`);
    }
});

test('an already-empty selection is returned unchanged, so no needless state write', () => {
    const empty = new Set();
    assert.equal(nextSelectionOnFilterChange(empty, 'status'), empty);
});

test('a non-Set selection is normalised to a Set', () => {
    const next = nextSelectionOnFilterChange(['d1'], 'expanded');
    assert.ok(next instanceof Set);
    assert.deepEqual(Array.from(next), ['d1']);
    assert.ok(nextSelectionOnFilterChange(null, 'status') instanceof Set);
    assert.ok(nextSelectionOnFilterChange(undefined, 'status') instanceof Set);
});

// ── buildRetestRun ──

const defect = (over = {}) => ({ id: 'd1', title: 'Checkout hangs on Safari', ...over });
const affected = (id, name) => ({ id, name, last_run_id: 'r1', last_run_name: 'Run #318', last_result_status: 'FAIL' });

test('retest run names the defect and carries its affected test cases', () => {
    const run = buildRetestRun(defect(), [affected('tc1', 'pay with saved card'), affected('tc2', '3DS challenge')]);
    assert.deepEqual(run, { name: 'Retest: Checkout hangs on Safari', test_case_ids: ['tc1', 'tc2'] });
});

test('retest run never carries a category — POST /runs rejects both at once', () => {
    const run = buildRetestRun(defect(), [affected('tc1', 'a')]);
    assert.deepEqual(Object.keys(run).sort(), ['name', 'test_case_ids']);
});

test('test ids are deduped and id-less entries dropped', () => {
    const run = buildRetestRun(defect(), [
        affected('tc1', 'a'), affected('tc1', 'a again'), { name: 'no id' }, null, affected('tc2', 'b'),
    ]);
    assert.deepEqual(run.test_case_ids, ['tc1', 'tc2']);
});

test('no tests => null (this is what disables the Retest button)', () => {
    assert.equal(buildRetestRun(defect(), []), null);
    assert.equal(buildRetestRun(defect(), null), null);
    assert.equal(buildRetestRun(defect(), undefined), null);
    assert.equal(buildRetestRun(defect(), [{ name: 'deleted case' }, null]), null);
});

test('no defect => null', () => {
    assert.equal(buildRetestRun(null, [affected('tc1', 'a')]), null);
    assert.equal(buildRetestRun(undefined, [affected('tc1', 'a')]), null);
});

test('a long title is truncated so the run list stays readable', () => {
    const long = 'x'.repeat(500);
    const run = buildRetestRun(defect({ title: long }), [affected('tc1', 'a')]);
    assert.equal(run.name.startsWith('Retest: xxx'), true);
    assert.equal(run.name.endsWith('…'), true);
    assert.equal(run.name.length, 'Retest: '.length + RETEST_TITLE_MAX);
});

test('a title at the limit is left alone', () => {
    const exact = 'y'.repeat(RETEST_TITLE_MAX);
    const run = buildRetestRun(defect({ title: exact }), [affected('tc1', 'a')]);
    assert.equal(run.name, `Retest: ${exact}`);
});

test('a blank or missing title falls back to a plain name', () => {
    assert.equal(buildRetestRun(defect({ title: '   ' }), [affected('tc1', 'a')]).name, 'Retest');
    assert.equal(buildRetestRun({ id: 'd1' }, [affected('tc1', 'a')]).name, 'Retest');
});
