import React from 'react';
import { AIC } from './constants';
import { AIBtn } from './primitives';

export function BudgetConfirmModal({ warning, onConfirm, onClose }) {
    if (!warning) return null;
    return (
        <div onClick={onClose} style={{
            position: 'fixed', inset: 0, zIndex: 60, background: 'var(--aig-modal-backdrop)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
        }}>
            <div onClick={e => e.stopPropagation()} data-testid="budget-confirm" style={{
                width: 400, background: 'var(--bg-secondary)', borderRadius: 12,
                border: `1px solid ${AIC.border}`, padding: 18, boxShadow: 'var(--shadow-md)',
            }}>
                <h3 style={{ margin: '0 0 8px', fontSize: 14, color: 'var(--aig-tone-amber-fg)' }}>Budget warning</h3>
                <p style={{ margin: '0 0 12px', fontSize: 12.5, color: AIC.dim, lineHeight: 1.6 }}>{warning.error}</p>
                <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
                    <button className="action-btn" onClick={onClose}>Cancel</button>
                    <AIBtn variant="primary" onClick={onConfirm}>Proceed anyway</AIBtn>
                </div>
            </div>
        </div>
    );
}
