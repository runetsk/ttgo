import React, { useMemo, useState, useEffect, useRef } from 'react';
import { AIC } from './constants';
import { AIBtn, Pill, Segmented } from './primitives';
import { buildDraftDiff, leftParts, rightParts } from '../../utils/draftDiff';

const STEP_LABEL = { changed: '±', added: '+', removed: '−', unchanged: '=' };
const COMPARE_VIEW_KEY = 'aig.compareView';

function readCompareView() {
    try { return localStorage.getItem(COMPARE_VIEW_KEY) === 'unified' ? 'unified' : 'split'; }
    catch { return 'split'; }
}

function DiffText({ parts }) {
    if (!parts) return null;
    return (
        <span>
            {parts.map((p, i) => {
                if (p.added) return <mark key={i} style={{ background: 'var(--aig-success-bg)', color: 'var(--aig-success-fg)', borderRadius: 3, padding: '0 1px' }}>{p.value}</mark>;
                if (p.removed) return <mark key={i} style={{ background: 'var(--aig-danger-bg)', color: 'var(--aig-danger-fg)', textDecoration: 'line-through', borderRadius: 3, padding: '0 1px' }}>{p.value}</mark>;
                return <span key={i}>{p.value}</span>;
            })}
        </span>
    );
}

function UnifiedView({ diff }) {
    return (
        <div style={{ fontSize: 12.5, color: AIC.dim, lineHeight: 1.6 }}>
            <div style={{ marginBottom: 8 }}><b style={{ color: AIC.text }}>Name:</b> <DiffText parts={diff.name} /></div>
            {diff.category.changed && (
                <div style={{ marginBottom: 8 }}><b style={{ color: AIC.text }}>Category:</b> {diff.category.from} → {diff.category.to}</div>
            )}
            <div style={{ marginBottom: 8 }}><b style={{ color: AIC.text }}>Description:</b> <DiffText parts={diff.description} /></div>
            {(diff.sourceRefs.added.length > 0 || diff.sourceRefs.removed.length > 0) && (
                <div style={{ marginBottom: 8 }}>
                    <b style={{ color: AIC.text }}>Refs:</b>{' '}
                    {diff.sourceRefs.added.map(r => <span key={`a-${r}`} style={{ color: 'var(--aig-success-fg)', marginRight: 6 }}>+{r}</span>)}
                    {diff.sourceRefs.removed.map(r => <span key={`r-${r}`} style={{ color: 'var(--aig-danger-fg)', marginRight: 6 }}>−{r}</span>)}
                </div>
            )}
            <b style={{ color: AIC.text }}>Steps:</b>
            {diff.steps.map(st => (
                st.type === 'unchanged' ? (
                    <div key={st.index} style={{ padding: '5px 8px', marginTop: 4, fontSize: 11.5, color: AIC.muted }}>
                        Step {st.index + 1} · unchanged
                    </div>
                ) : (
                    <div key={st.index} style={{
                        display: 'flex', gap: 8, padding: '5px 8px', marginTop: 4,
                        border: `1px solid ${AIC.border}`, borderRadius: 6, background: 'var(--aig-surface-tint)',
                    }}>
                        <span style={{ fontWeight: 700, color: AIC.muted }}>{STEP_LABEL[st.type]} #{st.index + 1}</span>
                        <div style={{ flex: 1 }}>
                            <div><DiffText parts={st.action} /></div>
                            <div style={{ color: AIC.muted }}><DiffText parts={st.expected} /></div>
                        </div>
                    </div>
                )
            ))}
        </div>
    );
}

// One side of a word-diff: highlight only THIS side's edits (removed on the
// left, added on the right). Pass parts already narrowed via leftParts/rightParts.
function SideDiffText({ parts, side }) {
    if (!parts) return null;
    return (
        <span>
            {parts.map((p, i) => {
                const hi = side === 'left' ? p.removed : p.added;
                if (!hi) return <span key={i}>{p.value}</span>;
                const style = side === 'left'
                    ? { background: 'var(--aig-danger-bg)', color: 'var(--aig-danger-fg)', textDecoration: 'line-through' }
                    : { background: 'var(--aig-success-bg)', color: 'var(--aig-success-fg)' };
                return <mark key={i} style={{ ...style, borderRadius: 3, padding: '0 1px' }}>{p.value}</mark>;
            })}
        </span>
    );
}

const STEP_CHIP = {
    changed: { tone: 'indigo', label: 'changed' },
    added: { tone: 'green', label: 'added' },
    removed: { tone: 'red', label: 'removed' },
};

function FieldLabel({ children }) {
    return (
        <div style={{
            fontSize: 10.5, textTransform: 'uppercase', letterSpacing: '0.08em',
            color: AIC.muted, fontWeight: 700, marginBottom: 6,
        }}>{children}</div>
    );
}

function UnchangedTag() {
    return (
        <span style={{
            marginLeft: 6, padding: '1px 6px', borderRadius: 999,
            background: 'var(--aig-surface-tint-strong)', color: AIC.muted,
            fontSize: 9.5, letterSpacing: 0, textTransform: 'none', fontWeight: 600,
        }}>unchanged</span>
    );
}

