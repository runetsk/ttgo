import React, { useState } from 'react';
import { resultDefects } from '../api';

const SEVERITIES = ['critical', 'major', 'minor', 'trivial'];

export default function CreateDefectModal({ runId, resultId, testName, errorMessage, stackTrace, onClose, onCreated }) {
    const [title, setTitle] = useState(testName ? `[Defect] ${testName}` : '');
    const [description, setDescription] = useState(() => {
        const actual = errorMessage || stackTrace || '';
        return actual ? `Actual result:\n${actual}` : '';
    });
    const [severity, setSeverity] = useState('minor');
    const [externalUrl, setExternalUrl] = useState('');
    const [externalKey, setExternalKey] = useState('');
    const [submitting, setSubmitting] = useState(false);

    const handleSubmit = (e) => {
        e.preventDefault();
        if (!title.trim()) return;
        setSubmitting(true);
        resultDefects.create(runId, resultId, {
            title: title.trim(), description: description.trim(), severity,
            external_url: externalUrl.trim(), external_key: externalKey.trim(),
            external_provider: externalUrl.trim() ? 'External' : '',
        })
            .then(({ defect }) => { onCreated?.(defect); onClose(); })
            .catch(() => {})
            .finally(() => setSubmitting(false));
    };

    return (
        <div style={{ position: 'fixed', inset: 0, zIndex: 1000, background: 'rgba(0,0,0,0.6)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}
            onClick={e => { if (e.target === e.currentTarget) onClose(); }}>
            <div className="glass-panel" style={{ width: '100%', maxWidth: 520, padding: 24, borderRadius: 10 }}>
                <h3 style={{ margin: '0 0 16px', fontSize: '1rem' }}>New Defect</h3>
                <form onSubmit={handleSubmit}>
                    <label style={labelStyle}>Title *</label>
                    <input className="modern-input" style={{ width: '100%', marginBottom: 12 }} value={title} onChange={e => setTitle(e.target.value)} required disabled={submitting} />
                    <label style={labelStyle}>Description</label>
                    <textarea className="modern-input" style={{ width: '100%', marginBottom: 12, minHeight: 90, resize: 'vertical' }} value={description} onChange={e => setDescription(e.target.value)} disabled={submitting} />
                    <label style={labelStyle}>Severity</label>
                    <select className="modern-input" style={{ width: '100%', marginBottom: 12 }} value={severity} onChange={e => setSeverity(e.target.value)} disabled={submitting}>
                        {SEVERITIES.map(s => <option key={s} value={s}>{s}</option>)}
                    </select>
                    <label style={labelStyle}>External link (optional)</label>
                    <div style={{ display: 'flex', gap: 8, marginBottom: 12 }}>
                        <input className="modern-input" style={{ flex: '0 0 130px' }} placeholder="Key e.g. PROJ-1" value={externalKey} onChange={e => setExternalKey(e.target.value)} disabled={submitting} />
                        <input className="modern-input" style={{ flex: 1 }} placeholder="https://…" value={externalUrl} onChange={e => setExternalUrl(e.target.value)} disabled={submitting} />
                    </div>
                    <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 8 }}>
                        <button type="button" className="action-btn" onClick={onClose} disabled={submitting}>Cancel</button>
                        <button type="submit" className="primary-btn" disabled={submitting || !title.trim()} style={{ opacity: submitting ? 0.6 : 1 }}>
                            {submitting ? 'Creating…' : 'Create Defect'}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    );
}

const labelStyle = { display: 'block', fontSize: '0.78rem', color: 'var(--text-secondary)', marginBottom: 4, textTransform: 'uppercase', letterSpacing: '0.04em' };
