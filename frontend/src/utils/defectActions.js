// Pure action layer for the Defects triage queue: the three decisions the page's write
// paths make before they touch the network — what a bulk update sends, what happens to a
// selection when the view changes, and what a Retest run is made of.
//
// Kept DOM-free and dependency-free, like utils/defectQueue.js and utils/bulkTriage.js:
// this repo has no component-render test infrastructure, so a pure helper is the only
// testable seam for these decisions.

// The only keys POST /defects/bulk-update understands (models.BulkUpdateDefectsRequest).
// Anything else in `fields` is dropped rather than forwarded: Go ignores unknown JSON
// keys, so a camelCase typo would otherwise ride along and silently write nothing.
const BULK_FIELD_KEYS = ['status', 'severity', 'assignee_id'];

// The wire value that clears an assignee. Absent means "leave it alone", so unassigning
// has to be an explicit empty string — null would decode to nil and change nothing.
export const UNASSIGNED = '';

// buildBulkPayload returns the body for POST /defects/bulk-update, or null when the action
// must not fire at all (nothing selected — the endpoint 400s on an empty id list).
//
// `fields` carries any of { status, severity, assignee_id }. Empty status/severity values
// are dropped, because the endpoint has no concept of clearing them. The assignee is the
// one field with a meaningful empty: a present-but-empty assignee_id (null or '') is
// normalised to UNASSIGNED, so a picker's "Unassign" option cannot be mistaken for
// "unchanged". To leave assignees alone, omit the key entirely.
export function buildBulkPayload(selectedIds, fields) {
    const ids = Array.from(new Set(Array.from(selectedIds || []).filter(Boolean)));
    if (ids.length === 0) return null;

    const source = fields || {};
    const payload = { ids };
    for (const key of BULK_FIELD_KEYS) {
        if (key === 'assignee_id') {
            // presence, not truthiness: '' and null both mean unassign here.
            if (source.assignee_id === undefined) continue;
            payload.assignee_id = source.assignee_id ? String(source.assignee_id) : UNASSIGNED;
            continue;
        }
        if (source[key]) payload[key] = source[key];
    }
    return payload;
}

// Page state changes that leave a selection valid. Everything else clears it.
//
// The list is deliberately a safe-list rather than a list of clearing dimensions: an
// unrecognised name then clears, which is the harmless direction. The failure this rule
// exists to prevent is a bulk close landing on rows the user filtered away and can no
// longer see, so guessing wrong must cost a re-tick, never a wrong write.
export const SELECTION_SAFE_CHANGES = ['expanded', 'focus', 'selection'];

// nextSelectionOnFilterChange returns the selection to keep after `changed` changed.
// Every filter-bar control clears it — query, status tab, severity chips and Sort alike.
// Sort is in there even though reordering hides nothing: "N selected" would otherwise
// refer to rows that have just moved out from under the user mid-scan.
//
// The current selection is returned by identity when it survives (and when it is already
// empty), so the page can assign the result back to state without forcing a re-render.
export function nextSelectionOnFilterChange(selection, changed) {
    const current = selection instanceof Set ? selection : new Set(selection || []);
    if (SELECTION_SAFE_CHANGES.includes(changed)) return current;
    return current.size === 0 ? current : new Set();
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
