import React, { useMemo } from 'react';
import { AIC } from './constants';
import { AIBtn } from './primitives';
import { buildDraftDiff } from '../../utils/draftDiff';

function DiffText({ parts }) {
    if (!parts) return null;
    return (
        <span>
            {parts.map((p, i) => {
                if (p.added) {
                    return <span key={i} style={{ background: 'var(--aig-success-bg)', color: 'var(--aig-success-fg)', borderRadius: 3 }}>{p.value}</span>;
                }
                if (p.removed) {
                    return <span key={i} style={{ background: 'var(--aig-danger-bg)', color: 'var(--aig-danger-fg)', textDecoration: 'line-through', borderRadius: 3 }}>{p.value}</span>;
                }
                return <span key={i}>{p.value}</span>;
            })}
        </span>
    );
}

const STEP_LABEL = { changed: '±', added: '+', removed: '−', unchanged: '=' };

export function DraftCompareModal({ original, alternative, onChoose, onClose }) {
    const diff = useMemo(
        () => (original && alternative ? buildDraftDiff(original, alternative) : null),
        [original, alternative]
    );
    if (!diff) return null;
    return (
        <div onClick={onClose} style={{
            position: 'fixed', inset: 0, zIndex: 60, background: 'var(--aig-modal-backdrop)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
        }}>
            <div onClick={e => e.stopPropagation()} data-testid="draft-compare" style={{
                width: 560, maxHeight: '80vh', overflowY: 'auto', background: 'var(--bg-secondary)',
                borderRadius: 12, border: `1px solid ${AIC.border}`, padding: 18, boxShadow: 'var(--shadow-md)',
            }}>
                <h3 style={{ margin: '0 0 4px', fontSize: 14, color: AIC.text }}>Compare versions</h3>
                <p style={{ margin: '0 0 12px', fontSize: 11.5, color: AIC.muted }}>
                    v{original.version} (current) → v{alternative.version} (regenerated). Choosing keeps one and marks the other superseded.
                </p>

                <div style={{ fontSize: 12.5, color: AIC.dim, lineHeight: 1.6 }}>
                    <div style={{ marginBottom: 8 }}>
                        <b style={{ color: AIC.text }}>Name:</b> <DiffText parts={diff.name} />
                    </div>
                    {diff.category.changed && (
                        <div style={{ marginBottom: 8 }}>
                            <b style={{ color: AIC.text }}>Category:</b> {diff.category.from} → {diff.category.to}
                        </div>
                    )}
                    <div style={{ marginBottom: 8 }}>
                        <b style={{ color: AIC.text }}>Description:</b> <DiffText parts={diff.description} />
                    </div>
                    {(diff.sourceRefs.added.length > 0 || diff.sourceRefs.removed.length > 0) && (
                        <div style={{ marginBottom: 8 }}>
                            <b style={{ color: AIC.text }}>Refs:</b>{' '}
                            {diff.sourceRefs.added.map(rr => <span key={rr} style={{ color: 'var(--aig-success-fg)', marginRight: 6 }}>+{rr}</span>)}
                            {diff.sourceRefs.removed.map(rr => <span key={rr} style={{ color: 'var(--aig-danger-fg)', marginRight: 6 }}>−{rr}</span>)}
                        </div>
                    )}
                    <b style={{ color: AIC.text }}>Steps:</b>
                    {diff.steps.map(st => (
                        <div key={st.index} style={{
                            display: 'flex', gap: 8, padding: '5px 8px', marginTop: 4,
                            border: `1px solid ${AIC.border}`, borderRadius: 6,
                            opacity: st.type === 'unchanged' ? 0.55 : 1, background: 'var(--aig-surface-tint)',
                        }}>
                            <span style={{ fontWeight: 700, color: AIC.muted }}>{STEP_LABEL[st.type]} #{st.index + 1}</span>
                            <div style={{ flex: 1 }}>
                                {st.type === 'unchanged' ? (
                                    <span style={{ color: AIC.muted }}>unchanged</span>
                                ) : (
                                    <>
                                        <div><DiffText parts={st.action} /></div>
                                        <div style={{ color: AIC.muted }}><DiffText parts={st.expected} /></div>
                                    </>
                                )}
                            </div>
                        </div>
                    ))}
                </div>

                <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 14 }}>
                    <AIBtn onClick={() => onChoose(original.id)}>Keep original</AIBtn>
                    <AIBtn variant="success" onClick={() => onChoose(alternative.id)}>Use new version</AIBtn>
                </div>
            </div>
        </div>
    );
}
