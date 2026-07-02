import { createContext, useContext, useState, useCallback, useRef, useEffect } from 'react';
import { aiGeneration, aiImport, getFolderTree, requirements as requirementsApi } from '../api';
import { toast } from '../toast';

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

    // Draft lifecycle
    const [drafts, setDrafts] = useState([]);
    const [acceptedIds, setAcceptedIds] = useState(new Set());
    const [discardedIds, setDiscardedIds] = useState(new Set());
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
    }, [importDrafts, importFormat, importUnparseable, importDuplicateNames,
        importTruncated, importTotalFound, importDebug,
        importReviewDrafts, importAcceptedIds, importDiscardedIds]);

    // ── Persist active requirement id across reloads ─────────────────
    // Rehydrate active requirement on mount (one-shot) — must run BEFORE the
    // persist effect so it doesn't clobber the stored id while state is null.
    const rehydratedRef = useRef(false);
    useEffect(() => {
        if (rehydratedRef.current) return;
        rehydratedRef.current = true;
        let storedId = null;
        try { storedId = sessionStorage.getItem('ttgo_aigen_active_req_id'); } catch { /* sessionStorage unavailable — treat as no stored id */ }
        if (!storedId) return;
        requirementsApi.get(storedId)
            .then(req => { if (req?.id) setActiveRequirement(req); })
            .catch(() => {
                try { sessionStorage.removeItem('ttgo_aigen_active_req_id'); } catch { /* sessionStorage quota/unavailable — non-critical, skip cleanup */ }
            });
    }, []);

    useEffect(() => {
        if (activeRequirement?.id) {
            try { sessionStorage.setItem('ttgo_aigen_active_req_id', String(activeRequirement.id)); } catch { /* sessionStorage quota exceeded — non-critical, skip persistence */ }
        }
        // Note: we never clear here — the rehydrate effect would race with this
        // on mount (activeRequirement is null before fetch resolves). clearSession
        // removes the key explicitly.
    }, [activeRequirement?.id]);

    // ── Eager-load folders & providers on mount ──────────────────────
    const foldersLoadedRef = useRef(false);
    useEffect(() => {
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
    }, []); // eslint-disable-line react-hooks/exhaustive-deps

    // ── Derived values ──────────────────────────────────────────────
    const pendingDrafts = drafts.filter(
        d => !discardedIds.has(d.temp_id) && !acceptedIds.has(d.temp_id)
    );
    const pendingCount = pendingDrafts.length;
    const hasUnsaved = drafts.length > 0 && pendingCount > 0;
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
        setAcceptedIds(new Set());
        setDiscardedIds(new Set());
        setCoverageLevel('thorough');
        setDetailLevel('Standard');
        setAdditionalInstructions('');

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
            const result = await aiGeneration.generateTests(activeRequirement.id, {
                provider_id: selectedProviderId,
                coverage_level: coverageLevel,
                detail_level: detailLevel,
                additional_instructions: additionalInstructions,
            });
            const newDrafts = result.drafts || [];
            setDrafts(prev => {
                const kept = prev.filter(d => acceptedIds.has(d.temp_id));
                return [...kept, ...newDrafts];
            });
            setDiscardedIds(new Set());
            setHasGenerated(true);
            if (result.template_warning) setTemplateWarning(result.template_warning);
            if (result.debug) setLastDebug(result.debug);
        } catch (err) {
            setGenerationError(err?.response?.data?.error || err.message || 'Generation failed');
        } finally {
            setGenerating(false);
        }
    }, [activeRequirement, selectedProviderId, coverageLevel, detailLevel, additionalInstructions, acceptedIds]);

    const acceptDraft = useCallback(async (draft) => {
        if (!activeRequirement) return;
        if (!selectedFolderId) {
            toast.error('Select a folder first');
            return;
        }
        setAccepting(true);
        try {
            await aiGeneration.acceptGeneratedTests(activeRequirement.id, {
                folder_id: selectedFolderId,
                tests: [draft],
                group_by_category: groupByCategory,
            });
            setAcceptedIds(prev => new Set([...prev, draft.temp_id]));
            toast.success(`"${draft.name}" accepted`);
            onAcceptedRef.current?.();
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Failed to accept test case');
        } finally {
            setAccepting(false);
        }
    }, [activeRequirement, selectedFolderId, groupByCategory]);

    const acceptAllPending = useCallback(async () => {
        if (!activeRequirement) return;
        const pending = drafts.filter(
            d => !discardedIds.has(d.temp_id) && !acceptedIds.has(d.temp_id)
        );
        if (pending.length === 0 || !selectedFolderId) {
            toast.error('Select a folder first');
            return;
        }
        setAccepting(true);
        try {
            await aiGeneration.acceptGeneratedTests(activeRequirement.id, {
                folder_id: selectedFolderId,
                tests: pending,
                group_by_category: groupByCategory,
            });
            setAcceptedIds(new Set([...acceptedIds, ...pending.map(d => d.temp_id)]));
            toast.success(`${pending.length} test case${pending.length !== 1 ? 's' : ''} accepted`);
            onAcceptedRef.current?.();
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Failed to accept test cases');
        } finally {
            setAccepting(false);
        }
    }, [activeRequirement, drafts, discardedIds, acceptedIds, selectedFolderId, groupByCategory]);

    const discardDraft = useCallback((tempId) => {
        setDiscardedIds(prev => new Set([...prev, tempId]));
    }, []);

    const discardAllPending = useCallback(() => {
        const pending = drafts.filter(d => !acceptedIds.has(d.temp_id)).map(d => d.temp_id);
        setDiscardedIds(prev => new Set([...prev, ...pending]));
    }, [drafts, acceptedIds]);

    const acceptDrafts = useCallback(async (draftsToAccept) => {
        if (!activeRequirement || !selectedFolderId || draftsToAccept.length === 0) return;
        setAccepting(true);
        try {
            await aiGeneration.acceptGeneratedTests(activeRequirement.id, {
                folder_id: selectedFolderId,
                tests: draftsToAccept,
                group_by_category: groupByCategory,
            });
            setAcceptedIds(prev => new Set([...prev, ...draftsToAccept.map(d => d.temp_id)]));
            toast.success(`${draftsToAccept.length} test case${draftsToAccept.length !== 1 ? 's' : ''} accepted`);
            onAcceptedRef.current?.();
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Failed to accept test cases');
        } finally {
            setAccepting(false);
        }
    }, [activeRequirement, selectedFolderId, groupByCategory]);

    const discardDrafts = useCallback((tempIds) => {
        setDiscardedIds(prev => new Set([...prev, ...tempIds]));
    }, []);

    const editDraft = useCallback((tempId, changes) => {
        setDrafts(prev => prev.map(d => d.temp_id === tempId ? { ...d, ...changes } : d));
    }, []);

    // Switch to a different requirement without resetting provider/folder/generation params
    const switchRequirement = useCallback((req) => {
        setActiveRequirement(req);
        setGenerationError('');
        setTemplateWarning('');
        setHasGenerated(false);
        setLastDebug(null);
        setDrafts([]);
        setAcceptedIds(new Set());
        setDiscardedIds(new Set());
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
        setGenerating(false);
        setGenerationError('');
        setTemplateWarning('');
        setHasGenerated(false);
        setLastDebug(null);
        setDrafts([]);
        setAcceptedIds(new Set());
        setDiscardedIds(new Set());
        setAccepting(false);
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
        acceptedIds,
        discardedIds,
        accepting,
        // Derived
        pendingDrafts,
        pendingCount,
        hasUnsaved,
        hasSession,
        // Methods
        openSession,
        switchRequirement,
        startGeneration,
        acceptDraft,
        acceptDrafts,
        acceptAllPending,
        discardDraft,
        discardDrafts,
        discardAllPending,
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
