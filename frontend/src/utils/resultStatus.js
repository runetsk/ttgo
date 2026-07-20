// One definition of "is this run result a failure?" for the whole frontend.
// Mirrors the backend's models.IsFailureStatus (FAIL, ERROR) — the failing-results
// queries there already select `status IN ('FAIL','ERROR')`.
//
// This exists because the check used to be written inline in five places and one
// of them drifted to FAIL-only, which silently hid the defect_type control (and
// the AI suggestion chip) on ERROR rows. Add no new inline variants; import this.
//
// NOTE: this is the RESULT-level status set. The RUN-level statuses
// (RUNNING/PASS/FAIL, no ERROR) are a different concept — do not apply this there.
const FAILURE_STATUSES = new Set(['FAIL', 'ERROR']);

export function isFailureStatus(status) {
    return FAILURE_STATUSES.has(status);
}
