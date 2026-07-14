/* eslint-disable react-refresh/only-export-components -- co-exports the REJECTION_REASONS taxonomy constant with the RejectPopover component; they're one conceptual unit and splitting would add a file for no runtime benefit */
import React, { useEffect, useState } from 'react';
import { AIC } from './constants';
import { AIBtn } from './primitives';

// Mirrors backend models.AIRejectionReasons — do not add keys the API rejects.
export const REJECTION_REASONS = [
    { key: 'duplicate', label: 'Duplicate' },
    { key: 'irrelevant', label: 'Irrelevant' },
    { key: 'incorrect', label: 'Incorrect' },
    { key: 'too_vague', label: 'Too vague' },
    { key: 'incomplete_coverage', label: 'Incomplete coverage' },
    { key: 'poor_steps', label: 'Poor steps' },
    { key: 'other', label: 'Other' },
];

export function RejectPopover({ open, onClose, onSubmit }) {
    const [reason, setReason] = useState('duplicate');
    const [note, setNote] = useState('');

    useEffect(() => {
        if (!open) return undefined;
        const onKey = (e) => { if (e.key === 'Escape') onClose(); };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
    }, [open, onClose]);

    if (!open) return null;
    return (
        <div onClick={onClose} style={{
            position: 'fixed', inset: 0, zIndex: 60, background: 'var(--aig-modal-backdrop)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
        }}>
            <div onClick={e => e.stopPropagation()} data-testid="reject-popover" style={{
                width: 360, background: 'var(--bg-secondary)', borderRadius: 12,
                border: `1px solid ${AIC.border}`, padding: 16, boxShadow: 'var(--shadow-md)',
            }}>
                <h3 style={{ margin: '0 0 10px', fontSize: 13.5, color: AIC.text }}>Reject draft</h3>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginBottom: 10 }}>
                    {REJECTION_REASONS.map(rr => (
                        <button key={rr.key} type="button" onClick={() => setReason(rr.key)} style={{
                            padding: '3px 10px', borderRadius: 999, fontSize: 11, cursor: 'pointer',
                            border: `1px solid ${reason === rr.key ? 'var(--aig-danger-border)' : AIC.border}`,
                            background: reason === rr.key ? 'var(--aig-danger-bg)' : 'transparent',
                            color: reason === rr.key ? 'var(--aig-danger-fg)' : AIC.dim,
                        }}>{rr.label}</button>
                    ))}
                </div>
                <textarea className="modern-input" rows={2} maxLength={2000}
                    style={{ width: '100%', resize: 'vertical', fontSize: 12 }}
                    placeholder="Optional note" value={note} onChange={e => setNote(e.target.value)} />
                <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 12 }}>
                    <button className="action-btn" onClick={onClose}>Cancel</button>
                    <AIBtn variant="danger" onClick={() => { onSubmit(reason, note); onClose(); }}>Reject</AIBtn>
                </div>
            </div>
        </div>
    );
}
