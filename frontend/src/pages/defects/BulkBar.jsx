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

export default function BulkBar({ selectedIds, onApplied, onClear }) {
    const [menu, setMenu] = useState(null); // 'assign' | 'severity' | null
    const [users, setUsers] = useState(null); // null until the assign menu has loaded them
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');
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

    const toggleMenu = (name) => setMenu(current => (current === name ? null : name));

    // Users load when the Assign menu is first opened rather than on mount: the bar mounts
    // and unmounts with every selection, and most selections never open the menu.
    const openAssign = async () => {
        const opening = menu !== 'assign';
        toggleMenu('assign');
        if (!opening || users !== null || loadingUsers.current) return;
        loadingUsers.current = true;
        try {
            setUsers(await getAssignableUsers());
        } catch {
            // api.js already toasted; say so inline too, so an empty menu is not read as
            // "nobody can be assigned".
            setUsers([]);
            setError('Could not load the assignable users.');
        } finally {
            loadingUsers.current = false;
        }
    };

    const apply = async (fields) => {
        const payload = buildBulkPayload(ids, fields);
        if (!payload || busy) return;
        setMenu(null);
        setBusy(true);
        setError('');
        try {
            const updated = await defectsApi.bulkUpdate(payload.ids, payload);
            onApplied?.(Array.isArray(updated) ? updated : []);
        } catch {
            // The interceptor's toast disappears; this stays next to the selection it
            // failed on, and the selection is kept so the action can simply be retried.
            setError('Bulk update failed — nothing was changed.');
        } finally {
            setBusy(false);
        }
    };

    if (ids.length === 0) return null;

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
                        {users === null && <p className="defects-menu-note">Loading users…</p>}
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

            {error && <span className="defects-bulkbar-error" role="alert">{error}</span>}

            <button
                type="button"
                className="defects-bulk-clear defects-bulkbar-spacer"
                onClick={() => onClear?.()}
                data-testid="defects-bulk-clear"
            >
                Clear
            </button>
        </div>
    );
}
