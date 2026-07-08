import { useState, useEffect, useRef, useMemo } from 'react';
import { aiGeneration } from '../../api';
import { AIC, MONO, stripHtmlForDisplay } from './constants';

// Theme-aware tone per dynamic segment source. title/description share the
// "Requirement" legend entry.
const SEGMENT_TONES = {
    title:        { bg: 'rgba(99,102,241,0.16)', fg: 'var(--aig-tone-indigo-fg)', label: 'Requirement' },
    description:  { bg: 'rgba(99,102,241,0.16)', fg: 'var(--aig-tone-indigo-fg)', label: 'Requirement' },
    children:     { bg: 'rgba(168,85,247,0.16)', fg: 'var(--aig-tone-purple-fg)', label: 'Children' },
    coverage:     { bg: 'rgba(20,184,166,0.16)', fg: 'var(--aig-tone-teal-fg)',   label: 'Coverage' },
    detail:       { bg: 'rgba(34,197,94,0.16)',  fg: 'var(--aig-tone-green-fg)',  label: 'Detail' },
    instructions: { bg: 'rgba(234,179,8,0.16)',  fg: 'var(--aig-tone-amber-fg)',  label: 'Instructions' },
};

const HTML_BEARING = new Set(['description', 'children']);

// Occurrence-indexed key so repeated placeholders diff independently.
function keyedSegments(segments) {
    const counts = {};
    return (segments || []).map(s => {
        const occ = (counts[s.type] = (counts[s.type] || 0) + 1);
        return { ...s, key: `${s.type}:${occ}` };
    });
}

