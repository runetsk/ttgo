import React, { useState, useMemo, useEffect, useRef } from 'react';
import { Link } from 'react-router-dom';
import { useAIGeneration, DETAIL_LEVELS, COVERAGE_LEVELS } from '../contexts/AIGenerationContext';
import FolderTreeSelect from '../components/FolderTreeSelect';
import AIGenReviewPanel from '../components/AIGenReviewPanel';
import AIImportPanel from '../components/AIImportPanel';
import AIImportReview from '../components/AIImportReview';
import SafeHTML from '../components/shared/SafeHTML';
import { requirements as requirementsApi } from '../api';

// Theme-aware color tokens. Values reference the CSS variables defined in
// index.css so the Studio follows the global light/dark theme toggle.
const AIC = {
    bg1: 'var(--bg-primary)',
    bg2: 'var(--bg-secondary)',
    bg3: 'var(--bg-tertiary)',
    text: 'var(--text-primary)',
    dim: 'var(--text-secondary)',
    muted: 'var(--sidebar-muted)',
    border: 'var(--border-color)',
    border2: 'var(--glass-border)',
    surfaceTint: 'var(--aig-surface-tint)',
    surfaceSunken: 'var(--aig-surface-sunken)',
    indigo: '#6366f1',
    indigoSoft: 'var(--aig-indigo-soft)',
    teal: '#14b8a6',
    tealSoft: '#5eead4',
    green: '#22c55e',
    red: '#ef4444',
    amber: '#eab308',
};

const MONO = "'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, monospace";

// ── Icons ────────────────────────────────────────────────────────────────────
const Icon = {
    sparkles: (s = 14) => (
        <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M12 3v4"/><path d="M12 17v4"/><path d="M3 12h4"/><path d="M17 12h4"/><path d="M5.6 5.6l2.8 2.8"/><path d="M15.6 15.6l2.8 2.8"/><path d="M5.6 18.4l2.8-2.8"/><path d="M15.6 8.4l2.8-2.8"/></svg>
    ),
    bolt: (s = 14) => (
        <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>
    ),
    check: (s = 14) => (
        <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polyline points="20 6 9 17 4 12"/></svg>
    ),
    x: (s = 14) => (
        <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
    ),
    chevronR: (s = 14) => (
        <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polyline points="9 18 15 12 9 6"/></svg>
    ),
    edit: (s = 14) => (
        <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M12 20h9"/><path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4z"/></svg>
    ),
    copy: (s = 14) => (
        <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
    ),
    dots: (s = 14) => (
        <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/><circle cx="5" cy="12" r="1"/></svg>
    ),
    plus: (s = 14) => (
        <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
    ),
    alert: (s = 14) => (
        <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
    ),
    history: (s = 14) => (
        <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M3 3v5h5"/><path d="M3.05 13a9 9 0 1 0 .5-4"/><path d="M12 7v5l3 2"/></svg>
    ),
};

// ── Category ordering + colors ───────────────────────────────────────────────
const CATEGORY_ORDER = [
    'Critical', 'Functional', 'Regression', 'Negative', 'Boundary', 'Edge Case',
    'Security', 'Performance', 'API', 'Mobile/Responsive', 'Accessibility', 'UI',
];

function categoryTone(cat) {
    const key = (cat || '').toLowerCase();
    if (key.includes('critical') || key.includes('negative')) return { bg: 'rgba(239,68,68,0.12)', bd: 'rgba(239,68,68,0.35)', fg: 'var(--aig-tone-red-fg)' };
    if (key.includes('regression') || key.includes('functional')) return { bg: 'rgba(99,102,241,0.12)', bd: 'rgba(99,102,241,0.35)', fg: 'var(--aig-tone-indigo-fg)' };
    if (key.includes('security')) return { bg: 'rgba(234,179,8,0.12)', bd: 'rgba(234,179,8,0.35)', fg: 'var(--aig-tone-amber-fg)' };
    if (key.includes('access')) return { bg: 'rgba(20,184,166,0.12)', bd: 'rgba(20,184,166,0.35)', fg: 'var(--aig-tone-teal-fg)' };
    if (key.includes('perf')) return { bg: 'rgba(34,197,94,0.12)', bd: 'rgba(34,197,94,0.35)', fg: 'var(--aig-tone-green-fg)' };
    if (key.includes('ui') || key.includes('mobile')) return { bg: 'rgba(168,85,247,0.12)', bd: 'rgba(168,85,247,0.35)', fg: 'var(--aig-tone-purple-fg)' };
    if (key.includes('edge') || key.includes('boundary')) return { bg: 'rgba(234,179,8,0.12)', bd: 'rgba(234,179,8,0.35)', fg: 'var(--aig-tone-amber-fg)' };
    return { bg: 'var(--aig-surface-tint-strong)', bd: AIC.border, fg: AIC.dim };
}

function groupDrafts(drafts, by, linkedReqId) {
    if (by === 'none') return [{ key: 'all', label: 'All drafts', items: drafts }];
    const map = new Map();
    const push = (k, label, d) => {
        if (!map.has(k)) map.set(k, { key: k, label, items: [] });
        map.get(k).items.push(d);
    };
    drafts.forEach(d => {
        if (by === 'category') {
            const c = (d.category || '').trim() || 'Uncategorized';
            push(c.toLowerCase(), c, d);
        } else if (by === 'requirement') {
            push(linkedReqId || '__none', linkedReqId || 'No requirement', d);
        }
    });
    const arr = Array.from(map.values());
    const orderLower = CATEGORY_ORDER.map(x => x.toLowerCase());
    if (by === 'category') {
        arr.sort((a, b) => {
            const ai = orderLower.indexOf(a.label.toLowerCase());
            const bi = orderLower.indexOf(b.label.toLowerCase());
            if (ai === -1 && bi === -1) return a.label.localeCompare(b.label);
            if (ai === -1) return 1;
            if (bi === -1) return -1;
            return ai - bi;
        });
    }
    return arr;
}

// ── Primitive components ─────────────────────────────────────────────────────
function SectionLabel({ children, right }) {
    return (
        <div style={{
            display: 'flex', justifyContent: 'space-between', alignItems: 'center',
            fontSize: 10.5, textTransform: 'uppercase', letterSpacing: '0.1em',
            color: AIC.muted, fontWeight: 700, marginBottom: 10,
        }}>
            <span>{children}</span>
            {right}
        </div>
    );
}

function Pill({ tone = 'neutral', children, style }) {
    const tones = {
        indigo: { bg: 'rgba(99,102,241,0.14)', bd: 'rgba(99,102,241,0.3)', fg: AIC.indigoSoft },
        teal: { bg: 'rgba(20,184,166,0.14)', bd: 'rgba(20,184,166,0.3)', fg: '#5eead4' },
        green: { bg: 'rgba(34,197,94,0.12)', bd: 'rgba(34,197,94,0.3)', fg: '#86efac' },
        red: { bg: 'rgba(239,68,68,0.12)', bd: 'rgba(239,68,68,0.3)', fg: '#fca5a5' },
        amber: { bg: 'rgba(234,179,8,0.12)', bd: 'rgba(234,179,8,0.3)', fg: '#fde047' },
        neutral: { bg: 'var(--aig-surface-tint-strong)', bd: AIC.border, fg: AIC.dim },
    };
    const t = tones[tone] || tones.neutral;
    return (
        <span style={{
            display: 'inline-flex', alignItems: 'center', gap: 4,
            padding: '2px 8px', borderRadius: 999,
            fontSize: 10.5, fontWeight: 600, letterSpacing: '0.02em',
            background: t.bg, border: `1px solid ${t.bd}`, color: t.fg,
            ...(style || {}),
        }}>{children}</span>
    );
}

function StatusPill({ status }) {
    if (status === 'accepted') return <Pill tone="green">{Icon.check(10)} Accepted</Pill>;
    if (status === 'rejected') return <Pill tone="red">{Icon.x(10)} Rejected</Pill>;
    return <Pill tone="indigo">{Icon.sparkles(10)} AI Draft</Pill>;
}

function ReqChip({ id }) {
    if (!id) return null;
    return (
        <span style={{
            display: 'inline-flex', alignItems: 'center', gap: 4,
            fontFamily: MONO,
            fontSize: 11, padding: '2px 6px', borderRadius: 4,
            background: 'rgba(99,102,241,0.1)', border: '1px solid rgba(99,102,241,0.25)',
            color: AIC.indigoSoft, fontWeight: 500,
        }}>{id}</span>
    );
}

function Segmented({ options, value, onChange }) {
    return (
        <div style={{
            display: 'inline-flex', background: AIC.bg1, border: `1px solid ${AIC.border2}`,
            padding: 3, borderRadius: 7, gap: 2,
        }}>
            {options.map(o => (
                <button key={o.value}
                    onClick={() => onChange(o.value)}
                    style={{
                        padding: '5px 11px', fontSize: 12, fontFamily: 'inherit', cursor: 'pointer',
                        border: 'none', borderRadius: 5,
                        background: value === o.value ? AIC.bg3 : 'transparent',
                        color: value === o.value ? AIC.text : AIC.dim,
                        fontWeight: value === o.value ? 600 : 400,
                    }}>{o.label}</button>
            ))}
        </div>
    );
}

function AIBtn({ variant = 'default', onClick, disabled, children, style, title }) {
    const base = {
        padding: '7px 13px', fontSize: 12.5, borderRadius: 7, cursor: disabled ? 'not-allowed' : 'pointer',
        fontFamily: 'inherit', display: 'inline-flex', alignItems: 'center', gap: 6,
        transition: 'all .15s',
        opacity: disabled ? 0.5 : 1,
    };
    const variants = {
        default: { background: AIC.bg3, border: `1px solid ${AIC.border}`, color: AIC.text },
        primary: {
            background: 'linear-gradient(135deg, #6366f1, #4f46e5)', border: 'none', color: '#fff',
            fontWeight: 600, boxShadow: '0 4px 14px rgba(99,102,241,0.35)',
        },
        ghost: { background: 'transparent', border: `1px solid transparent`, color: AIC.text },
        success: { background: 'var(--aig-success-bg)', border: '1px solid var(--aig-success-border)', color: 'var(--aig-success-fg)' },
        danger: { background: 'var(--aig-danger-bg)', border: '1px solid var(--aig-danger-border)', color: 'var(--aig-danger-fg)' },
    };
    return (
        <button onClick={onClick} disabled={disabled} title={title}
            style={{ ...base, ...variants[variant], ...(style || {}) }}>
            {children}
        </button>
    );
}

function FilterTab({ active, onClick, children }) {
    return (
        <button onClick={onClick} style={{
            padding: '4px 10px', fontSize: 12, borderRadius: 6, border: 'none', cursor: 'pointer',
            background: active ? AIC.bg3 : 'transparent', color: active ? AIC.text : AIC.dim,
            fontWeight: active ? 600 : 400, fontFamily: 'inherit',
        }}>{children}</button>
    );
}

function GeneratingDots() {
    return (
        <span style={{ display: 'inline-flex', gap: 4 }}>
            {[0, 1, 2].map(i => (
                <span key={i} style={{
                    width: 6, height: 6, borderRadius: '50%',
                    background: 'linear-gradient(135deg, #6366f1, #14b8a6)',
                    animation: `aigAiBlip 1.1s ease-in-out ${i * 0.14}s infinite`,
                }} />
            ))}
        </span>
    );
}

function DraftSkeleton({ delay }) {
    return (
        <div style={{
            padding: '14px 16px', borderRadius: 8, background: AIC.bg2,
            border: `1px solid ${AIC.border}`, margin: '0 20px 8px',
            animation: `aigSlideUp .5s ease ${delay}s both`, overflow: 'hidden', position: 'relative',
        }}>
            <div style={{
                height: 13, width: '55%',
                background: 'linear-gradient(90deg, rgba(99,102,241,0.15), rgba(20,184,166,0.15), rgba(99,102,241,0.15))',
                backgroundSize: '200% 100%', animation: 'aigShimmer 1.5s linear infinite',
                borderRadius: 3, marginBottom: 9,
            }} />
            <div style={{ height: 9, width: '80%', background: 'var(--aig-surface-tint-strong)', borderRadius: 3, marginBottom: 5 }} />
            <div style={{ height: 9, width: '70%', background: 'var(--aig-surface-tint-strong)', borderRadius: 3 }} />
        </div>
    );
}

