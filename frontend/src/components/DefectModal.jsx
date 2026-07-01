import React, { useState } from 'react';
import { defects as defectsApi } from '../api';

const SEVERITIES = ['critical', 'major', 'minor', 'trivial'];

/**
 * DefectModal — create or edit a native defect.
 * Props: mode ('create'|'edit'), defect (object, edit only), onClose(), onSaved(saved).
 * Render with a `key` (e.g. defect id or 'create') so state re-initializes per target.
 */
export default function DefectModal({ mode = 'create', defect = null, onClose, onSaved }) {
    const [title, setTitle] = useState(defect?.title || '');
    const [description, setDescription] = useState(defect?.description || '');
    const [severity, setSeverity] = useState(defect?.severity || 'minor');
    const [status, setStatus] = useState(defect?.status || 'open');
    const [provider, setProvider] = useState(defect?.external_provider || '');
    const [extKey, setExtKey] = useState(defect?.external_key || '');
    const [extUrl, setExtUrl] = useState(defect?.external_url || '');
    const [submitting, setSubmitting] = useState(false);

    const submit = (e) => {
        e.preventDefault();
        if (!title.trim()) return;
        setSubmitting(true);
        const payload = {
            title: title.trim(), description: description.trim(), severity, status,
            external_provider: provider.trim(), external_key: extKey.trim(), external_url: extUrl.trim(),
        };
        const req = mode === 'edit' ? defectsApi.update(defect.id, payload) : defectsApi.create(payload);
        req.then(saved => { onSaved?.(saved); onClose(); })
            .catch(() => {})
            .finally(() => setSubmitting(false));
    };

    return (
        <div style={overlay} onClick={e => { if (e.target === e.currentTarget) onClose(); }}>
            <div className="glass-panel" style={{ width: '100%', maxWidth: 560, padding: '22px 26px 24px', borderRadius: 12, background: 'var(--bg-secondary)', border: '1px solid var(--border-color)' }}>
                <h3 style={{ margin: '0 0 14px', paddingBottom: 12, borderBottom: '1px solid var(--border-color)', fontSize: '1.05rem', fontWeight: 700 }}>
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
                            <select className="modern-input" style={inp} value={severity} onChange={e => setSeverity(e.target.value)} disabled={submitting}>
                                {SEVERITIES.map(s => <option key={s} value={s}>{s}</option>)}
                            </select>
                        </div>
                        <div style={{ flex: 1 }}>
                            <label style={lbl}>Status</label>
                            <select className="modern-input" style={inp} value={status} onChange={e => setStatus(e.target.value)} disabled={submitting}>
                                <option value="open">open</option>
                                <option value="closed">closed</option>
                            </select>
                        </div>
                    </div>

                    <label style={lbl}>External link <span style={{ color: 'var(--text-secondary)', fontWeight: 400, textTransform: 'none', letterSpacing: 0 }}>(optional)</span></label>
                    <div style={{ display: 'flex', gap: 8, marginBottom: 8 }}>
                        <input className="modern-input" style={{ ...inp, flex: 1 }} placeholder="Provider (e.g. Jira)" value={provider} onChange={e => setProvider(e.target.value)} disabled={submitting} />
                        <input className="modern-input" style={{ ...inp, flex: 1 }} placeholder="Key (e.g. PROJ-1)" value={extKey} onChange={e => setExtKey(e.target.value)} disabled={submitting} />
                    </div>
                    <input className="modern-input" style={inp} placeholder="https://…" value={extUrl} onChange={e => setExtUrl(e.target.value)} disabled={submitting} />

                    <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 22, paddingTop: 16, borderTop: '1px solid var(--border-color)' }}>
                        <button type="button" className="action-btn" onClick={onClose} disabled={submitting}>Cancel</button>
                        <button type="submit" className="primary-btn" disabled={submitting || !title.trim()} style={{ opacity: submitting ? 0.6 : 1 }}>
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