export default function PromptPreviewPanel({ ai }) {
    const [expanded, setExpanded] = useState(() => {
        try { return localStorage.getItem('aig-prompt-preview-open') === '1'; } catch { return false; }
    });
    const [preview, setPreview] = useState(null);
    const [error, setError] = useState(false);
    const [loading, setLoading] = useState(false);
    const [showRaw, setShowRaw] = useState(false);
    const [changedKeys, setChangedKeys] = useState(() => new Set());
    const [pulseTick, setPulseTick] = useState(0); // bumps when content changes → collapsed-header chip
    const reqSeq = useRef(0);
    const prevSegsRef = useRef(null);

    const reqId = ai.activeRequirement?.id || '';
    const { coverageLevel, detailLevel, additionalInstructions } = ai;

    useEffect(() => {
        const seq = ++reqSeq.current;
        const timer = setTimeout(() => {
            setLoading(true);
            aiGeneration.promptPreview({
                requirement_id: reqId,
                coverage_level: coverageLevel,
                detail_level: detailLevel,
                additional_instructions: additionalInstructions,
            }).then(data => {
                if (seq !== reqSeq.current) return; // stale response — a newer request is in flight
                const next = keyedSegments(data.segments);
                const prev = prevSegsRef.current;
                const changed = new Set();
                if (prev) {
                    const prevText = new Map(prev.map(s => [s.key, s.text]));
                    next.forEach(s => {
                        if (s.type !== 'template' && prevText.get(s.key) !== s.text) changed.add(s.key);
                    });
                }
                prevSegsRef.current = next;
                setChangedKeys(changed);
                setPreview({ ...data, segments: next });
                setError(false);
                if (prev && changed.size > 0) setPulseTick(n => n + 1);
            }).catch(() => {
                if (seq === reqSeq.current) setError(true);
            }).finally(() => {
                if (seq === reqSeq.current) setLoading(false);
            });
        }, 350);
        return () => clearTimeout(timer);
    }, [reqId, coverageLevel, detailLevel, additionalInstructions]);

    const toggleExpanded = () => setExpanded(v => {
        const next = !v;
        try { localStorage.setItem('aig-prompt-preview-open', next ? '1' : '0'); } catch { /* localStorage unavailable — non-critical */ }
        return next;
    });

    const segments = useMemo(() => preview?.segments || [], [preview?.segments]);
    const legend = useMemo(() => {
        const seen = new Map();
        segments.forEach(s => {
            if (s.type === 'template' || s.empty) return;
            const t = SEGMENT_TONES[s.type];
            if (t && !seen.has(t.label)) seen.set(t.label, t);
        });
        return [...seen.values()];
    }, [segments]);

    const tooltipFor = (type) => ({
        title: 'From linked requirement — title',
        description: 'From linked requirement — description',
        children: 'Child requirements of the linked requirement',
        coverage: `Coverage guidance (${coverageLevel})`,
        detail: `Detail level (${detailLevel})`,
        instructions: 'Your additional instructions',
    }[type]);

    const chipStyle = {
        display: 'inline-flex', alignItems: 'center', gap: 4,
        padding: '2px 8px', borderRadius: 10,
        fontSize: 10.5, fontWeight: 600, letterSpacing: '0.02em',
        color: AIC.dim, background: AIC.surfaceTint,
        border: `1px solid ${AIC.border2}`, fontFamily: MONO,
    };

    const renderSegment = (s) => {
        if (s.type === 'template') {
            return <span key={s.key} style={{ color: AIC.muted }}>{s.text}</span>;
        }
        if (s.empty && (s.type === 'title' || s.type === 'description') && !reqId) {
            return (
                <span key={s.key} className="aig-preview-gap"
                    title="Link a requirement to fill this part of the prompt">
                    no requirement linked
                </span>
            );
        }
        if (s.empty) return null; // e.g. no additional instructions — nothing to show
        const tone = SEGMENT_TONES[s.type] || {};
        const text = (!showRaw && HTML_BEARING.has(s.type)) ? stripHtmlForDisplay(s.text) : s.text;
        return (
            <span key={s.key}
                className={changedKeys.has(s.key) ? 'aig-preview-flash' : undefined}
                title={tooltipFor(s.type)}
                style={{ background: tone.bg, color: tone.fg, borderRadius: 3, padding: '0 2px' }}>
                {text}
            </span>
        );
    };

    return (
        <div style={{
            marginTop: 10, borderRadius: 8,
            background: 'var(--aig-surface-sunken)',
            border: `1px solid ${AIC.border}`, overflow: 'hidden',
        }}>
            <style>{`
                .aig-preview-flash { animation: aigPreviewFlash 0.8s ease-out; }
                @keyframes aigPreviewFlash {
                    0% { box-shadow: inset 0 0 0 999px rgba(99,102,241,0.30); }
                    100% { box-shadow: inset 0 0 0 999px rgba(99,102,241,0); }
                }
                .aig-preview-gap {
                    display: inline-flex; align-items: center;
                    padding: 0 6px; margin: 0 2px;
                    border: 1px dashed rgba(234,179,8,0.55); border-radius: 4px;
                    color: var(--aig-tone-amber-fg); background: rgba(234,179,8,0.08);
                    font-style: italic;
                }
                .aig-preview-updated-chip {
                    font-size: 9.5px; font-weight: 700; letter-spacing: 0.05em; text-transform: uppercase;
                    color: var(--aig-tone-indigo-fg);
                    background: rgba(99,102,241,0.15); border: 1px solid rgba(99,102,241,0.3);
                    padding: 1px 6px; border-radius: 8px;
                    animation: aigPreviewChipFade 1.8s ease-out forwards;
                }
                @keyframes aigPreviewChipFade {
                    0%, 70% { opacity: 1; }
                    100% { opacity: 0; }
                }
                @media (prefers-reduced-motion: reduce) {
                    .aig-preview-flash, .aig-preview-updated-chip { animation: none; }
                }
            `}</style>

            <button onClick={toggleExpanded} style={{
                display: 'flex', width: '100%', alignItems: 'center',
                justifyContent: 'space-between', gap: 10,
                padding: '8px 12px', background: 'transparent', border: 'none',
                cursor: 'pointer', fontFamily: 'inherit', color: AIC.text,
            }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 7 }}>
                    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                        strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"
                        style={{ color: AIC.indigoSoft }}>
                        <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/>
                    </svg>
                    <span style={{
                        fontSize: 11, fontWeight: 600, color: AIC.dim,
                        letterSpacing: '0.06em', textTransform: 'uppercase',
                        opacity: loading ? 0.6 : 1, transition: 'opacity .15s',
                    }}>Prompt preview</span>
                    {!expanded && pulseTick > 0 && (
                        <span key={pulseTick} className="aig-preview-updated-chip">updated</span>
                    )}
                    {error && (
                        <span style={{ fontSize: 10.5, color: AIC.amber, fontWeight: 600 }}>
                            Preview unavailable
                        </span>
                    )}
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    {preview && <span style={chipStyle}>{preview.template_type} template</span>}
                    {preview && (
                        <span style={chipStyle} title="Response token budget for the selected coverage level">
                            budget {preview.max_tokens?.toLocaleString()} tok
                        </span>
                    )}
                    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                        strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"
                        style={{
                            color: AIC.muted, transition: 'transform 0.2s',
                            transform: expanded ? 'rotate(180deg)' : 'rotate(0deg)',
                        }}>
                        <polyline points="6 9 12 15 18 9"/>
                    </svg>
                </div>
            </button>

            {expanded && (
                <div style={{ padding: '8px 12px 12px', borderTop: `1px solid ${AIC.border2}` }}>
                    <div style={{
                        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                        gap: 10, marginBottom: 8, flexWrap: 'wrap',
                    }}>
                        <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                            {legend.map(t => (
                                <span key={t.label} style={{
                                    background: t.bg, color: t.fg,
                                    padding: '1px 8px', borderRadius: 999,
                                    fontSize: 9.5, fontWeight: 600, letterSpacing: '0.04em',
                                }}>{t.label}</span>
                            ))}
                        </div>
                        <div style={{
                            display: 'inline-flex', padding: 2, borderRadius: 5,
                            border: `1px solid ${AIC.border2}`, background: AIC.surfaceTint,
                        }}>
                            {['Clean', 'Raw'].map(mode => {
                                const active = mode === 'Raw' ? showRaw : !showRaw;
                                return (
                                    <button key={mode} onClick={() => setShowRaw(mode === 'Raw')}
                                        style={{
                                            padding: '2px 8px', borderRadius: 3, border: 'none',
                                            fontSize: 10, fontWeight: 600, letterSpacing: '0.04em',
                                            textTransform: 'uppercase', fontFamily: 'inherit', cursor: 'pointer',
                                            background: active ? 'var(--aig-accent-soft-bg)' : 'transparent',
                                            color: active ? AIC.indigoSoft : AIC.muted,
                                        }}
                                        title={mode === 'Raw'
                                            ? 'Show exact text that will be sent to the LLM (may contain HTML)'
                                            : 'Strip HTML tags for readability'}
                                    >{mode}</button>
                                );
                            })}
                        </div>
                    </div>

                    {preview?.template_warning && (
                        <div style={{
                            marginBottom: 8, padding: '5px 8px', borderRadius: 5,
                            fontSize: 10.5, color: AIC.amber,
                            background: 'rgba(234,179,8,0.06)', border: '1px solid rgba(234,179,8,0.22)',
                        }}>{preview.template_warning}</div>
                    )}

                    {preview?.system_message && (
                        <div style={{
                            marginBottom: 8, padding: '6px 8px', borderRadius: 5,
                            background: AIC.surfaceTint, border: `1px solid ${AIC.border2}`,
                            fontSize: 10.5, lineHeight: 1.5, color: AIC.muted, fontFamily: MONO,
                        }}>
                            <span style={{
                                fontWeight: 700, letterSpacing: '0.06em', textTransform: 'uppercase',
                                fontSize: 9, marginRight: 6, color: AIC.dim,
                            }}>system</span>
                            {preview.system_message}
                        </div>
                    )}

                    {segments.length > 0 ? (
                        <pre style={{
                            margin: 0, padding: 12,
                            background: AIC.bg1, border: `1px solid ${AIC.border}`, borderRadius: 6,
                            fontSize: 11, lineHeight: 1.6, color: AIC.dim, fontFamily: MONO,
                            whiteSpace: 'pre-wrap', wordBreak: 'break-word',
                            maxHeight: 260, overflow: 'auto',
                            opacity: loading ? 0.65 : 1, transition: 'opacity .15s',
                        }}>{segments.map(renderSegment)}</pre>
                    ) : (
                        <div style={{ fontSize: 11.5, color: AIC.muted, padding: '8px 2px' }}>
                            {error ? 'Could not load the prompt preview.' : 'Loading preview…'}
                        </div>
                    )}
                </div>
            )}
        </div>
    );
}
