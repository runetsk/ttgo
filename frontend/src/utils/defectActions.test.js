import test from 'node:test';
import assert from 'node:assert/strict';
import {
    RETEST_TITLE_MAX,
    UNASSIGNED,
    applyBulkResult,
    assigneeOptions,
    buildBulkPayload,
    bulkLockFor,
    buildDefectPayload,
    buildRetestRun,
    isBulkLocked,
    isEditSnapshotStale,
    rememberBulkTouched,
    BULK_LOCK_MESSAGE,
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

// ── bulkLockFor ──

test('the lock is the set of ids the apply captured', () => {
    const lock = bulkLockFor(['d1', 'd2', 'd3']);
    assert.equal(lock.has('d1'), true);
    assert.equal(lock.has('d3'), true);
    assert.equal(lock.has('elsewhere'), false);
});

// The bug this exists to prevent: the register locks none of its filter controls during an
// apply and every one of them CLEARS the selection (afterFilterChange), so the freeze can
// only be built from what the call captured. Built from the selection instead, one re-sort
// mid-flight released the row's modal, Delete and Retest on the rows still being written to
// — and that modal is a full-record editor seeded from the pre-bulk snapshot, so saving it
// put the old status and severity back over the bulk write.
test('the lock is unmoved by whatever the selection does next', () => {
    const selected = new Set(['d1', 'd2', 'd3']);
    const lock = bulkLockFor(Array.from(selected));

    selected.clear(); // a sort change, a keystroke, a tile — all of them do this
    assert.equal(selected.size, 0);
    assert.deepEqual([...lock].sort(), ['d1', 'd2', 'd3']);

    selected.add('someone-else'); // and re-ticking other rows does not extend it
    assert.equal(lock.has('someone-else'), false);
    assert.equal(lock.size, 3);
});

// Nothing in flight is a single null check, and a call that somehow sent no ids must not
// leave the table frozen with no way back.
test('nothing captured means nothing is locked', () => {
    assert.equal(bulkLockFor([]), null);
    assert.equal(bulkLockFor(null), null);
    assert.equal(bulkLockFor(undefined), null);
    assert.equal(bulkLockFor(['', null, undefined]), null);
});

test('a Set is accepted, ids are deduplicated and blanks dropped', () => {
    assert.deepEqual([...bulkLockFor(new Set(['d1', 'd2']))].sort(), ['d1', 'd2']);
    assert.equal(bulkLockFor(['d1', 'd1', 'd1']).size, 1);
    assert.deepEqual([...bulkLockFor(['d1', '', null])], ['d1']);
});

// ── isBulkLocked ──
//
// The question a write asks about ITSELF, as opposed to the boolean a button reads. Five
// rounds of guarding the paths INTO these writes (the checkbox, then the row's buttons, then
// keying those on the captured ids) each closed one path and left another: a dialog opened
// before the apply started passes through none of them, because it was opened while nothing
// was locked and no overlay in this app makes the page behind it unreachable.

test('a captured id is locked, anything else is not', () => {
    const lock = bulkLockFor(['d1', 'd2']);
    assert.equal(isBulkLocked(lock, 'd1'), true);
    assert.equal(isBulkLocked(lock, 'd2'), true);
    assert.equal(isBulkLocked(lock, 'd3'), false);
});

test('nothing in flight locks nothing', () => {
    assert.equal(isBulkLocked(null, 'd1'), false);
    assert.equal(isBulkLocked(undefined, 'd1'), false);
    assert.equal(isBulkLocked(bulkLockFor([]), 'd1'), false);
});

// A create has no id and a malformed row may have none either. Neither can be "the defect a
// bulk is writing to", and neither may be refused a save over it.
test('a missing id is never locked', () => {
    const lock = bulkLockFor(['d1']);
    for (const id of ['', null, undefined]) assert.equal(isBulkLocked(lock, id), false);
});

// Defensive: the predicate is called on every save/delete/retest, so a lock that is somehow
// not a Set must read as "not locked" rather than throw inside a click handler and take the
// whole page down.
test('a lock that is not a Set reads as unlocked instead of throwing', () => {
    assert.equal(isBulkLocked({}, 'd1'), false);
    assert.equal(isBulkLocked('d1', 'd1'), false);
    assert.equal(isBulkLocked(['d1'], 'd1'), false);
});

// ── isEditSnapshotStale ──
//
// The sixth hole, and the reason "is a bulk in flight" was the wrong question. The lock
// releases when the request lands; the dialog's snapshot does not stop being stale. Guarding
// only the flight moved the overwrite to the far side of it — wait for the apply, press Save on
// the dialog still standing, and the full-record PATCH reverts it with nothing shown at all.

test('an in-flight capture is stale, exactly like isBulkLocked', () => {
    const lock = bulkLockFor(['d1']);
    assert.equal(isEditSnapshotStale(lock, new Set(), 'd1'), true);
    assert.equal(isEditSnapshotStale(lock, new Set(), 'd2'), false);
});

// The one that matters: nothing is in flight any more, and the snapshot is STILL stale.
test('a landed apply leaves the snapshot stale after the lock has released', () => {
    assert.equal(isEditSnapshotStale(null, new Set(['d1']), 'd1'), true);
});

test('a defect no apply touched is never stale', () => {
    assert.equal(isEditSnapshotStale(null, new Set(['d1']), 'd2'), false);
    assert.equal(isEditSnapshotStale(null, new Set(), 'd1'), false);
});

// A create has no id, and must not be refused a save over a bulk on some other row.
test('a missing id is never stale', () => {
    const touched = new Set(['d1']);
    for (const id of ['', null, undefined]) {
        assert.equal(isEditSnapshotStale(bulkLockFor(['d1']), touched, id), false);
    }
});

// Defensive, like isBulkLocked: read at the moment of a click, so a malformed set must read
// as "not stale" rather than throw and take the page down.
test('a touched set that is not a Set reads as not stale instead of throwing', () => {
    assert.equal(isEditSnapshotStale(null, undefined, 'd1'), false);
    assert.equal(isEditSnapshotStale(null, {}, 'd1'), false);
    assert.equal(isEditSnapshotStale(null, ['d1'], 'd1'), false);
});

// ── rememberBulkTouched ──
//
// What feeds the set isEditSnapshotStale reads. The staleness has to survive the request, but
// the RECORD of it must not survive the dialog: the first version added every applied id and was
// only ever cleared by opening a dialog, so a session that did nothing but bulk applies grew it
// for the lifetime of the page.

// The case the guard exists for, unchanged: the dialog is open on the defect the apply rewrote.
test('the open dialog\'s own defect is recorded, and reads back as stale', () => {
    const touched = new Set();
    rememberBulkTouched(touched, 'd1', ['d1', 'd2']);
    assert.deepEqual([...touched], ['d1']);
    // Nothing is in flight any more and the save must still be refused.
    assert.equal(isEditSnapshotStale(null, touched, 'd1'), true);
});

// The growth bound. Everything else in the apply is somebody else's row, and no dialog holds a
// snapshot of it — the next one opened is seeded from the row the apply produced.
test('no dialog open records nothing at all', () => {
    for (const editing of [null, undefined, '']) {
        const touched = new Set();
        rememberBulkTouched(touched, editing, ['d1', 'd2', 'd3']);
        assert.equal(touched.size, 0);
    }
});

test('the other ids in the same apply are not recorded', () => {
    const touched = new Set();
    rememberBulkTouched(touched, 'd1', ['d2', 'd3']);
    assert.equal(touched.size, 0);
    assert.equal(isEditSnapshotStale(null, touched, 'd2'), false);
});

// Twenty applies with one dialog open cannot leave more than that one dialog's defect behind.
test('repeated applies keep the set at one id', () => {
    const touched = new Set();
    for (let i = 0; i < 20; i++) rememberBulkTouched(touched, 'd1', [`x${i}`, 'd1', `y${i}`]);
    assert.deepEqual([...touched], ['d1']);
});

test('accepts any iterable of requested ids, and tolerates none', () => {
    assert.deepEqual([...rememberBulkTouched(new Set(), 'd1', new Set(['d1']))], ['d1']);
    assert.equal(rememberBulkTouched(new Set(), 'd1', []).size, 0);
    assert.equal(rememberBulkTouched(new Set(), 'd1', null).size, 0);
    assert.equal(rememberBulkTouched(new Set(), 'd1', undefined).size, 0);
});

// Defensive like the two predicates above: it runs inside a response handler, so a malformed
// set must be a no-op rather than take the page down mid-apply.
test('a touched set that is not a Set is handed straight back', () => {
    assert.doesNotThrow(() => rememberBulkTouched(undefined, 'd1', ['d1']));
    assert.doesNotThrow(() => rememberBulkTouched({}, 'd1', ['d1']));
    assert.deepEqual(rememberBulkTouched(['d1'], 'd1', ['d1']), ['d1']);
});

// The refusal is shown, never swallowed: DefectModal's request path already ends in
// `.catch(() => {})`, so a guard that returned silently would look exactly like the overwrite
// it prevents — a Save that appears to do nothing.
test('the refusal has a message to show, and it says what to do', () => {
    assert.equal(typeof BULK_LOCK_MESSAGE, 'string');
    assert.ok(BULK_LOCK_MESSAGE.length > 0);
    assert.match(BULK_LOCK_MESSAGE, /bulk update/i);
});

// ── applyBulkResult ──

const row = (id, over = {}) => ({ id, title: `Defect ${id}`, status: 'open', ...over });

test('returned rows replace the ones on the page, in place', () => {
    const rows = [row('d1'), row('d2'), row('d3')];
    const next = applyBulkResult(rows, [row('d1', { status: 'closed' }), row('d2', { status: 'closed' })], ['d1', 'd2']);
    assert.deepEqual(next.map(r => r.id), ['d1', 'd2', 'd3']);
    assert.deepEqual(next.map(r => r.status), ['closed', 'closed', 'open']);
});

// The endpoint tolerates unknown ids rather than rejecting the call, so an id that was sent
// and did not come back is a defect somebody else deleted since this page loaded. Left in the
// list it sat there unchanged after a "successful" apply — the user is told they closed
// something that no longer exists.
test('an id that was sent but not returned is dropped — it no longer exists server-side', () => {
    const rows = [row('d1'), row('gone'), row('d3')];
    const next = applyBulkResult(rows, [row('d1', { status: 'closed' })], ['d1', 'gone']);
    assert.deepEqual(next.map(r => r.id), ['d1', 'd3']);
});

test('a whole selection that has been deleted leaves the rest of the queue alone', () => {
    const rows = [row('gone1'), row('d2'), row('gone2')];
    assert.deepEqual(applyBulkResult(rows, [], ['gone1', 'gone2']).map(r => r.id), ['d2']);
});

// Absence from the response says nothing about a row that was never part of the call.
test('rows outside the request are never dropped, whatever came back', () => {
    const rows = [row('d1'), row('untouched')];
    const next = applyBulkResult(rows, [row('d1', { severity: 'critical' })], ['d1']);
    assert.deepEqual(next.map(r => r.id), ['d1', 'untouched']);
});

test('no ids requested => the list is handed back untouched', () => {
    const rows = [row('d1')];
    assert.equal(applyBulkResult(rows, [], []), rows);
    assert.equal(applyBulkResult(rows, [], undefined), rows);
});

test('a Set of ids works too, and neither argument has to be an array', () => {
    const rows = [row('d1'), row('gone')];
    assert.deepEqual(applyBulkResult(rows, null, new Set(['gone'])).map(r => r.id), ['d1']);
    assert.deepEqual(applyBulkResult(null, null, ['d1']), []);
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