function Stepper({ steps }) {
    if (!steps || steps.length === 0) {
        return <div style={{ color: AIC.muted, fontSize: 12 }}>No steps yet.</div>;
    }
    return (
        <div>
            <div style={{
                display: 'grid', gridTemplateColumns: '26px 1fr 1fr', gap: 12,
                padding: '0 0 6px 0',
                fontSize: 9.5, color: AIC.muted, fontWeight: 600,
                letterSpacing: '0.08em', textTransform: 'uppercase',
                borderBottom: `1px solid ${AIC.border}`,
            }}>
                <div></div>
                <div>Action</div>
                <div>Expected</div>
            </div>
            {steps.map((s, i) => (
                <div key={i} style={{
                    display: 'grid', gridTemplateColumns: '26px 1fr 1fr', gap: 12, alignItems: 'start',
                    padding: '10px 0',
                    borderBottom: i < steps.length - 1 ? `1px solid ${AIC.border}` : 'none',
                }}>
                    <div style={{
                        width: 22, height: 22, borderRadius: '50%',
                        background: 'var(--aig-accent-soft-bg)',
                        border: '1px solid var(--aig-accent-soft-border)',
                        color: AIC.indigoSoft, fontSize: 11, fontWeight: 700,
                        display: 'flex', alignItems: 'center', justifyContent: 'center',
                        fontFamily: MONO,
                    }}>{i + 1}</div>
                    <div style={{ fontSize: 12.5, color: AIC.text, lineHeight: 1.45 }}>{s.action}</div>
                    <div style={{ fontSize: 12.5, color: AIC.text, lineHeight: 1.45 }}>
                        {s.expected_result || ''}
                    </div>
                </div>
            ))}
        </div>
    );
}

function MiniStat({ label, value }) {
    return (
        <div style={{ padding: '8px 10px', background: AIC.bg2, border: `1px solid ${AIC.border}`, borderRadius: 7 }}>
            <div style={{
                fontSize: 10, color: AIC.muted, textTransform: 'uppercase',
                letterSpacing: '0.08em', fontWeight: 600,
            }}>{label}</div>
            <div style={{
                fontSize: 15, color: AIC.text, fontWeight: 600,
                fontVariantNumeric: 'tabular-nums', marginTop: 2,
            }}>{value}</div>
        </div>
    );
}

// ── Linked-requirement preview card ─────────────────────────────────────────
function LinkedReqCard({ req, disabled, onUnlink }) {
    const [expanded, setExpanded] = useState(false);
    const [hovered, setHovered] = useState(null); // 'open' | 'unlink' | null

    const { plain, wordCount, charCount, isLong } = useMemo(() => {
        const html = req.description || '';
        const stripped = html.replace(/<style[\s\S]*?<\/style>/gi, '')
                             .replace(/<script[\s\S]*?<\/script>/gi, '')
                             .replace(/<[^>]+>/g, ' ')
                             .replace(/&nbsp;/g, ' ')
                             .replace(/&amp;/g, '&')
                             .replace(/&lt;/g, '<')
                             .replace(/&gt;/g, '>')
                             .replace(/\s+/g, ' ')
                             .trim();
        const words = stripped ? stripped.split(/\s+/).length : 0;
        return { plain: stripped, wordCount: words, charCount: stripped.length, isLong: stripped.length > 260 };
    }, [req.description]);

    const hasDesc = !!plain;
    const contextLevel = wordCount >= 120 ? 'rich' : wordCount >= 40 ? 'ok' : 'thin';
    const contextColor = contextLevel === 'rich' ? 'var(--accent-green)' : contextLevel === 'ok' ? 'var(--warning-color)' : 'var(--accent-red)';
    const contextLabel = contextLevel === 'rich' ? 'Rich context' : contextLevel === 'ok' ? 'OK context' : hasDesc ? 'Thin context' : 'No context';

    const actionBtnBase = {
        display: 'inline-flex', alignItems: 'center', gap: 4,
        padding: '4px 8px', fontSize: 11, borderRadius: 5,
        border: '1px solid transparent',
        background: 'transparent', color: AIC.muted,
        cursor: disabled ? 'not-allowed' : 'pointer',
        fontFamily: 'inherit', fontWeight: 500,
        textDecoration: 'none', transition: 'color .12s, border-color .12s, background .12s',
    };

    return (
        <div style={{
            position: 'relative',
            borderRadius: 8,
            background: 'linear-gradient(180deg, rgba(99,102,241,0.06), rgba(99,102,241,0.015))',
            border: `1px solid ${AIC.border}`,
            overflow: 'hidden',
        }}>
            {/* Indigo accent rail */}
            <div style={{
                position: 'absolute', left: 0, top: 0, bottom: 0, width: 2,
                background: 'linear-gradient(180deg, #818cf8, #6366f1 60%, rgba(99,102,241,0.4))',
            }} />

            {/* Header */}
            <div style={{ padding: '10px 10px 8px 12px' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 6 }}>
                    <span style={{
                        display: 'inline-flex', alignItems: 'center',
                        fontFamily: MONO, fontSize: 10, fontWeight: 600,
                        color: AIC.indigoSoft,
                        background: 'var(--aig-accent-soft-bg)',
                        border: '1px solid var(--aig-accent-soft-border)',
                        padding: '2px 6px', borderRadius: 4,
                        letterSpacing: '0.02em',
                    }}>{req.identifier}</span>
                    <span style={{
                        display: 'inline-flex', alignItems: 'center', gap: 3,
                        fontSize: 9.5, color: contextColor, fontWeight: 600,
                        marginLeft: 'auto',
                    }} title={`${wordCount} words · ${charCount} characters`}>
                        <span style={{
                            width: 5, height: 5, borderRadius: '50%', background: contextColor,
                            boxShadow: `0 0 6px ${contextColor}`,
                        }} />
                        {contextLabel}
                    </span>
                </div>
                <div style={{
                    fontSize: 12.5, color: AIC.text, fontWeight: 600,
                    lineHeight: 1.35, letterSpacing: '-0.005em',
                }}>{req.title}</div>
            </div>

            {/* Description */}
            <div style={{ padding: '0 10px 10px 12px' }}>
                {hasDesc ? (
                    <div style={{ position: 'relative' }}>
                        <div style={{
                            position: 'relative',
                            maxHeight: expanded ? 260 : 68,
                            overflow: 'auto',
                            padding: '8px 10px',
                            background: 'var(--aig-surface-sunken)',
                            borderRadius: 6,
                            border: `1px solid ${AIC.border}`,
                            transition: 'max-height .2s ease',
                        }}>
                            {expanded ? (
                                <SafeHTML
                                    html={req.description}
                                    style={{ fontSize: 11.5, color: AIC.dim, lineHeight: 1.55 }}
                                />
                            ) : (
                                <div style={{
                                    fontSize: 11.5, color: AIC.dim, lineHeight: 1.55,
                                    display: '-webkit-box', WebkitBoxOrient: 'vertical',
                                    WebkitLineClamp: 3, overflow: 'hidden',
                                }}>{plain}</div>
                            )}
                        </div>
                        {/* bottom fade when collapsed & long */}
                        {!expanded && isLong && (
                            <div style={{
                                position: 'absolute', left: 1, right: 1, bottom: 1, height: 22,
                                background: 'linear-gradient(180deg, transparent 0%, var(--aig-fade-color) 100%)',
                                borderRadius: '0 0 5px 5px',
                                pointerEvents: 'none',
                            }} />
                        )}
                        {isLong && (
                            <button onClick={() => setExpanded(v => !v)}
                                style={{
                                    marginTop: 4, padding: 0,
                                    background: 'transparent', border: 'none',
                                    color: AIC.indigoSoft, fontSize: 10.5, fontWeight: 600,
                                    cursor: 'pointer', fontFamily: 'inherit',
                                    display: 'inline-flex', alignItems: 'center', gap: 3,
                                }}>
                                {expanded ? 'Show less' : 'Show full description'}
                                <svg width="9" height="9" viewBox="0 0 24 24" fill="none"
                                    stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"
                                    style={{ transform: expanded ? 'rotate(180deg)' : 'none', transition: 'transform .15s' }}>
                                    <polyline points="6 9 12 15 18 9" />
                                </svg>
                            </button>
                        )}
                    </div>
                ) : (
                    <div style={{
                        display: 'flex', alignItems: 'flex-start', gap: 6,
                        padding: '8px 10px', fontSize: 11, color: '#fbbf24',
                        background: 'rgba(234,179,8,0.06)', border: '1px solid rgba(234,179,8,0.22)',
                        borderRadius: 6, lineHeight: 1.4,
                    }}>
                        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                            strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"
                            style={{ flexShrink: 0, marginTop: 1 }}>
                            <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
                            <line x1="12" y1="9" x2="12" y2="13" />
                            <line x1="12" y1="17" x2="12.01" y2="17" />
                        </svg>
                        <span>No description — the AI has limited context. Add one on the Requirements page.</span>
                    </div>
                )}
            </div>

            {/* Footer actions */}
            <div style={{
                display: 'flex', alignItems: 'center', gap: 4,
                padding: '6px 8px 8px 10px',
                borderTop: `1px solid ${AIC.border2}`,
                background: 'var(--aig-surface-footer)',
            }}>
                <Link to={`/requirements/${req.id}`}
                    title="Open in Requirements page"
                    onMouseEnter={() => setHovered('open')}
                    onMouseLeave={() => setHovered(null)}
                    style={{
                        ...actionBtnBase,
                        color: hovered === 'open' ? AIC.indigoSoft : AIC.muted,
                        borderColor: hovered === 'open' ? 'rgba(129,140,248,0.35)' : 'transparent',
                        background: hovered === 'open' ? 'rgba(99,102,241,0.1)' : 'transparent',
                    }}>
                    <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                        strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
                        <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                        <polyline points="15 3 21 3 21 9" />
                        <line x1="10" y1="14" x2="21" y2="3" />
                    </svg>
                    Open
                </Link>
                <div style={{ flex: 1 }} />
                <button
                    onClick={onUnlink}
                    disabled={disabled}
                    title="Unlink this requirement"
                    onMouseEnter={() => setHovered('unlink')}
                    onMouseLeave={() => setHovered(null)}
                    style={{
                        ...actionBtnBase,
                        color: hovered === 'unlink' ? '#fca5a5' : AIC.muted,
                        borderColor: hovered === 'unlink' ? 'rgba(244,63,94,0.4)' : 'transparent',
                        background: hovered === 'unlink' ? 'rgba(244,63,94,0.08)' : 'transparent',
                    }}>
                    <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                        strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
                        <path d="M9.88 9.88a3 3 0 0 0 4.24 4.24" />
                        <path d="M10.73 5.08A10.43 10.43 0 0 1 12 5c7 0 10 7 10 7a13.16 13.16 0 0 1-1.67 2.68" />
                        <path d="M6.61 6.61A13.526 13.526 0 0 0 2 12s3 7 10 7a9.74 9.74 0 0 0 5.39-1.61" />
                        <line x1="2" y1="2" x2="22" y2="22" />
                    </svg>
                    Unlink
                </button>
            </div>
        </div>
    );
}

