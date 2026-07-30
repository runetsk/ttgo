// Pure action layer for the Defects triage queue: the decisions the page's write paths
// make before they touch the network — what a bulk update sends, what the edit modal
// sends, who its assignee picker can offer, what happens to a selection when the view
// changes, and what a Retest run is made of.
//
// Kept DOM-free and dependency-free, like utils/defectQueue.js and utils/bulkTriage.js:
// this repo has no component-render test infrastructure, so a pure helper is the only
// testable seam for these decisions.

// The wire value that clears an assignee. Absent means "leave it alone", so unassigning
// has to be an explicit empty string — null would decode to nil and change nothing.
export const UNASSIGNED = '';

// buildBulkPayload returns the body for POST /defects/bulk-update, or null when the action
// must not fire at all (nothing selected — the endpoint 400s on an empty id list).
//
// `fields` carries any of { status, severity, assignee_id } — nothing else reaches the
// wire, because Go ignores unknown JSON keys and a camelCase typo would otherwise ride
// along and silently write nothing. Empty status/severity values are dropped, since the
// endpoint has no concept of clearing them. The assignee is the one field with a
// meaningful empty: a present-but-empty assignee_id (null or '') is normalised to
// UNASSIGNED, so a picker's "Unassign" option cannot be mistaken for "unchanged". To
// leave assignees alone, omit the key entirely.
export function buildBulkPayload(selectedIds, fields) {
    const ids = Array.from(new Set(Array.from(selectedIds || []).filter(Boolean)));
    if (ids.length === 0) return null;

    const source = fields || {};
    const payload = { ids };
    if (source.status) payload.status = source.status;
    if (source.severity) payload.severity = source.severity;
    // presence, not truthiness: '' and null both mean unassign here.
    if (source.assignee_id !== undefined) {
        payload.assignee_id = source.assignee_id ? String(source.assignee_id) : UNASSIGNED;
    }
    return payload;
}

// bulkLockFor turns the ids a bulk apply captured at click time into the freeze the queue
// holds for the round-trip: the set of rows that call is writing to, or null when nothing
// is in flight.
//
// The CAPTURED ids are the whole point of it. The register locks none of its filter
// controls during an apply, and every one of them deliberately clears the selection (a bulk
// action must never land on rows that have been filtered out of sight) — so a freeze read
// off the live selection released the moment somebody re-sorted, typed in the search box or
// pressed a tile, handing back the edit modal, Delete and Retest on the very rows the
// request was still writing to. The modal is a full-record editor seeded from the pre-bulk
// snapshot, so the save that followed put the old status and severity straight back over
// the bulk write.
//
// An empty capture collapses to null rather than an empty Set, so "is anything in flight"
// stays a single null check and a call that somehow sent no ids cannot freeze the table.
export function bulkLockFor(ids) {
    const locked = new Set(Array.from(ids || []).filter(Boolean));
    return locked.size > 0 ? locked : null;
}

// isBulkLocked is the question every per-defect write on this page asks IMMEDIATELY BEFORE it
// issues its request: is a bulk apply currently writing to this defect?
//
// It is deliberately not phrased as "should this button be disabled". Disabling the button was
// tried — the checkbox, then the row's Open detail / Delete / Retest, then keying those on the
// captured ids — and each round a different way to the same write was found, because a
// DISABLED BUTTON ONLY GUARDS THE PATH THAT GOES THROUGH IT. The one that finally had no
// button at all: a dialog opened BEFORE the apply started. Nothing was locked when it opened,
// its Save sits inside an overlay with no focus containment (see hooks/useDialogFocus), and
// the bulk bar stays in the DOM behind it and is reachable by Tab. So the guard belongs on the
// write, where every path has to pass through it, and the hazard is total: DefectModal is a
// full-record editor seeded from the PRE-bulk snapshot, so its PATCH puts the old status and
// severity straight back over the bulk write.
//
// `lock` is bulkLockFor's output — a Set of captured ids, or null when nothing is in flight.
export function isBulkLocked(lock, id) {
    if (!lock || !id || typeof lock.has !== 'function') return false;
    return lock.has(id);
}