function SplitField({ label, parts, changed }) {
    const text = (parts || []).map(p => p.value).join('');
    if (!changed) {
        if (!text) return null;
        return (
            <div style={{ padding: '10px 0', borderBottom: `1px solid ${AIC.border}` }}>
                <FieldLabel>{label}<UnchangedTag /></FieldLabel>
                <div style={{ fontSize: 13, color: AIC.dim, lineHeight: 1.5 }}>{text}</div>
            </div>
        );
    }
    return (
        <div style={{ padding: '10px 0', borderBottom: `1px solid ${AIC.border}` }}>
            <FieldLabel>{label}</FieldLabel>
            <div className="aig-compare-cols" style={{
                display: 'grid', gridTemplateColumns: 'minmax(0,1fr) minmax(0,1fr)',
                gap: 14, fontSize: 13, lineHeight: 1.5,
            }}>
                <div style={{ color: AIC.dim }}><SideDiffText parts={leftParts(parts)} side="left" /></div>
                <div style={{ color: AIC.text }}><SideDiffText parts={rightParts(parts)} side="right" /></div>
            </div>
        </div>
    );
}

function StepRow({ step }) {
    if (step.type === 'unchanged') {
        return (
            <div style={{ padding: '7px 0', borderBottom: `1px solid ${AIC.border}`, fontSize: 11.5, color: AIC.muted }}>
                Step {step.index + 1} · unchanged
            </div>
        );
    }
    const chip = STEP_CHIP[step.type];
    const accent = step.type === 'added' ? 'var(--aig-success-border)'
        : step.type === 'removed' ? 'var(--aig-danger-border)' : AIC.indigo;
    return (
        <div style={{
            padding: '10px 0 10px 12px', borderBottom: `1px solid ${AIC.border}`,
            borderLeft: `2px solid ${accent}`,
        }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
                <span style={{
                    fontSize: 10.5, fontWeight: 700, color: AIC.muted,
                    textTransform: 'uppercase', letterSpacing: '0.05em',
                }}>Step {step.index + 1}</span>
                <Pill tone={chip.tone}>{chip.label}</Pill>
            </div>
            <div className="aig-compare-cols" style={{
                display: 'grid', gridTemplateColumns: 'minmax(0,1fr) minmax(0,1fr)',
                gap: 14, fontSize: 12.5, lineHeight: 1.5,
            }}>
                <div>
                    <div style={{ color: AIC.text }}><SideDiffText parts={leftParts(step.action)} side="left" /></div>
                    <div style={{ color: AIC.muted, marginTop: 4 }}><SideDiffText parts={leftParts(step.expected)} side="left" /></div>
                </div>
                <div>
                    <div style={{ color: AIC.text }}><SideDiffText parts={rightParts(step.action)} side="right" /></div>
                    <div style={{ color: AIC.muted, marginTop: 4 }}><SideDiffText parts={rightParts(step.expected)} side="right" /></div>
                </div>
            </div>
        </div>
    );
}

function SummaryBar({ summary }) {
    const chips = [];
    if (summary.nameChanged) chips.push({ tone: 'amber', label: 'Name changed' });
    if (summary.categoryChanged) chips.push({ tone: 'amber', label: 'Category changed' });
    if (summary.descriptionChanged) chips.push({ tone: 'amber', label: 'Description reworded' });
    const delta = summary.stepsChanged + summary.stepsAdded + summary.stepsRemoved;
    const total = delta + summary.stepsUnchanged;
    if (delta > 0) chips.push({ tone: 'indigo', label: `${delta} of ${total} steps changed` });
    if (summary.refsAdded > 0) chips.push({ tone: 'green', label: `+${summary.refsAdded} reference${summary.refsAdded > 1 ? 's' : ''}` });
    if (summary.refsRemoved > 0) chips.push({ tone: 'red', label: `−${summary.refsRemoved} reference${summary.refsRemoved > 1 ? 's' : ''}` });
    if (chips.length === 0) chips.push({ tone: 'neutral', label: 'No differences' });
    return (
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginBottom: 12 }}>
            {chips.map((c, i) => <Pill key={i} tone={c.tone}>{c.label}</Pill>)}
        </div>
    );
}