// ── Left pane: Context ───────────────────────────────────────────────────────
function StudioContextPane({ ai, onCollapse, allReqs, allReqsLoading, onPickReq, onCreateNew }) {
    const [open, setOpen] = useState({ req: true, params: false, output: false });
    const toggle = k => setOpen(s => ({ ...s, [k]: !s[k] }));
    const provider = ai.providers.find(p => p.id === ai.selectedProviderId);
    const folder = ai.folders.find(f => f.id === ai.selectedFolderId);
    const coverageLabel = COVERAGE_LEVELS.find(c => c.value === ai.coverageLevel)?.label || ai.coverageLevel;
    const detailLabel = DETAIL_LEVELS.find(d => d.value === ai.detailLevel)?.label || ai.detailLevel;
    const disabled = ai.generating || ai.accepting;

    const sectionHeader = (key, label, count) => (
        <button onClick={() => toggle(key)} style={{
            width: '100%', display: 'flex', alignItems: 'center', gap: 6,
            padding: '6px 0', background: 'transparent', border: 'none', cursor: 'pointer',
            color: open[key] ? AIC.text : AIC.muted,
            fontSize: 10.5, fontWeight: 600, letterSpacing: '0.08em',
            textTransform: 'uppercase', fontFamily: 'inherit',
        }}>
            <span style={{
                transform: open[key] ? 'rotate(90deg)' : 'rotate(0)',
                transition: 'transform .15s', display: 'inline-flex', color: AIC.muted,
            }}>{Icon.chevronR(10)}</span>
            <span style={{ flex: 1, textAlign: 'left' }}>{label}</span>
            {count != null && (
                <span style={{ fontFamily: MONO, fontSize: 10, color: AIC.muted, fontWeight: 500 }}>{count}</span>
            )}
        </button>
    );

    return (
        <aside style={{ padding: '10px 14px 20px', overflowY: 'auto', height: 'var(--aig-studio-h, calc(100vh - 180px))' }}>
            <div style={{
                display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                marginBottom: 6,
            }}>
                <span style={{
                    fontSize: 10.5, fontWeight: 600, letterSpacing: '0.08em',
                    textTransform: 'uppercase', color: AIC.muted,
                }}>Context</span>
                {onCollapse && (
                    <button className="aig-pane-collapse-btn" title="Collapse panel" onClick={onCollapse}>
                        <span style={{ transform: 'rotate(180deg)', display: 'inline-flex' }}>{Icon.chevronR(11)}</span>
                    </button>
                )}
            </div>

            {/* Requirement */}
            {sectionHeader('req', 'Linked requirement', ai.activeRequirement ? 1 : 0)}
            {open.req && !ai.activeRequirement && (
                <div style={{ marginTop: 6, marginBottom: 14 }}>
                    <ContextReqPicker
                        allReqs={allReqs}
                        allReqsLoading={allReqsLoading}
                        onPickReq={onPickReq}
                        onCreateNew={onCreateNew}
                    />
                </div>
            )}
            {open.req && ai.activeRequirement && (
                <div style={{ marginTop: 6, marginBottom: 14 }}>
                    <LinkedReqCard
                        req={ai.activeRequirement}
                        disabled={disabled}
                        onUnlink={() => {
                            if (ai.hasUnsaved && !window.confirm('Unlink requirement? Un-accepted drafts will be discarded.')) return;
                            ai.clearSession();
                        }}
                    />
                </div>
            )}

            {/* Parameters */}
            {sectionHeader('params', 'Parameters')}
            {open.params ? (
                <div style={{ marginTop: 6, marginBottom: 14 }}>
                    <div style={{
                        display: 'flex', alignItems: 'baseline', gap: 6,
                        fontSize: 10.5, color: AIC.text, fontWeight: 600,
                        letterSpacing: '0.04em', marginBottom: 2,
                    }}>
                        <span>Coverage</span>
                        <span style={{ color: AIC.muted, fontWeight: 400, fontSize: 10 }}>— which scenarios to test</span>
                    </div>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 4, marginBottom: 12 }}>
                        {COVERAGE_LEVELS.map((c, i) => {
                            const active = ai.coverageLevel === c.value;
                            const activeIdx = COVERAGE_LEVELS.findIndex(l => l.value === ai.coverageLevel);
                            return (
                                <button key={c.value}
                                    onClick={() => ai.setCoverageLevel(c.value)}
                                    disabled={disabled}
                                    title={c.desc}
                                    style={{
                                        display: 'flex', alignItems: 'center', gap: 8,
                                        padding: '6px 8px', textAlign: 'left',
                                        borderRadius: 6,
                                        border: `1px solid ${active ? 'rgba(99,102,241,0.5)' : AIC.border}`,
                                        background: active ? 'rgba(99,102,241,0.12)' : AIC.bg2,
                                        color: active ? AIC.text : AIC.dim,
                                        cursor: disabled ? 'not-allowed' : 'pointer',
                                        fontFamily: 'inherit',
                                    }}>
                                    <div style={{ display: 'flex', gap: 2, flexShrink: 0 }}>
                                        {[0, 1, 2].map(d => (
                                            <span key={d} style={{
                                                width: 5, height: 5, borderRadius: '50%',
                                                background: d <= i
                                                    ? (active ? 'var(--aig-indigo-strong)' : i <= activeIdx ? 'rgba(129,140,248,0.4)' : 'var(--aig-scale-dim)')
                                                    : 'var(--aig-scale-track)',
                                            }} />
                                        ))}
                                    </div>
                                    <div style={{ minWidth: 0, flex: 1 }}>
                                        <div style={{
                                            fontSize: 11.5, fontWeight: 600,
                                            color: active ? AIC.indigoSoft : AIC.text,
                                            lineHeight: 1.2,
                                        }}>{c.label}</div>
                                        <div style={{
                                            fontSize: 10.5, color: AIC.muted,
                                            lineHeight: 1.3, marginTop: 1,
                                            whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
                                        }}>{c.desc}</div>
                                    </div>
                                </button>
                            );
                        })}
                    </div>

                    <div style={{
                        display: 'flex', alignItems: 'baseline', gap: 6,
                        fontSize: 10.5, color: AIC.text, fontWeight: 600,
                        letterSpacing: '0.04em', marginBottom: 2,
                    }}>
                        <span>Detail</span>
                        <span style={{ color: AIC.muted, fontWeight: 400, fontSize: 10 }}>— step granularity</span>
                    </div>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                        {DETAIL_LEVELS.map((d, i) => {
                            const active = ai.detailLevel === d.value;
                            const activeIdx = DETAIL_LEVELS.findIndex(l => l.value === ai.detailLevel);
                            return (
                                <button key={d.value}
                                    onClick={() => ai.setDetailLevel(d.value)}
                                    disabled={disabled}
                                    title={d.desc}
                                    style={{
                                        display: 'flex', alignItems: 'center', gap: 8,
                                        padding: '6px 8px', textAlign: 'left',
                                        borderRadius: 6,
                                        border: `1px solid ${active ? 'rgba(99,102,241,0.5)' : AIC.border}`,
                                        background: active ? 'rgba(99,102,241,0.12)' : AIC.bg2,
                                        color: active ? AIC.text : AIC.dim,
                                        cursor: disabled ? 'not-allowed' : 'pointer',
                                        fontFamily: 'inherit',
                                    }}>
                                    <div style={{ display: 'flex', alignItems: 'flex-end', gap: 2, flexShrink: 0, height: 13 }}>
                                        {[0, 1, 2].map(b => (
                                            <span key={b} style={{
                                                width: 3, borderRadius: 1,
                                                height: 5 + b * 3,
                                                background: b <= i
                                                    ? (active ? 'var(--aig-indigo-strong)' : i <= activeIdx ? 'rgba(129,140,248,0.4)' : 'var(--aig-scale-dim)')
                                                    : 'var(--aig-scale-track)',
                                            }} />
                                        ))}
                                    </div>
                                    <div style={{ minWidth: 0, flex: 1 }}>
                                        <div style={{
                                            fontSize: 11.5, fontWeight: 600,
                                            color: active ? AIC.indigoSoft : AIC.text,
                                            lineHeight: 1.2,
                                        }}>{d.label}</div>
                                        <div style={{
                                            fontSize: 10.5, color: AIC.muted,
                                            lineHeight: 1.3, marginTop: 1,
                                            whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
                                        }}>{d.desc}</div>
                                    </div>
                                </button>
                            );
                        })}
                    </div>
                </div>
            ) : (
                <div style={{ padding: '2px 0 4px 18px', fontSize: 11, color: AIC.muted, lineHeight: 1.5 }}>
                    {coverageLabel} · <span style={{ color: AIC.dim }}>{detailLabel}</span>
                </div>
            )}

            {/* Output */}
            {sectionHeader('output', 'Output')}
            {open.output ? (
                <div style={{ marginTop: 6 }}>
                    <div style={{ fontSize: 10, color: AIC.muted, fontWeight: 500, marginBottom: 4, letterSpacing: '0.04em' }}>Destination folder</div>
                    <div style={{ marginBottom: 10 }}>
                        <FolderTreeSelect
                            folders={ai.folders}
                            value={ai.selectedFolderId}
                            onChange={ai.setSelectedFolderId}
                            disabled={disabled}
                        />
                    </div>
                    <label style={{
                        display: 'flex', alignItems: 'center', gap: 6,
                        fontSize: 11.5, color: AIC.dim, cursor: 'pointer',
                        padding: '2px 0', marginBottom: 10,
                    }}>
                        <input type="checkbox"
                            checked={ai.groupByCategory}
                            onChange={e => ai.setGroupByCategory(e.target.checked)}
                            disabled={disabled}
                            style={{ accentColor: AIC.indigo }} />
                        Create subfolders by group
                    </label>

                    <div style={{
                        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                        marginBottom: 4,
                    }}>
                        <div style={{ fontSize: 10, color: AIC.muted, fontWeight: 500, letterSpacing: '0.04em' }}>LLM Provider</div>
                        <Link
                            to="/settings#ai-test-generation"
                            title="Manage LLM providers in Settings"
                            style={{
                                display: 'inline-flex', alignItems: 'center', gap: 3,
                                fontSize: 10, color: AIC.muted, textDecoration: 'none',
                                letterSpacing: '0.02em',
                            }}
                            onMouseEnter={e => { e.currentTarget.style.color = AIC.indigoSoft; }}
                            onMouseLeave={e => { e.currentTarget.style.color = AIC.muted; }}
                        >
                            <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                <circle cx="12" cy="12" r="3"/>
                                <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
                            </svg>
                            Manage
                        </Link>
                    </div>
                    <select value={ai.selectedProviderId}
                        onChange={e => ai.setSelectedProviderId(e.target.value)}
                        disabled={disabled || ai.providers.length === 0}
                        className="modern-select"
                        style={{
                            width: '100%',
                            background: AIC.bg2, border: `1px solid ${AIC.border}`, color: AIC.text,
                            padding: '6px 8px', fontSize: 12, borderRadius: 6, fontFamily: 'inherit',
                        }}>
                        {ai.providers.length === 0 && <option value="">No providers configured</option>}
                        {ai.providers.map(p => (
                            <option key={p.id} value={p.id}>{p.label}{p.is_default ? ' ★' : ''}</option>
                        ))}
                    </select>
                    {provider?.model_name && (
                        <div style={{
                            marginTop: 6, fontSize: 10.5, color: AIC.muted,
                            fontFamily: MONO,
                        }}>{provider.model_name}</div>
                    )}
                </div>
            ) : (
                <div style={{ padding: '2px 0 4px 18px', fontSize: 11, color: AIC.muted, lineHeight: 1.5 }}>
                    {provider?.label || 'No provider'} → <span style={{ color: AIC.dim }}>{folder?.name || '—'}</span>
                </div>
            )}
        </aside>
    );
}

// ── Middle top: Header ───────────────────────────────────────────────────────
function StudioHeader({ ai, counts, totalDrafts, stage, onAcceptAll, onDiscardAll, onGenerate, onImport, disabled }) {
    const noProviders = ai.providers.length === 0;
    const noReq = !ai.activeRequirement;
    return (
        <div style={{ padding: '14px 20px 12px', borderBottom: `1px solid ${AIC.border}` }}>
            <div style={{
                display: 'flex', alignItems: 'center', gap: 8,
                fontSize: 11.5, color: AIC.muted, marginBottom: 6,
            }}>
                <span>AI</span>
                {Icon.chevronR(11)}
                <span>Generate tests</span>
                {ai.activeRequirement && (<>
                    {Icon.chevronR(11)}
                    <span style={{ color: AIC.text, fontFamily: MONO }}>{ai.activeRequirement.identifier}</span>
                </>)}
            </div>

            <div style={{
                display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                gap: 12, flexWrap: 'wrap',
            }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 0, flexWrap: 'wrap' }}>
                    <h1 style={{
                        margin: 0, fontSize: 19, fontWeight: 700,
                        letterSpacing: '-0.01em', color: AIC.text,
                    }}>Generate tests with AI</h1>
                    {stage === 'review' && (
                        <span style={{ fontSize: 12, color: AIC.dim }}>
                            · <b style={{ color: AIC.text }}>{totalDrafts}</b> drafts
                            · <b style={{ color: '#86efac' }}>{counts.accepted}</b> accepted
                            · <b style={{ color: '#fca5a5' }}>{counts.rejected}</b> rejected
                        </span>
                    )}
                    {stage === 'generating' && (
                        <span className="aig-gradient-text" style={{ fontSize: 12, fontWeight: 600 }}>
                            generating <GeneratingDots />
                        </span>
                    )}
                </div>

                <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                    {stage === 'review' && counts.pending > 0 && (
                        <>
                            <AIBtn variant="danger" onClick={onDiscardAll} disabled={disabled}>
                                {Icon.x(13)} Discard all
                            </AIBtn>
                            <AIBtn variant="success" onClick={onAcceptAll} disabled={disabled}>
                                {Icon.check(13)} Accept all ({counts.pending})
                            </AIBtn>
                        </>
                    )}
                    {onImport && stage !== 'generating' && (
                        <AIBtn variant="ghost" onClick={onImport} disabled={disabled}>
                            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/>
                            </svg>
                            Import
                            {ai.importDrafts?.length > 0 && (
                                <span style={{
                                    marginLeft: 4, padding: '1px 6px', borderRadius: 8,
                                    background: 'rgba(20,184,166,0.2)', color: '#5eead4',
                                    fontSize: 10.5, fontWeight: 700,
                                }}>{ai.importDrafts.length}</span>
                            )}
                        </AIBtn>
                    )}
                    {stage !== 'generating' && (
                        <AIBtn variant="primary" onClick={onGenerate} disabled={disabled || noProviders || noReq}
                            title={noReq ? 'Link a requirement to get started' : undefined}>
                            {Icon.sparkles(13)} {ai.hasGenerated ? 'Regenerate' : 'Generate'}
                        </AIBtn>
                    )}
                </div>
            </div>
        </div>
    );
}

