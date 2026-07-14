import React from 'react';
import { AIC } from './constants';
import { AIBtn } from './primitives';

// Two-phase modal. plan = {clean, excludedInvalid, excludedDuplicates, overriddenWarnings}
// result = accept response {created_ids, count, subfolders_created} | null (confirm phase)
export function CommitSummaryModal({ plan, result, onConfirm, onClose }) {
    if (!plan) return null;
    return (
        <div onClick={onClose} style={{
            position: 'fixed', inset: 0, zIndex: 60, background: 'var(--aig-modal-backdrop)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
        }}>
            <div onClick={e => e.stopPropagation()} data-testid="commit-summary" style={{
                width: 420, background: 'var(--bg-secondary)', borderRadius: 12,
                border: `1px solid ${AIC.border}`, padding: 18, boxShadow: 'var(--shadow-md)',
            }}>
                {!result ? (
                    <>
                        <h3 style={{ margin: '0 0 10px', fontSize: 14, color: AIC.text }}>Accept all clean drafts</h3>
                        <ul style={{ margin: 0, paddingLeft: 18, fontSize: 12.5, color: AIC.dim, lineHeight: 1.7 }}>
                            <li><b style={{ color: 'var(--aig-success-fg)' }}>{plan.clean.length}</b> clean draft(s) will become test cases</li>
                            {plan.overriddenWarnings > 0 && (
                                <li><b style={{ color: 'var(--aig-tone-amber-fg)' }}>{plan.overriddenWarnings}</b> of them carry quality warnings (accepted anyway)</li>
                            )}
                            {plan.excludedInvalid > 0 && <li><b>{plan.excludedInvalid}</b> invalid draft(s) excluded</li>}
                            {plan.excludedDuplicates > 0 && <li><b>{plan.excludedDuplicates}</b> high-confidence duplicate(s) excluded</li>}
                        </ul>
                        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 14 }}>
                            <button className="action-btn" onClick={onClose}>Cancel</button>
                            <AIBtn variant="success" onClick={onConfirm} disabled={!plan.clean.length}>
                                Accept {plan.clean.length} draft(s)
                            </AIBtn>
                        </div>
                    </>
                ) : (
                    <>
                        <h3 style={{ margin: '0 0 10px', fontSize: 14, color: AIC.text }}>Commit summary</h3>
                        <ul style={{ margin: 0, paddingLeft: 18, fontSize: 12.5, color: AIC.dim, lineHeight: 1.7 }}>
                            <li><b style={{ color: 'var(--aig-success-fg)' }}>{result.count}</b> test case(s) created and linked to the requirement</li>
                            <li><b>{result.subfolders_created}</b> category subfolder(s) created</li>
                            {plan.overriddenWarnings > 0 && <li>{plan.overriddenWarnings} accepted with overridden warnings</li>}
                        </ul>
                        <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 14 }}>
                            <AIBtn onClick={onClose}>Done</AIBtn>
                        </div>
                    </>
                )}
            </div>
        </div>
    );
}
