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