// ── Middle: Composer ────────────────────────────────────────────────────────
function StudioComposer({ ai, stage, disabled }) {
    const tagPresets = ['Happy paths', 'Edge cases', 'Negative', 'Accessibility', 'Performance', 'Security'];
    const addTag = (t) => {
        const cur = (ai.additionalInstructions || '').trim();
        if (cur.toLowerCase().includes(t.toLowerCase())) return;
        ai.setAdditionalInstructions(cur ? `${cur}, ${t.toLowerCase()}` : `Include ${t.toLowerCase()}`);
    };
    return (
        <div style={{ padding: '14px 20px', borderBottom: `1px solid ${AIC.border}`, position: 'relative' }}>
            <div style={{
                display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                marginBottom: 8,
            }}>
                <SectionLabel>Prompt</SectionLabel>
                <div style={{ display: 'flex', gap: 6 }}>
                    {ai.activeRequirement && <ReqChip id={ai.activeRequirement.identifier} />}
                    {ai.providers.find(p => p.id === ai.selectedProviderId) && (
                        <Pill tone="neutral">
                            Model: {ai.providers.find(p => p.id === ai.selectedProviderId)?.label}
                        </Pill>
                    )}
                </div>
            </div>
            <textarea
                value={ai.additionalInstructions}
                onChange={e => ai.setAdditionalInstructions(e.target.value)}
                rows={4}
                disabled={disabled}
                placeholder="Describe the tests you want the AI to draft — focus areas, scenarios, constraints…"
                style={{
                    width: '100%', background: AIC.bg2, border: `1px solid ${AIC.border}`,
                    color: AIC.text, padding: '9px 12px', fontSize: 13.5, lineHeight: 1.55,
                    borderRadius: 7, fontFamily: 'inherit', resize: 'vertical', minHeight: 90,
                    outline: 'none',
                }}
            />
            <div style={{ display: 'flex', gap: 6, marginTop: 8, flexWrap: 'wrap' }}>
                {tagPresets.map(t => (
                    <button key={t} onClick={() => addTag(t)} disabled={disabled}
                        style={{
                            display: 'inline-flex', alignItems: 'center', gap: 5,
                            padding: '4px 9px', borderRadius: 6,
                            fontSize: 11.5, color: AIC.text,
                            background: 'rgba(99,102,241,0.1)', border: '1px solid rgba(99,102,241,0.25)',
                            cursor: disabled ? 'not-allowed' : 'pointer', fontFamily: 'inherit',
                        }}>{t}</button>
                ))}
            </div>

            {stage === 'generating' && (
                <div style={{ marginTop: 14 }}>
                    <div style={{
                        display: 'flex', justifyContent: 'space-between',
                        fontSize: 11, color: AIC.muted, marginBottom: 6,
                    }}>
                        <span>Drafting candidates…</span>
                        <GeneratingDots />
                    </div>
                    <div style={{
                        height: 4, background: 'var(--aig-surface-hover)',
                        borderRadius: 2, overflow: 'hidden', position: 'relative',
                    }}>
                        <div className="aig-progress-shuttle" style={{
                            position: 'absolute', top: 0, bottom: 0, width: '40%',
                            background: 'linear-gradient(90deg, transparent, #6366f1, #14b8a6, transparent)',
                        }} />
                    </div>
                </div>
            )}
        </div>
    );
}

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

function stripHtmlForDisplay(html) {
    if (!html) return '';
    return String(html)
        .replace(/<style[\s\S]*?<\/style>/gi, '')
        .replace(/<script[\s\S]*?<\/script>/gi, '')
        .replace(/<\/(p|div|br|li|tr|h[1-6])>/gi, '\n')
        .replace(/<br\s*\/?>/gi, '\n')
        .replace(/<[^>]+>/g, '')
        .replace(/&nbsp;/g, ' ')
        .replace(/&amp;/g, '&')
        .replace(/&lt;/g, '<')
        .replace(/&gt;/g, '>')
        .replace(/&quot;/g, '"')
        .replace(/&#39;/g, "'")
        .replace(/\n{3,}/g, '\n\n')
        .trim();
}

function LlmFeedbackPanel({ debug }) {
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

// ── Middle: Drafts list ─────────────────────────────────────────────────────
function DraftRow({ draft, status, selected, onSelect, onAccept, onReject, disabled }) {
    const dimmed = status === 'rejected';
    return (
        <div onClick={onSelect}
            style={{
                padding: '12px 20px', borderBottom: `1px solid ${AIC.border2}`, cursor: 'pointer',
                background: selected ? 'rgba(99,102,241,0.08)' : 'transparent',
                borderLeft: selected ? `3px solid ${AIC.indigo}` : `3px solid transparent`,
                opacity: dimmed ? 0.55 : 1,
                transition: 'background .15s',
            }}>
            <div style={{ display: 'flex', gap: 12, alignItems: 'flex-start' }}>
                <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{
                        display: 'flex', gap: 8, alignItems: 'center',
                        marginBottom: 5, flexWrap: 'wrap',
                    }}>
                        <StatusPill status={status} />
                        {draft.category && (() => {
                            const t = categoryTone(draft.category);
                            return (
                                <span style={{
                                    padding: '2px 8px', borderRadius: 999,
                                    fontSize: 10.5, fontWeight: 600, letterSpacing: '0.02em',
                                    background: t.bg, border: `1px solid ${t.bd}`, color: t.fg,
                                }}>{draft.category}</span>
                            );
                        })()}
                    </div>
                    <div style={{
                        fontSize: 13.5, color: AIC.text, fontWeight: 600, marginBottom: 3,
                        textDecoration: dimmed ? 'line-through' : 'none',
                    }}>
                        {draft.name || 'Untitled draft'}
                    </div>
                    {draft.description && (
                        <div style={{
                            fontSize: 12, color: AIC.dim, lineHeight: 1.45, marginBottom: 6,
                            display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical',
                            overflow: 'hidden',
                        }}>
                            {String(draft.description).replace(/<[^>]+>/g, '')}
                        </div>
                    )}
                    <div style={{
                        display: 'flex', gap: 16, fontSize: 11, color: AIC.muted,
                        alignItems: 'center', flexWrap: 'wrap',
                    }}>
                        <span>{(draft.steps || []).length} steps</span>
                    </div>
                </div>
                {status === 'pending' && (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 5 }}
                        onClick={e => e.stopPropagation()}>
                        <AIBtn variant="success" onClick={onAccept} disabled={disabled}
                            style={{ padding: '5px 10px', fontSize: 11.5 }}>
                            {Icon.check(12)} Accept
                        </AIBtn>
                        <AIBtn variant="danger" onClick={onReject} disabled={disabled}
                            style={{ padding: '5px 10px', fontSize: 11.5 }}>
                            {Icon.x(12)} Reject
                        </AIBtn>
                    </div>
                )}
            </div>
        </div>
    );
}

function StudioDraftsList({
    ai, drafts, allDrafts, statuses, selectedId, onSelect,
    filter, setFilter, groupBy, setGroupBy, onAccept, onReject, onAcceptGroup,
    counts, stage, disabled,
}) {
    const [collapsedGroups, setCollapsedGroups] = useState({});
    const toggleGroup = (k) => setCollapsedGroups(s => ({ ...s, [k]: !s[k] }));
    const linkedReqId = ai.activeRequirement?.identifier;

    if (stage === 'compose') {
        return (
            <div style={{
                flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center',
                padding: 40, minHeight: 280,
            }}>
                <div style={{ textAlign: 'center', maxWidth: 360, color: AIC.dim }}>
                    <div style={{
                        width: 48, height: 48, borderRadius: 12, margin: '0 auto 14px',
                        background: 'linear-gradient(135deg, rgba(99,102,241,0.2), rgba(20,184,166,0.2))',
                        border: '1px solid rgba(99,102,241,0.3)',
                        display: 'flex', alignItems: 'center', justifyContent: 'center',
                        color: AIC.indigoSoft,
                    }}>{Icon.sparkles(22)}</div>
                    <div style={{ fontSize: 14, color: AIC.text, fontWeight: 600, marginBottom: 4 }}>Ready to generate</div>
                    <div style={{ fontSize: 12.5, lineHeight: 1.55 }}>
                        Review your prompt and linked context, then hit <b>Generate</b>. Drafts will appear here for you to accept or reject — nothing is saved until you approve.
                    </div>
                </div>
            </div>
        );
    }

    if (stage === 'generating') {
        return (
            <div style={{ flex: 1, padding: '12px 0', overflowY: 'auto' }}>
                <div style={{ padding: '0 20px' }}>
                    <SectionLabel>Drafting <GeneratingDots /></SectionLabel>
                </div>
                {[1, 2, 3, 4].map(i => <DraftSkeleton key={i} delay={i * 0.12} />)}
            </div>
        );
    }

    const groups = groupDrafts(drafts, groupBy, linkedReqId);

    return (
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0 }}>
            <div style={{
                padding: '10px 20px', borderBottom: `1px solid ${AIC.border2}`,
                display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                gap: 12, flexWrap: 'wrap',
            }}>
                <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
                    <FilterTab active={filter === 'all'} onClick={() => setFilter('all')}>All <span style={{ opacity: 0.6 }}>{allDrafts.length}</span></FilterTab>
                    <FilterTab active={filter === 'pending'} onClick={() => setFilter('pending')}>Pending <span style={{ opacity: 0.6 }}>{counts.pending}</span></FilterTab>
                    <FilterTab active={filter === 'accepted'} onClick={() => setFilter('accepted')}>Accepted <span style={{ opacity: 0.6 }}>{counts.accepted}</span></FilterTab>
                    <FilterTab active={filter === 'rejected'} onClick={() => setFilter('rejected')}>Rejected <span style={{ opacity: 0.6 }}>{counts.rejected}</span></FilterTab>
                </div>
                <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                    <span style={{
                        fontSize: 10.5, color: AIC.muted, textTransform: 'uppercase',
                        letterSpacing: '0.08em', fontWeight: 600,
                    }}>Group by</span>
                    <Segmented value={groupBy} onChange={setGroupBy} options={[
                        { value: 'category', label: 'Category' },
                        { value: 'requirement', label: 'Req' },
                        { value: 'none', label: 'Flat' },
                    ]} />
                </div>
            </div>

            <div style={{ flex: 1, overflowY: 'auto' }}>
                {groups.map(grp => {
                    const pendingIds = grp.items.filter(d => statuses[d.temp_id] === 'pending').map(d => d.temp_id);
                    const tone = groupBy === 'category'
                        ? categoryTone(grp.label)
                        : { bg: 'var(--aig-surface-tint)', bd: AIC.border, fg: AIC.dim };
                    const collapsed = groupBy !== 'none' && collapsedGroups[grp.key];
                    return (
                        <div key={grp.key} style={{
                            marginBottom: groupBy !== 'none' ? 10 : 0,
                            border: groupBy !== 'none' ? `1px solid ${tone.bd}` : 'none',
                            borderLeftWidth: groupBy !== 'none' ? 3 : 0,
                            borderRadius: groupBy !== 'none' ? 8 : 0,
                            overflow: 'hidden',
                            marginLeft: groupBy !== 'none' ? 12 : 0,
                            marginRight: groupBy !== 'none' ? 12 : 0,
                        }}>
                            {groupBy !== 'none' && (
                                <div onClick={() => toggleGroup(grp.key)}
                                    style={{
                                        display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                                        padding: '10px 20px 10px 17px', position: 'sticky', top: 0, zIndex: 1,
                                        background: `linear-gradient(${tone.bg}, ${tone.bg}), var(--bg-primary)`,
                                        borderBottom: `1px solid ${tone.bd}`,
                                        cursor: 'pointer',
                                    }}>
                                    <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                                        <span style={{
                                            transform: collapsed ? 'rotate(0)' : 'rotate(90deg)',
                                            transition: 'transform .15s', display: 'inline-flex', color: tone.fg,
                                        }}>{Icon.chevronR(11)}</span>
                                        <span style={{
                                            padding: '2px 10px', borderRadius: 999,
                                            fontSize: 11, fontWeight: 600, letterSpacing: '0.02em',
                                            background: tone.bg, border: `1px solid ${tone.bd}`, color: tone.fg,
                                        }}>{grp.label}</span>
                                        <span style={{
                                            fontSize: 10.5, color: tone.fg, opacity: 0.8,
                                            fontVariantNumeric: 'tabular-nums', fontWeight: 600,
                                        }}>
                                            {grp.items.length}
                                        </span>
                                    </div>
                                    {pendingIds.length > 0 && (
                                        <AIBtn variant="ghost"
                                            style={{ padding: '3px 8px', fontSize: 11, color: tone.fg }}
                                            onClick={e => { e.stopPropagation(); onAcceptGroup(grp.items.filter(d => statuses[d.temp_id] === 'pending')); }}
                                            disabled={disabled}>
                                            {Icon.check(11)} Accept {pendingIds.length}
                                        </AIBtn>
                                    )}
                                </div>
                            )}
                            {!collapsed && grp.items.map(d => (
                                <DraftRow key={d.temp_id}
                                    draft={d}
                                    status={statuses[d.temp_id]}
                                    selected={selectedId === d.temp_id}
                                    onSelect={() => onSelect(d.temp_id)}
                                    onAccept={() => onAccept(d)}
                                    onReject={() => onReject(d.temp_id)}
                                    disabled={disabled}
                                />
                            ))}
                        </div>
                    );
                })}
                {drafts.length === 0 && (
                    <div style={{
                        padding: 40, textAlign: 'center', color: AIC.muted, fontSize: 12.5,
                    }}>
                        No drafts match this filter.
                    </div>
                )}
            </div>
        </div>
    );
}

