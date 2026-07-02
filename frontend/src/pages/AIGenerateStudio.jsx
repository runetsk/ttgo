import React, { useState, useMemo, useEffect, useCallback } from 'react';
import { Link } from 'react-router-dom';
import { useAIGeneration, DETAIL_LEVELS, COVERAGE_LEVELS } from '../contexts/AIGenerationContext';
import FolderTreeSelect from '../components/FolderTreeSelect';
import AIGenReviewPanel from '../components/AIGenReviewPanel';
import AIImportPanel from '../components/AIImportPanel';
import AIImportReview from '../components/AIImportReview';
import SafeHTML from '../components/shared/SafeHTML';
import { requirements as requirementsApi } from '../api';
import {
    AIC, MONO, Icon, categoryTone, groupDrafts, stripHtmlForDisplay,
    LEFT_MIN, LEFT_MAX, LEFT_DEFAULT, RIGHT_MIN, RIGHT_MAX, RIGHT_DEFAULT,
    clamp, readStoredWidth, pageStyles,
} from './aiStudio/constants';
import {
    SectionLabel, Pill, StatusPill, ReqChip, Segmented, AIBtn, FilterTab,
    GeneratingDots, DraftSkeleton, Stepper, MiniStat,
} from './aiStudio/primitives';
import {
    LinkedReqCard, StudioContextPane, RequirementSwitcher,
    CreateRequirementModal, ContextReqPicker,
} from './aiStudio/context';

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
        } catch { /* sessionStorage unavailable — fall through to default closed state */ }
        return false;
    });
    const [allReqs, setAllReqs] = useState([]);
    const [allReqsLoading, setAllReqsLoading] = useState(true);
    const [createModalOpen, setCreateModalOpen] = useState(false);

    useEffect(() => {
        // eslint-disable-next-line react-hooks/set-state-in-effect -- async load result: fetches the requirements catalog on mount
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
        try { localStorage.setItem('aig-studio-left-w', String(leftWidth)); } catch { /* localStorage quota exceeded — non-critical, skip persistence */ }
    }, [leftWidth]);
    useEffect(() => {
        try { localStorage.setItem('aig-studio-right-w', String(rightWidth)); } catch { /* localStorage quota exceeded — non-critical, skip persistence */ }
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

    const drafts = useMemo(() => ai.drafts || [], [ai.drafts]);
    const stage = ai.generating
        ? 'generating'
        : drafts.length > 0 ? 'review' : 'compose';
    const disabled = ai.generating || ai.accepting;

    const statusOf = useCallback((d) => ai.acceptedIds.has(d.temp_id)
        ? 'accepted'
        : ai.discardedIds.has(d.temp_id)
            ? 'rejected'
            : 'pending', [ai.acceptedIds, ai.discardedIds]);

    const statuses = useMemo(() => {
        const o = {};
        drafts.forEach(d => { o[d.temp_id] = statusOf(d); });
        return o;
    }, [drafts, statusOf]);

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
        // eslint-disable-next-line react-hooks/set-state-in-effect -- repairs the selection when it becomes stale/unset as drafts change; selectedDraftId remains independently user-selectable afterward via onSelect
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


