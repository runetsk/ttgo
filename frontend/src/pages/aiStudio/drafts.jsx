import { useState } from 'react';
import SafeHTML from '../../components/shared/SafeHTML';
import { AIC, MONO, Icon, categoryTone, groupDrafts } from './constants';
import {
    SectionLabel, Pill, StatusPill, ReqChip, Segmented, AIBtn, FilterTab,
    GeneratingDots, DraftSkeleton, Stepper, MiniStat,
} from './primitives';
import PromptPreviewPanel from './PromptPreviewPanel';

// ── Middle top: Header ───────────────────────────────────────────────────────
export function StudioHeader({ ai, counts, totalDrafts, stage, onAcceptAll, onDiscardAll, onGenerate, onImport, disabled }) {
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
export function StudioComposer({ ai, stage, disabled }) {
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
                placeholder="Add additional instructions (optional) — they are appended to the prompt below…"
                style={{
                    width: '100%', background: AIC.bg2, border: `1px solid ${AIC.border}`,
                    color: AIC.text, padding: '9px 12px', fontSize: 13.5, lineHeight: 1.55,
                    borderRadius: 7, fontFamily: 'inherit', resize: 'vertical', minHeight: 90,
                    outline: 'none',
                }}
            />

            <PromptPreviewPanel ai={ai} />

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

// ── Middle: Drafts list ─────────────────────────────────────────────────────
export function DraftRow({ draft, status, selected, onSelect, onAccept, onReject, disabled }) {
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

export function StudioDraftsList({
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
export function StudioDraftDetail({
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
