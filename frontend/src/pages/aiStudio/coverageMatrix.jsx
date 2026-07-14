import React, { useState } from 'react';
import { AIC } from './constants';

const STATUS_TONE = {
    covered: { fg: 'var(--aig-success-fg)', label: 'covered' },
    uncovered: { fg: 'var(--aig-danger-fg)', label: 'uncovered' },
    over_represented: { fg: 'var(--aig-tone-amber-fg)', label: 'over-represented' },
};

export function CoverageMatrixPanel({ coverage, drafts, onSelectDraft, onFilterUncovered }) {
    const [open, setOpen] = useState(false);
    if (!coverage || !(coverage.targets || []).length) return null;

    const byPosition = new Map(drafts.map(d => [d.position, d]));
    const t = coverage.targets;

    return (
        <div data-testid="coverage-matrix" style={{ borderBottom: `1px solid ${AIC.border}` }}>
            <button type="button" onClick={() => setOpen(o => !o)} style={{
                width: '100%', display: 'flex', alignItems: 'center', gap: 8,
                padding: '7px 14px', background: 'transparent', border: 'none',
                cursor: 'pointer', color: AIC.dim, fontSize: 11.5,
            }}>
                <span style={{ fontWeight: 700, color: AIC.text }}>Coverage</span>
                <span>{coverage.covered_count}/{t.length} covered</span>
                {coverage.uncovered_count > 0 && (
                    <span style={{ color: 'var(--aig-danger-fg)' }}>{coverage.uncovered_count} uncovered</span>
                )}
                {coverage.over_represented_count > 0 && (
                    <span style={{ color: 'var(--aig-tone-amber-fg)' }}>{coverage.over_represented_count} over-rep.</span>
                )}
                <span style={{ marginLeft: 'auto' }}>{open ? '▾' : '▸'}</span>
            </button>
            {open && (
                <div style={{ padding: '2px 14px 10px', display: 'flex', flexDirection: 'column', gap: 4 }}>
                    {t.map(target => {
                        const tone = STATUS_TONE[target.status] || STATUS_TONE.covered;
                        return (
                            <div key={target.id} style={{ display: 'flex', alignItems: 'baseline', gap: 8, fontSize: 11.5 }}>
                                <span style={{ fontFamily: 'monospace', color: AIC.text, minWidth: 64 }}>{target.id}</span>
                                <span style={{ color: tone.fg, minWidth: 100 }}>{tone.label}</span>
                                <span style={{ color: AIC.muted, flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                                    {target.text}
                                </span>
                                <span style={{ display: 'flex', gap: 4 }}>
                                    {(target.draft_positions || []).map(pos => {
                                        const d = byPosition.get(pos);
                                        return (
                                            <button key={pos} type="button" title={d?.name}
                                                onClick={() => d && onSelectDraft(d.id)}
                                                style={{
                                                    padding: '0 6px', borderRadius: 4, fontSize: 10.5, cursor: 'pointer',
                                                    border: `1px solid ${AIC.border}`, background: 'var(--aig-surface-tint)', color: AIC.dim,
                                                }}>#{pos}</button>
                                        );
                                    })}
                                </span>
                            </div>
                        );
                    })}
                    {(coverage.batch_findings || []).map((f, i) => (
                        <div key={i} style={{ fontSize: 11, color: 'var(--aig-tone-amber-fg)' }}>⚠ {f.message}</div>
                    ))}
                    {coverage.uncovered_count > 0 && (
                        <button type="button" className="action-btn" style={{ alignSelf: 'flex-start', marginTop: 4 }}
                            onClick={onFilterUncovered}>
                            Show drafts without refs
                        </button>
                    )}
                </div>
            )}
        </div>
    );
}
