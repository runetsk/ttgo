import { useState, useMemo } from 'react';
import { AIC, MONO, stripHtmlForDisplay } from './constants';

// ── LLM Feedback panel ──────────────────────────────────────────────────────
function DebugRow({ label, value, mono, strong, highlight }) {
    return (
        <div style={{
            display: 'flex', justifyContent: 'space-between', alignItems: 'center',
            gap: 12, padding: '4px 0',
            borderBottom: `1px dashed ${AIC.border2}`,
        }}>
            <span style={{ fontSize: 11, color: AIC.muted, letterSpacing: '0.02em' }}>{label}</span>
            <span style={{
                fontSize: mono ? 11.5 : 12, fontWeight: strong ? 700 : 500,
                color: highlight ? AIC.amber : AIC.text,
                fontFamily: mono ? MONO : 'inherit',
                textAlign: 'right',
            }}>{value}</span>
        </div>
    );
}

export function LlmFeedbackPanel({ debug }) {
    const [expanded, setExpanded] = useState(false);
    const [ctxExpanded, setCtxExpanded] = useState(false);
    const [showRaw, setShowRaw] = useState(false);
    const displayedContext = useMemo(
        () => showRaw ? (debug?.request_context || '') : stripHtmlForDisplay(debug?.request_context),
        [debug?.request_context, showRaw]
    );
    if (!debug) return null;
    const durationLabel = debug.duration_ms >= 1000
        ? `${(debug.duration_ms / 1000).toFixed(1)}s`
        : `${debug.duration_ms}ms`;
    const durationLabelFine = debug.duration_ms >= 1000
        ? `${(debug.duration_ms / 1000).toFixed(2)}s`
        : `${debug.duration_ms}ms`;
    const chipStyle = {
        display: 'inline-flex', alignItems: 'center', gap: 4,
        padding: '2px 8px', borderRadius: 10,
        fontSize: 10.5, fontWeight: 600, letterSpacing: '0.02em',
        color: AIC.dim, background: AIC.surfaceTint,
        border: `1px solid ${AIC.border2}`, fontFamily: MONO,
    };
    return (
        <div style={{
            margin: '0 20px 14px', borderRadius: 8,
            background: 'var(--aig-surface-sunken)',
            border: `1px solid ${AIC.border}`,
            overflow: 'hidden',
        }}>
            <button
                onClick={() => setExpanded(v => !v)}
                style={{
                    display: 'flex', width: '100%', alignItems: 'center',
                    justifyContent: 'space-between', gap: 10,
                    padding: '8px 12px', background: 'transparent', border: 'none',
                    cursor: 'pointer', fontFamily: 'inherit', color: AIC.text,
                }}
            >
                <div style={{ display: 'flex', alignItems: 'center', gap: 7 }}>
                    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ color: AIC.indigoSoft }}>
                        <circle cx="12" cy="12" r="3"/>
                        <path d="M12 1v4M12 19v4M4.22 4.22l2.83 2.83M16.95 16.95l2.83 2.83M1 12h4M19 12h4M4.22 19.78l2.83-2.83M16.95 7.05l2.83-2.83"/>
                    </svg>
                    <span style={{
                        fontSize: 11, fontWeight: 600, color: AIC.dim,
                        letterSpacing: '0.06em', textTransform: 'uppercase',
                    }}>LLM Feedback</span>
                    {debug.retried && (
                        <span style={{
                            marginLeft: 4, padding: '1px 6px', borderRadius: 8,
                            fontSize: 9.5, fontWeight: 700, letterSpacing: '0.04em',
                            textTransform: 'uppercase',
                            background: 'rgba(234,179,8,0.15)',
                            border: '1px solid rgba(234,179,8,0.4)',
                            color: AIC.amber,
                        }}>retried</span>
                    )}
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <span style={chipStyle}>⏱ {durationLabel}</span>
                    {debug.usage && (
                        <span style={chipStyle}>🪙 {debug.usage.total_tokens?.toLocaleString() ?? '—'} tok</span>
                    )}
                    <svg
                        width="12" height="12" viewBox="0 0 24 24" fill="none"
                        stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"
                        style={{ color: AIC.muted, transition: 'transform 0.2s', transform: expanded ? 'rotate(180deg)' : 'rotate(0deg)' }}
                    >
                        <polyline points="6 9 12 15 18 9"/>
                    </svg>
                </div>
            </button>

            {expanded && (
                <div style={{ padding: '8px 12px 12px', borderTop: `1px solid ${AIC.border2}` }}>
                    <div style={{
                        display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0 24px',
                    }}>
                        <DebugRow label="Duration" value={durationLabelFine} />
                        <DebugRow label="Model" value={debug.model || '—'} mono />
                        <DebugRow label="Provider" value={`${debug.provider_label || '—'}${debug.provider_type ? ` (${debug.provider_type})` : ''}`} />
                        <DebugRow label="Finish reason" value={debug.finish_reason || '—'} />
                        <DebugRow label="Max tokens budget" value={debug.max_tokens_budget?.toLocaleString() ?? '—'} />
                        <DebugRow label="Auto-retried" value={debug.retried ? 'Yes' : 'No'} highlight={debug.retried} />
                        {debug.usage ? (
                            <>
                                <DebugRow label="Prompt tokens" value={debug.usage.prompt_tokens?.toLocaleString() ?? '—'} />
                                <DebugRow label="Completion tokens" value={debug.usage.completion_tokens?.toLocaleString() ?? '—'} />
                                <DebugRow label="Total tokens" value={debug.usage.total_tokens?.toLocaleString() ?? '—'} strong />
                            </>
                        ) : (
                            <DebugRow label="Token usage" value="Not reported by provider" />
                        )}
                    </div>

                    {debug.request_context && (
                        <div style={{ marginTop: 10, borderTop: `1px solid ${AIC.border2}`, paddingTop: 8 }}>
                            <div style={{
                                display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 10,
                            }}>
                                <button
                                    onClick={() => setCtxExpanded(v => !v)}
                                    style={{
                                        background: 'none', border: 'none', cursor: 'pointer',
                                        padding: 0, display: 'flex', alignItems: 'center', gap: 6,
                                        color: AIC.dim, fontFamily: 'inherit',
                                    }}
                                >
                                    <svg
                                        width="10" height="10" viewBox="0 0 24 24" fill="none"
                                        stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"
                                        style={{ transition: 'transform 0.2s', transform: ctxExpanded ? 'rotate(180deg)' : 'rotate(0deg)' }}
                                    >
                                        <polyline points="6 9 12 15 18 9"/>
                                    </svg>
                                    <span style={{
                                        fontSize: 10.5, fontWeight: 600, letterSpacing: '0.06em',
                                        textTransform: 'uppercase',
                                    }}>Full Request Context</span>
                                </button>
                                {ctxExpanded && (
                                    <div style={{
                                        display: 'inline-flex', padding: 2, borderRadius: 5,
                                        border: `1px solid ${AIC.border2}`, background: AIC.surfaceTint,
                                    }}>
                                        {['Clean', 'Raw'].map(mode => {
                                            const active = mode === 'Raw' ? showRaw : !showRaw;
                                            return (
                                                <button key={mode}
                                                    onClick={() => setShowRaw(mode === 'Raw')}
                                                    style={{
                                                        padding: '2px 8px', borderRadius: 3, border: 'none',
                                                        fontSize: 10, fontWeight: 600, letterSpacing: '0.04em',
                                                        textTransform: 'uppercase', fontFamily: 'inherit',
                                                        cursor: 'pointer',
                                                        background: active ? 'var(--aig-accent-soft-bg)' : 'transparent',
                                                        color: active ? AIC.indigoSoft : AIC.muted,
                                                    }}
                                                    title={mode === 'Raw' ? 'Show exact text sent to the LLM (may contain HTML)' : 'Strip HTML tags for readability'}
                                                >{mode}</button>
                                            );
                                        })}
                                    </div>
                                )}
                            </div>
                            {ctxExpanded && (
                                <pre style={{
                                    marginTop: 8, padding: 12,
                                    background: AIC.bg1,
                                    border: `1px solid ${AIC.border}`,
                                    borderRadius: 6,
                                    fontSize: 11, lineHeight: 1.6,
                                    color: AIC.dim, fontFamily: MONO,
                                    whiteSpace: 'pre-wrap', wordBreak: 'break-word',
                                    maxHeight: 320, overflow: 'auto',
                                }}>
                                    {displayedContext}
                                </pre>
                            )}
                        </div>
                    )}
                </div>
            )}
        </div>
    );
}
