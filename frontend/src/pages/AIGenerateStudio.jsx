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
    AIC, MONO, Icon,
    LEFT_MIN, LEFT_MAX, LEFT_DEFAULT, RIGHT_MIN, RIGHT_MAX, RIGHT_DEFAULT,
    clamp, readStoredWidth, pageStyles,
} from './aiStudio/constants';
import { AIBtn } from './aiStudio/primitives';
import {
    LinkedReqCard, StudioContextPane, RequirementSwitcher,
    CreateRequirementModal, ContextReqPicker,
} from './aiStudio/context';
import {
    StudioHeader, StudioComposer, StudioDraftsList, StudioDraftDetail,
} from './aiStudio/drafts';
import { LlmFeedbackPanel } from './aiStudio/LlmFeedbackPanel';

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


