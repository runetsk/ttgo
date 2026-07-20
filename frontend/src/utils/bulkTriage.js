// Pure decision layer for the bulk "Set defect type" action in the run-detail bulk bar.
// The component owns the network call, the toast and the selection state; everything that
// can be decided without React lives here, because this repo has no component-render test
// infrastructure — a pure helper is the only testable seam.

// The canonical defect types, in the same order as the per-row <select> in ResultsTab.
// Mirrors the backend's models.ValidDefectTypes (minus "", which only the non-failure
// clear path writes and which is never a user choice here).
export const BULK_DEFECT_TYPE_OPTIONS = [
    { value: 'to_investigate', label: '🔍 To Investigate' },
    { value: 'product_bug', label: '🐞 Product Bug' },
    { value: 'automation_bug', label: '🤖 Automation Bug' },
    { value: 'system_issue', label: '⚙️ System Issue' },
];

// buildBulkDefectTypePayload returns the request body for a triage-mode bulk update, or
// null when the action must not fire at all (nothing selected, or an unknown defect type).
// A null return is also what drives the picker's enabled state, so the two can never
// disagree.
//
// ⚠️ Triage mode is defined by the ABSENCE of `status`. Never add a status key here: that
// would switch the endpoint to Mode 1, which applies the status to every selected row and
// would overwrite the status of the PASS rows in a mixed selection.
export function buildBulkDefectTypePayload(selectedIds, defectType) {
    const resultIds = Array.from(selectedIds || []);
    if (resultIds.length === 0) return null;
    if (!BULK_DEFECT_TYPE_OPTIONS.some(o => o.value === defectType)) return null;
    return { result_ids: resultIds, defect_type: defectType };
}

// summarizeBulkTriage turns the endpoint's {updated, skipped} into the toast to show.
// Skipped rows are reported rather than dropped silently: triaging a mixed selection is
// the normal way to use the picker, so "these 3 weren't failures" is information the user
// needs to trust the result — otherwise the action looks like it partly failed.
export function summarizeBulkTriage(response) {
    const updated = Number(response?.updated) || 0;
    const skipped = Number(response?.skipped) || 0;
    const skippedText = `${skipped} skipped (not failures)`;
    if (updated === 0 && skipped === 0) return { tone: 'info', message: 'Nothing updated' };
    if (updated === 0) return { tone: 'warning', message: `Nothing updated — ${skippedText}` };
    if (skipped === 0) return { tone: 'success', message: `${updated} updated` };
    return { tone: 'warning', message: `${updated} updated, ${skippedText}` };
}
