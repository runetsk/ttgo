import React from 'react';
import { m } from './styles';

/* ── Delete Confirm Modal ──────────────────────────── */
export default function DeleteModal({ provider, onCancel, onConfirm }) {
    return (
        <div onClick={onCancel} style={m.backdrop}>
            <div onClick={e => e.stopPropagation()} style={{ ...m.modal, maxWidth: 380 }}>
                <div style={{ padding: '28px 28px 24px' }}>
                    <div style={m.deleteIcon}>
                        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/>
                        </svg>
                    </div>
                    <h3 style={{ margin: '0 0 8px', fontSize: '1rem', fontWeight: 700 }}>Delete Provider</h3>
                    <p style={{ margin: '0 0 24px', fontSize: '0.875rem', color: 'var(--text-secondary)', lineHeight: 1.6 }}>
                        Delete <strong style={{ color: 'var(--text-primary)' }}>{provider.label}</strong>? This action cannot be undone.
                    </p>
                    <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
                        <button className="action-btn" onClick={onCancel}>Cancel</button>
                        <button
                            className="primary-btn"
                            style={{ background: 'var(--accent-red)', boxShadow: '0 4px 12px rgba(239,68,68,0.25)' }}
                            onClick={onConfirm}
                        >
                            Delete
                        </button>
                    </div>
                </div>
            </div>
        </div>
    );
}
