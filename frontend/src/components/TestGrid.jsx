import React, { useEffect, useState, useCallback, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { getTests, createTest, getCategories, assignCategory, deleteTest, deleteTests, updateFolder, qtest as qtestApi, exportTests } from '../api';
import { toast } from '../toast';
import Modal from './Modal';
import ColumnPicker from './ColumnPicker';
import QTestImportModal from './QTestImportModal';
import { useColumnPreference } from '../hooks/useColumnPreference';
import { useColumnWidths } from '../hooks/useColumnWidths';
import { stripHtml } from '../utils/htmlUtils';

// T003: Column definitions for the Test Cases grid
const COLUMN_DEFS = [
    { key: 'id',                    label: 'ID',             mandatory: false, defaultVisible: true,  defaultWidth: 100 },
    { key: 'name',                  label: 'Test Name',      mandatory: true,  defaultVisible: true,  defaultWidth: 200 },
    { key: 'description',           label: 'Description',    mandatory: false, defaultVisible: false, defaultWidth: 180 },
    { key: 'categories',                label: 'Categories',         mandatory: false, defaultVisible: true,  defaultWidth: 150 },
    { key: 'steps_count',           label: 'Steps',          mandatory: false, defaultVisible: false, defaultWidth: 80 },
    { key: 'reverification_flagged',label: 'Reverification', mandatory: false, defaultVisible: false, defaultWidth: 130 },
    { key: 'open_defects',          label: 'Open Defects',   mandatory: false, defaultVisible: false, defaultWidth: 110 },
    { key: 'linked_requirements',   label: 'Requirements',   mandatory: false, defaultVisible: false, defaultWidth: 150 },
    { key: 'qtest_status',          label: 'QTest Status',   mandatory: false, defaultVisible: false, defaultWidth: 130 },
    { key: 'created_at',            label: 'Created',        mandatory: false, defaultVisible: true,  defaultWidth: 120 },
    { key: 'updated_at',            label: 'Updated',        mandatory: false, defaultVisible: true,  defaultWidth: 120 },
];

function flattenModules(modules, depth = 0) {
    if (!modules) return [];
    return modules.flatMap(m => [
        { id: m.id, name: m.name, depth, hasChildren: !!(m.children && m.children.length > 0) },
        ...flattenModules(m.children, depth + 1),
    ]);
}

function getVisibleModules(flatList, expandedIds) {
    const result = [];
    const hiddenDepths = new Set();
    for (const m of flatList) {
        // If a parent is collapsed, skip children at deeper depth
        let hidden = false;
        for (const d of hiddenDepths) {
            if (m.depth > d) { hidden = true; break; }
        }
        if (hidden) continue;
        result.push(m);
        // If this node has children but is not expanded, mark its depth so children are hidden
        if (m.hasChildren && !expandedIds.has(m.id)) {
            hiddenDepths.clear();
            hiddenDepths.add(m.depth);
        } else {
            // Clear any hidden depth markers at or above this level
            for (const d of [...hiddenDepths]) {
                if (m.depth <= d) hiddenDepths.delete(d);
            }
        }
    }
    return result;
}

export default function TestGrid({ selectedFolders, selectedTestId }) {
    const [folderEditName, setFolderEditName] = useState(null); // null = not editing, string = draft value
    const folderNameInputRef = useRef(null);
    const [tests, setTests] = useState([]);
    const [categories, setCategories] = useState([]);
    const [selectedCategoryId, setSelectedCategoryId] = useState("");
    const [modal, setModal] = useState(null);
    const [filterText, setFilterText] = useState("");
    const [columnFilters, setColumnFilters] = useState({
        id: "",
        name: "",
        description: "",
        categories: "",
        steps_count: "",
        reverification_flagged: "",
        open_defects: "",
        linked_requirements: "",
        qtest_status: "",
        created_at: "",
        updated_at: ""
    });
    const [showColumnFilters, setShowColumnFilters] = useState(false);
    const [selectedTestIds, setSelectedTestIds] = useState([]);
    const [lastSelectedTestId, setLastSelectedTestId] = useState(null);
    const [qtestEnabled, setQtestEnabled] = useState(false);
    const [qtestMappings, setQtestMappings] = useState({});
    const [showImportDialog, setShowImportDialog] = useState(false);
    const [showUploadDialog, setShowUploadDialog] = useState(false);
    const [uploadModuleId, setUploadModuleId] = useState('');
    const [uploadConflict, setUploadConflict] = useState('skip');
    const [uploading, setUploading] = useState(false);
    const [uploadResult, setUploadResult] = useState(null);
    const [qtestModules, setQtestModules] = useState([]);
    const [loadingModules, setLoadingModules] = useState(false);
    const [uploadModuleSearch, setUploadModuleSearch] = useState('');
    const [expandedModuleIds, setExpandedModuleIds] = useState(new Set());
    const [enabledProjects, setEnabledProjects] = useState([]);
    const [uploadProjectId, setUploadProjectId] = useState('');
    const [qtestStatusFilter, setQtestStatusFilter] = useState('');
    const [showExportDropdown, setShowExportDropdown] = useState(false);
    const [showExportModal, setShowExportModal] = useState(false);
    const [exportFields, setExportFields] = useState({
        description: true,
        steps: true,
        categories: true,
        custom_values: true,
        linked_requirements: true,
    });
    const [exporting, setExporting] = useState(false);
    const exportDropdownRef = useRef(null);
    const navigate = useNavigate();

    // T004: Column visibility — hook + helper
    const [visibleKeys, toggleColumn, resetColumns] = useColumnPreference('test-cases', COLUMN_DEFS);
    const isVisible = (key) => visibleKeys.has(key);

    // Column widths — drag-to-resize + localStorage persistence
    const { columnWidths, startResize, resetWidths, resetColumnWidth, isResizing } = useColumnWidths('test-cases', COLUMN_DEFS);

    // T007: Wrapper that clears the column's filter when toggling it off
    const handleToggleColumn = useCallback((key) => {
        toggleColumn(key);
        setColumnFilters(prev => ({ ...prev, [key]: '' }));
    }, [toggleColumn]);

    // Combined reset: visibility + widths
    const handleResetAll = useCallback(() => {
        resetColumns();
        resetWidths();
    }, [resetColumns, resetWidths]);

    const loadTests = useCallback(() => {
        if (selectedFolders && selectedFolders.length > 0) {
            const ids = selectedFolders.map(f => f.id);
            return getTests(ids, undefined, { view: 'list' }).then(data => setTests(data || []));
        }
        return Promise.resolve();
    }, [selectedFolders]);

    const loadCategories = useCallback(() => getCategories().then(data => setCategories(data.categories || [])), []);

    // Dedup refs to prevent StrictMode double-fire
    const categoriesLoadedRef = useRef(false);
    const testsLoadingRef = useRef('');
    // Track which test IDs we've already loaded qtest mappings for
    const qtestLoadedRef = useRef({ idsKey: '', loading: false });

    useEffect(() => {
        let cancelled = false;

        qtestApi.getConfig()
            .then(cfg => {
                if (!cancelled) {
                    setQtestEnabled(!!cfg?.enabled);
                }
            })
            .catch(() => {
                if (!cancelled) {
                    setQtestEnabled(false);
                }
            });

        return () => { cancelled = true; };
    }, []);

    // Load qtest mappings for the current test set when integration is enabled.
    useEffect(() => {
        let cancelled = false;
        if (!qtestEnabled) {
            setQtestMappings({});
            qtestLoadedRef.current = { idsKey: '', loading: false };
            return;
        }
        if (tests.length === 0) {
            setQtestMappings({});
            qtestLoadedRef.current = { idsKey: '', loading: false };
            return;
        }

        const idsKey = tests.map(t => t.id).sort().join(',');
        // Skip if we already loaded (or are loading) for this exact set of tests
        if (idsKey === qtestLoadedRef.current.idsKey) return;

        if (!visibleKeys.has('qtest_status')) {
            toggleColumn('qtest_status');
        }
        if (qtestLoadedRef.current.idsKey === idsKey) return;
        qtestLoadedRef.current = { idsKey, loading: true };

        qtestApi.batchGetMappings(tests.map(t => t.id))
            .then(data => {
                if (cancelled) return;
                setQtestMappings(data.mappings || {});
                qtestLoadedRef.current = { idsKey, loading: false };
            })
            .catch(() => {
                if (!cancelled) {
                    qtestLoadedRef.current = { idsKey: '', loading: false };
                }
            });

        return () => { cancelled = true; };
    }, [qtestEnabled, tests, toggleColumn, visibleKeys]);

    // Force-refresh qtest mappings (after upload/sync)
    const reloadQtestMappings = useCallback(() => {
        qtestLoadedRef.current = { idsKey: '', loading: false };
        if (tests.length === 0) return;
        qtestApi.batchGetMappings(tests.map(t => t.id))
            .then(data => {
                setQtestMappings(data.mappings || {});
                const idsKey = tests.map(t => t.id).sort().join(',');
                qtestLoadedRef.current = { idsKey, loading: false };
            });
    }, [tests]);

    const handleExport = useCallback(() => {
        const fields = ['name', ...Object.entries(exportFields).filter(([, v]) => v).map(([k]) => k)];
        setExporting(true);
        exportTests(selectedTestIds, fields)
            .then(() => {
                toast.success(`Exported ${selectedTestIds.length} test case(s)`);
                setShowExportModal(false);
                setShowExportDropdown(false);
            })
            .catch(() => {})
            .finally(() => setExporting(false));
    }, [selectedTestIds, exportFields]);

    // Close export dropdown on outside click
    useEffect(() => {
        if (!showExportDropdown) return;
        const handleClick = (e) => {
            if (exportDropdownRef.current && !exportDropdownRef.current.contains(e.target)) {
                setShowExportDropdown(false);
            }
        };
        document.addEventListener('mousedown', handleClick);
        return () => document.removeEventListener('mousedown', handleClick);
    }, [showExportDropdown]);

    useEffect(() => {
        if (categoriesLoadedRef.current) return;
        categoriesLoadedRef.current = true;
        getCategories().then(data => setCategories(data.categories || []));
    }, []);

    // Reset filter/selection state synchronously when folder selection changes.
    // Using the "store previous value" pattern so we don't trigger setState in an
    // effect (which causes cascading renders per react-hooks/set-state-in-effect).
    const folderIdsKey = selectedFolders?.map(f => f.id).sort().join(',') || '';
    const [prevFolderIdsKey, setPrevFolderIdsKey] = useState(folderIdsKey);
    if (prevFolderIdsKey !== folderIdsKey) {
        setPrevFolderIdsKey(folderIdsKey);
        setSelectedTestIds([]);
        setFilterText("");
        setColumnFilters({ id: "", name: "", description: "", categories: "", steps_count: "", reverification_flagged: "", open_defects: "", linked_requirements: "", qtest_status: "", created_at: "", updated_at: "" });
        if (!folderIdsKey) {
            setTests([]);
        }
    }

    useEffect(() => {
        if (folderIdsKey && folderIdsKey === testsLoadingRef.current) return;
        testsLoadingRef.current = folderIdsKey;
        if (folderIdsKey && selectedFolders && selectedFolders.length > 0) {
            const ids = selectedFolders.map(f => f.id);
            getTests(ids, undefined, { view: 'list' }).then(data => setTests(data || []));
        }
    }, [folderIdsKey, selectedFolders]);

    const handleCreate = () => {
        if (selectedFolders.length !== 1) return;
        const folder = selectedFolders[0];
        setModal({
            type: 'prompt',
            title: 'New Test Case',
            message: `Create a new test case in "${folder.name}"`,
            placeholder: 'Test case name...',
            onConfirm: (name) => {
                if (name) createTest(name, folder.id, "").then(() => {
                    setModal(null);
                    loadTests();
                });
            }
        });
    };




    const handleBulkAssign = () => {
        if (!selectedCategoryId) return;
        const promises = selectedTestIds.map(id => assignCategory(id, selectedCategoryId));
        Promise.all(promises).then(() => {
            loadTests();
            setSelectedTestIds([]);
            setSelectedCategoryId("");
        });
    };

    const handleDelete = (e, test) => {
        e.stopPropagation();
        setModal({
            type: 'confirm',
            title: 'Delete Test Case',
            message: `Delete "${test.name}"? This cannot be undone. Historical run results will be preserved.`,
            confirmText: 'Delete',
            confirmStyle: 'danger',
            onConfirm: () => {
                deleteTest(test.id).then(() => {
                    setModal(null);
                    loadTests();
                });
            }
        });
    };

    const handleBulkDelete = () => {
        const count = selectedTestIds.length;
        setModal({
            type: 'confirm',
            title: 'Delete Test Cases',
            message: `Delete ${count} test case${count !== 1 ? 's' : ''}? This cannot be undone. Historical run results will be preserved.`,
            confirmText: 'Delete',
            confirmStyle: 'danger',
            onConfirm: () => {
                deleteTests(selectedTestIds).then(() => {
                    setModal(null);
                    setSelectedTestIds([]);
                    loadTests();
                });
            }
        });
    };

    const loadModulesForProject = (projectId) => {
        setQtestModules([]);
        setUploadModuleId('');
        setExpandedModuleIds(new Set());
        if (!projectId) { setLoadingModules(false); return; }
        setLoadingModules(true);
        qtestApi.listModules(parseInt(projectId))
            .then(mods => setQtestModules(mods || []))
            .catch(() => toast.error('Failed to load QTest modules'))
            .finally(() => setLoadingModules(false));
    };

    const handleUploadProjectChange = (projectId) => {
        setUploadProjectId(projectId);
        setUploadModuleSearch('');
        loadModulesForProject(projectId);
    };

    const handleQtestUpload = () => {
        if (selectedTestIds.length === 0) { toast.error('Select test cases to upload'); return; }
        setShowUploadDialog(true);
        setUploadResult(null);
        setUploadModuleSearch('');
        setUploadModuleId('');
        setExpandedModuleIds(new Set());
        setLoadingModules(true);
        qtestApi.listEnabledProjects()
            .then(projects => {
                setEnabledProjects(projects || []);
                const def = (projects || []).find(p => p.is_default);
                const selected = def || (projects && projects[0]);
                if (selected) {
                    setUploadProjectId(String(selected.project_id));
                    qtestApi.listModules(selected.project_id)
                        .then(mods => setQtestModules(mods || []))
                        .catch(() => toast.error('Failed to load QTest modules'))
                        .finally(() => setLoadingModules(false));
                } else {
                    setUploadProjectId('');
                    setLoadingModules(false);
                }
            })
            .catch(() => { toast.error('Failed to load QTest projects'); setLoadingModules(false); });
    };

    const doUpload = () => {
        if (!uploadModuleId) { toast.error('Select a QTest module'); return; }
        if (!uploadProjectId) { toast.error('Select a QTest project'); return; }
        setUploading(true);
        qtestApi.upload(selectedTestIds, parseInt(uploadModuleId), uploadConflict, parseInt(uploadProjectId))
            .then(result => {
                setUploadResult(result);
                if (result.succeeded > 0) toast.success(`Uploaded ${result.succeeded} test case(s) to QTest`);
                if (result.rate_limited) toast.error('QTest rate limit reached — some items were not uploaded');
                reloadQtestMappings();
            })
            .catch(err => toast.error(err.response?.data?.error || 'Upload failed'))
            .finally(() => setUploading(false));
    };

    const handleQtestSync = () => {
        if (selectedTestIds.length === 0) { toast.error('Select test cases to sync'); return; }
        qtestApi.sync(selectedTestIds)
            .then(result => {
                if (result.succeeded > 0) toast.success(`Synced ${result.succeeded} test case(s) to QTest`);
                if (result.failed > 0) toast.error(`${result.failed} test case(s) failed to sync`);
                if (result.rate_limited) toast.error('QTest rate limit reached');
                reloadQtestMappings();
            })
            .catch(err => toast.error(err.response?.data?.error || 'Sync failed'));
    };

    const handleQtestImportComplete = () => {
        loadTests();
        reloadQtestMappings();
        window.dispatchEvent(new CustomEvent('folder-tree-changed'));
    };

    const toggleSelectAll = () => {
        const pageIds = pagedTests.map(t => t.id);
        const allPageSelected = pageIds.length > 0 && pageIds.every(id => selectedTestIds.includes(id));
        if (allPageSelected) {
            setSelectedTestIds(prev => prev.filter(id => !pageIds.includes(id)));
        } else {
            setSelectedTestIds(prev => [...new Set([...prev, ...pageIds])]);
        }
    };

    const toggleSelect = (e, testId) => {
        e.stopPropagation();
        setSelectedTestIds(prev =>
            prev.includes(testId) ? prev.filter(id => id !== testId) : [...prev, testId]
        );
        setLastSelectedTestId(testId);
    };

    const handleRowClick = (e, testId) => {
        if (e.metaKey || e.ctrlKey) {
            e.preventDefault();
            setSelectedTestIds(prev =>
                prev.includes(testId) ? prev.filter(id => id !== testId) : [...prev, testId]
            );
            setLastSelectedTestId(testId);
            return;
        }
        if (e.shiftKey && lastSelectedTestId) {
            e.preventDefault();
            const ids = pagedTests.map(t => t.id);
            const startIdx = ids.indexOf(lastSelectedTestId);
            const endIdx = ids.indexOf(testId);
            if (startIdx !== -1 && endIdx !== -1) {
                const range = ids.slice(Math.min(startIdx, endIdx), Math.max(startIdx, endIdx) + 1);
                setSelectedTestIds(prev => [...new Set([...prev, ...range])]);
            } else {
                setSelectedTestIds([testId]);
                setLastSelectedTestId(testId);
            }
            return;
        }
        const folderCtx = selectedFolders && selectedFolders.length === 1 ? selectedFolders[0] : null;
        if (folderCtx && folderCtx.id) {
            navigate(`/library/folders/${folderCtx.id}/tests/${testId}`);
        } else {
            navigate(`/library/tests/${testId}`);
        }
    };

    const handleColumnFilterChange = (col, value) => {
        setColumnFilters(prev => ({ ...prev, [col]: value }));
    };

    const formatDate = (dateStr) => {
        if (!dateStr || dateStr.startsWith("0001")) return "-";
        const date = new Date(dateStr);
        if (isNaN(date.getTime())) return "-";
        return date.toLocaleDateString();
    };


    const truncate = (text, max = 60) => {
        if (!text || text.length <= max) return text || "";
        return text.substring(0, max) + '…';
    };

    const [sortConfig, setSortConfig] = useState({ key: 'name', direction: 'asc' });
    const [currentPage, setCurrentPage] = useState(1);
    const [pageSize, setPageSize] = useState(50);

    // Reset to page 1 whenever filters or folder selection change.
    // Uses the "store previous value" pattern to avoid setState in an effect.
    const filtersFingerprint = JSON.stringify([filterText, columnFilters, folderIdsKey]);
    const [prevFiltersFingerprint, setPrevFiltersFingerprint] = useState(filtersFingerprint);
    if (prevFiltersFingerprint !== filtersFingerprint) {
        setPrevFiltersFingerprint(filtersFingerprint);
        setCurrentPage(1);
    }

    const getPageNumbers = (current, total) => {
        if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1);
        if (current <= 4) return [1, 2, 3, 4, 5, '...', total];
        if (current >= total - 3) return [1, '...', total - 4, total - 3, total - 2, total - 1, total];
        return [1, '...', current - 1, current, current + 1, '...', total];
    };

    const requestSort = (key) => {
        setSortConfig(prev => {
            if (prev.key === key) {
                return { key, direction: prev.direction === 'asc' ? 'desc' : 'asc' };
            }
            return { key, direction: 'asc' };
        });
    };

    const getSortIndicator = (key) => {
        if (sortConfig.key !== key) return <span style={{ opacity: 0.3, marginLeft: 4 }}>↕</span>;
        return <span style={{ marginLeft: 4, color: 'var(--accent-indigo)' }}>{sortConfig.direction === 'asc' ? '↑' : '↓'}</span>;
    };

    if (!selectedFolders || selectedFolders.length === 0) {
        return (
            <div className="test-grid-container" style={{ justifyContent: 'center', alignItems: 'center', flex: 1, display: 'flex' }}>
                <div style={{ textAlign: 'center', color: 'var(--text-secondary)' }}>
                    <div style={{ fontSize: '3rem', marginBottom: 16 }}>📁</div>
                    <h3 style={{ fontWeight: 500 }}>Select folders to view tests</h3>
                </div>
            </div>
        );
    }

    const filteredTests = (tests || []).filter(t => {
        // Global search
        const globalSearch = filterText.toLowerCase();
        const globalMatch = filterText === "" ||
            t.name.toLowerCase().includes(globalSearch) ||
            t.id.toLowerCase().includes(globalSearch) ||
            t.categories?.some(s => s.name.toLowerCase().includes(globalSearch));

        if (!globalMatch) return false;

        // Column filters
        const idFilter = columnFilters.id.toLowerCase();
        const nameFilter = columnFilters.name.toLowerCase();
        const descFilter = columnFilters.description.toLowerCase();
        const categoriesFilter = columnFilters.categories.toLowerCase();
        const stepsFilter = columnFilters.steps_count.toLowerCase();
        const reverifFilter = columnFilters.reverification_flagged.toLowerCase();
        const defectsFilter = columnFilters.open_defects.toLowerCase();
        const reqsFilter = columnFilters.linked_requirements.toLowerCase();
        const createdFilter = columnFilters.created_at.toLowerCase();
        const updatedFilter = columnFilters.updated_at.toLowerCase();

        const idMatch = idFilter === "" || t.id.toLowerCase().includes(idFilter);
        const nameMatch = nameFilter === "" || t.name.toLowerCase().includes(nameFilter);
        const descMatch = descFilter === "" || stripHtml(t.description).toLowerCase().includes(descFilter);
        const categoryMatch = categoriesFilter === "" || t.categories?.some(s => s.name.toLowerCase().includes(categoriesFilter));
        const stepsMatch = stepsFilter === "" || String(t.steps_count ?? t.steps?.length ?? 0).includes(stepsFilter);
        const reverifMatch = reverifFilter === "" || (t.reverification_flagged ? 'yes' : 'no').includes(reverifFilter);
        const defectsMatch = defectsFilter === "" || String(t.open_defect_count || 0).includes(defectsFilter);
        const reqsMatch = reqsFilter === "" || t.linked_requirements?.some(r => r.identifier.toLowerCase().includes(reqsFilter));
        const createdMatch = createdFilter === "" || formatDate(t.created_at).toLowerCase().includes(createdFilter);
        const updatedMatch = updatedFilter === "" || formatDate(t.updated_at).toLowerCase().includes(updatedFilter);

        // QTest status filter
        if (qtestStatusFilter) {
            const mapping = qtestMappings[t.id];
            const status = mapping ? mapping.sync_status : 'not_linked';
            if (status !== qtestStatusFilter) return false;
        }

        return idMatch && nameMatch && descMatch && categoryMatch && stepsMatch && reverifMatch && defectsMatch && reqsMatch && createdMatch && updatedMatch;
    });

    const sortedTests = [...filteredTests].sort((a, b) => {
        if (!sortConfig.key) return 0;

        let aValue = a[sortConfig.key];
        let bValue = b[sortConfig.key];

        if (sortConfig.key === 'categories') {
            aValue = a.categories?.map(s => s.name).sort().join(', ') || '';
            bValue = b.categories?.map(s => s.name).sort().join(', ') || '';
        } else if (sortConfig.key === 'description') {
            aValue = stripHtml(a.description).toLowerCase();
            bValue = stripHtml(b.description).toLowerCase();
        } else if (sortConfig.key === 'steps_count') {
            aValue = a.steps?.length || 0;
            bValue = b.steps?.length || 0;
        } else if (sortConfig.key === 'reverification_flagged') {
            aValue = a.reverification_flagged ? 1 : 0;
            bValue = b.reverification_flagged ? 1 : 0;
        } else if (sortConfig.key === 'open_defects') {
            aValue = a.open_defect_count || 0;
            bValue = b.open_defect_count || 0;
        } else if (sortConfig.key === 'linked_requirements') {
            aValue = a.linked_requirements?.map(r => r.identifier).sort().join(', ') || '';
            bValue = b.linked_requirements?.map(r => r.identifier).sort().join(', ') || '';
        }

        if (aValue < bValue) return sortConfig.direction === 'asc' ? -1 : 1;
        if (aValue > bValue) return sortConfig.direction === 'asc' ? 1 : -1;
        return 0;
    });

    const isMultiRoot = selectedFolders.length > 1;
    const totalPages = Math.max(1, Math.ceil(sortedTests.length / pageSize));
    const pagedTests = sortedTests.slice((currentPage - 1) * pageSize, currentPage * pageSize);

    const handleFolderRenameStart = () => {
        if (isMultiRoot) return;
        setFolderEditName(selectedFolders[0].name);
        setTimeout(() => folderNameInputRef.current?.select(), 0);
    };

    const handleFolderRenameSave = () => {
        const trimmed = folderEditName?.trim();
        if (!trimmed) { setFolderEditName(null); return; }
        if (trimmed === selectedFolders[0].name) { setFolderEditName(null); return; }
        updateFolder(selectedFolders[0].id, trimmed)
            .then(() => {
                setFolderEditName(null);
                window.dispatchEvent(new CustomEvent('folder-tree-changed'));
            })
            .catch(() => { toast.error('Failed to rename folder'); setFolderEditName(null); });
    };

    const handleFolderRenameKeyDown = (e) => {
        if (e.key === 'Enter') handleFolderRenameSave();
        if (e.key === 'Escape') setFolderEditName(null);
    };

    return (
        <div className="test-grid-container" data-testid="test-grid">
            <header className="grid-header" style={{ marginBottom: filteredTests.length > 0 ? 16 : 32 }}>
                <div>
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginBottom: 8, display: 'flex', alignItems: 'center', gap: 6, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                        <span>Library</span>
                        <span style={{ opacity: 0.5 }}>/</span>
                        <span style={{ color: 'var(--accent-indigo)', fontWeight: 600 }}>
                            {isMultiRoot ? `${selectedFolders.length} folders selected` : selectedFolders[0].name}
                        </span>
                    </div>
                    {!isMultiRoot && folderEditName !== null ? (
                        <input
                            ref={folderNameInputRef}
                            className="modern-input grid-title"
                            value={folderEditName}
                            onChange={e => setFolderEditName(e.target.value)}
                            onBlur={handleFolderRenameSave}
                            onKeyDown={handleFolderRenameKeyDown}
                            style={{ padding: '2px 8px', fontSize: 'inherit', fontWeight: 'inherit', width: 'auto', minWidth: 120 }}
                            autoFocus
                        />
                    ) : (
                        <h2 className="grid-title" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                            {isMultiRoot ? "Bulk View" : selectedFolders[0].name}
                            {!isMultiRoot && (
                                <button
                                    onClick={handleFolderRenameStart}
                                    title="Rename folder"
                                    style={{ background: 'none', border: 'none', cursor: 'pointer', opacity: 0.45, fontSize: '0.75em', padding: '2px 4px', lineHeight: 1 }}
                                    onMouseEnter={e => e.currentTarget.style.opacity = '1'}
                                    onMouseLeave={e => e.currentTarget.style.opacity = '0.45'}
                                >✏️</button>
                            )}
                        </h2>
                    )}
                </div>
                <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
                    <div className="search-box" style={{ position: 'relative' }} data-testid="search-bar">
                        <input
                            type="text"
                            placeholder="Quick search..."
                            className="modern-input"
                            style={{ paddingLeft: 36, width: 200 }}
                            value={filterText}
                            onChange={(e) => setFilterText(e.target.value)}
                            data-testid="search-input"
                        />
                        <span style={{ position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)', opacity: 0.5 }}>🔍</span>
                    </div>
                    {qtestEnabled && (
                        <select
                            className="modern-select"
                            value={qtestStatusFilter}
                            onChange={e => setQtestStatusFilter(e.target.value)}
                            style={{ fontSize: '0.8rem', padding: '4px 8px' }}
                        >
                            <option value="">All QTest Status</option>
                            <option value="not_linked">Not Linked</option>
                            <option value="synced">Synced</option>
                            <option value="changes_pending">Changes Pending</option>
                            <option value="broken">Broken</option>
                        </select>
                    )}
                    <button
                        className={`action-btn ${showColumnFilters ? 'active' : ''}`}
                        onClick={() => setShowColumnFilters(!showColumnFilters)}
                        style={{ padding: '8px 12px', background: showColumnFilters ? 'var(--bg-tertiary)' : 'transparent' }}
                        title="Column Filters"
                    >
                        {showColumnFilters ? 'Hide Filters' : 'Column Filters'}
                    </button>
                    {/* T005: Column visibility picker */}
                    <ColumnPicker
                        columnDefs={COLUMN_DEFS}
                        visibleKeys={visibleKeys}
                        onToggle={handleToggleColumn}
                        onReset={handleResetAll}
                    />
                    <div style={{ position: 'relative' }} ref={exportDropdownRef}>
                        <button
                            className="action-btn"
                            onClick={() => setShowExportDropdown(!showExportDropdown)}
                            title="Export test cases"
                        >
                            Export &#9662;
                        </button>
                        {showExportDropdown && (
                            <div style={{
                                position: 'absolute', top: '100%', right: 0, marginTop: 4,
                                background: 'var(--bg-primary)', border: '1px solid var(--border-color)',
                                borderRadius: 8, boxShadow: '0 4px 12px rgba(0,0,0,0.15)',
                                minWidth: 200, zIndex: 100, overflow: 'hidden'
                            }}>
                                <button
                                    style={{
                                        width: '100%', padding: '10px 16px', border: 'none',
                                        background: 'transparent', textAlign: 'left', cursor: selectedTestIds.length > 0 ? 'pointer' : 'not-allowed',
                                        color: selectedTestIds.length > 0 ? 'var(--text-primary)' : 'var(--text-tertiary)',
                                        fontSize: '0.9rem',
                                    }}
                                    disabled={selectedTestIds.length === 0}
                                    onClick={() => { setShowExportModal(true); setShowExportDropdown(false); }}
                                    onMouseEnter={e => { if (selectedTestIds.length > 0) e.target.style.background = 'var(--bg-secondary)'; }}
                                    onMouseLeave={e => { e.target.style.background = 'transparent'; }}
                                >
                                    Export Selected as JSON
                                    {selectedTestIds.length > 0 && (
                                        <span style={{ marginLeft: 8, fontSize: '0.8rem', opacity: 0.6 }}>({selectedTestIds.length})</span>
                                    )}
                                </button>
                            </div>
                        )}
                    </div>
                    {qtestEnabled && (
                        <button
                            className="action-btn"
                            onClick={() => setShowImportDialog(true)}
                            disabled={isMultiRoot}
                            style={{ opacity: isMultiRoot ? 0.5 : 1, cursor: isMultiRoot ? 'not-allowed' : 'pointer' }}
                            title={isMultiRoot ? 'Import is available for a single folder selection' : 'Import test cases from QTest'}
                        >
                            Import from QTest
                        </button>
                    )}
                    <button
                        className="primary-btn"
                        onClick={handleCreate}
                        disabled={isMultiRoot}
                        style={{ opacity: isMultiRoot ? 0.5 : 1, cursor: isMultiRoot ? 'not-allowed' : 'pointer' }}
                        data-testid="create-test-button"
                        title={isMultiRoot ? "Creation disabled for multiple selection" : "New Test"}
                    >
                        + New Test
                    </button>
                </div>
            </header>

            {selectedTestIds.length > 0 && (
                <div className="bulk-action-bar" style={{
                    display: 'flex', alignItems: 'center', gap: 16, padding: '12px 20px',
                    background: 'rgba(99, 102, 241, 0.1)', border: '1px solid var(--accent-indigo)',
                    borderRadius: 8, marginBottom: 24, animation: 'fadeIn 0.2s ease'
                }}>
                    <div style={{ fontWeight: 600, color: 'var(--accent-indigo)' }}>{selectedTestIds.length} items selected</div>
                    <div style={{ height: 20, width: 1, background: 'var(--border-color)' }}></div>
                    <div style={{ display: 'flex', gap: 8 }}>
                        <button className="action-btn" onClick={handleBulkDelete} style={{ color: 'var(--accent-red)' }} data-testid="bulk-delete-tests-button">🗑️ Delete</button>
                        {qtestEnabled && (
                            <button className="action-btn" onClick={handleQtestUpload} disabled={selectedTestIds.length === 0} style={{ fontSize: '0.8rem' }}>
                                Upload to QTest
                            </button>
                        )}
                        {qtestEnabled && (
                            <button className="action-btn" onClick={handleQtestSync} disabled={selectedTestIds.length === 0} style={{ fontSize: '0.8rem' }}>
                                Sync to QTest
                            </button>
                        )}
                    </div>
                    <div style={{ marginLeft: 'auto', display: 'flex', gap: 8, alignItems: 'center' }}>
                        <select
                            className="modern-select"
                            onChange={(e) => setSelectedCategoryId(e.target.value)}
                            value={selectedCategoryId}
                            style={{ minWidth: 120 }}
                        >
                            <option value="">Bulk Tag...</option>
                            {categories.map(s => <option key={s.id} value={s.id}>{s.name}</option>)}
                        </select>
                        <button className="primary-btn" onClick={handleBulkAssign} disabled={!selectedCategoryId} style={{ padding: '6px 14px' }}>Apply</button>
                    </div>
                </div>
            )}

            <div className="table-scroll-x">
                <table className="modern-table resizable" data-testid="test-table">
                    <thead>
                        <tr>
                            <th style={{ width: 40, paddingRight: 0 }}>
                                <input
                                    type="checkbox"
                                    checked={pagedTests.length > 0 && pagedTests.every(t => selectedTestIds.includes(t.id))}
                                    onChange={toggleSelectAll}
                                    style={{ transform: 'scale(1.1)', cursor: 'pointer' }}
                                />
                            </th>
                            {/* T006: Optional columns gated by isVisible() */}
                            {isVisible('id') && (
                                <th className="col-resize-th" style={{ width: columnWidths['id'], cursor: 'pointer', userSelect: 'none' }} onClick={() => requestSort('id')}>
                                    ID {getSortIndicator('id')}
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('id', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('id'); }} />
                                </th>
                            )}
                            {/* name is mandatory — always rendered */}
                            <th className="col-resize-th" style={{ width: columnWidths['name'], cursor: 'pointer', userSelect: 'none' }} onClick={() => requestSort('name')}>
                                Test Name {getSortIndicator('name')}
                                <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('name', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('name'); }} />
                            </th>
                            {isVisible('description') && (
                                <th className="col-resize-th" style={{ width: columnWidths['description'], cursor: 'pointer', userSelect: 'none' }} onClick={() => requestSort('description')}>
                                    Description {getSortIndicator('description')}
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('description', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('description'); }} />
                                </th>
                            )}
                            {isVisible('categories') && (
                                <th className="col-resize-th" style={{ width: columnWidths['categories'], cursor: 'pointer', userSelect: 'none' }} onClick={() => requestSort('categories')}>
                                    Categories {getSortIndicator('categories')}
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('categories', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('categories'); }} />
                                </th>
                            )}
                            {isVisible('steps_count') && (
                                <th className="col-resize-th" style={{ width: columnWidths['steps_count'], cursor: 'pointer', userSelect: 'none' }} onClick={() => requestSort('steps_count')}>
                                    Steps {getSortIndicator('steps_count')}
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('steps_count', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('steps_count'); }} />
                                </th>
                            )}
                            {isVisible('reverification_flagged') && (
                                <th className="col-resize-th" style={{ width: columnWidths['reverification_flagged'], cursor: 'pointer', userSelect: 'none' }} onClick={() => requestSort('reverification_flagged')}>
                                    Reverification {getSortIndicator('reverification_flagged')}
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('reverification_flagged', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('reverification_flagged'); }} />
                                </th>
                            )}
                            {isVisible('open_defects') && (
                                <th className="col-resize-th" style={{ width: columnWidths['open_defects'], cursor: 'pointer', userSelect: 'none' }} onClick={() => requestSort('open_defects')}>
                                    Open Defects {getSortIndicator('open_defects')}
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('open_defects', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('open_defects'); }} />
                                </th>
                            )}
                            {isVisible('linked_requirements') && (
                                <th className="col-resize-th" style={{ width: columnWidths['linked_requirements'], cursor: 'pointer', userSelect: 'none' }} onClick={() => requestSort('linked_requirements')}>
                                    Requirements {getSortIndicator('linked_requirements')}
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('linked_requirements', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('linked_requirements'); }} />
                                </th>
                            )}
                            {isVisible('qtest_status') && (
                                <th className="col-resize-th" style={{ width: columnWidths['qtest_status'], cursor: 'pointer', userSelect: 'none' }} onClick={() => requestSort('qtest_status')}>
                                    QTest Status {getSortIndicator('qtest_status')}
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('qtest_status', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('qtest_status'); }} />
                                </th>
                            )}
                            {isVisible('created_at') && (
                                <th className="col-resize-th" style={{ width: columnWidths['created_at'], cursor: 'pointer', userSelect: 'none' }} onClick={() => requestSort('created_at')}>
                                    Created {getSortIndicator('created_at')}
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('created_at', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('created_at'); }} />
                                </th>
                            )}
                            {isVisible('updated_at') && (
                                <th className="col-resize-th" style={{ width: columnWidths['updated_at'], cursor: 'pointer', userSelect: 'none' }} onClick={() => requestSort('updated_at')}>
                                    Updated {getSortIndicator('updated_at')}
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('updated_at', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('updated_at'); }} />
                                </th>
                            )}
                            <th style={{ width: 56, padding: 0 }}></th>
                        </tr>
                        {showColumnFilters && (
                            <tr className="filter-row" style={{ background: 'var(--bg-secondary)' }}>
                                <th></th>
                                {/* T006: Filter inputs only shown for visible columns */}
                                {isVisible('id') && (
                                    <th>
                                        <input
                                            className="modern-input"
                                            placeholder="ID..."
                                            style={{ width: '100%', fontSize: '0.75rem', padding: '4px 8px' }}
                                            value={columnFilters.id}
                                            onChange={(e) => handleColumnFilterChange('id', e.target.value)}
                                        />
                                    </th>
                                )}
                                <th>
                                    <input
                                        className="modern-input"
                                        placeholder="Name..."
                                        style={{ width: '100%', fontSize: '0.75rem', padding: '4px 8px' }}
                                        value={columnFilters.name}
                                        onChange={(e) => handleColumnFilterChange('name', e.target.value)}
                                    />
                                </th>
                                {isVisible('description') && (
                                    <th>
                                        <input
                                            className="modern-input"
                                            placeholder="Description..."
                                            style={{ width: '100%', fontSize: '0.75rem', padding: '4px 8px' }}
                                            value={columnFilters.description}
                                            onChange={(e) => handleColumnFilterChange('description', e.target.value)}
                                        />
                                    </th>
                                )}
                                {isVisible('categories') && (
                                    <th>
                                        <input
                                            className="modern-input"
                                            placeholder="Categories..."
                                            style={{ width: '100%', fontSize: '0.75rem', padding: '4px 8px' }}
                                            value={columnFilters.categories}
                                            onChange={(e) => handleColumnFilterChange('categories', e.target.value)}
                                        />
                                    </th>
                                )}
                                {isVisible('steps_count') && (
                                    <th>
                                        <input
                                            className="modern-input"
                                            placeholder="Steps..."
                                            style={{ width: '100%', fontSize: '0.75rem', padding: '4px 8px' }}
                                            value={columnFilters.steps_count}
                                            onChange={(e) => handleColumnFilterChange('steps_count', e.target.value)}
                                        />
                                    </th>
                                )}
                                {isVisible('reverification_flagged') && (
                                    <th>
                                        <input
                                            className="modern-input"
                                            placeholder="Yes/No..."
                                            style={{ width: '100%', fontSize: '0.75rem', padding: '4px 8px' }}
                                            value={columnFilters.reverification_flagged}
                                            onChange={(e) => handleColumnFilterChange('reverification_flagged', e.target.value)}
                                        />
                                    </th>
                                )}
                                {isVisible('open_defects') && (
                                    <th>
                                        <input
                                            className="modern-input"
                                            placeholder="Defects..."
                                            style={{ width: '100%', fontSize: '0.75rem', padding: '4px 8px' }}
                                            value={columnFilters.open_defects}
                                            onChange={(e) => handleColumnFilterChange('open_defects', e.target.value)}
                                        />
                                    </th>
                                )}
                                {isVisible('linked_requirements') && (
                                    <th>
                                        <input
                                            className="modern-input"
                                            placeholder="Requirements..."
                                            style={{ width: '100%', fontSize: '0.75rem', padding: '4px 8px' }}
                                            value={columnFilters.linked_requirements}
                                            onChange={(e) => handleColumnFilterChange('linked_requirements', e.target.value)}
                                        />
                                    </th>
                                )}
                                {isVisible('qtest_status') && (
                                    <th>
                                        <input
                                            className="modern-input"
                                            placeholder="QTest..."
                                            style={{ width: '100%', fontSize: '0.75rem', padding: '4px 8px' }}
                                            value={columnFilters.qtest_status}
                                            onChange={(e) => handleColumnFilterChange('qtest_status', e.target.value)}
                                        />
                                    </th>
                                )}
                                {isVisible('created_at') && (
                                    <th>
                                        <input
                                            className="modern-input"
                                            placeholder="Date..."
                                            style={{ width: '100%', fontSize: '0.75rem', padding: '4px 8px' }}
                                            value={columnFilters.created_at}
                                            onChange={(e) => handleColumnFilterChange('created_at', e.target.value)}
                                        />
                                    </th>
                                )}
                                {isVisible('updated_at') && (
                                    <th>
                                        <input
                                            className="modern-input"
                                            placeholder="Date..."
                                            style={{ width: '100%', fontSize: '0.75rem', padding: '4px 8px' }}
                                            value={columnFilters.updated_at}
                                            onChange={(e) => handleColumnFilterChange('updated_at', e.target.value)}
                                        />
                                    </th>
                                )}
                                <th></th>
                            </tr>
                        )}
                    </thead>
                    <tbody>
                        {pagedTests.map(t => (
                            <tr
                                key={t.id}
                                data-testid="test-row"
                                onClick={(e) => handleRowClick(e, t.id)}
                                className={selectedTestId === t.id ? 'test-row-selected' : ''}
                                style={{ cursor: 'pointer' }}
                            >
                                <td style={{ paddingRight: 0 }} onClick={e => e.stopPropagation()}>
                                    <input
                                        type="checkbox"
                                        checked={selectedTestIds.includes(t.id)}
                                        onChange={(e) => toggleSelect(e, t.id)}
                                        style={{ transform: 'scale(1.1)', cursor: 'pointer' }}
                                    />
                                </td>
                                {/* T006: Optional cells gated by isVisible() */}
                                {isVisible('id') && (
                                    <td>
                                        <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', fontFamily: 'monospace' }} title={t.id}>
                                            {t.id.substring(0, 8)}
                                        </span>
                                    </td>
                                )}
                                {/* name is mandatory — always rendered */}
                                <td style={{ maxWidth: columnWidths['name'], overflow: 'hidden' }}>
                                    <div style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
                                        <div style={{ fontWeight: 500, color: 'var(--text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={t.name}>{t.name}</div>
                                        {isMultiRoot && (
                                            <span style={{ fontSize: '0.65rem', padding: '2px 6px', background: 'var(--bg-tertiary)', borderRadius: 4, color: 'var(--text-secondary)' }}>
                                                ID: {t.folder_id.substring(0, 4)}
                                            </span>
                                        )}
                                    </div>
                                </td>
                                {isVisible('description') && (
                                    <td style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', maxWidth: 220, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={stripHtml(t.description)}>
                                        {truncate(stripHtml(t.description))}
                                    </td>
                                )}
                                {isVisible('categories') && (
                                    <td onClick={e => e.stopPropagation()}>
                                        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
                                            {t.categories?.map(s => (
                                                <span key={s.id} className="category-tag" style={{ fontSize: '0.65rem', padding: '2px 6px' }}>{s.name}</span>
                                            ))}
                                        </div>
                                    </td>
                                )}
                                {isVisible('steps_count') && (
                                    <td style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', textAlign: 'center' }}>
                                        {t.steps_count ?? t.steps?.length ?? 0}
                                    </td>
                                )}
                                {isVisible('reverification_flagged') && (
                                    <td style={{ textAlign: 'center' }}>
                                        {t.reverification_flagged
                                            ? <span style={{ fontSize: '0.75rem', padding: '2px 8px', background: 'rgba(245, 158, 11, 0.15)', color: 'var(--accent-amber, #f59e0b)', borderRadius: 4, fontWeight: 500 }}>Needs Re-verify</span>
                                            : <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>—</span>
                                        }
                                    </td>
                                )}
                                {isVisible('open_defects') && (
                                    <td style={{ fontSize: '0.75rem', textAlign: 'center', color: (t.open_defect_count || 0) > 0 ? 'var(--accent-red)' : 'var(--text-secondary)', fontWeight: (t.open_defect_count || 0) > 0 ? 600 : 400 }}>
                                        {t.open_defect_count || 0}
                                    </td>
                                )}
                                {isVisible('linked_requirements') && (
                                    <td style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                                        {t.linked_requirements?.length > 0
                                            ? <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
                                                {t.linked_requirements.map(r => (
                                                    <span key={r.id} style={{ fontSize: '0.65rem', padding: '2px 6px', background: 'rgba(99, 102, 241, 0.1)', color: 'var(--accent-indigo)', borderRadius: 4 }}>{r.identifier}</span>
                                                ))}
                                              </div>
                                            : '—'
                                        }
                                    </td>
                                )}
                                {isVisible('qtest_status') && (() => {
                                    const mapping = qtestMappings[t.id];
                                    const qstatus = mapping ? mapping.sync_status : 'not_linked';
                                    const statusLabels = { synced: 'Synced', changes_pending: 'Pending', broken: 'Broken', not_linked: '—' };
                                    const statusBg = { synced: 'rgba(52,211,153,0.15)', changes_pending: 'rgba(251,191,36,0.15)', broken: 'rgba(248,113,113,0.15)', not_linked: 'transparent' };
                                    const statusColor = { synced: '#34d399', changes_pending: '#fbbf24', broken: '#f87171', not_linked: 'var(--text-secondary)' };
                                    return (
                                        <td style={{ textAlign: 'center' }}>
                                            <span style={{ fontSize: '0.75rem', padding: '2px 6px', borderRadius: 4, background: statusBg[qstatus], color: statusColor[qstatus] }}>
                                                {statusLabels[qstatus]}
                                            </span>
                                        </td>
                                    );
                                })()}
                                {isVisible('created_at') && (
                                    <td style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                                        {formatDate(t.created_at)}
                                    </td>
                                )}
                                {isVisible('updated_at') && (
                                    <td style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                                        {formatDate(t.updated_at)}
                                    </td>
                                )}
                                <td onClick={e => e.stopPropagation()} style={{ textAlign: 'center', padding: '10px 0' }}>
                                    <button
                                        className="action-btn"
                                        onClick={(e) => handleDelete(e, t)}
                                        style={{ color: 'var(--accent-red)', padding: '4px 8px', opacity: 0.7, display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}
                                        title="Delete test case"
                                        data-testid={`delete-test-button-${t.id}`}
                                    >🗑️</button>
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>

            {filteredTests.length === 0 && (
                <div style={{ textAlign: 'center', padding: '60px 0', color: 'var(--text-secondary)' }}>
                    <div style={{ fontSize: '2rem', marginBottom: 12 }}>🔍</div>
                    <p>No tests found matching your criteria</p>
                </div>
            )}

            {sortedTests.length > 0 && (
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginTop: 16, padding: '12px 0', flexWrap: 'wrap', gap: 8 }}>
                    {/* Left: count */}
                    <div style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', minWidth: 160 }}>
                        Showing {sortedTests.length === 0 ? 0 : (currentPage - 1) * pageSize + 1}–{Math.min(currentPage * pageSize, sortedTests.length)} of {sortedTests.length}
                    </div>

                    {/* Center: page buttons */}
                    {totalPages > 1 && (
                        <div style={{ display: 'flex', gap: 4, alignItems: 'center' }}>
                            <button
                                className="action-btn"
                                onClick={() => setCurrentPage(p => Math.max(1, p - 1))}
                                disabled={currentPage === 1}
                                style={{ padding: '4px 10px', opacity: currentPage === 1 ? 0.4 : 1 }}
                            >‹ Prev</button>

                            {getPageNumbers(currentPage, totalPages).map((pg, i) =>
                                pg === '...' ? (
                                    <span key={`ellipsis-${i}`} style={{ padding: '0 4px', color: 'var(--text-secondary)' }}>…</span>
                                ) : (
                                    <button
                                        key={pg}
                                        className={`action-btn ${currentPage === pg ? 'primary-btn' : ''}`}
                                        onClick={() => setCurrentPage(pg)}
                                        style={{ minWidth: 34, padding: '4px 8px' }}
                                    >{pg}</button>
                                )
                            )}

                            <button
                                className="action-btn"
                                onClick={() => setCurrentPage(p => Math.min(totalPages, p + 1))}
                                disabled={currentPage === totalPages}
                                style={{ padding: '4px 10px', opacity: currentPage === totalPages ? 0.4 : 1 }}
                            >Next ›</button>
                        </div>
                    )}

                    {/* Right: page size */}
                    <div style={{ display: 'flex', gap: 8, alignItems: 'center', fontSize: '0.875rem', color: 'var(--text-secondary)' }}>
                        <span>Per page:</span>
                        <select
                            className="modern-select"
                            value={pageSize}
                            onChange={(e) => { setPageSize(Number(e.target.value)); setCurrentPage(1); }}
                            style={{ padding: '4px 8px' }}
                        >
                            <option value={25}>25</option>
                            <option value={50}>50</option>
                            <option value={100}>100</option>
                        </select>
                    </div>
                </div>
            )}

            {showImportDialog && !isMultiRoot && (
                <QTestImportModal
                    initialFolderId={selectedFolders[0].id}
                    onClose={() => setShowImportDialog(false)}
                    onImported={handleQtestImportComplete}
                />
            )}

            {showUploadDialog && (() => {
                const flatModules = flattenModules(qtestModules);
                const moduleSearch = (uploadModuleSearch || '').toLowerCase();
                const isSearching = moduleSearch.length > 0;
                const displayModules = isSearching
                    ? flatModules.filter(m => m.name.toLowerCase().includes(moduleSearch))
                    : getVisibleModules(flatModules, expandedModuleIds);
                const selectedModule = flatModules.find(m => String(m.id) === String(uploadModuleId));
                const total = selectedTestIds.length;
                const hasResult = !!uploadResult;
                const allSucceeded = hasResult && uploadResult.succeeded === total && uploadResult.failed === 0;
                const hasFailed = hasResult && uploadResult.failed > 0;

                const toggleExpand = (id) => {
                    setExpandedModuleIds(prev => {
                        const next = new Set(prev);
                        if (next.has(id)) next.delete(id);
                        else next.add(id);
                        return next;
                    });
                };

                return (
                    <div className="modal-overlay" onClick={() => !uploading && setShowUploadDialog(false)}>
                        <div className="modal-content" onClick={e => e.stopPropagation()} style={{ maxWidth: 620, width: '90vw', padding: 0, overflow: 'hidden' }}>
                            {/* Header */}
                            <div style={{
                                padding: '24px 28px 20px',
                                borderBottom: '1px solid var(--border-color)',
                                display: 'flex', alignItems: 'center', gap: 14,
                            }}>
                                <div style={{
                                    width: 40, height: 40, borderRadius: 10,
                                    background: 'linear-gradient(135deg, #6366f1, #8b5cf6)',
                                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                                    fontSize: '1.2rem', flexShrink: 0,
                                }}>⬆</div>
                                <div style={{ flex: 1 }}>
                                    <h3 style={{ margin: 0, fontSize: '1.2rem', fontWeight: 700, color: 'var(--text-primary)' }}>
                                        Upload to QTest
                                    </h3>
                                    <p style={{ margin: 0, fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
                                        {total} test case{total !== 1 ? 's' : ''} selected
                                        {uploadProjectId && enabledProjects.length > 0 && (() => {
                                            const proj = enabledProjects.find(p => String(p.project_id) === uploadProjectId);
                                            return proj ? (
                                                <span style={{ marginLeft: 6 }}>
                                                    · Project: <span style={{ fontWeight: 600, color: 'var(--text-primary)' }}>{proj.project_name}</span>
                                                </span>
                                            ) : null;
                                        })()}
                                    </p>
                                </div>
                            </div>

                            {/* Body */}
                            <div style={{ padding: '24px 28px' }}>
                                {!hasResult ? (
                                    <>
                                        {/* Project picker */}
                                        {enabledProjects.length > 1 && (
                                            <div style={{ marginBottom: 20 }}>
                                                <label style={{
                                                    display: 'block', marginBottom: 8,
                                                    fontSize: '0.8rem', fontWeight: 600,
                                                    color: 'var(--text-secondary)',
                                                    textTransform: 'uppercase', letterSpacing: '0.05em',
                                                }}>Target Project</label>
                                                <select
                                                    className="modern-select"
                                                    value={uploadProjectId}
                                                    onChange={e => handleUploadProjectChange(e.target.value)}
                                                    style={{ width: '100%' }}
                                                >
                                                    {enabledProjects.map(p => (
                                                        <option key={p.project_id} value={String(p.project_id)}>
                                                            {p.project_name}{p.is_default ? ' (Default)' : ''}
                                                        </option>
                                                    ))}
                                                </select>
                                            </div>
                                        )}
                                        {enabledProjects.length === 0 && !loadingModules && (
                                            <div style={{
                                                padding: '16px', borderRadius: 10,
                                                background: 'rgba(251,191,36,0.08)',
                                                border: '1px solid rgba(251,191,36,0.3)',
                                                fontSize: '0.85rem', color: 'var(--text-secondary)',
                                                marginBottom: 20,
                                            }}>
                                                No projects enabled — configure projects in QTest Settings first
                                            </div>
                                        )}
                                        {/* Module picker */}
                                        <div style={{ marginBottom: 24 }}>
                                            <label style={{
                                                display: 'block', marginBottom: 8,
                                                fontSize: '0.8rem', fontWeight: 600,
                                                color: 'var(--text-secondary)',
                                                textTransform: 'uppercase', letterSpacing: '0.05em',
                                            }}>Target Module</label>
                                            {loadingModules ? (
                                                <div style={{
                                                    padding: '40px 0', textAlign: 'center',
                                                    color: 'var(--text-secondary)', fontSize: '0.9rem',
                                                }}>
                                                    <div style={{
                                                        width: 28, height: 28, margin: '0 auto 10px',
                                                        border: '2px solid var(--border-color)',
                                                        borderTopColor: 'var(--accent-indigo)',
                                                        borderRadius: '50%',
                                                        animation: 'spin 0.6s linear infinite',
                                                    }} />
                                                    Loading modules from QTest...
                                                </div>
                                            ) : (
                                                <div style={{
                                                    border: '1px solid var(--border-color)',
                                                    borderRadius: 10, overflow: 'hidden',
                                                    background: 'var(--bg-primary)',
                                                }}>
                                                    {/* Search input */}
                                                    <div style={{
                                                        padding: '10px 14px',
                                                        borderBottom: '1px solid var(--border-color)',
                                                        display: 'flex', alignItems: 'center', gap: 8,
                                                    }}>
                                                        <span style={{ color: 'var(--text-secondary)', fontSize: '0.9rem' }}>🔍</span>
                                                        <input
                                                            type="text"
                                                            placeholder="Search modules..."
                                                            value={uploadModuleSearch || ''}
                                                            onChange={e => setUploadModuleSearch(e.target.value)}
                                                            style={{
                                                                flex: 1, border: 'none', outline: 'none',
                                                                background: 'transparent', fontSize: '0.9rem',
                                                                color: 'var(--text-primary)',
                                                            }}
                                                        />
                                                        {uploadModuleSearch && (
                                                            <button
                                                                onClick={() => setUploadModuleSearch('')}
                                                                style={{
                                                                    background: 'none', border: 'none',
                                                                    cursor: 'pointer', color: 'var(--text-secondary)',
                                                                    fontSize: '0.85rem', padding: '2px 4px',
                                                                }}
                                                            >✕</button>
                                                        )}
                                                    </div>
                                                    {/* Module tree */}
                                                    <div style={{
                                                        maxHeight: 300, overflowY: 'auto',
                                                        padding: '4px 0',
                                                    }}>
                                                        {displayModules.length === 0 ? (
                                                            <div style={{
                                                                padding: '20px 14px', textAlign: 'center',
                                                                color: 'var(--text-secondary)', fontSize: '0.9rem',
                                                            }}>
                                                                {isSearching ? 'No modules match your search' : 'No modules found'}
                                                            </div>
                                                        ) : displayModules.map(m => {
                                                            const isSelected = String(m.id) === String(uploadModuleId);
                                                            const isExpanded = expandedModuleIds.has(m.id);
                                                            return (
                                                                <div
                                                                    key={m.id}
                                                                    style={{
                                                                        padding: '8px 14px',
                                                                        paddingLeft: isSearching ? 14 : 14 + m.depth * 20,
                                                                        cursor: 'pointer',
                                                                        fontSize: '0.9rem',
                                                                        display: 'flex', alignItems: 'center', gap: 6,
                                                                        background: isSelected
                                                                            ? 'var(--accent-indigo-subtle, rgba(99,102,241,0.1))'
                                                                            : 'transparent',
                                                                        color: isSelected
                                                                            ? 'var(--accent-indigo)'
                                                                            : 'var(--text-primary)',
                                                                        fontWeight: isSelected ? 600 : 400,
                                                                        transition: 'background 0.1s',
                                                                    }}
                                                                    onMouseEnter={e => {
                                                                        if (!isSelected)
                                                                            e.currentTarget.style.background = 'var(--bg-hover, rgba(255,255,255,0.05))';
                                                                    }}
                                                                    onMouseLeave={e => {
                                                                        if (!isSelected)
                                                                            e.currentTarget.style.background = 'transparent';
                                                                    }}
                                                                    onClick={() => setUploadModuleId(String(m.id))}
                                                                >
                                                                    {/* Expand/collapse toggle */}
                                                                    {m.hasChildren && !isSearching ? (
                                                                        <span
                                                                            onClick={e => { e.stopPropagation(); toggleExpand(m.id); }}
                                                                            style={{
                                                                                width: 20, height: 20,
                                                                                display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
                                                                                borderRadius: 4, flexShrink: 0,
                                                                                fontSize: '0.7rem', color: 'var(--text-secondary)',
                                                                                transition: 'transform 0.15s',
                                                                                transform: isExpanded ? 'rotate(90deg)' : 'rotate(0deg)',
                                                                            }}
                                                                        >▶</span>
                                                                    ) : (
                                                                        <span style={{ width: 20, flexShrink: 0 }} />
                                                                    )}
                                                                    <span style={{ fontSize: '0.85rem', opacity: 0.5 }}>📁</span>
                                                                    <span style={{ flex: 1 }}>{m.name}</span>
                                                                    {isSelected && <span style={{ fontSize: '0.8rem' }}>✓</span>}
                                                                </div>
                                                            );
                                                        })}
                                                    </div>
                                                    {/* Selected indicator */}
                                                    {selectedModule && (
                                                        <div style={{
                                                            padding: '10px 14px',
                                                            borderTop: '1px solid var(--border-color)',
                                                            fontSize: '0.85rem',
                                                            color: 'var(--accent-indigo)',
                                                            display: 'flex', alignItems: 'center', gap: 6,
                                                        }}>
                                                            <span>✓</span>
                                                            <span style={{ fontWeight: 600 }}>{selectedModule.name}</span>
                                                        </div>
                                                    )}
                                                </div>
                                            )}
                                        </div>

                                        {/* Conflict resolution */}
                                        <div style={{ marginBottom: 4 }}>
                                            <label style={{
                                                display: 'block', marginBottom: 10,
                                                fontSize: '0.8rem', fontWeight: 600,
                                                color: 'var(--text-secondary)',
                                                textTransform: 'uppercase', letterSpacing: '0.05em',
                                            }}>If Already Linked</label>
                                            <div style={{ display: 'flex', gap: 10 }}>
                                                {[
                                                    { value: 'skip', label: 'Skip', desc: 'Keep existing' },
                                                    { value: 'update', label: 'Update', desc: 'Overwrite in QTest' },
                                                ].map(opt => (
                                                    <div
                                                        key={opt.value}
                                                        onClick={() => setUploadConflict(opt.value)}
                                                        style={{
                                                            flex: 1, padding: '12px 16px',
                                                            borderRadius: 10, cursor: 'pointer',
                                                            border: `2px solid ${uploadConflict === opt.value ? 'var(--accent-indigo)' : 'var(--border-color)'}`,
                                                            background: uploadConflict === opt.value
                                                                ? 'var(--accent-indigo-subtle, rgba(99,102,241,0.08))'
                                                                : 'transparent',
                                                            transition: 'all 0.15s',
                                                        }}
                                                    >
                                                        <div style={{
                                                            fontSize: '0.9rem', fontWeight: 600,
                                                            color: uploadConflict === opt.value ? 'var(--accent-indigo)' : 'var(--text-primary)',
                                                        }}>{opt.label}</div>
                                                        <div style={{
                                                            fontSize: '0.8rem', color: 'var(--text-secondary)', marginTop: 2,
                                                        }}>{opt.desc}</div>
                                                    </div>
                                                ))}
                                            </div>
                                        </div>
                                    </>
                                ) : (
                                    /* Upload results */
                                    <div style={{ textAlign: 'center', padding: '12px 0' }}>
                                        <div style={{
                                            width: 56, height: 56, borderRadius: '50%',
                                            margin: '0 auto 20px',
                                            display: 'flex', alignItems: 'center', justifyContent: 'center',
                                            fontSize: '1.8rem',
                                            background: allSucceeded
                                                ? 'rgba(34,197,94,0.12)'
                                                : hasFailed
                                                    ? 'rgba(239,68,68,0.12)'
                                                    : 'rgba(251,191,36,0.12)',
                                        }}>
                                            {allSucceeded ? '✅' : hasFailed ? '⚠️' : 'ℹ️'}
                                        </div>
                                        <h4 style={{
                                            margin: '0 0 8px', fontSize: '1.15rem', fontWeight: 700,
                                            color: 'var(--text-primary)',
                                        }}>
                                            {allSucceeded ? 'Upload Complete' : hasFailed ? 'Partial Upload' : 'Upload Finished'}
                                        </h4>

                                        <div style={{
                                            display: 'flex', gap: 16, justifyContent: 'center',
                                            margin: '20px 0',
                                        }}>
                                            {[
                                                { label: 'Succeeded', value: uploadResult.succeeded, color: '#22c55e' },
                                                { label: 'Skipped', value: uploadResult.skipped, color: '#a3a3a3' },
                                                { label: 'Failed', value: uploadResult.failed, color: '#ef4444' },
                                            ].filter(s => s.value > 0).map(s => (
                                                <div key={s.label} style={{
                                                    padding: '14px 28px', borderRadius: 10,
                                                    background: 'var(--bg-primary)',
                                                    border: '1px solid var(--border-color)',
                                                    minWidth: 100,
                                                }}>
                                                    <div style={{
                                                        fontSize: '1.5rem', fontWeight: 700,
                                                        color: s.color,
                                                    }}>{s.value}</div>
                                                    <div style={{
                                                        fontSize: '0.8rem', color: 'var(--text-secondary)',
                                                        marginTop: 4,
                                                    }}>{s.label}</div>
                                                </div>
                                            ))}
                                        </div>

                                        {uploadResult.rate_limited && (
                                            <div style={{
                                                padding: '10px 14px', borderRadius: 8,
                                                background: 'rgba(251,191,36,0.1)',
                                                border: '1px solid rgba(251,191,36,0.3)',
                                                fontSize: '0.85rem', color: '#fbbf24',
                                                marginTop: 8,
                                            }}>
                                                ⚡ Rate limit reached — some items were not processed
                                            </div>
                                        )}
                                    </div>
                                )}
                            </div>

                            {/* Footer */}
                            <div style={{
                                padding: '16px 28px',
                                borderTop: '1px solid var(--border-color)',
                                display: 'flex', justifyContent: 'flex-end', gap: 12,
                            }}>
                                <button
                                    className="action-btn"
                                    onClick={() => { setShowUploadDialog(false); setUploadModuleSearch(''); }}
                                    disabled={uploading}
                                    style={{ fontSize: '0.85rem' }}
                                >
                                    {hasResult ? 'Close' : 'Cancel'}
                                </button>
                                {!hasResult && (
                                    <button
                                        className="primary-btn"
                                        onClick={doUpload}
                                        disabled={uploading || !uploadModuleId}
                                        style={{
                                            fontSize: '0.85rem',
                                            display: 'flex', alignItems: 'center', gap: 6,
                                            opacity: (!uploadModuleId && !uploading) ? 0.5 : 1,
                                        }}
                                    >
                                        {uploading ? (
                                            <>
                                                <span style={{
                                                    width: 14, height: 14, display: 'inline-block',
                                                    border: '2px solid rgba(255,255,255,0.3)',
                                                    borderTopColor: '#fff',
                                                    borderRadius: '50%',
                                                    animation: 'spin 0.6s linear infinite',
                                                }} />
                                                Uploading...
                                            </>
                                        ) : (
                                            <>⬆ Upload {total} test case{total !== 1 ? 's' : ''}</>
                                        )}
                                    </button>
                                )}
                            </div>
                        </div>
                    </div>
                );
            })()}

            {modal && (
                <Modal
                    {...modal}
                    onCancel={() => setModal(null)}
                />
            )}

            {showExportModal && (
                <div className="modal-overlay" onClick={() => setShowExportModal(false)}>
                    <div className="modal-content" onClick={e => e.stopPropagation()} style={{ maxWidth: 420 }}>
                        <header className="modal-header">
                            <h2>Export Test Cases</h2>
                        </header>
                        <div className="modal-body" style={{ padding: '20px 24px' }}>
                            <p style={{ marginTop: 0, color: 'var(--text-secondary)' }}>
                                {selectedTestIds.length} test{selectedTestIds.length !== 1 ? 's' : ''} selected
                            </p>
                            <p style={{ fontWeight: 600, marginBottom: 8 }}>Include fields:</p>
                            {[
                                { key: 'description', label: 'Description' },
                                { key: 'steps', label: 'Steps' },
                                { key: 'categories', label: 'Categories' },
                                { key: 'custom_values', label: 'Custom Values' },
                                { key: 'linked_requirements', label: 'Linked Requirements' },
                            ].map(({ key, label }) => (
                                <label key={key} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '6px 0', cursor: 'pointer' }}>
                                    <input
                                        type="checkbox"
                                        checked={exportFields[key]}
                                        onChange={() => setExportFields(prev => ({ ...prev, [key]: !prev[key] }))}
                                        style={{ transform: 'scale(1.1)' }}
                                    />
                                    {label}
                                </label>
                            ))}
                        </div>
                        <footer className="modal-footer" style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, padding: '16px 24px' }}>
                            <button className="action-btn" onClick={() => setShowExportModal(false)}>Cancel</button>
                            <button className="primary-btn" onClick={handleExport} disabled={exporting}>
                                {exporting ? 'Exporting...' : 'Export'}
                            </button>
                        </footer>
                    </div>
                </div>
            )}
        </div>
    );
}
