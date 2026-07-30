import React, { useEffect, useState } from 'react';
import { defects as defectsApi, getAssignableUsers } from '../api';
import useDialogFocus from '../hooks/useDialogFocus';
import { assigneeOptions, buildDefectPayload, BULK_LOCK_MESSAGE } from '../utils/defectActions';
import { DEFECT_STATUS_OPTIONS } from '../utils/defectQueue';

const SEVERITIES = ['critical', 'major', 'minor', 'trivial'];

/**
 * DefectModal — create or edit a native defect. Reached from the Defects register's
 * "+ New defect" and from a row expand's "Open detail".
 * Props: mode ('create'|'edit'), defect (object, edit only), isSnapshotStale(id) (optional predicate,
 * see submit), onClose(), onSaved(saved).
 * Render with a `key` (e.g. defect id or 'create') so state re-initializes per target.
 */
export default function DefectModal({ mode = 'create', defect = null, isSnapshotStale, onClose, onSaved }) {
    const [title, setTitle] = useState(defect?.title || '');
    const [description, setDescription] = useState(defect?.description || '');
    const [severity, setSeverity] = useState(defect?.severity || 'minor');
    const [status, setStatus] = useState(defect?.status || 'open');
    const [assigneeId, setAssigneeId] = useState(defect?.assignee_id || '');
    const [users, setUsers] = useState(null); // null until the assignable list has loaded
    const [provider, setProvider] = useState(defect?.external_provider || '');
    const [extKey, setExtKey] = useState(defect?.external_key || '');
    const [extUrl, setExtUrl] = useState(defect?.external_url || '');
    const [submitting, setSubmitting] = useState(false);
    // Why the last Save did nothing, when the guard below refused it. Never silent: the request
    // path's own .catch already swallows failures, and a Save button that visibly does nothing
    // is exactly how the overwrite this guard prevents used to go unnoticed.
    const [refused, setRefused] = useState('');
    const dialogRef = useDialogFocus();

    // Loaded on mount rather than on first open, unlike BulkBar's menu: this modal only
    // exists while someone is editing one defect, and the picker is on screen the whole time.
    useEffect(() => {
        let cancelled = false;
        getAssignableUsers()
            .then(list => { if (!cancelled) setUsers(list || []); })
            .catch(() => { if (!cancelled) setUsers([]); }); // api.js has already toasted
        return () => { cancelled = true; };
    }, []);

    const submit = (e) => {
        e.preventDefault();
        // The write refuses ITSELF when a bulk apply has invalidated this form's snapshot,
        // rather than trusting that the button which opened this dialog was disabled. A dialog
        // opened BEFORE that apply started was opened while nothing was locked, and neither the
        // overlay nor the page behind it stops the bulk bar from being reached afterwards — so
        // the disabled "Open detail" button is not on the path this save took. Nothing below
        // this line may run: the form is seeded from the PRE-bulk snapshot and sends the full
        // record, so the PATCH would put the old status and severity back over the bulk write.
        //
        // The question is "is my snapshot stale", NOT "is a request in flight" — the two come
        // apart the moment the apply lands, and guarding only the flight simply moved this
        // overwrite to the other side of it. See utils/defectActions isEditSnapshotStale.
        if (mode === 'edit' && isSnapshotStale?.(defect?.id)) {
            setRefused(BULK_LOCK_MESSAGE);
            return;
        }
        setRefused('');
        // buildDefectPayload sends every field — this form is a full-record editor — with
        // one exception: an assignee that has not been touched is omitted, so editing a
        // defect owned by a since-deactivated user cannot be rejected for echoing that
        // owner's id back. `defect` is what tells it "unchanged" from "cleared".
        const payload = buildDefectPayload({
            title, description, severity, status, assignee_id: assigneeId,
            external_provider: provider, external_key: extKey, external_url: extUrl,
        }, mode === 'edit' ? defect : null);
        if (!payload) return;
        setSubmitting(true);
        const req = mode === 'edit' ? defectsApi.update(defect.id, payload) : defectsApi.create(payload);
        req.then(saved => { onSaved?.(saved); onClose(); })
            .catch(() => {})
            .finally(() => setSubmitting(false));
    };

    return (
        <div style={overlay} onClick={e => { if (e.target === e.currentTarget) onClose(); }}>
            {/* role/aria-modal say the page behind is unreachable; useDialogFocus is what makes
                that true for the keyboard. tabIndex={-1} lets the panel hold focus itself while
                a submitting form has every control disabled. */}
            <div
                ref={dialogRef}
                tabIndex={-1}
                role="dialog"
                aria-modal="true"
                aria-labelledby="defect-modal-title"
                data-testid="defect-dialog"
                className="glass-panel"
                style={{ width: '100%', maxWidth: 560, padding: '22px 26px 24px', borderRadius: 12, background: 'var(--bg-secondary)', border: '1px solid var(--border-color)', outline: 'none' }}
            >
                <h3 id="defect-modal-title" style={{ margin: '0 0 14px', paddingBottom: 12, borderBottom: '1px solid var(--border-color)', fontSize: '1.05rem', fontWeight: 700 }}>
                    {mode === 'edit' ? 'Edit Defect' : 'New Defect'}
                </h3>
                <form onSubmit={submit}>
                    <label style={lbl}>Title <span style={{ color: '#ef4444' }}>*</span></label>
                    <input className="modern-input" style={inp} value={title} onChange={e => setTitle(e.target.value)} required disabled={submitting} autoFocus />

                    <label style={lbl}>Description</label>
                    <textarea className="modern-input" style={{ ...inp, minHeight: 84, resize: 'vertical', lineHeight: 1.5 }} value={description} onChange={e => setDescription(e.target.value)} disabled={submitting} placeholder="What's wrong, steps to reproduce, expected vs actual…" />

                    <div style={{ display: 'flex', gap: 12 }}>
                        <div style={{ flex: 1 }}>
                            <label style={lbl}>Severity</label>
                            <select className="modern-input" style={inp} value={severity} onChange={e => setSeverity(e.target.value)} disabled={submitting} data-testid="defect-severity">
                                {SEVERITIES.map(s => <option key={s} value={s}>{s}</option>)}
                            </select>
                        </div>
                        <div style={{ flex: 1 }}>
                            <label style={lbl}>Status</label>
                            {/* Stored statuses, not the queue's four derived ones: Needs triage
                                vs In progress follows from ownership, so it is the Assignee
                                field below that moves a defect between those two. */}
                            <select className="modern-input" style={inp} value={status} onChange={e => setStatus(e.target.value)} disabled={submitting}>
                                {DEFECT_STATUS_OPTIONS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
                            </select>
                        </div>
                    </div>

                    <label style={lbl} htmlFor="defect-assignee">Assignee</label>
                    <select
                        id="defect-assignee"
                        className="modern-input"
                        style={inp}
                        value={assigneeId}
                        onChange={e => setAssigneeId(e.target.value)}
                        disabled={submitting || users === null}
                        data-testid="defect-assignee"
                    >
                        <option value="">Unassigned</option>
                        {/* An owner who is no longer assignable is kept in the list so an
                            unrelated save cannot drop them, but they must not read as an
                            ordinary colleague someone could pick on purpose. */}
                        {assigneeOptions(users, defect).map(o => (
                            <option key={o.id} value={o.id}>{o.inactive ? `${o.label} (inactive)` : o.label}</option>
                        ))}
                    </select>

                    <label style={lbl}>External link <span style={{ color: 'var(--text-secondary)', fontWeight: 400, textTransform: 'none', letterSpacing: 0 }}>(optional)</span></label>
                    <div style={{ display: 'flex', gap: 8, marginBottom: 8 }}>
                        <input className="modern-input" style={{ ...inp, flex: 1 }} placeholder="Provider (e.g. Jira)" value={provider} onChange={e => setProvider(e.target.value)} disabled={submitting} />
                        <input className="modern-input" style={{ ...inp, flex: 1 }} placeholder="Key (e.g. PROJ-1)" value={extKey} onChange={e => setExtKey(e.target.value)} disabled={submitting} />
                    </div>
                    <input className="modern-input" style={inp} placeholder="https://…" value={extUrl} onChange={e => setExtUrl(e.target.value)} disabled={submitting} />

                    {refused && (
                        <p
                            className="defects-bulkbar-error"
                            role="alert"
                            style={{ display: 'block', margin: '16px 0 0' }}
                            data-testid="defect-refused"
                        >
                            {refused}
                        </p>
                    )}

                    <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 22, paddingTop: 16, borderTop: '1px solid var(--border-color)' }}>
                        <button type="button" className="action-btn" onClick={onClose} disabled={submitting}>Cancel</button>
                        <button type="submit" className="primary-btn" disabled={submitting || !title.trim()} style={{ opacity: submitting ? 0.6 : 1 }} data-testid="defect-save">
                            {submitting ? 'Saving…' : (mode === 'edit' ? 'Save' : 'Create')}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    );
}

const overlay = { position: 'fixed', inset: 0, zIndex: 1000, background: 'rgba(0,0,0,0.6)', display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 16 };
const lbl = { display: 'block', fontSize: '0.72rem', fontWeight: 600, color: 'var(--text-primary)', margin: '14px 0 5px', textTransform: 'uppercase', letterSpacing: '0.03em' };
const inp = { width: '100%' };
