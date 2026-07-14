/* eslint-disable react-refresh/only-export-components -- context/hook file intentionally co-exports its Provider and hook; splitting would ripple imports across the app with no runtime benefit */
import { createContext, useContext, useState, useCallback, useRef, useEffect, useMemo } from 'react';
import { aiGeneration, aiImport, getFolderTree, requirements as requirementsApi } from '../api';
import { toast } from '../toast';
import { useAuth } from './AuthContext';
import { runToDebug } from '../utils/aiRunDebug';

// ── Constants (re-exported for AIGeneratePage) ──────────────────────
export const DETAIL_LEVELS = [
    { value: 'Simplified', icon: '▹', label: 'Simplified', desc: 'High-level steps only' },
    { value: 'Standard',   icon: '▸', label: 'Standard',   desc: 'Balanced detail'       },
    { value: 'Detailed',   icon: '▶', label: 'Detailed',   desc: 'Granular sub-steps'    },
];

export const COVERAGE_LEVELS = [
    { value: 'essential',     icon: '◇', label: 'Essential',     desc: 'Core happy & sad paths' },
    { value: 'thorough',      icon: '◆', label: 'Thorough',      desc: 'Key paths, negatives, boundaries' },
    { value: 'comprehensive', icon: '◈', label: 'Comprehensive', desc: 'All categories including edge cases' },
];

export function flattenFolderTree(nodes, depth = 0) {
    const result = [];
    for (const node of (nodes || [])) {
        result.push({ id: node.id, name: node.name, depth });
        if (node.sub_folders?.length) {
            result.push(...flattenFolderTree(node.sub_folders, depth + 1));
        }
    }
    return result;
}

// ── Context ─────────────────────────────────────────────────────────
const AIGenerationContext = createContext(null);