function SplitView({ diff, original, alternative }) {
    return (
        <div>
            <div className="aig-compare-cols" style={{
                display: 'grid', gridTemplateColumns: 'minmax(0,1fr) minmax(0,1fr)', gap: 14,
                position: 'sticky', top: 0, zIndex: 1, background: 'var(--bg-secondary)',
                padding: '4px 0 6px', fontSize: 10, textTransform: 'uppercase',
                letterSpacing: '0.08em', fontWeight: 700, borderBottom: `1px solid ${AIC.border}`,
            }}>
                <div style={{ color: AIC.muted }}>Original · v{original.version}</div>
                <div style={{ color: AIC.indigoSoft }}>New · v{alternative.version}</div>
            </div>

            <SplitField label="Name" parts={diff.name} changed={diff.summary.nameChanged} />

            {diff.category.changed && (
                <div style={{ padding: '10px 0', borderBottom: `1px solid ${AIC.border}` }}>
                    <FieldLabel>Category</FieldLabel>
                    <div className="aig-compare-cols" style={{
                        display: 'grid', gridTemplateColumns: 'minmax(0,1fr) minmax(0,1fr)', gap: 14, fontSize: 13,
                    }}>
                        <div style={{ color: AIC.dim }}>{diff.category.from}</div>
                        <div style={{ color: AIC.text }}>{diff.category.to}</div>
                    </div>
                </div>
            )}

            <SplitField label="Description" parts={diff.description} changed={diff.summary.descriptionChanged} />

            {(diff.sourceRefs.added.length > 0 || diff.sourceRefs.removed.length > 0) && (
                <div style={{ padding: '10px 0', borderBottom: `1px solid ${AIC.border}` }}>
                    <FieldLabel>Refs</FieldLabel>
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, fontSize: 12.5 }}>
                        {diff.sourceRefs.added.map(r => <span key={`a-${r}`} style={{ color: 'var(--aig-success-fg)' }}>+{r}</span>)}
                        {diff.sourceRefs.removed.map(r => <span key={`r-${r}`} style={{ color: 'var(--aig-danger-fg)' }}>−{r}</span>)}
                    </div>
                </div>
            )}

            <div style={{ marginTop: 12 }}>
                <FieldLabel>Steps</FieldLabel>
                {diff.steps.map(st => <StepRow key={st.index} step={st} />)}
            </div>
        </div>
    );
}

export function DraftCompareModal({ original, alternative, onChoose, onClose }) {
    const diff = useMemo(
        () => (original && alternative ? buildDraftDiff(original, alternative) : null),
        [original, alternative]
    );
    const [view, setView] = useState(readCompareView);
    const dialogRef = useRef(null);
    const restoreFocusRef = useRef(null);
    const onCloseRef = useRef(onClose);

    useEffect(() => {
        try { localStorage.setItem(COMPARE_VIEW_KEY, view); } catch { /* ignore */ }
    }, [view]);

    // Keep the latest onClose in a ref so the focus/Escape effect below can run
    // mount/unmount-only — the caller passes a new onClose closure every render,
    // and depending on it would tear down + re-run the effect (and churn focus)
    // on incidental parent re-renders.
    useEffect(() => { onCloseRef.current = onClose; });

    useEffect(() => {
        restoreFocusRef.current = document.activeElement;
        dialogRef.current?.focus();
        const onKey = (e) => { if (e.key === 'Escape') onCloseRef.current(); };
        document.addEventListener('keydown', onKey);
        return () => {
            document.removeEventListener('keydown', onKey);
            restoreFocusRef.current?.focus?.();
        };
    }, []);

    if (!diff) return null;
    return (
        <div onClick={onClose} style={{
            position: 'fixed', inset: 0, zIndex: 60, background: 'var(--aig-modal-backdrop)',
            display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 16,
        }}>
            <div ref={dialogRef} tabIndex={-1} role="dialog" aria-modal="true" aria-labelledby="aig-compare-title"
                onClick={e => e.stopPropagation()} data-testid="draft-compare" style={{
                    width: 'min(840px, 94vw)', maxHeight: '85vh', display: 'flex', flexDirection: 'column',
                    background: 'var(--bg-secondary)', borderRadius: 12, border: `1px solid ${AIC.border}`,
                    boxShadow: 'var(--shadow-md)', outline: 'none',
                }}>
                <div style={{ padding: '16px 18px 0' }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 12 }}>
                        <div style={{ minWidth: 0 }}>
                            <h3 id="aig-compare-title" style={{ margin: '0 0 4px', fontSize: 14, color: AIC.text }}>Compare versions</h3>
                            <p style={{ margin: '0 0 12px', fontSize: 11.5, color: AIC.muted }}>
                                v{original.version} (current) → v{alternative.version} (regenerated). Choosing keeps one and marks the other superseded.
                            </p>
                        </div>
                        <Segmented value={view} onChange={setView} options={[
                            { value: 'split', label: 'Split' },
                            { value: 'unified', label: 'Unified' },
                        ]} />
                    </div>
                    <SummaryBar summary={diff.summary} />
                </div>
                <div style={{ padding: '0 18px 8px', overflowY: 'auto', flex: 1 }}>
                    {view === 'split'
                        ? <SplitView diff={diff} original={original} alternative={alternative} />
                        : <UnifiedView diff={diff} />}
                </div>
                <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, padding: '14px 18px', borderTop: `1px solid ${AIC.border}` }}>
                    <AIBtn onClick={() => onChoose(original.id)}>Keep original</AIBtn>
                    <AIBtn variant="success" onClick={() => onChoose(alternative.id)}>Use new version</AIBtn>
                </div>
            </div>
        </div>
    );
}