// isEditSnapshotStale is the question DefectModal's PATCH asks — a strictly WIDER one than
// isBulkLocked, and the difference is where the sixth hole was.
//
// isBulkLocked is about a request being in flight, so it stops being true the moment that
// request lands. A dialog's SNAPSHOT does not work that way: the form was seeded when the
// dialog opened, a bulk apply then rewrote the row underneath it, and the form still holds the
// pre-bulk values afterwards. Guarding only the in-flight window therefore just moved the
// overwrite later — wait for the apply to finish, press Save on the dialog that is still
// standing, and its full-record PATCH puts the old status and severity back exactly as before,
// with no refusal shown at all because nothing is locked any more.
//
// So staleness is remembered rather than sampled: `touched` is the set of ids a bulk apply has
// written to since this dialog opened. It does not expire — only opening a dialog clears it,
// because a dialog opened AFTER an apply is seeded from the row that apply produced.
//
// Deliberately NOT used for Delete or Retest: neither carries a snapshot, so neither can write
// stale values back, and refusing them after an apply had finished would be a lie.
export function isEditSnapshotStale(lock, touched, id) {
    if (isBulkLocked(lock, id)) return true;
    if (!id || !touched || typeof touched.has !== 'function') return false;
    return touched.has(id);
}

// rememberBulkTouched is the other side of isEditSnapshotStale: what a landed bulk apply has to
// leave behind for the dialog standing over it. Deliberately NARROWER than "every id that was
// applied".
//
// The record exists for exactly one thing — an edit dialog opened BEFORE an apply still holds
// the pre-bulk snapshot after that apply lands, and its full-record PATCH would put the old
// values back. That can only be true of the defect a dialog is open on RIGHT NOW: with no
// dialog up there is no snapshot to refuse, and the next dialog is seeded from the row the
// apply produced. Recording every applied id was correct but unbounded — only OPENING a dialog
// cleared the set, so a session that spent the afternoon doing bulk applies and never opened
// one grew it for the life of the page.
//
// `editingId` is the open edit dialog's defect, or falsy when no edit dialog is up. Mutates and
// returns `touched` — it is the page's ref, and nothing re-renders off it.
export function rememberBulkTouched(touched, editingId, requestedIds) {
    if (!touched || typeof touched.add !== 'function' || !editingId) return touched;
    for (const id of requestedIds || []) {
        if (id === editingId) {
            touched.add(id);
            break;
        }
    }
    return touched;
}

// What a refused write says. One string, because all three refusals (save, delete, retest) are
// the same fact and a silent no-op is what the guard must never be — the modal's own
// `.catch(() => {})` already swallows enough.
export const BULK_LOCK_MESSAGE =
    'A bulk update is being applied to this defect. Wait for it to finish, then reopen this — saving now would undo it.';

// applyBulkResult folds a bulk-update response back into the page's row list.
//
// Two things happen at once, and only one of them is obvious. Every id the server DID return
// is replaced outright by the row it sent back — bulk-update enriches its rows exactly like
// GET /defects (assignee_name AND linked_test_count), so nothing has to be re-merged by hand.
// Every id that was REQUESTED but is missing from the response is dropped instead: the endpoint
// tolerates unknown ids rather than rejecting the call (they simply match nothing), so an id
// that comes back missing is a defect somebody else deleted since this page loaded. Left in
// place it would sit there unchanged after a "successful" apply, telling the user they had just
// closed something that no longer exists.
//
// Rows outside `requestedIds` are untouched: they were never part of the call, and their absence
// from the response says nothing at all about them.
export function applyBulkResult(rows, updated, requestedIds) {
    const list = Array.isArray(rows) ? rows : [];
    const byID = new Map((Array.isArray(updated) ? updated : []).map(d => [d.id, d]));
    const requested = new Set(Array.from(requestedIds || []));
    if (requested.size === 0) return list;

    const next = [];
    for (const row of list) {
        const replacement = byID.get(row.id);
        if (replacement) {
            next.push(replacement);
            continue;
        }
        // Requested, not returned — gone server-side.
        if (requested.has(row.id)) continue;
        next.push(row);
    }
    return next;
}

