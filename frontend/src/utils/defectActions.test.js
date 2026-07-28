import test from 'node:test';
import assert from 'node:assert/strict';
import {
    RETEST_TITLE_MAX,
    UNASSIGNED,
    assigneeOptions,
    buildBulkPayload,
    buildDefectPayload,
    buildRetestRun,
} from './defectActions.js';
import { DEFECT_STATUS_OPTIONS, deriveStatus } from './defectQueue.js';

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

// ── assigneeOptions ──

const user = (id, over = {}) => ({ id, display_name: `User ${id}`, email: `${id}@example.com`, ...over });

test('every assignable user becomes an option, display name first', () => {
    assert.deepEqual(assigneeOptions([user('u1'), user('u2')], null), [
        { id: 'u1', label: 'User u1' },
        { id: 'u2', label: 'User u2' },
    ]);
});

test('a user with no display name falls back to their email, like AssigneePicker', () => {
    assert.deepEqual(assigneeOptions([user('u1', { display_name: '' })], null), [
        { id: 'u1', label: 'u1@example.com' },
    ]);
    assert.deepEqual(assigneeOptions([{ id: 'u9' }], null), [{ id: 'u9', label: 'u9' }]);
});

test('no users yet (still loading) => no options, but never a throw', () => {
    assert.deepEqual(assigneeOptions(null, null), []);
    assert.deepEqual(assigneeOptions(undefined, null), []);
    assert.deepEqual(assigneeOptions([], null), []);
    assert.deepEqual(assigneeOptions([null, {}, { id: '' }], null), []);
});

test('a current owner missing from the assignable list is kept, so saving cannot drop them', () => {
    const defect = { assignee_id: 'gone', assignee_name: 'Mara Reyes' };
    const options = assigneeOptions([user('u1')], defect);
    assert.deepEqual(options, [
        { id: 'gone', label: 'Mara Reyes', inactive: true },
        { id: 'u1', label: 'User u1' },
    ]);
});

test('a deactivated owner with no resolved name still shows as their id, not as nothing', () => {
    const options = assigneeOptions([], { assignee_id: 'gone' });
    assert.deepEqual(options, [{ id: 'gone', label: 'gone', inactive: true }]);
});

test('a current owner who IS assignable is listed once, in list order', () => {
    const options = assigneeOptions([user('u1'), user('u2')], { assignee_id: 'u2', assignee_name: 'User u2' });
    assert.deepEqual(options.map(o => o.id), ['u1', 'u2']);
});

test('an unassigned defect adds no synthetic entry', () => {
    assert.deepEqual(assigneeOptions([user('u1')], { assignee_id: null }), [{ id: 'u1', label: 'User u1' }]);
    assert.deepEqual(assigneeOptions([user('u1')], {}), [{ id: 'u1', label: 'User u1' }]);
});

test('duplicate users are collapsed', () => {
    assert.deepEqual(assigneeOptions([user('u1'), user('u1')], null).map(o => o.id), ['u1']);
});

// ── buildDefectPayload ──

const form = (over = {}) => ({
    title: 'Checkout hangs on Safari',
    description: 'Steps to reproduce…',
    severity: 'major',
    status: 'fixed',
    assignee_id: 'u1',
    external_provider: 'Jira',
    external_key: 'PROJ-1',
    external_url: 'https://example.com/PROJ-1',
    ...over,
});

test('the modal sends every field on every save — it is a full-record editor', () => {
    assert.deepEqual(buildDefectPayload(form()), {
        title: 'Checkout hangs on Safari',
        description: 'Steps to reproduce…',
        severity: 'major',
        status: 'fixed',
        assignee_id: 'u1',
        external_provider: 'Jira',
        external_key: 'PROJ-1',
        external_url: 'https://example.com/PROJ-1',
    });
});

test('cleared text fields are sent as "" so they actually clear, never omitted', () => {
    const payload = buildDefectPayload(form({
        description: '  ', external_provider: '', external_key: undefined, external_url: null,
    }));
    for (const key of ['description', 'external_provider', 'external_key', 'external_url']) {
        assert.equal(key in payload, true, `${key} must still be sent`);
        assert.equal(payload[key], '');
    }
});

