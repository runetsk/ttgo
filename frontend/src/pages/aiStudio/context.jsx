import { useState, useMemo, useEffect, useRef } from 'react';
import { Link } from 'react-router-dom';
import { DETAIL_LEVELS, COVERAGE_LEVELS } from '../../contexts/AIGenerationContext';
import FolderTreeSelect from '../../components/FolderTreeSelect';
import SafeHTML from '../../components/shared/SafeHTML';
import { requirements as requirementsApi } from '../../api';
import { AIC, MONO, Icon, pageStyles } from './constants';
import { HistorySection } from './history';

// ── Linked-requirement preview card ─────────────────────────────────────────
export function LinkedReqCard({ req, disabled, onUnlink }) {
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
export function StudioContextPane({ ai, onCollapse, allReqs, allReqsLoading, onPickReq, onCreateNew }) {
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

            <HistorySection ai={ai} />
        </aside>
    );
}

// ── RequirementSwitcher ─────────────────────────────────────────────────────
export function RequirementSwitcher({ current, allReqs, onSwitch, onCreateNew, hasUnsaved, disabled }) {
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
export function CreateRequirementModal({ onClose, onCreated }) {
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
export function ContextReqPicker({ allReqs, allReqsLoading, onPickReq, onCreateNew }) {
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
