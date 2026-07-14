import React, { useState, useMemo, useEffect, useCallback } from 'react';
import { Link } from 'react-router-dom';
import { useAIGeneration } from '../contexts/AIGenerationContext';
import AIGenReviewPanel from '../components/AIGenReviewPanel';
import AIImportPanel from '../components/AIImportPanel';
import AIImportReview from '../components/AIImportReview';
import { requirements as requirementsApi } from '../api';
import { filterDrafts, isDraftClean, draftFlags } from '../utils/draftReview';
import useStudioShortcuts from '../hooks/useStudioShortcuts';
import {
    AIC, Icon,
    LEFT_MIN, LEFT_MAX, LEFT_DEFAULT, RIGHT_MIN, RIGHT_MAX, RIGHT_DEFAULT,
    clamp, readStoredWidth, pageStyles,
} from './aiStudio/constants';
import { AIBtn } from './aiStudio/primitives';
import { StudioContextPane, CreateRequirementModal } from './aiStudio/context';
import {
    StudioHeader, StudioComposer, StudioDraftsList, StudioDraftDetail,
} from './aiStudio/drafts';
import { RejectPopover } from './aiStudio/rejectPopover';
import { CommitSummaryModal } from './aiStudio/commitSummary';
import { LlmFeedbackPanel } from './aiStudio/LlmFeedbackPanel';
import { CoverageMatrixPanel } from './aiStudio/coverageMatrix';
import { ImportModal, StudioBanner, StudioStyles, PageShellStyles } from './aiStudio/shell';

// ── Main container ──────────────────────────────────────────────────────────
export default function AIGenerateStudio() {
    const ai = useAIGeneration();
    const [leftCollapsed, setLeftCollapsed] = useState(false);
    const [rightCollapsed, setRightCollapsed] = useState(false);
    const [filter, setFilter] = useState('pending');
    const [groupBy, setGroupBy] = useState('category');
    const [selectedDraftId, setSelectedDraftId] = useState(null);
    // Reject popover target: a draft id, the '__all__' sentinel (reject all pending), or null (closed).
    const [rejectTarget, setRejectTarget] = useState(null);
    // Accept-all-clean commit flow: {clean, excludedInvalid, excludedDuplicates, overriddenWarnings} | null
    const [acceptPlan, setAcceptPlan] = useState(null);
    const [acceptResult, setAcceptResult] = useState(null);

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

    // Server `draft.status` is the source of truth (set by accept/reject calls).
    const statusOf = useCallback(
        (d) => (d.status === 'accepted' || d.status === 'rejected') ? d.status : 'pending',
        []
    );

    const statuses = useMemo(() => {
        const o = {};
        drafts.forEach(d => { o[d.id] = statusOf(d); });
        return o;
    }, [drafts, statusOf]);

    const counts = useMemo(() => {
        const c = { pending: 0, accepted: 0, rejected: 0 };
        Object.values(statuses).forEach(s => { c[s] = (c[s] || 0) + 1; });
        return c;
    }, [statuses]);

    const filtered = useMemo(() => filterDrafts(drafts, filter), [drafts, filter]);

    useEffect(() => {
        if (selectedDraftId && drafts.find(d => d.id === selectedDraftId)) return;
        const pending = drafts.find(d => statuses[d.id] === 'pending');
        // eslint-disable-next-line react-hooks/set-state-in-effect -- repairs the selection when it becomes stale/unset as drafts change; selectedDraftId remains independently user-selectable afterward via onSelect
        setSelectedDraftId(pending?.id || drafts[0]?.id || null);
    }, [drafts, selectedDraftId, statuses]);

    const selectedDraft = drafts.find(d => d.id === selectedDraftId) || null;

    const handleGenerate = () => {
        if (ai.hasUnsaved && !window.confirm('This will replace un-accepted drafts. Continue?')) return;
        ai.startGeneration();
    };
    const handleAccept = (d) => ai.acceptDraft(d);
    const handleReject = (id) => setRejectTarget(id);
    const handleDiscardAll = () => setRejectTarget('__all__');
    const handleAcceptGroup = (group) => ai.acceptDrafts(group);

    // Single popover flow for both "reject one" and "reject all pending": the
    // sentinel target loops the chosen reason/note over every pending draft.
    const handleRejectSubmit = (reason, note) => {
        if (rejectTarget === '__all__') {
            ai.pendingDrafts.forEach(d => ai.rejectDraft(d.id, reason, note));
        } else if (rejectTarget) {
            ai.rejectDraft(rejectTarget, reason, note);
        }
    };

    // Opens the confirm phase: clean pending drafts vs. what gets excluded/overridden.
    const handleAcceptAllClean = () => {
        const pending = drafts.filter(d => statusOf(d) === 'pending');
        const clean = pending.filter(isDraftClean);
        setAcceptResult(null);
        setAcceptPlan({
            clean,
            excludedInvalid: pending.filter(d => draftFlags(d).invalid).length,
            excludedDuplicates: pending.filter(d => !draftFlags(d).invalid && draftFlags(d).highConfidenceDuplicate).length,
            overriddenWarnings: clean.filter(d => draftFlags(d).warnings || draftFlags(d).possibleDuplicate).length,
        });
    };
    const confirmAcceptAllClean = async () => {
        const res = await ai.acceptDrafts(acceptPlan.clean);
        if (res) setAcceptResult(res);
        else setAcceptPlan(null); // toast already shown by context on failure
    };

    const selectByOffset = useCallback((dir) => {
        if (!filtered.length) return;
        const idx = filtered.findIndex(d => d.id === selectedDraftId);
        const next = filtered[Math.min(filtered.length - 1, Math.max(0, idx + dir))] || filtered[0];
        setSelectedDraftId(next.id);
    }, [filtered, selectedDraftId]);

    useStudioShortcuts({
        enabled: stage === 'review' && !rejectTarget && !acceptPlan,
        onNext: () => selectByOffset(1),
        onPrev: () => selectByOffset(-1),
        onAccept: () => { const d = filtered.find(x => x.id === selectedDraftId); if (d && statusOf(d) === 'pending') handleAccept(d); },
        onReject: () => { const d = filtered.find(x => x.id === selectedDraftId); if (d && statusOf(d) === 'pending') handleReject(d.id); },
    });

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
                        onAcceptAll={handleAcceptAllClean}
                        onDiscardAll={handleDiscardAll}
                        onGenerate={handleGenerate}
                        onImport={() => setImportOpen(true)}
                        disabled={disabled}
                    />
                    <StudioComposer ai={ai} stage={stage} disabled={disabled} />
                    {ai.lastDebug && <LlmFeedbackPanel debug={ai.lastDebug} />}
                    <CoverageMatrixPanel
                        coverage={ai.coverage}
                        drafts={drafts}
                        onSelectDraft={setSelectedDraftId}
                        onFilterUncovered={() => setFilter('uncovered')}
                    />
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
                        ai={ai}
                        draft={selectedDraft}
                        status={selectedDraft ? statuses[selectedDraft.id] : null}
                        onAccept={() => selectedDraft && handleAccept(selectedDraft)}
                        onReject={() => selectedDraft && handleReject(selectedDraft.id)}
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

            {rejectTarget && (
                <RejectPopover
                    key={rejectTarget}
                    open
                    onClose={() => setRejectTarget(null)}
                    onSubmit={handleRejectSubmit}
                />
            )}

            <CommitSummaryModal
                plan={acceptPlan}
                result={acceptResult}
                onConfirm={confirmAcceptAllClean}
                onClose={() => { setAcceptPlan(null); setAcceptResult(null); }}
            />
        </>
    );
}

