import React, { useEffect } from 'react';
import { AIC } from './constants';
import { SectionLabel } from './primitives';

const STATUS_COLOR = {
    completed: 'var(--aig-success-fg)', failed: 'var(--aig-danger-fg)',
    cancelled: 'var(--aig-tone-amber-fg)', running: AIC.indigoSoft, pending: AIC.muted,
};

export function HistorySection({ ai }) {
    const { history, loadHistory, loadRun, cloneRun, runId, activeRequirement } = ai;

    useEffect(() => {
        if (activeRequirement) loadHistory();
    }, [activeRequirement, loadHistory, runId]);

    if (!history || !history.length) return null;
    return (
        <div data-testid="run-history" style={{ marginTop: 14 }}>
            <SectionLabel>History</SectionLabel>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                {history.slice(0, 10).map(run => (
                    <div key={run.id} style={{
                        border: `1px solid ${run.id === runId ? AIC.indigoSoft : AIC.border}`,
                        borderRadius: 8, padding: '7px 9px', fontSize: 11, color: AIC.dim,
                        background: 'var(--aig-surface-tint)',
                    }}>
                        <div style={{ display: 'flex', gap: 6, alignItems: 'baseline' }}>
                            <span style={{ color: STATUS_COLOR[run.status] || AIC.muted, fontWeight: 700 }}>{run.status}</span>
                            <span>{run.provider_label || run.provider_type}</span>
                            <span style={{ color: AIC.muted }}>{run.model_name}</span>
                            <span style={{ marginLeft: 'auto', color: AIC.muted }}>
                                {new Date(run.created_at).toLocaleString()}
                            </span>
                        </div>
                        <div style={{ display: 'flex', gap: 6, marginTop: 4, alignItems: 'center' }}>
                            <span style={{ color: AIC.muted }}>{run.coverage_level} · {run.total_tokens || 0} tok</span>
                            <span style={{ marginLeft: 'auto', display: 'flex', gap: 4 }}>
                                {run.status === 'completed' && run.id !== runId && (
                                    <button className="action-btn" onClick={() => loadRun(run.id)}>Open</button>
                                )}
                                <button className="action-btn" onClick={() => cloneRun(run)}>Clone</button>
                            </span>
                        </div>
                    </div>
                ))}
            </div>
        </div>
    );
}
