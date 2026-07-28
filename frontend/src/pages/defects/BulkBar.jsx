import { useEffect, useRef, useState } from 'react';
import { defects as defectsApi, getAssignableUsers } from '../../api';
import { buildBulkPayload, UNASSIGNED } from '../../utils/defectActions';
import { SEVERITY_ORDER } from '../../utils/defectQueue';

// The bar that appears once rows are ticked: assign, re-severity or close the whole
// selection in one round-trip instead of one PATCH per defect.
//
// Every payload is built by buildBulkPayload (utils/defectActions), which is where the
// wire shape is unit-tested — this file never assembles a body by hand. The bar also owns
// the call itself, the way AssigneePicker owns its assign: that keeps "the call failed"
// and "the failure is on screen" in one place instead of depending on the page to reject.
// The page gets the server's updated rows through onApplied and patches them into state.

// SEVERITY_ORDER carries the stored (lower-case) values; the chips capitalise in CSS but
// a menu item is plain text, so the label is built here.
function severityLabel(severity) {
    return severity.charAt(0).toUpperCase() + severity.slice(1);
}

export default function BulkBar({ selectedIds, onApplied, onClear, onHeightChange, onBusyChange }) {
    const [menu, setMenu] = useState(null); // 'assign' | 'severity' | null
    const [users, setUsers] = useState(null); // null until the assign menu has loaded them
    const [usersFailed, setUsersFailed] = useState(false);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState(null); // { epoch, message } | null — see below
    const barRef = useRef(null);
    const loadingUsers = useRef(false);

    // Close the open menu on an outside click or Escape — same handling as ColumnPicker.
    useEffect(() => {
        if (!menu) return undefined;
        const onDown = (e) => { if (!barRef.current?.contains(e.target)) setMenu(null); };
        const onKey = (e) => { if (e.key === 'Escape') setMenu(null); };
        document.addEventListener('mousedown', onDown);
        document.addEventListener('keydown', onKey);
        return () => {
            document.removeEventListener('mousedown', onDown);
            document.removeEventListener('keydown', onKey);
        };
    }, [menu]);

    const ids = Array.from(selectedIds || []);
    // Identifies WHICH rows are selected, not the order they were ticked in.
    const selectionKey = ids.slice().sort().join(',');

    // A failure belongs to the selection it happened on, and to that one visit to it.
    // The page renders this bar unconditionally and it only returns null on an empty
    // selection, so the component never unmounts and the message would otherwise outlive
    // that selection: press Clear after a failed apply, tick a different set of rows, and
    // the old failure would be sitting over a selection nothing has been tried on.
    //
    // Hence an EPOCH rather than a comparison against the id set: every change to the
    // selection mints a new one, including the trip through empty that Clear makes, so
    // re-ticking byte-for-byte the same rows is still a new selection and not the one the
    // failure happened on. The bump is React's adjust-state-during-render pattern rather
    // than an effect: it re-renders before the browser paints, so a stale message is never
    // on screen for a frame.
    const [mark, setMark] = useState({ key: selectionKey, epoch: 0 });
    if (mark.key !== selectionKey) setMark(prev => ({ key: selectionKey, epoch: prev.epoch + 1 }));

    // Rendering off the epoch is also what makes a LATE failure safe: an apply that is
    // still in flight when the selection moves resolves against an epoch that has already
    // been retired, so its message is simply never shown. A guard on the write could not
    // do this — by then the closure only knows the selection it was fired on.
    const shownError = error && error.epoch === mark.epoch ? error.message : '';

    // The bar is out of flow (position: absolute), so it takes no space of its own and
    // the scroller has to buy its height back as extra scroll range — otherwise the last
    // row sits behind the bar with nowhere left to scroll. That height is MEASURED rather
    // than assumed: the bar wraps, and an inline failure message takes it from one 43px
    // row to three (132px at a 680px viewport) and further still below that — a range no
    // constant in the stylesheet covers. See .defects-bulk-spacer in App.css.
    const hasSelection = ids.length > 0;
    useEffect(() => {
        const node = barRef.current;
        if (!node || typeof ResizeObserver === 'undefined') return undefined;
        const observer = new ResizeObserver(() => onHeightChange?.(node.offsetHeight));
        observer.observe(node);
        return () => observer.disconnect();
    }, [hasSelection, onHeightChange]);

    const toggleMenu = (name) => setMenu(current => (current === name ? null : name));

    const loadUsers = async () => {
        if (loadingUsers.current) return;
        loadingUsers.current = true;
        setUsersFailed(false); // back to "Loading users…" while the retry is in flight
        try {
            setUsers(await getAssignableUsers() || []);
        } catch {
            // Deliberately NOT setUsers([]) — the same call DefectRow's affected-tests
            // load makes and for the same reason: `users !== null` is the load guard, so
            // pinning it to [] would turn one transient failure into "No assignable
            // users." for the life of the page, with no way back but a reload. Left null,
            // both the Retry below and simply re-opening the menu try again.
            setUsersFailed(true);
        } finally {
            loadingUsers.current = false;
        }
    };

    // Users load when the Assign menu is first opened rather than on mount: most
    // selections never open the menu.
    const openAssign = () => {
        const opening = menu !== 'assign';
        toggleMenu('assign');
        if (!opening || users !== null) return;
        loadUsers();
    };

    const apply = async (fields) => {
        const payload = buildBulkPayload(ids, fields);
        if (!payload || busy) return;
        const epoch = mark.epoch; // the selection this call is being fired on
        setMenu(null);
        setBusy(true);
        // Reported upward so the page can freeze the row checkboxes and select-all for
        // the round-trip: `ids` is captured here, and a selection that moves under an
        // in-flight call both mis-attributes its failure and loses whatever the user
        // built when the success handler clears. The bar's own buttons are covered by
        // `busy` below — Clear included, or the one control that empties the selection
        // would still be live while the request runs.
        onBusyChange?.(true);
        setError(null);
        try {
            const updated = await defectsApi.bulkUpdate(payload.ids, payload);
            onApplied?.(Array.isArray(updated) ? updated : []);
        } catch (err) {
            // The interceptor's toast disappears; this stays next to the selection it
            // failed on — which is what the epoch is for: the write is unconditional and
            // the render decides whether that selection is still the one on screen.
            // The server's own reason is kept when there is one — "too many ids (max 500
            // per request)" is actionable, a generic failure notice is not.
            const detail = err?.response?.data?.error;
            setError({
                epoch,
                message: detail ? `Bulk update failed — ${detail}.` : 'Bulk update failed — nothing was changed.',
            });
        } finally {
            setBusy(false);
            onBusyChange?.(false);
        }
    };

    if (!hasSelection) return null;

    return (
        <div className="defects-bulkbar" ref={barRef} role="group" aria-label="Bulk actions" data-testid="defects-bulk-bar">
            <span className="defects-bulkbar-label">{ids.length} selected</span>
            <span className="defects-bulkbar-divider" aria-hidden="true" />

            <div className="defects-menu-wrap">
                <button
                    type="button"
                    className="defects-ghost-btn"
                    aria-haspopup="true"
                    aria-expanded={menu === 'assign'}
                    disabled={busy}
                    onClick={openAssign}
                    data-testid="defects-bulk-assign"
                >
                    Assign…
                </button>
                {menu === 'assign' && (
                    <div className="defects-menu" role="group" aria-label="Assign the selection">
                        {users === null && !usersFailed && <p className="defects-menu-note">Loading users…</p>}
                        {usersFailed && (
                            <p className="defects-menu-note">
                                Couldn&apos;t load the assignable users.{' '}
                                <button
                                    type="button"
                                    className="defects-inline-retry"
                                    onClick={loadUsers}
                                    data-testid="defects-bulk-users-retry"
                                >
                                    Retry
                                </button>
                            </p>
                        )}
                        {users !== null && users.length === 0 && (
                            <p className="defects-menu-note">No assignable users.</p>
                        )}
                        {(users || []).map(user => (
                            <button
                                key={user.id}
                                type="button"
                                className="defects-menu-item"
                                onClick={() => apply({ assignee_id: user.id })}
                            >
                                {user.display_name || user.email}
                            </button>
                        ))}
                        <div className="defects-menu-divider" />
                        <button
                            type="button"
                            className="defects-menu-item"
                            onClick={() => apply({ assignee_id: UNASSIGNED })}
                            data-testid="defects-bulk-unassign"
                        >
                            Unassign
                        </button>
                        {/* Assignment IS the triage action here, so taking the owner away is
                            not a no-op — it pushes an open defect back into Needs triage. */}
                        <p className="defects-menu-note">
                            Unassigning sends an open defect back to Needs triage.
                        </p>
                    </div>
                )}
            </div>

            <div className="defects-menu-wrap">
                <button
                    type="button"
                    className="defects-ghost-btn"
                    aria-haspopup="true"
                    aria-expanded={menu === 'severity'}
                    disabled={busy}
                    onClick={() => toggleMenu('severity')}
                    data-testid="defects-bulk-severity"
                >
                    Set severity…
                </button>
                {menu === 'severity' && (
                    <div className="defects-menu" role="group" aria-label="Set the selection's severity">
                        {SEVERITY_ORDER.map(severity => (
                            <button
                                key={severity}
                                type="button"
                                className="defects-menu-item"
                                onClick={() => apply({ severity })}
                            >
                                {severityLabel(severity)}
                            </button>
                        ))}
                    </div>
                )}
            </div>

            <button
                type="button"
                className="defects-ghost-btn defects-ghost-btn--confirm"
                disabled={busy}
                onClick={() => apply({ status: 'closed' })}
                data-testid="defects-bulk-close"
            >
                Mark verified &amp; close
            </button>

            {shownError && <span className="defects-bulkbar-error" role="alert">{shownError}</span>}

            <button
                type="button"
                className="defects-bulk-clear"
                disabled={busy}
                onClick={() => onClear?.()}
                data-testid="defects-bulk-clear"
            >
                Clear
            </button>
        </div>
    );
}