test('every text field is trimmed', () => {
    const payload = buildDefectPayload(form({
        title: '  spaced  ', description: '  body  ', external_key: '  PROJ-2  ',
    }));
    assert.equal(payload.title, 'spaced');
    assert.equal(payload.description, 'body');
    assert.equal(payload.external_key, 'PROJ-2');
});

test('unassigning sends "" — the same wire value the bulk bar uses', () => {
    for (const empty of ['', null, undefined]) {
        const payload = buildDefectPayload(form({ assignee_id: empty }));
        assert.equal('assignee_id' in payload, true);
        assert.equal(payload.assignee_id, UNASSIGNED);
    }
    // and clearing a real owner is still a change, so it is still sent
    const cleared = buildDefectPayload(form({ assignee_id: '' }), { assignee_id: 'u1' });
    assert.equal(cleared.assignee_id, UNASSIGNED);
});

// The 400 this guard exists to prevent: a defect owned by a since-deactivated user.
// assigneeOptions keeps that owner in the picker so an unrelated save cannot drop
// them — but the server rejects any non-empty assignee_id that is not an active
// user, so echoing the id back made every edit of such a defect fail with a message
// the modal swallowed. Unchanged means unsent, which is exactly PATCH's semantics.
test('an unchanged assignee is omitted, so a deactivated owner cannot 400 the save', () => {
    const original = { id: 'd1', assignee_id: 'deactivated-user', assignee_name: 'Gone Greg' };
    const payload = buildDefectPayload(form({ assignee_id: 'deactivated-user', severity: 'critical' }), original);
    assert.equal('assignee_id' in payload, false, 'the untouched owner must not be echoed back');
    assert.equal(payload.severity, 'critical', 'the field actually being edited still goes');
});

test('an unchanged EMPTY assignee is omitted too', () => {
    for (const empty of ['', null, undefined]) {
        const payload = buildDefectPayload(form({ assignee_id: empty }), { id: 'd1', assignee_id: empty });
        assert.equal('assignee_id' in payload, false);
    }
});

test('any real assignee change is still sent, in both directions', () => {
    const from = id => buildDefectPayload(form({ assignee_id: id }), { id: 'd1', assignee_id: 'u1' });
    assert.equal(from('u2').assignee_id, 'u2', 'reassigning');
    assert.equal(from('').assignee_id, UNASSIGNED, 'unassigning');
    const assigning = buildDefectPayload(form({ assignee_id: 'u2' }), { id: 'd1', assignee_id: null });
    assert.equal(assigning.assignee_id, 'u2', 'assigning an unowned defect');
});

test('with no original (create) the assignee is always sent', () => {
    for (const original of [null, undefined]) {
        const payload = buildDefectPayload(form({ assignee_id: 'u1' }), original);
        assert.equal(payload.assignee_id, 'u1');
        assert.equal('assignee_id' in buildDefectPayload(form({ assignee_id: '' }), original), true);
    }
});

test('a blank title => null, the same condition that disables Save', () => {
    assert.equal(buildDefectPayload(form({ title: '   ' })), null);
    assert.equal(buildDefectPayload(form({ title: '' })), null);
    assert.equal(buildDefectPayload({}), null);
    assert.equal(buildDefectPayload(null), null);
    assert.equal(buildDefectPayload(undefined), null);
});

test('no key beyond the defect record ever reaches the wire', () => {
    const payload = buildDefectPayload(form({ id: 'd1', assigneeId: 'typo', linked_test_count: 4 }));
    assert.deepEqual(Object.keys(payload).sort(), [
        'assignee_id', 'description', 'external_key', 'external_provider',
        'external_url', 'severity', 'status', 'title',
    ]);
});

// ── the modal's status select ──

test('the modal offers exactly the three stored statuses, "fixed" included', () => {
    assert.deepEqual(DEFECT_STATUS_OPTIONS.map(o => o.value), ['open', 'fixed', 'closed']);
    assert.equal(DEFECT_STATUS_OPTIONS.find(o => o.value === 'fixed').label, 'Fixed (awaiting retest)');
});

test('every status the modal can save is one the queue can derive a bucket from', () => {
    const buckets = DEFECT_STATUS_OPTIONS.map(o => deriveStatus({ status: o.value }));
    // open with no assignee derives as triage; assigning it is what makes it "in progress".
    assert.deepEqual(buckets, ['triage', 'fixed', 'closed']);
    assert.equal(deriveStatus({ status: 'open', assignee_id: 'u1' }), 'progress');
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
