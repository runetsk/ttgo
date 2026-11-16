import test from 'node:test';
import assert from 'node:assert/strict';
import {
    testCaseSnapshot,
    requirementSnapshot,
    snapshotsEqual,
    isDirty,
    reconcileRefresh,
    parseEditConflict,
} from './editorDraft.js';

// ── testCaseSnapshot ──────────────────────────────────────────────────────────

test('testCaseSnapshot treats category order as insignificant', () => {
    const a = testCaseSnapshot({ name: 'T', categories: [{ id: 'c1' }, { id: 'c2' }] });
    const b = testCaseSnapshot({ name: 'T', categories: [{ id: 'c2' }, { id: 'c1' }] });
    assert.ok(snapshotsEqual(a, b), 'reordering categories is not an edit');
});

test('testCaseSnapshot treats step order as significant', () => {
    const steps = [
        { action: 'one', expected_result: '1' },
        { action: 'two', expected_result: '2' },
    ];
    const a = testCaseSnapshot({ steps });
    const b = testCaseSnapshot({ steps: [steps[1], steps[0]] });
    assert.ok(!snapshotsEqual(a, b), 'reordering steps is an edit');
});

test('testCaseSnapshot ignores volatile step fields (id, order_index)', () => {
    const a = testCaseSnapshot({ steps: [{ id: 's1', order_index: 0, action: 'do', expected_result: 'ok' }] });
    const b = testCaseSnapshot({ steps: [{ id: 's9', order_index: 7, action: 'do', expected_result: 'ok' }] });
    assert.ok(snapshotsEqual(a, b), 'only action/expected_result define a step edit');
});

test('testCaseSnapshot treats custom-value order as insignificant', () => {
    const a = testCaseSnapshot({ customValues: [{ custom_field_id: 'f1', value: '"x"' }, { custom_field_id: 'f2', value: '"y"' }] });
    const b = testCaseSnapshot({ customValues: [{ custom_field_id: 'f2', value: '"y"' }, { custom_field_id: 'f1', value: '"x"' }] });
    assert.ok(snapshotsEqual(a, b));
});

test('testCaseSnapshot detects a custom-value change', () => {
    const a = testCaseSnapshot({ customValues: [{ custom_field_id: 'f1', value: '"x"' }] });
    const b = testCaseSnapshot({ customValues: [{ custom_field_id: 'f1', value: '"z"' }] });
    assert.ok(!snapshotsEqual(a, b));
});

test('testCaseSnapshot tolerates missing/undefined fields', () => {
    const a = testCaseSnapshot({});
    const b = testCaseSnapshot({ name: '', description: '', steps: [], categories: [], customValues: [] });
    assert.ok(snapshotsEqual(a, b));
});

// ── requirementSnapshot ───────────────────────────────────────────────────────

test('requirementSnapshot compares the three editable fields', () => {
    const base = { identifier: 'R-1', title: 'T', description: 'D' };
    assert.ok(snapshotsEqual(requirementSnapshot(base), requirementSnapshot({ ...base })));
    assert.ok(!snapshotsEqual(requirementSnapshot(base), requirementSnapshot({ ...base, title: 'T2' })));
});

// ── isDirty ───────────────────────────────────────────────────────────────────

test('isDirty is false for an untouched draft and true after a change', () => {
    const baseline = requirementSnapshot({ identifier: 'R-1', title: 'T', description: 'D' });
    const clean = requirementSnapshot({ identifier: 'R-1', title: 'T', description: 'D' });
    const edited = requirementSnapshot({ identifier: 'R-1', title: 'T changed', description: 'D' });
    assert.equal(isDirty(baseline, clean), false);
    assert.equal(isDirty(baseline, edited), true);
});

// ── reconcileRefresh ──────────────────────────────────────────────────────────

const snap = (over = {}) => requirementSnapshot({ identifier: 'R-1', title: 'T', description: 'D', ...over });

test('reconcileRefresh adopts the server record when the editor is clean and idle', () => {
    const r = reconcileRefresh({ baselineSnap: snap(), draftSnap: snap(), saving: false, serverSnap: snap({ title: 'T2' }) });
    assert.deepEqual(r, { adopt: true, externallyChanged: false });
});

test('reconcileRefresh preserves a dirty draft and reports an external change', () => {
    const r = reconcileRefresh({
        baselineSnap: snap(),
        draftSnap: snap({ description: 'my edit' }),
        saving: false,
        serverSnap: snap({ title: 'server edit' }),
    });
    assert.equal(r.adopt, false, 'must not clobber unsaved edits');
    assert.equal(r.externallyChanged, true, 'server differs from the baseline the draft was built on');
});

test('reconcileRefresh preserves a dirty draft and reports no external change when the server is unchanged', () => {
    const r = reconcileRefresh({
        baselineSnap: snap(),
        draftSnap: snap({ description: 'my edit' }),
        saving: false,
        serverSnap: snap(),
    });
    assert.equal(r.adopt, false);
    assert.equal(r.externallyChanged, false, 'a refresh that matches the baseline is not a conflict');
});

test('reconcileRefresh never adopts while a save is in flight', () => {
    const r = reconcileRefresh({ baselineSnap: snap(), draftSnap: snap(), saving: true, serverSnap: snap({ title: 'T2' }) });
    assert.equal(r.adopt, false, 'a refresh must not race an in-flight save');
});

// ── parseEditConflict ─────────────────────────────────────────────────────────

const axiosErr = (status, data) => ({ response: { status, data } });

test('parseEditConflict returns the latest record on an edit_conflict 409', () => {
    const latest = { id: 't1', name: 'server copy', edit_token: 'abc' };
    const got = parseEditConflict(axiosErr(409, { code: 'edit_conflict', test: latest }), 'test');
    assert.deepEqual(got, latest);
});

test('parseEditConflict ignores a 409 without the edit_conflict code (e.g. duplicate identifier)', () => {
    assert.equal(parseEditConflict(axiosErr(409, { error: 'identifier already exists' }), 'requirement'), null);
});

test('parseEditConflict ignores non-409 errors', () => {
    assert.equal(parseEditConflict(axiosErr(500, { code: 'edit_conflict', test: {} }), 'test'), null);
    assert.equal(parseEditConflict(new Error('network'), 'test'), null);
    assert.equal(parseEditConflict(undefined, 'test'), null);
});
