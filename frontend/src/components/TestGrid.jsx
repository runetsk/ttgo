import React, { useEffect, useState, useCallback, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { getTests, createTest, getCategories, assignCategory, deleteTest, deleteTests, updateFolder, exportTests } from '../api';
import { toast } from '../toast';
import Modal from './Modal';
import ColumnPicker from './ColumnPicker';
import { useColumnPreference } from '../hooks/useColumnPreference';
import { useColumnWidths } from '../hooks/useColumnWidths';
import { stripHtml } from '../utils/htmlUtils';
import DateRangeFilter from './filters/DateRangeFilter';
import CategoryFilter from './filters/CategoryFilter';
import { inDateRange } from '../utils/dateFilter';
import { activeColumns } from '../utils/columnFeatures';

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
    { key: 'created_at',            label: 'Created',        mandatory: false, defaultVisible: true,  defaultWidth: 120 },
    { key: 'updated_at',            label: 'Updated',        mandatory: false, defaultVisible: true,  defaultWidth: 120 },
];

export default function TestGrid({ selectedFolders, selectedTestId }) {
    const [folderEditName, setFolderEditName] = useState(null); // null = not editing, string = draft value
    const folderNameInputRef = useRef(null);
    const [tests, setTests] = useState([]);
    const [categories, setCategories] = useState([]);
    const [selectedCategoryId, setSelectedCategoryId] = useState("");
    const [modal, setModal] = useState(null);
    const [filterText, setFilterText] = useState("");
    const [columnFilters, setColumnFilters] = useState({
        id: "", name: "", description: "",
        categories: [],                 // selected category IDs (match ANY)
        steps_count: "", reverification_flagged: "", open_defects: "",
        linked_requirements: "",
        created_at: { from: null, to: null },
        updated_at: { from: null, to: null },
    });
    const [showColumnFilters, setShowColumnFilters] = useState(false);
    const [selectedTestIds, setSelectedTestIds] = useState([]);
    const [lastSelectedTestId, setLastSelectedTestId] = useState(null);
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

    // Column widths — drag-to-resize + localStorage persistence
    const { columnWidths, startResize, resetWidths, resetColumnWidth, isResizing } = useColumnWidths('test-cases', COLUMN_DEFS);

    const featureColumnDefs = activeColumns(COLUMN_DEFS, {});
    const isVisible = (key) => visibleKeys.has(key) && featureColumnDefs.some(c => c.key === key);

    // T007: Wrapper that clears the column's filter when toggling it off
    const handleToggleColumn = useCallback((key) => {
        toggleColumn(key);
        const emptyVal = key === 'categories' ? [] : (key === 'created_at' || key === 'updated_at') ? { from: null, to: null } : '';
        setColumnFilters(prev => ({ ...prev, [key]: emptyVal }));
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
        setColumnFilters({ id: "", name: "", description: "", categories: [], steps_count: "", reverification_flagged: "", open_defects: "", linked_requirements: "", created_at: { from: null, to: null }, updated_at: { from: null, to: null } });
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
        const stepsFilter = columnFilters.steps_count.toLowerCase();
        const defectsFilter = columnFilters.open_defects.toLowerCase();
        const reqsFilter = columnFilters.linked_requirements.toLowerCase();

        const idMatch = idFilter === "" || t.id.toLowerCase().includes(idFilter);
        const nameMatch = nameFilter === "" || t.name.toLowerCase().includes(nameFilter);
        const descMatch = descFilter === "" || stripHtml(t.description).toLowerCase().includes(descFilter);
        const categoryMatch = columnFilters.categories.length === 0 || t.categories?.some(s => columnFilters.categories.includes(s.id));
        const stepsMatch = stepsFilter === "" || String(t.steps_count ?? t.steps?.length ?? 0).includes(stepsFilter);
        const reverifMatch = columnFilters.reverification_flagged === "" || (t.reverification_flagged ? 'yes' : 'no') === columnFilters.reverification_flagged;
        const defectsMatch = defectsFilter === "" || String(t.open_defect_count || 0).includes(defectsFilter);
        const reqsMatch = reqsFilter === "" || t.linked_requirements?.some(r => r.identifier.toLowerCase().includes(reqsFilter));
        const createdMatch = inDateRange(t.created_at, columnFilters.created_at);
        const updatedMatch = inDateRange(t.updated_at, columnFilters.updated_at);

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
                        columnDefs={featureColumnDefs}
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
                                        <CategoryFilter
                                            categories={categories}
                                            value={columnFilters.categories}
                                            onChange={(ids) => handleColumnFilterChange('categories', ids)}
                                            testId="filter-categories"
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
                                        <select
                                            className="col-filter-select"
                                            data-testid="filter-reverification"
                                            value={columnFilters.reverification_flagged}
                                            onChange={(e) => handleColumnFilterChange('reverification_flagged', e.target.value)}
                                        >
                                            <option value="">All</option>
                                            <option value="yes">Yes</option>
                                            <option value="no">No</option>
                                        </select>
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
                                {isVisible('created_at') && (
                                    <th>
                                        <DateRangeFilter
                                            value={columnFilters.created_at}
                                            onChange={(v) => handleColumnFilterChange('created_at', v)}
                                            testId="filter-created_at"
                                        />
                                    </th>
                                )}
                                {isVisible('updated_at') && (
                                    <th>
                                        <DateRangeFilter
                                            value={columnFilters.updated_at}
                                            onChange={(v) => handleColumnFilterChange('updated_at', v)}
                                            testId="filter-updated_at"
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