// ── Right pane: Detail ──────────────────────────────────────────────────────
function StudioDraftDetail({
    draft, status, onAccept, onReject, onCollapse, stage, linkedReqId, disabled,
}) {
    if (!draft || stage !== 'review') {
        return (
            <aside style={{ padding: 22, color: AIC.dim, fontSize: 12.5, lineHeight: 1.55 }}>
                <SectionLabel right={onCollapse ? (
                    <button className="aig-pane-collapse-btn" title="Collapse panel" onClick={onCollapse}>
                        {Icon.chevronR(11)}
                    </button>
                ) : null}>Draft detail</SectionLabel>
                <div style={{ color: AIC.muted }}>
                    {stage === 'compose' && 'Draft details will appear here after you generate.'}
                    {stage === 'generating' && 'Draft details will appear here once generation completes.'}
                </div>
            </aside>
        );
    }

    return (
        <aside style={{
            padding: '18px 18px 24px', overflowY: 'auto',
            height: 'var(--aig-studio-h, calc(100vh - 180px))', minHeight: 400,
        }}>
            <SectionLabel right={
                <div style={{ display: 'flex', gap: 4, alignItems: 'center' }}>
                    {onCollapse && (
                        <button className="aig-pane-collapse-btn" title="Collapse panel" onClick={onCollapse}>
                            {Icon.chevronR(11)}
                        </button>
                    )}
                </div>
            }>Draft detail</SectionLabel>

            <div style={{ display: 'flex', gap: 6, marginBottom: 10, flexWrap: 'wrap' }}>
                <StatusPill status={status} />
                {draft.category && (() => {
                    const t = categoryTone(draft.category);
                    return (
                        <span style={{
                            padding: '2px 8px', borderRadius: 999,
                            fontSize: 10.5, fontWeight: 600, letterSpacing: '0.02em',
                            background: t.bg, border: `1px solid ${t.bd}`, color: t.fg,
                        }}>{draft.category}</span>
                    );
                })()}
            </div>

            <h2 style={{
                fontSize: 15.5, fontWeight: 600, color: AIC.text,
                margin: '4px 0 10px', lineHeight: 1.35,
            }}>{draft.name || 'Untitled draft'}</h2>

            {status === 'pending' && (
                <div style={{ display: 'flex', gap: 8, marginBottom: 14 }}>
                    <AIBtn variant="success" onClick={onAccept} disabled={disabled}
                        style={{ flex: 1, justifyContent: 'center' }}>
                        {Icon.check(13)} Accept &amp; add
                    </AIBtn>
                    <AIBtn variant="danger" onClick={onReject} disabled={disabled}
                        style={{ flex: 1, justifyContent: 'center' }}>
                        {Icon.x(13)} Reject
                    </AIBtn>
                </div>
            )}

            {draft.description && (
                <div style={{
                    padding: '10px 12px', background: 'rgba(99,102,241,0.06)',
                    border: '1px solid rgba(99,102,241,0.2)', borderRadius: 8, marginBottom: 14,
                }}>
                    <div style={{
                        display: 'flex', gap: 6, alignItems: 'center',
                        color: AIC.indigoSoft, fontSize: 11, fontWeight: 600, marginBottom: 5,
                    }}>
                        {Icon.sparkles(11)} AI description
                    </div>
                    <SafeHTML html={draft.description}
                        style={{ fontSize: 12, color: AIC.dim, lineHeight: 1.55 }} />
                </div>
            )}

            <div style={{
                display: 'grid', gridTemplateColumns: '1fr 1fr',
                gap: 10, marginBottom: 14,
            }}>
                <MiniStat label="Steps" value={(draft.steps || []).length} />
                <MiniStat label="Est. review" value={`${Math.max(1, Math.round((draft.steps || []).length * 0.5))} min`} />
            </div>

            <SectionLabel>Requirements &amp; tags</SectionLabel>
            <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', marginBottom: 14 }}>
                {linkedReqId && <ReqChip id={linkedReqId} />}
                {draft.category && (() => {
                    const t = categoryTone(draft.category);
                    return (
                        <span style={{
                            padding: '2px 8px', borderRadius: 999,
                            fontSize: 10.5, fontWeight: 600, letterSpacing: '0.02em',
                            background: t.bg, border: `1px solid ${t.bd}`, color: t.fg,
                        }}>{draft.category}</span>
                    );
                })()}
            </div>

            <SectionLabel>Steps</SectionLabel>
            <Stepper steps={draft.steps} />
        </aside>
    );
}

// ── Width constraints for resizable panes ───────────────────────────────────
const LEFT_MIN = 220, LEFT_MAX = 520, LEFT_DEFAULT = 300;
const RIGHT_MIN = 260, RIGHT_MAX = 560, RIGHT_DEFAULT = 380;

function clamp(v, lo, hi) { return Math.max(lo, Math.min(hi, v)); }

function readStoredWidth(key, def, lo, hi) {
    try {
        const v = parseInt(localStorage.getItem(key) || '', 10);
        return isNaN(v) ? def : clamp(v, lo, hi);
    } catch { return def; }
}

// ── Main container ──────────────────────────────────────────────────────────
export default function AIGenerateStudio() {
    const ai = useAIGeneration();
    const [leftCollapsed, setLeftCollapsed] = useState(false);
    const [rightCollapsed, setRightCollapsed] = useState(false);
    const [filter, setFilter] = useState('pending');
    const [groupBy, setGroupBy] = useState('category');
    const [selectedDraftId, setSelectedDraftId] = useState(null);

    // Requirements-catalog + import-modal state (folded in from the former AIGeneratePage shell)
    const [importOpen, setImportOpen] = useState(() => {
        try {
            if (sessionStorage.getItem('ttgo_import_state')) return true;
        } catch {}
        return false;
    });
    const [allReqs, setAllReqs] = useState([]);
    const [allReqsLoading, setAllReqsLoading] = useState(true);
    const [createModalOpen, setCreateModalOpen] = useState(false);

    useEffect(() => {
        setAllReqsLoading(true);
        requirementsApi.list()
            .then(data => setAllReqs(Array.isArray(data) ? data : []))
            .catch(() => setAllReqs([]))
            .finally(() => setAllReqsLoading(false));
    }, []);

    const handleReqCreated = (newReq) => {
        setAllReqs(prev => [...prev, newReq]);
        setCreateModalOpen(false);
        if (ai.hasSession) ai.switchRequirement(newReq);
        else ai.openSession(newReq, '');
    };

    const [leftWidth, setLeftWidth] = useState(() =>
        readStoredWidth('aig-studio-left-w', LEFT_DEFAULT, LEFT_MIN, LEFT_MAX));
    const [rightWidth, setRightWidth] = useState(() =>
        readStoredWidth('aig-studio-right-w', RIGHT_DEFAULT, RIGHT_MIN, RIGHT_MAX));
    const [dragging, setDragging] = useState(null);

    useEffect(() => {
        try { localStorage.setItem('aig-studio-left-w', String(leftWidth)); } catch {}
    }, [leftWidth]);
    useEffect(() => {
        try { localStorage.setItem('aig-studio-right-w', String(rightWidth)); } catch {}
    }, [rightWidth]);

    const startResize = (side) => (e) => {
        e.preventDefault();
        setDragging(side);
        const startX = e.clientX;
        const startL = leftWidth;
        const startR = rightWidth;
        const onMove = (ev) => {
            const dx = ev.clientX - startX;
            if (side === 'left') setLeftWidth(clamp(startL + dx, LEFT_MIN, LEFT_MAX));
            else setRightWidth(clamp(startR - dx, RIGHT_MIN, RIGHT_MAX));
        };
        const onUp = () => {
            setDragging(null);
            window.removeEventListener('mousemove', onMove);
            window.removeEventListener('mouseup', onUp);
            document.body.style.cursor = '';
            document.body.style.userSelect = '';
        };
        document.body.style.cursor = 'col-resize';
        document.body.style.userSelect = 'none';
        window.addEventListener('mousemove', onMove);
        window.addEventListener('mouseup', onUp);
    };

    const handleResetWidth = (side) => () => {
        if (side === 'left') setLeftWidth(LEFT_DEFAULT);
        else setRightWidth(RIGHT_DEFAULT);
    };

    const drafts = ai.drafts || [];
    const stage = ai.generating
        ? 'generating'
        : drafts.length > 0 ? 'review' : 'compose';
    const disabled = ai.generating || ai.accepting;

    const statusOf = (d) => ai.acceptedIds.has(d.temp_id)
        ? 'accepted'
        : ai.discardedIds.has(d.temp_id)
            ? 'rejected'
            : 'pending';

    const statuses = useMemo(() => {
        const o = {};
        drafts.forEach(d => { o[d.temp_id] = statusOf(d); });
        return o;
    }, [drafts, ai.acceptedIds, ai.discardedIds]);

    const counts = useMemo(() => {
        const c = { pending: 0, accepted: 0, rejected: 0 };
        Object.values(statuses).forEach(s => { c[s] = (c[s] || 0) + 1; });
        return c;
    }, [statuses]);

    const filtered = useMemo(() => drafts.filter(d => {
        const s = statuses[d.temp_id];
        if (filter === 'all') return true;
        return s === filter;
    }), [drafts, statuses, filter]);

    useEffect(() => {
        if (selectedDraftId && drafts.find(d => d.temp_id === selectedDraftId)) return;
        const pending = drafts.find(d => statuses[d.temp_id] === 'pending');
        setSelectedDraftId(pending?.temp_id || drafts[0]?.temp_id || null);
    }, [drafts, selectedDraftId, statuses]);

    const selectedDraft = drafts.find(d => d.temp_id === selectedDraftId) || null;

    const handleGenerate = () => {
        if (ai.hasUnsaved && !window.confirm('This will replace un-accepted drafts. Continue?')) return;
        ai.startGeneration();
    };
    const handleAccept = (d) => ai.acceptDraft(d);
    const handleReject = (tempId) => ai.discardDraft(tempId);
    const handleAcceptAll = () => ai.acceptAllPending();
    const handleDiscardAll = () => {
        if (!window.confirm('Discard all pending drafts?')) return;
        ai.discardAllPending();
    };
    const handleAcceptGroup = (group) => ai.acceptDrafts(group);

    const studioGridNode = (
        <>
            {/* Error / warning banners */}
            {ai.templateWarning && (
                <StudioBanner tone="amber" icon={Icon.alert(13)}>
                    {ai.templateWarning}
                </StudioBanner>
            )}
            {ai.generationError && (
                <StudioBanner tone="red" icon={Icon.x(13)} action={
                    <AIBtn variant="ghost" onClick={ai.startGeneration} disabled={ai.generating}
                        style={{ fontSize: 11, padding: '3px 8px', color: '#fca5a5' }}>
                        Retry
                    </AIBtn>
                }>{ai.generationError}</StudioBanner>
            )}
            {ai.providers.length === 0 && (
                <StudioBanner tone="amber" icon={Icon.alert(13)}>
                    No LLM providers configured. Add one in <Link to="/settings" style={{ color: AIC.indigoSoft, textDecoration: 'none', fontWeight: 500 }}>Settings → AI Test Generation</Link>.
                </StudioBanner>
            )}

            <div
                className={`aig-studio-grid${leftCollapsed ? ' collapse-left' : ''}${rightCollapsed ? ' collapse-right' : ''}${dragging ? ' is-dragging' : ''}`}
                style={{
                    '--aig-left': `${leftWidth}px`,
                    '--aig-right': `${rightWidth}px`,
                }}
            >
                {/* LEFT */}
                {leftCollapsed ? (
                    <aside className="aig-pane-rail" style={{ borderRight: `1px solid ${AIC.border}` }}>
                        <button className="aig-rail-btn" title="Expand context" onClick={() => setLeftCollapsed(false)}>
                            {Icon.chevronR(13)}
                        </button>
                        <span className="aig-rail-label">Context</span>
                    </aside>
                ) : (
                    <StudioContextPane
                        ai={ai}
                        onCollapse={() => setLeftCollapsed(true)}
                        onChangeRequirement={null}
                        allReqs={allReqs}
                        allReqsLoading={allReqsLoading}
                        onPickReq={(r) => ai.openSession(r, '')}
                        onCreateNew={() => setCreateModalOpen(true)}
                    />
                )}

                {/* CENTER */}
                <div style={{
                    borderLeft: `1px solid ${AIC.border}`,
                    borderRight: `1px solid ${AIC.border}`,
                    display: 'flex', flexDirection: 'column', minWidth: 0,
                    height: 'var(--aig-studio-h, calc(100vh - 180px))',
                }}>
                    <StudioHeader
                        ai={ai}
                        counts={counts}
                        totalDrafts={drafts.length}
                        stage={stage}
                        onAcceptAll={handleAcceptAll}
                        onDiscardAll={handleDiscardAll}
                        onGenerate={handleGenerate}
                        onImport={() => setImportOpen(true)}
                        disabled={disabled}
                    />
                    <StudioComposer ai={ai} stage={stage} disabled={disabled} />
                    {ai.lastDebug && <LlmFeedbackPanel debug={ai.lastDebug} />}
                    <StudioDraftsList
                        ai={ai}
                        drafts={filtered}
                        allDrafts={drafts}
                        statuses={statuses}
                        selectedId={selectedDraftId}
                        onSelect={setSelectedDraftId}
                        filter={filter} setFilter={setFilter}
                        groupBy={groupBy} setGroupBy={setGroupBy}
                        onAccept={handleAccept}
                        onReject={handleReject}
                        onAcceptGroup={handleAcceptGroup}
                        counts={counts}
                        stage={stage}
                        disabled={disabled}
                    />
                </div>

                {/* RIGHT */}
                {rightCollapsed ? (
                    <aside className="aig-pane-rail" style={{ borderLeft: `1px solid ${AIC.border}` }}>
                        <button className="aig-rail-btn" title="Expand detail" onClick={() => setRightCollapsed(false)}>
                            <span style={{ transform: 'rotate(180deg)', display: 'inline-flex' }}>{Icon.chevronR(13)}</span>
                        </button>
                        <span className="aig-rail-label">Detail</span>
                    </aside>
                ) : (
                    <StudioDraftDetail
                        draft={selectedDraft}
                        status={selectedDraft ? statuses[selectedDraft.temp_id] : null}
                        onAccept={() => selectedDraft && handleAccept(selectedDraft)}
                        onReject={() => selectedDraft && handleReject(selectedDraft.temp_id)}
                        onCollapse={() => setRightCollapsed(true)}
                        stage={stage}
                        linkedReqId={ai.activeRequirement?.identifier}
                        disabled={disabled}
                    />
                )}

                {/* Drag handles — hidden when the adjacent pane is collapsed */}
                {!leftCollapsed && (
                    <div
                        className={`aig-resizer aig-resizer-left${dragging === 'left' ? ' is-active' : ''}`}
                        onMouseDown={startResize('left')}
                        onDoubleClick={handleResetWidth('left')}
                        title="Drag to resize · double-click to reset"
                        role="separator"
                        aria-orientation="vertical"
                        aria-label="Resize context panel"
                    />
                )}
                {!rightCollapsed && (
                    <div
                        className={`aig-resizer aig-resizer-right${dragging === 'right' ? ' is-active' : ''}`}
                        onMouseDown={startResize('right')}
                        onDoubleClick={handleResetWidth('right')}
                        title="Drag to resize · double-click to reset"
                        role="separator"
                        aria-orientation="vertical"
                        aria-label="Resize detail panel"
                    />
                )}
            </div>
        </>
    );

    const exitImport = () => {
        ai.clearImport();
        setImportOpen(false);
    };
    const importContent = ai.importDrafts.length > 0
        ? <AIImportReview onAccepted={exitImport} onBack={exitImport} />
        : <AIImportPanel onParsed={() => { /* drafts populated via context */ }} onCancel={exitImport} />;

    return (
        <>
            <StudioStyles />
            <PageShellStyles />
            <div style={pageStyles.studioOnlyWrap}>
                {studioGridNode}
            </div>

            {createModalOpen && (
                <CreateRequirementModal
                    onClose={() => setCreateModalOpen(false)}
                    onCreated={handleReqCreated}
                />
            )}

            {importOpen && (
                <ImportModal
                    onClose={exitImport}
                    hasDrafts={ai.importDrafts.length > 0}
                >
                    {importContent}
                </ImportModal>
            )}
        </>
    );
}

