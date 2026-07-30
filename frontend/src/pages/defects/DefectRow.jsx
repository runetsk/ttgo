import { memo, useCallback, useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { defects as defectsApi } from '../../api';
import {
    ageLabel,
    deriveStatus,
    isStale,
    ownerInitials,
    ownerLabel,
    statusLabel,
} from '../../utils/defectQueue';

// One queue row and the panel it expands into. Everything shown is derived by
// utils/defectQueue — this file only places it, the way TriageStrip and FilterBar
// render tables they do not own.
//
// The row owns the fetch for its own affected tests, the way BulkBar owns its
// bulk call: the expand is the only thing that needs them, and "the request
// failed" then displays next to the panel that asked for it. The loaded tests
// are handed back up through onRetest, because the run POST and the navigation
// that follows belong to the page.

// select · defect · status · owner · impact · age. The mockup's ID column is
// gone (a truncated UUID told nobody anything); external_key is a chip instead.
const COLUMN_COUNT = 6;

function formatDate(iso) {
    if (!iso) return 'an unknown date';
    const parsed = new Date(iso);
    return Number.isNaN(parsed.getTime()) ? 'an unknown date' : parsed.toLocaleDateString();
}

// ExecutionStatus is stored upper-case ("FAIL" / "ERROR"), which reads as shouting
// in a sentence — the rest of the page lower-cases severity and status the same way.
function resultWord(status) {
    return status ? String(status).toLowerCase() : 'failed';
}

function DefectRow({
    defect,
    now,
    selected = false,
    // True while ANY bulk apply is in flight — table-wide, because that call captured its
    // ids at click time and ticking any row at all moves the set it is writing to.
    selectionLocked = false,
    // True only for the rows THAT call captured: the ones actually being written to.
    actionsLocked = false,
    expanded = false,
    focused = false,
    // One inline message for this row's expand panel: a Retest that could not start, or a write
    // the page refused because a bulk apply is mid-flight on this defect.
    notice = '',
    onToggleSelect,
    onToggleExpand,
    onOpenDetail,
    onRetest,
    onDelete,
}) {
    const [affected, setAffected] = useState(null); // null until the expand has loaded them
    const [affectedFailed, setAffectedFailed] = useState(false);
    // The in-flight guard is a ref, not `affected`: `affected` stays null for the
    // whole request, so it could never have stopped a collapse-and-re-expand from
    // firing a second GET the way the old comment here claimed.
    const inFlight = useRef(false);

    const loadAffected = useCallback(() => {
        if (inFlight.current) return;
        inFlight.current = true;
        defectsApi.affectedTests(defect.id)
            .then(rows => { setAffected(rows || []); setAffectedFailed(false); })
            // Deliberately NOT setAffected([]): leaving it null is what lets a
            // re-expand (or Retry) try again, instead of pinning "couldn't load"
            // and a disabled Retest button for the life of the row.
            .catch(() => setAffectedFailed(true))
            .finally(() => { inFlight.current = false; });
    }, [defect.id]);

    // Retry clears the failure itself rather than loadAffected doing it on entry, the
    // way BulkBar's loadUsers can: this loader is also called from the effect below, and
    // `react-hooks/set-state-in-effect` forbids a setState reachable synchronously from
    // an effect body. A click handler is not an effect, so the reset lives here — without
    // it, pressing Retry left the same "Couldn't load…" on screen for the whole round-trip
    // and produced no visible change at all when the retry failed too.
    // Unguarded on purpose: if a load is already in flight, loadAffected self-guards and
    // that request still resolves into the "Loading…" this just put back.
    const retryAffected = () => {
        setAffectedFailed(false);
        loadAffected();
    };

    useEffect(() => {
        if (!expanded || affected !== null) return;
        loadAffected();
    }, [expanded, affected, loadAffected]);

    // Every write this row owns is frozen while a bulk apply is writing to THIS row.
    //
    // Scoped to the captured rows on purpose, unlike the table-wide checkbox freeze. The
    // checkbox is locked everywhere because what must hold still there is the id SET —
    // ticking any row moves the selection the in-flight call captured. What must hold still
    // here is the DEFECT, and the call only writes to the ids it captured: a row outside
    // them is not being changed by it, so freezing its actions would cost the user a
    // working modal for no hazard at all.
    //
    // Which is why `actionsLocked` is handed down from those captured ids rather than
    // derived here from `selected && selectionLocked`. That derivation was the bug: no
    // filter control is locked during an apply and every one of them clears the selection
    // by design (DefectsPage.afterFilterChange), so one re-sort, keystroke or tile click
    // mid-flight emptied `selected` and released this freeze on the rows that were still
    // being written to.
    //
    // The hazard on a captured row is total, not partial: `defect` is the row as it was
    // BEFORE the bulk write, DefectModal seeds its form from that snapshot, and
    // buildDefectPayload deliberately sends the full record on save (it is a full-record
    // editor — clearing a field has to be sent as ""). So a save that lands after the
    // bulk PATCHes the pre-bulk status and severity straight back over it, and the user's
    // "Mark verified & close" silently reverts for exactly that row. Delete and Retest are
    // locked with it: one destroys the row the call is mid-write on, the other navigates
    // away to a new run, which unmounts the page before the apply can report at all.
    //
    // These disabled buttons are the AFFORDANCE, not the guarantee. A dialog opened before the
    // apply started never passes through any of them, so the page also refuses each of those
    // three writes at the call itself (DefectsPage's isLocked / DefectModal.submit). Do not
    // treat this lock as the thing that keeps the register safe.
    const lockedTitle = actionsLocked ? 'A bulk update is being applied to this defect' : undefined;

    const status = deriveStatus(defect);
    const severity = defect.severity || '';
    const stale = isStale(defect, now);
    const assigned = Boolean(defect.assignee_id);
    const tests = affected || [];

    const rowClass = ['defects-row',
        severity && `defects-sev--${severity}`,
        selected && 'defects-row--selected',
        focused && 'defects-row--focused',
    ].filter(Boolean).join(' ');

    // Enter/Space expands, but only from the row itself — the checkbox inside it
    // has its own Space handling and must not toggle both.
    const handleKeyDown = (event) => {
        if (event.target !== event.currentTarget) return;
        if (event.key !== 'Enter' && event.key !== ' ') return;
        event.preventDefault();
        onToggleExpand?.(defect);
    };

    return (
        <>
            <tr
                id={`defect-row-${defect.id}`}
                className={rowClass}
                data-testid="defects-row"
                tabIndex={0}
                aria-expanded={expanded}
                onClick={() => onToggleExpand?.(defect)}
                onKeyDown={handleKeyDown}
            >
                <td className="defects-cell-select">
                    {/* Severity from the corner of the eye, before any text is read. */}
                    <span className="defects-sev-bar" aria-hidden="true" />
                    <input
                        type="checkbox"
                        className="defects-checkbox"
                        checked={selected}
                        disabled={selectionLocked}
                        aria-label={`Select ${defect.title}`}
                        data-testid="defects-row-select"
                        // Ticking a row must not also expand it.
                        onClick={event => event.stopPropagation()}
                        onChange={() => onToggleSelect?.(defect.id)}
                    />
                </td>

                <td>
                    <div className="defects-title-cell">
                        <span className="defects-defect-title" title={defect.title}>{defect.title}</span>
                        {severity && <span className="defects-sev-pill">{severity}</span>}
                        {defect.external_url ? (
                            <a
                                className="defects-key-chip"
                                href={defect.external_url}
                                target="_blank"
                                rel="noopener noreferrer"
                                onClick={event => event.stopPropagation()}
                            >
                                {defect.external_key || 'link'} ↗
                            </a>
                        ) : defect.external_key && (
                            <span className="defects-key-chip">{defect.external_key}</span>
                        )}
                    </div>
                </td>

                {/* Static text, never a button: the old pill wrote on click, so one
                    mis-click closed a live defect with no undo. Status changes now
                    go through the modal or the bulk bar. */}
                <td>
                    <span className={`defects-status-pill defects-status--${status}`}>
                        <span className="defects-status-dot" aria-hidden="true" />
                        {statusLabel(status)}
                    </span>
                </td>

                <td>
                    <span className={`defects-owner${assigned ? '' : ' defects-owner--empty'}`}>
                        <span
                            className={`defects-avatar${assigned ? '' : ' defects-avatar--empty'}`}
                            aria-hidden="true"
                        >
                            {assigned ? (ownerInitials(defect.assignee_name) || '?') : ''}
                        </span>
                        {ownerLabel(defect)}
                    </span>
                </td>

                <td>
                    <span className="defects-impact">
                        <svg
                            className="defects-impact-icon"
                            width="12"
                            height="12"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            strokeWidth="2"
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            aria-hidden="true"
                        >
                            <polyline points="9 11 12 14 22 4" />
                            <path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11" />
                        </svg>
                        {defect.linked_test_count ?? 0}
                    </span>
                </td>

                {/* Age is time since creation; the amber is staleness, a different
                    fact — nothing has happened to this defect in a week. */}
                <td
                    className={`defects-age${stale ? ' defects-age--stale' : ''}`}
                    title={`${stale ? 'No update since' : 'Last updated'} ${formatDate(defect.updated_at)}`}
                >
                    {ageLabel(defect, now)}
                </td>
            </tr>

            {expanded && (
                <tr data-testid="defects-expand">
                    <td className="defects-expand-cell" colSpan={COLUMN_COUNT}>
                        <div className="defects-expand">
                            <div className="defects-expand-col defects-expand-col--tests">
                                <span className="defects-expand-label">Affected tests</span>
                                {affected === null && !affectedFailed && (
                                    <span className="defects-affected-empty">Loading…</span>
                                )}
                                {affectedFailed && (
                                    <span className="defects-affected-empty">
                                        Couldn&apos;t load the affected tests.{' '}
                                        <button
                                            type="button"
                                            className="defects-inline-retry"
                                            onClick={retryAffected}
                                            data-testid="defects-affected-retry"
                                        >
                                            Retry
                                        </button>
                                    </span>
                                )}
                                {affected !== null && !affectedFailed && tests.length === 0 && (
                                    <span className="defects-affected-empty">No linked tests.</span>
                                )}
                                {tests.map(test => (
                                    <div key={test.id} className="defects-affected-row">
                                        <span className="defects-affected-dot" aria-hidden="true" />
                                        <Link className="defects-affected-link" to={`/library/tests/${test.id}`}>
                                            {test.name}
                                        </Link>
                                        {/* Where it last went red — the run to open to see the
                                            failure, or to compare a retest against. */}
                                        {test.last_run_id && (
                                            <Link
                                                className="defects-affected-link defects-affected-run"
                                                to={`/runs/run/${test.last_run_id}`}
                                                title={`Last ${resultWord(test.last_result_status)} in ${test.last_run_name || 'this run'}`}
                                            >
                                                {test.last_run_name || 'run'}
                                            </Link>
                                        )}
                                    </div>
                                ))}
                            </div>

                            <div className="defects-expand-col defects-expand-col--summary">
                                <span className="defects-expand-label">Summary</span>
                                {defect.description
                                    ? <span className="defects-expand-summary">{defect.description}</span>
                                    : <span className="defects-affected-empty">No description.</span>}
                            </div>

                            <div className="defects-expand-col defects-expand-col--actions">
                                <span className="defects-expand-label">Actions</span>
                                <div className="defects-expand-actions">
                                    <button
                                        type="button"
                                        className="defects-ghost-btn"
                                        disabled={actionsLocked}
                                        title={lockedTitle}
                                        onClick={() => onOpenDetail?.(defect)}
                                        data-testid="defects-open-detail"
                                    >
                                        Open detail
                                    </button>
                                    <button
                                        type="button"
                                        className="defects-ghost-btn"
                                        disabled={actionsLocked || tests.length === 0}
                                        title={lockedTitle || (tests.length === 0 ? 'No linked tests to retest' : undefined)}
                                        onClick={() => onRetest?.(defect, tests)}
                                        data-testid="defects-retest"
                                    >
                                        Retest
                                    </button>
                                    {/* The table itself has no destructive control — deleting is
                                        deliberately two steps now (expand, then confirm), where the
                                        old page put a × on every row next to a status toggle. */}
                                    {onDelete && (
                                        <button
                                            type="button"
                                            className="defects-ghost-btn defects-ghost-btn--danger"
                                            disabled={actionsLocked}
                                            title={lockedTitle}
                                            onClick={() => onDelete(defect)}
                                            data-testid="defects-delete"
                                        >
                                            Delete
                                        </button>
                                    )}
                                </div>
                                {notice && <span className="defects-bulkbar-error" role="alert" data-testid="defects-row-notice">{notice}</span>}
                            </div>
                        </div>
                    </td>
                </tr>
            )}
        </>
    );
}

// Memoised because the page re-derives the whole visible list on every keystroke:
// without this, typing in the search box re-renders every row still on screen.
// Every handler the page passes down is a useCallback, so the props really are
// stable between those renders.
export default memo(DefectRow);