export function AIGenerationProvider({ children }) {
    // All app-level fetches below are gated on the authenticated user (same
    // pattern as WebSocketProvider): a logged-out visitor on /login must not
    // fire protected calls that 401.
    const { user } = useAuth();

    // Session identity
    const [activeRequirement, setActiveRequirement] = useState(null);
    const [initialFolderId, setInitialFolderId] = useState('');

    // Provider config
    const [providers, setProviders] = useState([]);
    const [selectedProviderId, setSelectedProviderId] = useState('');

    // Global AI master switch (DB-backed). Default true → optimistic & fail-open.
    const [aiFeaturesEnabled, setAiFeaturesEnabled] = useState(true);

    // Generation params
    const [coverageLevel, setCoverageLevel] = useState('thorough');
    const [detailLevel, setDetailLevel] = useState('Standard');
    const [additionalInstructions, setAdditionalInstructions] = useState('');
    const [runCritic, setRunCritic] = useState(false);

    // Folder selection
    const [folders, setFolders] = useState([]);
    const [selectedFolderId, setSelectedFolderId] = useState('');
    const [groupByCategory, setGroupByCategory] = useState(true);

    // Generation progress
    const [generating, setGenerating] = useState(false);
    const [generationError, setGenerationError] = useState('');
    const [templateWarning, setTemplateWarning] = useState('');
    const [hasGenerated, setHasGenerated] = useState(false);
    const [lastDebug, setLastDebug] = useState(null); // debug info from last successful generation
    // AbortController for the in-flight createGeneration request (cancel from THIS session).
    const generationAbortRef = useRef(null);

    // Draft lifecycle — server `draft.status` is the source of truth.
    const [drafts, setDrafts] = useState([]);
    const [runId, setRunId] = useState(null);
    const [parentRunId, setParentRunId] = useState(null); // lineage for the NEXT generate (set by cloneRun); not the resumable run identity
    const [coverage, setCoverage] = useState(null);
    const [history, setHistory] = useState(null);
    const acceptedIds = useMemo(
        () => new Set(drafts.filter(d => d.status === 'accepted').map(d => d.id)),
        [drafts]
    );
    const [accepting, setAccepting] = useState(false);

    // Callback ref for post-accept refresh (e.g. RequirementsPage.load)
    const onAcceptedRef = useRef(null);

    // ── 014-ai-test-import: Import state (sessionStorage-backed) ────
    const _importCache = useRef(null);
    function _loadImportCache() {
        if (_importCache.current) return _importCache.current;
        try {
            const raw = sessionStorage.getItem('ttgo_import_state');
            _importCache.current = raw ? JSON.parse(raw) : null;
        } catch { _importCache.current = null; }
        return _importCache.current;
    }
    const _cached = _loadImportCache();

    const [importDrafts, setImportDrafts] = useState(() => _cached?.importDrafts || []);
    const [importParsing, setImportParsing] = useState(false);
    const [importFormat, setImportFormat] = useState(() => _cached?.importFormat || '');
    const [importUnparseable, setImportUnparseable] = useState(() => _cached?.importUnparseable || []);
    const [importDuplicateNames, setImportDuplicateNames] = useState(() => _cached?.importDuplicateNames || []);
    const [importTruncated, setImportTruncated] = useState(() => _cached?.importTruncated || false);
    const [importTotalFound, setImportTotalFound] = useState(() => _cached?.importTotalFound || 0);
    const [importError, setImportError] = useState('');
    const [importAccepting, setImportAccepting] = useState(false);
    const [importDebug, setImportDebug] = useState(() => _cached?.importDebug || null);

    // Import review state (persists across navigation AND reload)
    const [importReviewDrafts, setImportReviewDrafts] = useState(() => _cached?.importReviewDrafts || []);
    const [importAcceptedIds, setImportAcceptedIds] = useState(() => new Set(_cached?.importAcceptedIds || []));
    const [importDiscardedIds, setImportDiscardedIds] = useState(() => new Set(_cached?.importDiscardedIds || []));

    // Sets are recreated per update; join to stable strings so the persist
    // effect only fires when membership actually changes.
    const acceptedSig = [...importAcceptedIds].sort().join('|');
    const discardedSig = [...importDiscardedIds].sort().join('|');

    // ── Persist import state to sessionStorage ──────────────────────
    useEffect(() => {
        const hasData = importDrafts.length > 0 || importReviewDrafts.length > 0;
        if (!hasData) {
            sessionStorage.removeItem('ttgo_import_state');
            return;
        }
        try {
            sessionStorage.setItem('ttgo_import_state', JSON.stringify({
                importDrafts,
                importFormat,
                importUnparseable,
                importDuplicateNames,
                importTruncated,
                importTotalFound,
                importDebug,
                importReviewDrafts,
                importAcceptedIds: [...importAcceptedIds],
                importDiscardedIds: [...importDiscardedIds],
            }));
        } catch { /* quota exceeded — non-critical */ }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- sig strings stand in for Set identity; effect body reads the Sets
    }, [importDrafts, importFormat, importUnparseable, importDuplicateNames,
        importTruncated, importTotalFound, importDebug,
        importReviewDrafts, acceptedSig, discardedSig]);

    // ── Persist active requirement id across reloads ─────────────────
    // Rehydrate active requirement on mount (one-shot) — must run BEFORE the
    // persist effect so it doesn't clobber the stored id while state is null.
    const rehydratedRef = useRef(false);
    useEffect(() => {
        if (!user || rehydratedRef.current) return;
        rehydratedRef.current = true;
        let storedId = null;
        try { storedId = sessionStorage.getItem('ttgo_aigen_active_req_id'); } catch { /* sessionStorage unavailable — treat as no stored id */ }
        if (!storedId) return;
        requirementsApi.get(storedId)
            .then(req => {
                if (!req?.id) return;
                setActiveRequirement(req);
                // Refresh recovery: reattach the last generation run for this
                // requirement, if the stored run id still matches it.
                const storedRunId = sessionStorage.getItem('ttgo_aigen_run_id');
                if (storedRunId) {
                    aiGeneration.getGeneration(storedRunId)
                        .then(result => {
                            if (result.run?.requirement_id !== req.id) return; // stale key from another session
                            setRunId(result.run.id);
                            setDrafts(result.drafts || []);
                            setCoverage(result.coverage || null);
                            if (result.run) setLastDebug(runToDebug(result.run));
                            setHasGenerated(true);
                        })
                        .catch(() => sessionStorage.removeItem('ttgo_aigen_run_id'));
                }
            })
            .catch(() => {
                try { sessionStorage.removeItem('ttgo_aigen_active_req_id'); } catch { /* sessionStorage quota/unavailable — non-critical, skip cleanup */ }
            });
    }, [user]);

    useEffect(() => {
        if (activeRequirement?.id) {
            try { sessionStorage.setItem('ttgo_aigen_active_req_id', String(activeRequirement.id)); } catch { /* sessionStorage quota exceeded — non-critical, skip persistence */ }
        }
        // Note: we never clear here — the rehydrate effect would race with this
        // on mount (activeRequirement is null before fetch resolves). clearSession
        // removes the key explicitly.
    }, [activeRequirement?.id]);

    // ── Persist run id across reloads (refresh recovery) ─────────────
    useEffect(() => {
        // Only persist — never remove here. Removing on the null-on-mount pass
        // would race the rehydrate effect and wipe the key before it's read.
        // clearSession/switchRequirement remove it explicitly.
        if (!runId) return;
        try { sessionStorage.setItem('ttgo_aigen_run_id', runId); } catch { /* sessionStorage quota exceeded — non-critical, skip persistence */ }
    }, [runId]);

    // ── Eager-load folders & providers once authenticated ────────────
    const foldersLoadedRef = useRef(false);
    useEffect(() => {
        if (!user) {
            // Re-arm on logout so the next sign-in refetches fresh data.
            foldersLoadedRef.current = false;
            return;
        }
        if (foldersLoadedRef.current) return;
        foldersLoadedRef.current = true;

        getFolderTree()
            .then(tree => {
                const flat = flattenFolderTree(Array.isArray(tree) ? tree : [tree]);
                setFolders(flat);
                if (!selectedFolderId && flat.length > 0) {
                    setSelectedFolderId(flat[0].id);
                }
            })
            .catch(() => {});

        aiGeneration.listProviders()
            .then(data => {
                const enabled = (data || []).filter(p => p.enabled);
                setProviders(enabled);
                if (!selectedProviderId) {
                    const def = enabled.find(p => p.is_default) || enabled[0];
                    if (def) setSelectedProviderId(def.id);
                }
            })
            .catch(() => {});

        aiGeneration.getFeatureSettings()
            .then(cfg => {
                if (cfg && typeof cfg.enabled === 'boolean') setAiFeaturesEnabled(cfg.enabled);
            })
            .catch(() => { /* fail-open: leave AI visible on error */ });
    }, [user]); // eslint-disable-line react-hooks/exhaustive-deps

    // ── Derived values ──────────────────────────────────────────────
    const pendingDrafts = drafts.filter(d => d.status === 'pending');
    const pendingCount = pendingDrafts.length;
    const hasUnsaved = pendingCount > 0;
    const hasSession = activeRequirement !== null;

    // ── Methods ─────────────────────────────────────────────────────

    const setOnAcceptedCallback = useCallback((fn) => {
        onAcceptedRef.current = fn;
    }, []);

    const openSession = useCallback((requirement, folderId) => {
        setActiveRequirement(requirement);
        setInitialFolderId(folderId || '');
        // Reset generation state for new session
        setGenerationError('');
        setTemplateWarning('');
        setHasGenerated(false);
        setLastDebug(null);
        setDrafts([]);
        setRunId(null);
        sessionStorage.removeItem('ttgo_aigen_run_id');
        setCoverage(null);
        setHistory(null);
        setCoverageLevel('thorough');
        setDetailLevel('Standard');
        setAdditionalInstructions('');
        setRunCritic(false);

        // Fetch providers + folders
        aiGeneration.listProviders()
            .then(data => {
                const enabled = (data || []).filter(p => p.enabled);
                setProviders(enabled);
                const def = enabled.find(p => p.is_default) || enabled[0];
                if (def) setSelectedProviderId(def.id);
            })
            .catch(() => {});

        getFolderTree()
            .then(tree => {
                const flat = flattenFolderTree(Array.isArray(tree) ? tree : [tree]);
                setFolders(flat);
                if (folderId) {
                    setSelectedFolderId(folderId);
                } else if (flat.length > 0) {
                    setSelectedFolderId(flat[0].id);
                }
            })
            .catch(() => {});
    }, []);

    const startGeneration = useCallback(async () => {
        if (!activeRequirement) return;
        if (!selectedProviderId) {
            toast.error('Select an LLM provider first');
            return;
        }

        setGenerating(true);
        setGenerationError('');
        setTemplateWarning('');

        try {
            const controller = new AbortController();
            generationAbortRef.current = controller;
            const result = await aiGeneration.createGeneration({
                requirement_id: activeRequirement.id,
                provider_id: selectedProviderId,
                coverage_level: coverageLevel,
                detail_level: detailLevel,
                additional_instructions: additionalInstructions,
                run_critic: runCritic,
                idempotency_key: crypto.randomUUID(),
                parent_run_id: parentRunId || runId || undefined,
            }, { signal: controller.signal });
            const newDrafts = result.drafts || [];
            setRunId(result.run?.id || null);
            setParentRunId(null);
            setCoverage(result.coverage || null);
            setDrafts(prev => {
                const kept = prev.filter(d => acceptedIds.has(d.id));
                return [...kept, ...newDrafts];
            });
            setHasGenerated(true);
            if (result.template_warning) setTemplateWarning(result.template_warning);
            if (result.critic_warning) setTemplateWarning(result.critic_warning);
            if (result.run) setLastDebug(runToDebug(result.run));
        } catch (err) {
            if (err?.code === 'ERR_CANCELED') {
                setGenerationError('Generation cancelled');
            } else {
                setGenerationError(err?.response?.data?.error || err.message || 'Generation failed');
            }
        } finally {
            generationAbortRef.current = null;
            setGenerating(false);
        }
    }, [activeRequirement, selectedProviderId, coverageLevel, detailLevel, additionalInstructions, runCritic, acceptedIds, runId, parentRunId]);

    // Abort the in-flight generation request from THIS session. The server
    // observes r.Context() cancellation and stamps the run cancelled.
    const cancelActive = useCallback(() => {
        generationAbortRef.current?.abort();
    }, []);

    // Cancel a running run from history (another session/process owns it).
    const cancelRun = useCallback(async (id) => {
        try {
            await aiGeneration.cancelGeneration(id);
            toast.success('Cancellation requested');
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Could not cancel this run');
        }
    }, []);

    // Merge one server draft object into local state.
    const mergeDraft = useCallback((serverDraft) => {
        if (!serverDraft) return;
        setDrafts(prev => prev.map(d => (d.id === serverDraft.id ? serverDraft : d)));
    }, []);

    const saveDraftEdit = useCallback(async (draftId, changes) => {
        if (!runId) return null;
        const result = await aiGeneration.updateGenerationDraft(runId, draftId, changes);
        mergeDraft(result.draft);
        if (result.coverage) setCoverage(result.coverage);
        return result.draft;
    }, [runId, mergeDraft]);

    const rejectDraft = useCallback(async (draftId, reason, note) => {
        if (!runId) return;
        try {
            const result = await aiGeneration.rejectGenerationDraft(runId, draftId, { reason, note: note || '' });
            mergeDraft(result.draft);
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Failed to reject draft');
        }
    }, [runId, mergeDraft]);

    const restoreDraft = useCallback(async (draftId) => {
        if (!runId) return;
        try {
            const result = await aiGeneration.restoreGenerationDraft(runId, draftId);
            mergeDraft(result.draft);
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Failed to restore draft');
        }
    }, [runId, mergeDraft]);

    // Regenerate one pending draft into a NEW alternative, inserted right after
    // the original so the pair sits together for compare/choose (Task 7 UI).
    const regenerateDraft = useCallback(async (draftId, opts) => {
        if (!runId) return null;
        const result = await aiGeneration.regenerateGenerationDraft(runId, draftId, opts || {});
        setDrafts(prev => {
            const idx = prev.findIndex(d => d.id === draftId);
            if (idx < 0) return [...prev, result.draft];
            return [...prev.slice(0, idx + 1), result.draft, ...prev.slice(idx + 1)];
        });
        return result;
    }, [runId]);

    // Keep one version of a draft family; the losing versions come back
    // superseded (excluded from pending counts/accept-all).
    const chooseDraft = useCallback(async (draftId) => {
        if (!runId) return;
        try {
            const result = await aiGeneration.chooseGenerationDraft(runId, draftId);
            setDrafts(prev => prev.map(d => {
                if (d.id === result.draft.id) return result.draft;
                if ((result.superseded_ids || []).includes(d.id)) return { ...d, status: 'superseded' };
                return d;
            }));
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Could not choose this version');
        }
    }, [runId]);

    const loadRun = useCallback(async (id) => {
        const result = await aiGeneration.getGeneration(id);
        setRunId(result.run?.id || id);
        setDrafts(result.drafts || []);
        setCoverage(result.coverage || null);
        if (result.run) setLastDebug(runToDebug(result.run));
        setHasGenerated(true);
        return result;
    }, []);

    const refreshRun = useCallback(async () => {
        if (!runId) return null;
        return loadRun(runId);
    }, [runId, loadRun]);

    const loadHistory = useCallback(async () => {
        if (!activeRequirement) return;
        try {
            const result = await aiGeneration.listGenerations(activeRequirement.id);
            setHistory(result.runs || []);
        } catch {
            setHistory([]);
        }
    }, [activeRequirement]);

    // Clone: reuse a prior run's settings; keep lineage via parent_run_id.
    const cloneRun = useCallback((run) => {
        setCoverageLevel(run.coverage_level || 'thorough');
        setDetailLevel(run.detail_level || 'Standard');
        setAdditionalInstructions(run.additional_instructions || '');
        if (run.provider_id) setSelectedProviderId(run.provider_id);
        // Lineage for the NEXT generate — do NOT adopt the old run as the current
        // (resumable/persisted) run, or a reload would resurrect its drafts.
        setParentRunId(run.id);
        setRunId(null);
        setDrafts([]);
        setCoverage(null);
        setHasGenerated(false);
        try { sessionStorage.removeItem('ttgo_aigen_run_id'); } catch { /* storage unavailable */ }
        toast.success('Run settings cloned — press Generate');
    }, []);

    const acceptDraft = useCallback(async (draft) => {
        if (!activeRequirement || !runId) return;
        if (!selectedFolderId) {
            toast.error('Select a folder first');
            return;
        }
        setAccepting(true);
        try {
            const result = await aiGeneration.acceptGeneration(runId, {
                folder_id: selectedFolderId,
                draft_ids: [draft.id],
                group_by_category: groupByCategory,
            });
            toast.success(`"${draft.name}" accepted`);
            onAcceptedRef.current?.();
            try { await refreshRun(); } catch { /* accept already succeeded server-side; refresh is best-effort */ }
            return result;
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Failed to accept test case');
        } finally {
            setAccepting(false);
        }
    }, [activeRequirement, runId, selectedFolderId, groupByCategory, refreshRun]);

    const acceptAllPending = useCallback(async () => {
        if (!activeRequirement || !runId) return;
        const pending = drafts.filter(d => d.status === 'pending');
        if (pending.length === 0 || !selectedFolderId) {
            toast.error('Select a folder first');
            return;
        }
        setAccepting(true);
        try {
            const result = await aiGeneration.acceptGeneration(runId, {
                folder_id: selectedFolderId,
                draft_ids: pending.map(d => d.id),
                group_by_category: groupByCategory,
            });
            toast.success(`${pending.length} test case${pending.length !== 1 ? 's' : ''} accepted`);
            onAcceptedRef.current?.();
            try { await refreshRun(); } catch { /* accept already succeeded server-side; refresh is best-effort */ }
            return result;
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Failed to accept test cases');
        } finally {
            setAccepting(false);
        }
    }, [activeRequirement, runId, drafts, selectedFolderId, groupByCategory, refreshRun]);

    const acceptDrafts = useCallback(async (draftsToAccept) => {
        if (!activeRequirement || !runId || draftsToAccept.length === 0) return;
        if (!selectedFolderId) {
            toast.error('Select a folder first');
            return;
        }
        setAccepting(true);
        try {
            const result = await aiGeneration.acceptGeneration(runId, {
                folder_id: selectedFolderId,
                draft_ids: draftsToAccept.map(d => d.id),
                group_by_category: groupByCategory,
            });
            toast.success(`${draftsToAccept.length} test case${draftsToAccept.length !== 1 ? 's' : ''} accepted`);
            onAcceptedRef.current?.();
            try { await refreshRun(); } catch { /* accept already succeeded server-side; refresh is best-effort */ }
            return result;
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Failed to accept test cases');
        } finally {
            setAccepting(false);
        }
    }, [activeRequirement, runId, selectedFolderId, groupByCategory, refreshRun]);

    const editDraft = useCallback((id, changes) => {
        setDrafts(prev => prev.map(d => d.id === id ? { ...d, ...changes } : d));
    }, []);

    // Switch to a different requirement without resetting provider/folder/generation params
    const switchRequirement = useCallback((req) => {
        setActiveRequirement(req);
        setGenerationError('');
        setTemplateWarning('');
        setHasGenerated(false);
        setLastDebug(null);
        setDrafts([]);
        setRunId(null);
        setParentRunId(null);
        setCoverage(null);
        setHistory(null);
        sessionStorage.removeItem('ttgo_aigen_run_id');
    }, []);

    // ── 014-ai-test-import: Import methods ────────────────────────
    const parseImport = useCallback(async (content, formatHint, folderId) => {
        setImportParsing(true);
        setImportError('');
        setImportDebug(null);
        setImportDrafts([]);
        setImportUnparseable([]);
        setImportDuplicateNames([]);
        setImportTruncated(false);
        setImportTotalFound(0);
        try {
            const result = await aiImport.parse({
                content,
                format_hint: formatHint || '',
                folder_id: folderId || '',
            });
            const testCases = result.test_cases || [];
            setImportDrafts(testCases);
            setImportFormat(result.detected_format || '');
            setImportUnparseable(result.unparseable || []);
            setImportDuplicateNames(result.duplicate_names || []);
            setImportTruncated(result.truncated || false);
            setImportTotalFound(result.total_found || 0);
            setImportDebug(result.debug || null);
            // Reset review state for new parse
            setImportReviewDrafts([...testCases]);
            setImportAcceptedIds(new Set());
            setImportDiscardedIds(new Set());
            return result;
        } catch (err) {
            const msg = err?.response?.data?.error || err.message || 'Failed to parse content';
            setImportError(msg);
            throw err;
        } finally {
            setImportParsing(false);
        }
    }, []);

    const acceptImport = useCallback(async (folderId, requirementId, tests) => {
        setImportAccepting(true);
        try {
            const result = await aiImport.accept({
                folder_id: folderId,
                requirement_id: requirementId || '',
                tests,
            });
            toast.success(`${result.count} test case${result.count !== 1 ? 's' : ''} imported`);
            onAcceptedRef.current?.();
            return result;
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Failed to import test cases');
            throw err;
        } finally {
            setImportAccepting(false);
        }
    }, []);

    const clearImport = useCallback(() => {
        setImportDrafts([]);
        setImportParsing(false);
        setImportFormat('');
        setImportUnparseable([]);
        setImportDuplicateNames([]);
        setImportTruncated(false);
        setImportTotalFound(0);
        setImportError('');
        setImportAccepting(false);
        setImportDebug(null);
        setImportReviewDrafts([]);
        setImportAcceptedIds(new Set());
        setImportDiscardedIds(new Set());
        sessionStorage.removeItem('ttgo_import_state');
    }, []);

    // ── Import review methods (persist across navigation) ────────────
    const importAcceptDraft = useCallback((draft) => {
        setImportAcceptedIds(prev => new Set([...prev, draft.temp_id]));
    }, []);

    const importDiscardDraft = useCallback((tempId) => {
        setImportDiscardedIds(prev => new Set([...prev, tempId]));
        setImportAcceptedIds(prev => { const next = new Set(prev); next.delete(tempId); return next; });
    }, []);

    const importEditDraft = useCallback((tempId, changes) => {
        setImportReviewDrafts(prev => prev.map(d =>
            d.temp_id === tempId ? { ...d, ...changes } : d
        ));
    }, []);

    const clearSession = useCallback(() => {
        try { sessionStorage.removeItem('ttgo_aigen_active_req_id'); } catch { /* sessionStorage unavailable — non-critical, state is still cleared below */ }
        setActiveRequirement(null);
        setInitialFolderId('');
        // Don't clear providers/folders — they're eagerly loaded and shared
        setCoverageLevel('thorough');
        setDetailLevel('Standard');
        setAdditionalInstructions('');
        setRunCritic(false);
        setGenerating(false);
        setGenerationError('');
        setTemplateWarning('');
        setHasGenerated(false);
        setLastDebug(null);
        setDrafts([]);
        setRunId(null);
        setParentRunId(null);
        setCoverage(null);
        setHistory(null);
        setAccepting(false);
        sessionStorage.removeItem('ttgo_aigen_run_id');
    }, []);

    const value = {
        // Session identity
        activeRequirement,
        initialFolderId,
        // Provider
        providers,
        selectedProviderId,
        setSelectedProviderId,
        // Global AI master switch
        aiFeaturesEnabled,
        setAiFeaturesEnabled,
        // Generation params
        coverageLevel,
        setCoverageLevel,
        detailLevel,
        setDetailLevel,
        additionalInstructions,
        setAdditionalInstructions,
        runCritic,
        setRunCritic,
        // Folders
        folders,
        selectedFolderId,
        setSelectedFolderId,
        groupByCategory,
        setGroupByCategory,
        // Progress
        generating,
        generationError,
        templateWarning,
        hasGenerated,
        lastDebug,
        // Drafts
        drafts,
        runId,
        acceptedIds,
        coverage,
        accepting,
        // Run history
        history,
        // Derived
        pendingDrafts,
        pendingCount,
        hasUnsaved,
        hasSession,
        // Methods
        openSession,
        switchRequirement,
        startGeneration,
        cancelActive,
        cancelRun,
        acceptDraft,
        acceptDrafts,
        acceptAllPending,
        saveDraftEdit,
        rejectDraft,
        restoreDraft,
        regenerateDraft,
        chooseDraft,
        refreshRun,
        loadRun,
        loadHistory,
        cloneRun,
        editDraft,
        clearSession,
        setOnAcceptedCallback,
        // 014-ai-test-import
        importDrafts, setImportDrafts,
        importParsing,
        importFormat,
        importUnparseable,
        importDuplicateNames,
        importTruncated,
        importTotalFound,
        importError,
        importAccepting,
        importDebug,
        parseImport,
        acceptImport,
        clearImport,
        // Import review state (persists across navigation)
        importReviewDrafts, setImportReviewDrafts,
        importAcceptedIds, setImportAcceptedIds,
        importDiscardedIds, setImportDiscardedIds,
        importAcceptDraft,
        importDiscardDraft,
        importEditDraft,
    };

    return (
        <AIGenerationContext.Provider value={value}>
            {children}
        </AIGenerationContext.Provider>
    );
}

/** Convenience hook — must be used inside <AIGenerationProvider>. */
export function useAIGeneration() {
    const ctx = useContext(AIGenerationContext);
    if (!ctx) throw new Error('useAIGeneration must be used within AIGenerationProvider');
    return ctx;
}