function ImportModal({ onClose, hasDrafts, children }) {
    useEffect(() => {
        const onKey = (e) => { if (e.key === 'Escape') onClose(); };
        window.addEventListener('keydown', onKey);
        document.body.style.overflow = 'hidden';
        return () => {
            window.removeEventListener('keydown', onKey);
            document.body.style.overflow = '';
        };
    }, [onClose]);
    return (
        <div style={pageStyles.importModalBackdrop} onClick={onClose}>
            <div style={pageStyles.importModalCard} onClick={e => e.stopPropagation()}>
                <div style={pageStyles.importModalHead}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 0 }}>
                        <div style={{ ...pageStyles.aiIcon, width: 32, height: 32, background: 'linear-gradient(135deg, rgba(20,184,166,0.2), rgba(56,189,248,0.15))', borderColor: 'rgba(20,184,166,0.25)', color: '#14b8a6' }}>
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/>
                            </svg>
                        </div>
                        <div>
                            <h3 style={pageStyles.importModalTitle}>{hasDrafts ? 'Review imported drafts' : 'Import existing test cases'}</h3>
                            <p style={pageStyles.importModalSub}>Paste, upload, or drag AI-generated content — JSON, CSV, Markdown, or numbered lists.</p>
                        </div>
                    </div>
                    <button onClick={onClose} style={pageStyles.importModalClose} aria-label="Close import">
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
                        </svg>
                    </button>
                </div>
                <div style={pageStyles.importModalBody}>{children}</div>
            </div>
        </div>
    );
}

function StudioBanner({ tone = 'amber', icon, children, action }) {
    const toneMap = {
        amber: { bg: 'rgba(234,179,8,0.08)', bd: 'rgba(234,179,8,0.3)', fg: '#fde047' },
        red: { bg: 'rgba(239,68,68,0.08)', bd: 'rgba(239,68,68,0.3)', fg: '#fca5a5' },
    };
    const t = toneMap[tone] || toneMap.amber;
    return (
        <div style={{
            margin: '0 0 10px', padding: '8px 14px',
            background: t.bg, border: `1px solid ${t.bd}`, borderRadius: 8,
            display: 'flex', alignItems: 'center', gap: 10,
            fontSize: 12.5, color: t.fg,
        }}>
            <span style={{ flexShrink: 0, display: 'inline-flex' }}>{icon}</span>
            <span style={{ flex: 1 }}>{children}</span>
            {action}
        </div>
    );
}

// ── Injected styles for Studio-specific rules ───────────────────────────────
function StudioStyles() {
    return (
        <style>{`
            .aig-studio-grid {
                position: relative;
                display: grid;
                grid-template-columns: var(--aig-left, 300px) minmax(0, 1fr) var(--aig-right, 380px);
                transition: grid-template-columns .18s ease;
                border-top: 1px solid var(--border-color);
                background: var(--bg-primary);
            }
            .aig-studio-grid.is-dragging {
                transition: none;
                user-select: none;
            }
            .aig-studio-grid.collapse-left  { grid-template-columns: 40px minmax(0, 1fr) var(--aig-right, 380px) !important; }
            .aig-studio-grid.collapse-right { grid-template-columns: var(--aig-left, 300px) minmax(0, 1fr) 40px !important; }
            .aig-studio-grid.collapse-left.collapse-right { grid-template-columns: 40px minmax(0, 1fr) 40px !important; }
            @media (max-width: 960px) {
                .aig-studio-grid { grid-template-columns: minmax(0, 1fr) !important; }
                .aig-studio-grid > aside { height: auto !important; border-bottom: 1px solid ${AIC.border}; }
                .aig-resizer { display: none !important; }
            }

            .aig-resizer {
                position: absolute;
                top: 0; bottom: 0;
                width: 6px;
                cursor: col-resize;
                z-index: 5;
                background: transparent;
                touch-action: none;
            }
            .aig-resizer::after {
                content: '';
                position: absolute;
                top: 0; bottom: 0;
                left: 50%; width: 1px; margin-left: -0.5px;
                background: transparent;
                transition: background .14s ease;
            }
            .aig-resizer:hover::after,
            .aig-resizer.is-active::after {
                background: linear-gradient(180deg, transparent 0%, ${AIC.indigo} 25%, ${AIC.teal} 75%, transparent 100%);
                box-shadow: 0 0 8px rgba(99,102,241,0.5);
            }
            .aig-resizer-left  { left: calc(var(--aig-left, 300px) - 3px); }
            .aig-resizer-right { right: calc(var(--aig-right, 380px) - 3px); }
            .aig-studio-grid.collapse-left  .aig-resizer-left  { display: none; }
            .aig-studio-grid.collapse-right .aig-resizer-right { display: none; }

            .aig-pane-rail {
                width: 40px; height: var(--aig-studio-h, calc(100vh - 180px));
                display: flex; flex-direction: column; align-items: center;
                padding: 10px 0; gap: 14px;
                background: var(--aig-surface-tint);
            }
            .aig-rail-btn {
                width: 26px; height: 26px; border-radius: 6px;
                background: var(--aig-surface-tint);
                border: 1px solid var(--border-color);
                color: var(--text-secondary);
                display: inline-flex; align-items: center; justify-content: center;
                cursor: pointer;
            }
            .aig-rail-btn:hover { background: var(--aig-surface-tint-strong); color: var(--text-primary); }
            .aig-rail-label {
                writing-mode: vertical-rl; transform: rotate(180deg);
                font-size: 10.5px; color: var(--sidebar-muted); letter-spacing: 0.1em;
                text-transform: uppercase; font-weight: 600; user-select: none;
            }
            .aig-pane-collapse-btn {
                width: 22px; height: 22px; border-radius: 5px;
                background: transparent; border: 1px solid transparent;
                color: var(--sidebar-muted);
                display: inline-flex; align-items: center; justify-content: center;
                cursor: pointer; transition: all .12s;
            }
            .aig-pane-collapse-btn:hover {
                background: var(--aig-surface-hover); color: var(--text-primary);
                border-color: var(--border-color);
            }

            .aig-gradient-text {
                background: linear-gradient(100deg, #a5b4fc, #5eead4 45%, #a5b4fc);
                background-size: 200% 100%;
                -webkit-background-clip: text; background-clip: text; color: transparent;
                animation: aigShimmer 3.5s linear infinite;
            }
            .aig-progress-shuttle { animation: aigShuttle 1.4s cubic-bezier(0.4, 0, 0.6, 1) infinite; }

            @keyframes aigShimmer { to { background-position: -200% 0; } }
            @keyframes aigShuttle {
                0% { left: -40%; }
                100% { left: 100%; }
            }
            @keyframes aigAiBlip {
                0%, 80%, 100% { opacity: .35; transform: translateY(0); }
                40% { opacity: 1; transform: translateY(-2px); }
            }
            @keyframes aigSlideUp {
                from { opacity: 0; transform: translateY(8px); }
                to { opacity: 1; transform: none; }
            }
        `}</style>
    );
}

// ── Page shell styles (formerly lived in AIGeneratePage) ────────────────────
function PageShellStyles() {
    return (
        <style>{`
            @keyframes spin { to { transform: rotate(360deg); } }
            .aigen-back-link:hover { color: var(--text-primary) !important; }
            .aigen-req-switcher-btn:not(:disabled):hover { opacity: 0.85; }
            .aigen-switcher-row:hover { background: rgba(99,102,241,0.07) !important; }
            .aigen-switcher-create:hover { background: rgba(99,102,241,0.1) !important; }
            .aigen-empty-req-row:hover { background: rgba(99,102,241,0.07) !important; }
            .aigen-empty-req-row:last-child { border-bottom: none !important; }
            .aigen-manage-reqs-link:hover { color: var(--text-primary) !important; }
            .aigen-path-card:hover {
                border-color: rgba(99,102,241,0.3) !important;
                box-shadow: 0 4px 20px rgba(99,102,241,0.08) !important;
            }
            .aigen-import-cta:hover {
                background: rgba(20,184,166,0.15) !important;
                border-color: rgba(20,184,166,0.5) !important;
                transform: translateY(-1px);
                box-shadow: 0 4px 16px rgba(20,184,166,0.15);
            }
            .aigen-create-req-btn:hover {
                filter: brightness(1.1);
                transform: translateY(-1px);
            }
            .aigen-create-submit-btn:not(:disabled):hover { filter: brightness(1.1); }
            .aigen-modal-close:hover {
                color: var(--text-primary) !important;
                background: var(--aig-surface-hover) !important;
            }
        `}</style>
    );
}