// assigneeOptions is the list an assignee picker offers below its static "Unassigned"
// entry: the assignable users, labelled display-name-else-email the way AssigneePicker
// labels them.
//
// The one non-obvious part is the first entry. A defect can be owned by someone who has
// since been deactivated or deleted, and /users/assignable will not return them — so a
// picker built from that list alone would show the current owner as "Unassigned" and
// silently drop them on the next save. Their resolved assignee_name (which survives
// deactivation, see store.populateDefectAssigneeNames) is prepended instead, so saving an
// unrelated field cannot un-assign a defect behind the user's back.
export function assigneeOptions(users, defect) {
    const options = [];
    const seen = new Set();
    for (const user of users || []) {
        if (!user || !user.id || seen.has(user.id)) continue;
        seen.add(user.id);
        options.push({ id: user.id, label: user.display_name || user.email || user.id });
    }

    const current = defect && defect.assignee_id;
    if (current && !seen.has(current)) {
        options.unshift({ id: current, label: (defect.assignee_name || current), inactive: true });
    }
    return options;
}

// buildDefectPayload shapes DefectModal's body for POST /defects and PATCH /defects/{id},
// or null when the title is blank — the one required field, and the same condition that
// disables the Save button.
//
// Unlike buildBulkPayload, every text key is always present: the modal is a full-record
// editor, so clearing the external key has to be sent as "" to actually clear it.
//
// assignee_id is the single exception, and `original` (the defect being edited, absent on
// create) is what drives it. A defect can be owned by someone who has since been
// deactivated; assigneeOptions keeps them in the picker so an unrelated save cannot
// silently unassign them — but the server's ValidateAssignee rejects any non-empty id that
// is not an active user, so echoing that id back turned every edit of such a defect into a
// 400 the modal could not explain. An UNCHANGED assignee is therefore omitted, which is
// exactly PATCH's "leave this field alone". Any real change is still sent, UNASSIGNED ("")
// included, because null would decode to nil and change nothing.
export function buildDefectPayload(form, original) {
    const source = form || {};
    const title = String(source.title || '').trim();
    if (!title) return null;

    const text = (value) => String(value || '').trim();
    const payload = {
        title,
        description: text(source.description),
        severity: source.severity || '',
        status: source.status || '',
        external_provider: text(source.external_provider),
        external_key: text(source.external_key),
        external_url: text(source.external_url),
    };

    const assignee = source.assignee_id ? String(source.assignee_id) : UNASSIGNED;
    const current = original && original.assignee_id ? String(original.assignee_id) : UNASSIGNED;
    if (!original || assignee !== current) payload.assignee_id = assignee;
    return payload;
}

// A defect title is allowed up to 500 chars; a run name that long would wreck the run
// list, so the retest run carries a truncated one.
export const RETEST_TITLE_MAX = 80;

// buildRetestRun turns a defect and its affected tests into the body for POST /runs, or
// null when there is nothing to retest — which is also what disables the Retest button.
// No category_id: the run is defined by its explicit test_case_ids, and the endpoint
// rejects a body carrying both.
export function buildRetestRun(defect, affectedTests) {
    if (!defect) return null;

    const testCaseIds = [];
    const seen = new Set();
    for (const test of affectedTests || []) {
        const id = test && test.id;
        if (!id || seen.has(id)) continue;
        seen.add(id);
        testCaseIds.push(id);
    }
    if (testCaseIds.length === 0) return null;

    const title = String(defect.title || '').trim();
    const short = title.length > RETEST_TITLE_MAX
        ? `${title.slice(0, RETEST_TITLE_MAX - 1).trimEnd()}…`
        : title;
    return { name: short ? `Retest: ${short}` : 'Retest', test_case_ids: testCaseIds };
}
