import { useEffect, useState } from 'react';
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

export default function DefectRow({
    defect,
    now,
    selected = false,
    expanded = false,
    focused = false,
    retestError = '',
    onToggleSelect,
    onToggleExpand,
    onOpenDetail,
    onRetest,
}) {
    const [affected, setAffected] = useState(null); // null until the expand has loaded them
    const [affectedFailed, setAffectedFailed] = useState(false);

    // Fires once per row: the guard holds while the request is in flight, and the
    // deps only change when the row is first expanded.
    useEffect(() => {
        if (!expanded || affected !== null) return undefined;
        let cancelled = false;
        defectsApi.affectedTests(defect.id)
            .then(rows => { if (!cancelled) setAffected(rows || []); })
            .catch(() => { if (!cancelled) { setAffected([]); setAffectedFailed(true); } });
        return () => { cancelled = true; };
    }, [expanded, affected, defect.id]);

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
                                {affected === null && <span className="defects-affected-empty">Loading…</span>}
                                {affectedFailed && (
                                    <span className="defects-affected-empty">Couldn&apos;t load the affected tests.</span>
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
                                                title={`Last ${test.last_result_status || 'failed'} in ${test.last_run_name || 'this run'}`}
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
                                        onClick={() => onOpenDetail?.(defect)}
                                        data-testid="defects-open-detail"
                                    >
                                        Open detail
                                    </button>
                                    <button
                                        type="button"
                                        className="defects-ghost-btn"
                                        disabled={tests.length === 0}
                                        title={tests.length === 0 ? 'No linked tests to retest' : undefined}
                                        onClick={() => onRetest?.(defect, tests)}
                                        data-testid="defects-retest"
                                    >
                                        Retest
                                    </button>
                                </div>
                                {retestError && <span className="defects-bulkbar-error" role="alert">{retestError}</span>}
                            </div>
                        </div>
                    </td>
                </tr>
            )}
        </>
    );
}