// ── RequirementSwitcher ─────────────────────────────────────────────────────
function RequirementSwitcher({ current, allReqs, onSwitch, onCreateNew, hasUnsaved, disabled }) {
    const [open, setOpen] = useState(false);
    const [search, setSearch] = useState('');
    const containerRef = useRef(null);
    const searchRef = useRef(null);

    useEffect(() => {
        if (!open) return;
        const handler = (e) => {
            if (containerRef.current && !containerRef.current.contains(e.target)) {
                setOpen(false);
                setSearch('');
            }
        };
        document.addEventListener('mousedown', handler);
        return () => document.removeEventListener('mousedown', handler);
    }, [open]);

    useEffect(() => {
        if (open) setTimeout(() => searchRef.current?.focus(), 10);
    }, [open]);

    const others = allReqs.filter(r => r.id !== current?.id);
    const filtered = search.trim()
        ? others.filter(r =>
            r.title.toLowerCase().includes(search.toLowerCase()) ||
            r.identifier.toLowerCase().includes(search.toLowerCase())
          )
        : others;

    const handleSelect = (req) => {
        if (hasUnsaved && !window.confirm('Switch requirement? Un-accepted drafts will be discarded.')) return;
        onSwitch(req);
        setOpen(false);
        setSearch('');
    };

    return (
        <div ref={containerRef} style={{ position: 'relative', marginTop: 3 }}>
            <button
                type="button"
                onClick={() => !disabled && setOpen(o => !o)}
                disabled={disabled}
                className="aigen-req-switcher-btn"
                style={{
                    display: 'flex', alignItems: 'center', gap: 6,
                    background: 'none', border: 'none', padding: '2px 4px 2px 0',
                    cursor: disabled ? 'not-allowed' : 'pointer',
                    fontFamily: 'inherit', opacity: disabled ? 0.5 : 1,
                    maxWidth: '100%', minWidth: 0,
                }}
                title={others.length > 0 ? 'Switch requirement' : 'No other requirements'}
            >
                <span style={pageStyles.reqBadge}>{current.identifier}</span>
                <span style={pageStyles.reqTitle}>{current.title}</span>
                {others.length > 0 && (
                    <svg width="11" height="11" viewBox="0 0 24 24" fill="none"
                        stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"
                        style={{
                            color: 'var(--text-secondary)', flexShrink: 0,
                            transition: 'transform 0.15s',
                            transform: open ? 'rotate(180deg)' : 'rotate(0deg)',
                        }}>
                        <polyline points="6 9 12 15 18 9"/>
                    </svg>
                )}
            </button>

            {open && (
                <div style={{
                    position: 'absolute', top: 'calc(100% + 6px)', left: 0, zIndex: 300,
                    minWidth: 320, maxWidth: 480,
                    borderRadius: 9, border: '1px solid var(--border-color)',
                    background: 'var(--bg-secondary)',
                    boxShadow: '0 10px 32px rgba(0,0,0,0.32)',
                    overflow: 'hidden',
                }}>
                    <div style={{ padding: '8px 8px 6px', borderBottom: '1px solid var(--border-color)' }}>
                        <div style={{ position: 'relative' }}>
                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"
                                style={{ position: 'absolute', left: 8, top: '50%', transform: 'translateY(-50%)', color: 'var(--text-secondary)', pointerEvents: 'none' }}>
                                <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
                            </svg>
                            <input
                                ref={searchRef}
                                value={search}
                                onChange={e => setSearch(e.target.value)}
                                placeholder="Search requirements…"
                                onKeyDown={e => e.key === 'Escape' && (search ? setSearch('') : setOpen(false))}
                                style={{
                                    width: '100%', padding: '5px 8px 5px 28px',
                                    borderRadius: 5, border: '1px solid var(--border-color)',
                                    background: 'var(--aig-surface-tint)', color: 'var(--text-primary)',
                                    fontSize: '0.82rem', fontFamily: 'inherit', outline: 'none',
                                    boxSizing: 'border-box',
                                }}
                            />
                        </div>
                    </div>

                    <div style={{ maxHeight: 240, overflowY: 'auto', padding: '4px 0' }}>
                        {filtered.length === 0 ? (
                            <div style={{ padding: '14px 12px', fontSize: '0.83rem', color: 'var(--text-secondary)', textAlign: 'center' }}>
                                {search ? `No matches for "${search}"` : 'No other requirements'}
                            </div>
                        ) : filtered.map(r => (
                            <button
                                key={r.id}
                                type="button"
                                onClick={() => handleSelect(r)}
                                className="aigen-switcher-row"
                                style={{
                                    width: '100%', display: 'flex', alignItems: 'center', gap: 8,
                                    padding: '7px 12px', border: 'none',
                                    background: 'none', cursor: 'pointer', fontFamily: 'inherit',
                                    textAlign: 'left', transition: 'background 0.1s',
                                }}
                            >
                                <span style={{
                                    fontSize: '0.7rem', fontWeight: 700, color: AIC.indigoSoft,
                                    background: 'var(--aig-accent-soft-bg)', border: '1px solid var(--aig-accent-soft-border)',
                                    padding: '1px 6px', borderRadius: 4, flexShrink: 0,
                                    letterSpacing: '0.02em',
                                }}>{r.identifier}</span>
                                <span style={{
                                    fontSize: '0.85rem', color: 'var(--text-primary)',
                                    flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                                }}>{r.title}</span>
                            </button>
                        ))}
                    </div>

                    <div style={{ borderTop: '1px solid var(--border-color)', padding: '4px 0' }}>
                        <button
                            type="button"
                            onClick={() => { setOpen(false); setSearch(''); onCreateNew(); }}
                            className="aigen-switcher-row aigen-switcher-create"
                            style={{
                                width: '100%', display: 'flex', alignItems: 'center', gap: 8,
                                padding: '7px 12px', border: 'none',
                                background: 'none', cursor: 'pointer', fontFamily: 'inherit',
                                textAlign: 'left', transition: 'background 0.1s',
                                color: AIC.indigoSoft,
                            }}
                        >
                            <span style={{
                                width: 18, height: 18, borderRadius: 4, flexShrink: 0,
                                background: 'rgba(99,102,241,0.15)', border: '1px solid rgba(99,102,241,0.25)',
                                display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
                            }}>
                                <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                                    <line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>
                                </svg>
                            </span>
                            <span style={{ fontSize: '0.85rem', fontWeight: 500 }}>New requirement…</span>
                        </button>
                    </div>
                </div>
            )}
        </div>
    );
}

// ── CreateRequirementModal ──────────────────────────────────────────────────
function CreateRequirementModal({ onClose, onCreated }) {
    const [identifier, setIdentifier] = useState('');
    const [title, setTitle] = useState('');
    const [description, setDescription] = useState('');
    const [error, setError] = useState('');
    const [saving, setSaving] = useState(false);
    const firstRef = useRef(null);

    useEffect(() => { firstRef.current?.focus(); }, []);
    useEffect(() => {
        const handler = (e) => { if (e.key === 'Escape') onClose(); };
        document.addEventListener('keydown', handler);
        return () => document.removeEventListener('keydown', handler);
    }, [onClose]);

    const canSubmit = identifier.trim() && title.trim() && !saving;

    const handleSubmit = async (e) => {
        e.preventDefault();
        if (!canSubmit) return;
        setSaving(true);
        setError('');
        try {
            const newReq = await requirementsApi.create({
                identifier: identifier.trim(),
                title: title.trim(),
                description: description.trim(),
            });
            onCreated(newReq);
        } catch (err) {
            setError(err?.response?.data?.error || 'Failed to create requirement');
            setSaving(false);
        }
    };

    return (
        <div
            style={{
                position: 'fixed', inset: 0, zIndex: 1000,
                background: 'var(--aig-modal-backdrop)',
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                padding: 16,
            }}
            onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
        >
            <div style={{
                width: '100%', maxWidth: 440,
                borderRadius: 13, background: 'var(--bg-secondary)',
                border: '1px solid var(--border-color)',
                boxShadow: 'var(--shadow-md)',
                padding: '24px 24px 20px',
            }}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 20 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                        <div style={{
                            width: 30, height: 30, borderRadius: 8,
                            background: 'linear-gradient(135deg, rgba(99,102,241,0.2), rgba(20,184,166,0.15))',
                            border: '1px solid rgba(99,102,241,0.25)',
                            display: 'flex', alignItems: 'center', justifyContent: 'center',
                            color: AIC.indigoSoft,
                        }}>
                            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                                <line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>
                            </svg>
                        </div>
                        <h3 style={{ margin: 0, fontSize: '1rem', fontWeight: 700, color: 'var(--text-primary)' }}>
                            New Requirement
                        </h3>
                    </div>
                    <button
                        onClick={onClose}
                        style={{
                            background: 'none', border: 'none', cursor: 'pointer',
                            color: 'var(--text-secondary)', fontSize: '1rem',
                            padding: '4px 6px', borderRadius: 5, lineHeight: 1,
                            transition: 'color 0.15s',
                        }}
                        className="aigen-modal-close"
                    >✕</button>
                </div>

                <form onSubmit={handleSubmit}>
                    <div style={{ marginBottom: 14 }}>
                        <label style={pageStyles.fieldLabel}>
                            Identifier
                            <span style={{ color: '#f87171', marginLeft: 3 }}>*</span>
                        </label>
                        <input
                            ref={firstRef}
                            value={identifier}
                            onChange={e => setIdentifier(e.target.value)}
                            placeholder="e.g. REQ-001, EC-010, AUTH-05"
                            className="modern-input"
                            style={{ width: '100%', boxSizing: 'border-box' }}
                            disabled={saving}
                        />
                    </div>

                    <div style={{ marginBottom: 14 }}>
                        <label style={pageStyles.fieldLabel}>
                            Title
                            <span style={{ color: '#f87171', marginLeft: 3 }}>*</span>
                        </label>
                        <input
                            value={title}
                            onChange={e => setTitle(e.target.value)}
                            placeholder="Short description of the requirement"
                            className="modern-input"
                            style={{ width: '100%', boxSizing: 'border-box' }}
                            disabled={saving}
                        />
                    </div>

                    <div style={{ marginBottom: 20 }}>
                        <label style={{ ...pageStyles.fieldLabel, display: 'flex', alignItems: 'center', gap: 6 }}>
                            Description
                            <span style={pageStyles.optionalTag}>optional</span>
                        </label>
                        <textarea
                            value={description}
                            onChange={e => setDescription(e.target.value)}
                            placeholder="Detailed description helps the AI generate better test cases…"
                            className="modern-input"
                            style={{
                                width: '100%', minHeight: 78, resize: 'vertical',
                                boxSizing: 'border-box', fontSize: '0.85rem', lineHeight: 1.6,
                            }}
                            disabled={saving}
                        />
                    </div>

                    {error && (
                        <div style={{
                            marginBottom: 14, padding: '8px 12px',
                            borderRadius: 7, background: 'rgba(239,68,68,0.07)',
                            border: '1px solid rgba(239,68,68,0.2)',
                            fontSize: '0.83rem', color: '#f87171',
                        }}>
                            {error}
                        </div>
                    )}

                    <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
                        <button
                            type="button"
                            onClick={onClose}
                            disabled={saving}
                            style={{
                                padding: '7px 16px', borderRadius: 7,
                                border: '1px solid var(--border-color)',
                                background: 'none', cursor: 'pointer',
                                color: 'var(--text-secondary)', fontFamily: 'inherit',
                                fontSize: '0.875rem',
                            }}
                        >
                            Cancel
                        </button>
                        <button
                            type="submit"
                            disabled={!canSubmit}
                            className="aigen-create-submit-btn"
                            style={{
                                padding: '7px 18px', borderRadius: 7, border: 'none',
                                background: 'linear-gradient(135deg, #4f46e5 0%, #6366f1 100%)',
                                color: '#fff',
                                cursor: canSubmit ? 'pointer' : 'not-allowed',
                                fontFamily: 'inherit', fontSize: '0.875rem', fontWeight: 600,
                                opacity: canSubmit ? 1 : 0.45,
                                display: 'flex', alignItems: 'center', gap: 7,
                                transition: 'opacity 0.15s',
                            }}
                        >
                            {saving ? (
                                <>
                                    <span style={{
                                        display: 'inline-block', width: 12, height: 12,
                                        border: '2px solid rgba(255,255,255,0.3)',
                                        borderTopColor: '#fff', borderRadius: '50%',
                                        animation: 'spin 0.7s linear infinite',
                                    }} />
                                    Creating…
                                </>
                            ) : 'Create & Open'}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    );
}

// ── ContextPane requirement picker (inline, shown when no active requirement) ─
function ContextReqPicker({ allReqs, allReqsLoading, onPickReq, onCreateNew }) {
    const [search, setSearch] = useState('');
    const filtered = allReqs.filter(r =>
        r.title.toLowerCase().includes(search.toLowerCase()) ||
        r.identifier.toLowerCase().includes(search.toLowerCase())
    );

    if (allReqsLoading) {
        return (
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '10px 0' }}>
                <span style={{
                    display: 'inline-block', width: 12, height: 12,
                    border: '2px solid rgba(99,102,241,0.2)', borderTopColor: '#6366f1',
                    borderRadius: '50%', animation: 'spin 0.7s linear infinite',
                }} />
                <span style={{ fontSize: 11.5, color: AIC.muted }}>Loading…</span>
            </div>
        );
    }

    if (allReqs.length === 0) {
        return (
            <div style={{ marginTop: 4 }}>
                <p style={{ margin: '0 0 8px', fontSize: 11.5, color: AIC.muted, lineHeight: 1.5 }}>
                    No requirements yet.
                </p>
                <button onClick={onCreateNew} style={{
                    width: '100%', padding: '7px 10px', fontSize: 12, borderRadius: 6,
                    background: 'var(--aig-accent-soft-bg)', border: '1px solid var(--aig-accent-soft-border)',
                    color: AIC.indigoSoft, cursor: 'pointer', fontFamily: 'inherit', fontWeight: 500,
                }}>+ New requirement</button>
            </div>
        );
    }

    return (
        <div style={{ marginTop: 4 }}>
            <div style={{ position: 'relative', marginBottom: 6 }}>
                <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{
                    position: 'absolute', left: 9, top: '50%', transform: 'translateY(-50%)',
                    color: AIC.muted, pointerEvents: 'none',
                }}>
                    <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
                </svg>
                <input
                    autoFocus
                    value={search}
                    onChange={e => setSearch(e.target.value)}
                    placeholder="Search requirements…"
                    style={{
                        width: '100%', padding: '6px 10px 6px 26px', fontSize: 12,
                        background: AIC.bg2, border: `1px solid ${AIC.border}`,
                        color: AIC.text, borderRadius: 6, fontFamily: 'inherit', outline: 'none',
                    }}
                />
            </div>
            <div style={{ maxHeight: 200, overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: 3 }}>
                {filtered.length === 0 ? (
                    <div style={{ fontSize: 11, color: AIC.muted, padding: '8px 4px' }}>
                        No matches for "{search}"
                    </div>
                ) : filtered.map(r => (
                    <button
                        key={r.id}
                        onClick={() => onPickReq(r)}
                        className="aigen-context-req-row"
                        style={{
                            display: 'flex', alignItems: 'center', gap: 8,
                            padding: '6px 8px', borderRadius: 5, textAlign: 'left',
                            background: 'transparent', border: `1px solid transparent`,
                            color: AIC.text, cursor: 'pointer', fontFamily: 'inherit',
                            transition: 'background .15s, border-color .15s',
                        }}
                    >
                        <span style={{
                            fontFamily: MONO, fontSize: 10, fontWeight: 500,
                            color: AIC.indigoSoft, flexShrink: 0,
                            background: 'var(--aig-accent-soft-bg)', border: '1px solid var(--aig-accent-soft-border)',
                            padding: '1px 5px', borderRadius: 3,
                        }}>{r.identifier}</span>
                        <span style={{
                            flex: 1, fontSize: 11.5, color: AIC.dim, lineHeight: 1.35,
                            overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                        }}>{r.title}</span>
                    </button>
                ))}
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8, marginTop: 8 }}>
                <button onClick={onCreateNew} style={{
                    padding: '5px 8px', fontSize: 11, borderRadius: 5,
                    background: 'transparent', border: `1px dashed ${AIC.border}`,
                    color: AIC.dim, cursor: 'pointer', fontFamily: 'inherit',
                }}>+ New</button>
                <Link to="/requirements" style={{
                    alignSelf: 'center', fontSize: 11, color: AIC.muted, textDecoration: 'none',
                }}>Manage →</Link>
            </div>
        </div>
    );
}

