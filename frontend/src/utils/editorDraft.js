// Draft/baseline logic for the record editors (TestCaseDetail, RequirementDetailPage).
// No React, no network — the only testable seam in this repo (no component-render
// harness). The editors keep an immutable saved *baseline* and a separately editable
// *draft*; dirty state is derived from the editable fields only, and an incoming
// server refresh (re-fetch or WebSocket) must never clobber unsaved work.
//
// Snapshots reduce a record to just the editable fields in a stable, comparable
// shape: set-like associations (categories, custom values) are sorted so reordering
// is not an edit, while steps keep their array order because step order is meaningful.

function byId(a, b) {
    if (a.id < b.id) return -1;
    if (a.id > b.id) return 1;
    return 0;
}

export function testCaseSnapshot(state = {}) {
    const steps = Array.isArray(state.steps) ? state.steps : [];
    const categories = Array.isArray(state.categories) ? state.categories : [];
    const customValues = Array.isArray(state.customValues) ? state.customValues : [];
    return {
        name: state.name ?? '',
        description: state.description ?? '',
        // Position is the only step identity that matters to the editor; id and
        // order_index are volatile (new steps have none), so exclude them.
        steps: steps.map((s) => ({
            action: s?.action ?? '',
            expected_result: s?.expected_result ?? '',
        })),
        categoryIds: categories.map((c) => c?.id).filter((id) => id != null).sort(),
        customValues: customValues
            .map((cv) => ({ id: cv?.custom_field_id, value: cv?.value ?? '' }))
            .filter((cv) => cv.id != null)
            .sort(byId),
    };
}

export function requirementSnapshot(state = {}) {
    return {
        identifier: state.identifier ?? '',
        title: state.title ?? '',
        description: state.description ?? '',
    };
}

// Deep structural equality for the JSON-serialisable snapshots above: arrays compare
// positionally (order matters), objects compare by key set (key order does not).
export function snapshotsEqual(a, b) {
    if (a === b) return true;
    if (a === null || b === null || typeof a !== 'object' || typeof b !== 'object') {
        return Object.is(a, b);
    }
    const aArr = Array.isArray(a);
    const bArr = Array.isArray(b);
    if (aArr !== bArr) return false;
    if (aArr) {
        if (a.length !== b.length) return false;
        for (let i = 0; i < a.length; i += 1) {
            if (!snapshotsEqual(a[i], b[i])) return false;
        }
        return true;
    }
    const aKeys = Object.keys(a);
    const bKeys = Object.keys(b);
    if (aKeys.length !== bKeys.length) return false;
    for (const k of aKeys) {
        if (!Object.prototype.hasOwnProperty.call(b, k)) return false;
        if (!snapshotsEqual(a[k], b[k])) return false;
    }
    return true;
}

export function isDirty(baselineSnap, draftSnap) {
    return !snapshotsEqual(baselineSnap, draftSnap);
}

// reconcileRefresh decides what an incoming server record should do to an open editor.
//   adopt=true  → the caller replaces baseline+draft with the server record (safe refresh)
//   adopt=false → the caller keeps the user's draft; externallyChanged says whether the
//                 server copy actually differs from the baseline the draft was built on,
//                 so the UI can show "This record changed. Your unsaved edits are kept."
// A refresh is only adopted when the editor is clean AND no save is in flight.
export function reconcileRefresh({ baselineSnap, draftSnap, saving, serverSnap }) {
    if (!isDirty(baselineSnap, draftSnap) && !saving) {
        return { adopt: true, externallyChanged: false };
    }
    return { adopt: false, externallyChanged: !snapshotsEqual(baselineSnap, serverSnap) };
}

// parseEditConflict recognises the server's optimistic-concurrency rejection: a
// 409 whose body carries code "edit_conflict" and the latest record under `key`
// ("test" or "requirement"). Any other error (including a 409 for a duplicate
// identifier) returns null so the caller falls through to normal error handling.
export function parseEditConflict(err, key) {
    const res = err && err.response;
    if (!res || res.status !== 409) return null;
    const data = res.data || {};
    if (data.code !== 'edit_conflict') return null;
    return data[key] || null;
}