// ── Page shell styles (for header/tabs/empty state) ─────────────────────────
const pageStyles = {
    root: { padding: '24px 28px', minHeight: '100%' },
    container: { maxWidth: 860, margin: '0 auto' },
    studioWrap: { margin: '4px -28px 0', '--aig-studio-h': 'calc(100vh - 220px)' },
    studioOnlyWrap: { '--aig-studio-h': 'calc(100vh - 64px)' },

    pageHeader: {
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        gap: 16, marginBottom: 16,
    },
    aiIcon: {
        width: 38, height: 38, borderRadius: 10,
        background: 'linear-gradient(135deg, rgba(99,102,241,0.2), rgba(20,184,166,0.15))',
        border: '1px solid rgba(99,102,241,0.25)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        color: 'var(--aig-indigo-strong)', flexShrink: 0,
    },
    pageTitle: {
        margin: 0, fontSize: '1.15rem', fontWeight: 700,
        letterSpacing: '-0.01em', color: 'var(--text-primary)',
    },
    headerSub: {
        margin: '2px 0 0', fontSize: '0.82rem',
        color: 'var(--text-secondary)', fontWeight: 400,
    },
    reqBadge: {
        fontSize: '0.72rem', fontWeight: 700, color: 'var(--aig-indigo-strong)',
        background: 'var(--aig-accent-soft-bg)', border: '1px solid var(--aig-accent-soft-border)',
        padding: '1px 7px', borderRadius: 4, letterSpacing: '0.02em', flexShrink: 0,
    },
    reqTitle: {
        fontSize: '0.85rem', color: 'var(--text-secondary)',
        overflow: 'hidden', whiteSpace: 'nowrap', textOverflow: 'ellipsis',
    },
    backLink: {
        display: 'flex', alignItems: 'center', gap: 6,
        color: 'var(--text-secondary)', textDecoration: 'none',
        fontSize: '0.85rem', fontWeight: 500, flexShrink: 0,
        transition: 'color 0.15s',
    },
    accentBar: {
        height: 3, borderRadius: 2,
        background: 'linear-gradient(90deg, #6366f1 0%, #14b8a6 100%)',
        marginBottom: 14,
    },

    tabRow: {
        display: 'flex', gap: 4, marginBottom: 16, padding: '3px',
        borderRadius: 8, background: 'var(--bg-secondary, rgba(0,0,0,0.06))',
        width: 'fit-content',
    },
    tabBtn: {
        display: 'flex', alignItems: 'center', gap: 6,
        padding: '7px 16px', borderRadius: 6,
        fontSize: '0.82rem', fontWeight: 500,
        background: 'transparent', color: 'var(--text-secondary)',
        border: 'none', cursor: 'pointer', fontFamily: 'inherit',
        transition: 'all 0.15s',
    },
    tabBtnActive: {
        background: 'var(--bg-primary, #fff)',
        color: 'var(--text-primary)',
        boxShadow: '0 1px 3px rgba(0,0,0,0.08)',
    },
    tabBadge: {
        fontSize: '0.68rem', fontWeight: 600,
        padding: '1px 6px', borderRadius: 8,
        background: 'var(--accent, #6366f1)', color: '#fff', marginLeft: 2,
    },

    emptyGrid: {
        display: 'grid', gridTemplateColumns: '1.15fr 1fr',
        gap: 16, alignItems: 'start',
    },
    emptyPickerCard: {
        borderRadius: 12, background: 'var(--bg-secondary)',
        border: '1px solid var(--border-color)',
        padding: '18px 20px',
        transition: 'border-color 0.2s, box-shadow 0.2s',
    },
    emptyImportLink: {
        background: 'none', border: 'none', padding: 0, cursor: 'pointer',
        color: 'var(--text-secondary)', fontSize: '0.8rem',
        textDecoration: 'none',
    },
    emptyPathCard: {
        borderRadius: 12, background: 'var(--bg-secondary)',
        border: '1px solid var(--border-color)', overflow: 'hidden',
        transition: 'border-color 0.2s, box-shadow 0.2s',
    },
    emptyPathHead: {
        display: 'flex', alignItems: 'center', gap: 12,
        padding: '16px 18px 0',
    },
    emptyPathIconWrap: {
        width: 36, height: 36, borderRadius: 9,
        background: 'linear-gradient(135deg, rgba(99,102,241,0.2), rgba(99,102,241,0.1))',
        border: '1px solid rgba(99,102,241,0.25)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        color: 'var(--aig-indigo-strong)', flexShrink: 0,
    },
    emptyPathTitle: {
        margin: 0, fontSize: '1rem', fontWeight: 700,
        color: 'var(--text-primary)',
    },
    emptyPathDesc: {
        margin: '2px 0 0', fontSize: '0.78rem',
        color: 'var(--text-secondary)', lineHeight: 1.4,
    },
    emptyPathBody: { padding: '14px 18px 18px' },
    emptyPathLabel: {
        display: 'block', marginBottom: 8,
        fontSize: '0.75rem', fontWeight: 600,
        color: 'var(--text-secondary)',
        textTransform: 'uppercase', letterSpacing: '0.05em',
    },
    emptyLoading: {
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        gap: 10, padding: '20px 0',
    },

    emptySearchWrap: { position: 'relative', marginBottom: 8 },
    emptySearchIcon: {
        position: 'absolute', left: 10, top: '50%',
        transform: 'translateY(-50%)',
        color: 'var(--text-secondary)', pointerEvents: 'none',
    },
    emptySearchInput: {
        width: '100%', padding: '8px 32px',
        borderRadius: 7, border: '1px solid var(--border-color)',
        background: 'var(--bg-primary)', color: 'var(--text-primary)',
        fontSize: '0.875rem', fontFamily: 'inherit', outline: 'none',
        boxSizing: 'border-box', transition: 'border-color 0.15s',
    },
    emptySearchClear: {
        position: 'absolute', right: 8, top: '50%', transform: 'translateY(-50%)',
        background: 'none', border: 'none', cursor: 'pointer',
        color: 'var(--text-secondary)', fontSize: '0.75rem', padding: '2px 4px',
        borderRadius: 3,
    },
    emptyReqList: {
        maxHeight: 240, overflowY: 'auto',
        borderRadius: 8, border: '1px solid var(--border-color)',
        background: 'var(--bg-primary)', marginBottom: 10,
    },
    emptyReqRow: {
        width: '100%', display: 'flex', alignItems: 'center', gap: 8,
        padding: '8px 12px', border: 'none',
        borderBottom: '1px solid var(--border-color)',
        background: 'none', cursor: 'pointer', fontFamily: 'inherit',
        textAlign: 'left', transition: 'background 0.1s',
    },
    emptyReqBadge: {
        fontSize: '0.7rem', fontWeight: 700, color: 'var(--aig-indigo-strong)',
        background: 'var(--aig-accent-soft-bg)', border: '1px solid var(--aig-accent-soft-border)',
        padding: '1px 6px', borderRadius: 4, flexShrink: 0, letterSpacing: '0.02em',
    },
    emptyReqTitle: {
        fontSize: '0.875rem', color: 'var(--text-primary)',
        flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
    },
    emptyReqArrow: {
        color: 'var(--text-secondary)', flexShrink: 0, opacity: 0.4,
    },
    emptyReqNone: {
        padding: '14px 12px', fontSize: '0.83rem',
        color: 'var(--text-secondary)', textAlign: 'center',
    },
    emptyFooterRow: {
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        paddingTop: 8,
    },
    emptyCreateBtn: {
        display: 'inline-flex', alignItems: 'center', gap: 6,
        padding: '5px 12px', borderRadius: 6,
        border: '1px solid var(--aig-accent-soft-border)',
        background: 'var(--aig-accent-soft-bg)',
        color: 'var(--aig-indigo-strong)', fontSize: '0.82rem', fontWeight: 600,
        cursor: 'pointer', fontFamily: 'inherit',
        transition: 'all 0.15s',
    },
    emptyCreateBtnPrimary: {
        display: 'inline-flex', alignItems: 'center', gap: 6,
        padding: '8px 18px', borderRadius: 7, border: 'none',
        background: 'linear-gradient(135deg, #4f46e5 0%, #6366f1 100%)',
        color: '#fff', fontSize: '0.875rem', fontWeight: 600,
        cursor: 'pointer', fontFamily: 'inherit',
        boxShadow: '0 4px 14px rgba(99,102,241,0.35)',
        transition: 'all 0.15s',
    },

    importFeatureList: {
        display: 'flex', flexDirection: 'column', gap: 12, marginBottom: 18,
    },
    importFeature: {
        display: 'flex', alignItems: 'flex-start', gap: 10,
    },
    importFeatureIcon: {
        width: 28, height: 28, borderRadius: 7,
        background: 'rgba(20,184,166,0.08)',
        border: '1px solid rgba(20,184,166,0.15)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        color: '#14b8a6', flexShrink: 0, marginTop: 1,
    },
    importFeatureTitle: {
        display: 'block', fontSize: '0.84rem', fontWeight: 600,
        color: 'var(--text-primary)', lineHeight: 1.3,
    },
    importFeatureDesc: {
        display: 'block', fontSize: '0.76rem',
        color: 'var(--text-secondary)', lineHeight: 1.4, marginTop: 1,
    },
    importCtaBtn: {
        width: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center',
        gap: 8, padding: '10px 18px', borderRadius: 8,
        border: '1px solid rgba(20,184,166,0.35)',
        background: 'rgba(20,184,166,0.08)',
        color: '#14b8a6', fontSize: '0.86rem', fontWeight: 600,
        cursor: 'pointer', fontFamily: 'inherit',
        transition: 'all 0.18s',
    },

    fieldLabel: {
        display: 'block', marginBottom: 6,
        fontSize: '0.78rem', color: 'var(--text-secondary)', fontWeight: 500,
    },
    optionalTag: {
        fontSize: '0.7rem', fontWeight: 500,
        color: 'var(--text-secondary)',
        background: 'var(--aig-surface-tint-strong)',
        border: '1px solid var(--border-color)',
        padding: '0 6px', borderRadius: 4,
        textTransform: 'none', letterSpacing: '0.01em', marginLeft: 2,
    },

    importModalBackdrop: {
        position: 'fixed', inset: 0,
        background: 'var(--aig-modal-backdrop)', backdropFilter: 'blur(4px)',
        display: 'flex', alignItems: 'flex-start', justifyContent: 'center',
        padding: '6vh 16px 16px', zIndex: 100, overflowY: 'auto',
    },
    importModalCard: {
        width: '100%', maxWidth: 920,
        background: 'var(--bg-primary)',
        border: '1px solid var(--border-color)', borderRadius: 14,
        boxShadow: 'var(--shadow-md)',
        display: 'flex', flexDirection: 'column', minHeight: 0,
        maxHeight: '88vh',
    },
    importModalHead: {
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        gap: 16, padding: '14px 18px',
        borderBottom: '1px solid var(--border-color)',
    },
    importModalTitle: {
        margin: 0, fontSize: '1rem', fontWeight: 700,
        color: 'var(--text-primary)', letterSpacing: '-0.005em',
    },
    importModalSub: {
        margin: '2px 0 0', fontSize: '0.78rem',
        color: 'var(--text-secondary)', lineHeight: 1.4,
    },
    importModalClose: {
        flexShrink: 0, width: 30, height: 30, borderRadius: 8,
        background: 'transparent', border: '1px solid var(--border-color)',
        color: 'var(--text-secondary)', cursor: 'pointer',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
    },
    importModalBody: { padding: '14px 18px 18px', overflowY: 'auto' },
};
