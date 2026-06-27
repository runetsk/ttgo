import React, { useEffect, useState, useCallback } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { getTestRuns, getCategories, getRunFolders } from '../api';
import CreateRunModal from '../components/CreateRunModal';
import Modal from '../components/Modal';
import ColumnPicker from '../components/ColumnPicker';
import { useColumnPreference } from '../hooks/useColumnPreference';
import { useColumnWidths } from '../hooks/useColumnWidths';
import { useSubscription } from '../hooks/useSubscription';
import { useWebSocket } from '../hooks/useWebSocket';
import { toast } from '../toast';

// Column definitions for the Test Runs grid
const COLUMN_DEFS = [
    { key: 'name',         label: 'Name',         mandatory: true,  defaultVisible: true,  defaultWidth: 200 },
    { key: 'status',       label: 'Status',       mandatory: false, defaultVisible: true,  defaultWidth: 110 },
    { key: 'passed',       label: 'Passed',       mandatory: false, defaultVisible: true,  defaultWidth: 70 },
    { key: 'failed',       label: 'Failed',       mandatory: false, defaultVisible: true,  defaultWidth: 70 },
    { key: 'retried',      label: 'Passed after retry', mandatory: false, defaultVisible: true, defaultWidth: 120 },
    { key: 'defect_types', label: 'Defect Types', mandatory: false, defaultVisible: false, defaultWidth: 160 },
    { key: 'defect_links', label: 'Defect Links', mandatory: false, defaultVisible: true,  defaultWidth: 100 },
    { key: 'skipped',      label: 'Skipped',      mandatory: false, defaultVisible: false, defaultWidth: 70 },
    { key: 'pending',      label: 'Pending',      mandatory: false, defaultVisible: false, defaultWidth: 70 },
    { key: 'total',        label: 'Total',        mandatory: false, defaultVisible: true,  defaultWidth: 60 },
    { key: 'folder',       label: 'Folder',       mandatory: false, defaultVisible: false, defaultWidth: 120 },
    { key: 'category',        label: 'Category',        mandatory: false, defaultVisible: true,  defaultWidth: 120 },
    { key: 'comments',     label: 'Comments',     mandatory: false, defaultVisible: true,  defaultWidth: 80 },
    { key: 'created_at',   label: 'Created At',   mandatory: false, defaultVisible: true,  defaultWidth: 160 },
    { key: 'updated_at',   label: 'Updated At',   mandatory: false, defaultVisible: false, defaultWidth: 160 },
];

export default function TestRunList({ selectedFolderId = null, onRunsLoaded }) {
    const navigate = useNavigate();
    const [runs, setRuns] = useState([]);
    const [categories, setCategories] = useState([]); // For filter + modal
    const [folders, setFolders] = useState([]); // For folder name column
    const [showModal, setShowModal] = useState(false);
    const [modal, setModal] = useState(null);
    const [filterStatus, setFilterStatus] = useState("");
    const [filterCategory, setFilterCategory] = useState("");
    const [sortBy, setSortBy] = useState("created_at");
    const [sortOrder, setSortOrder] = useState("DESC");
    const [page, setPage] = useState(1);
    const [pageSize, setPageSize] = useState(20);
    const [total, setTotal] = useState(0);
    const [showFilters, setShowFilters] = useState(false);

    const [selectedRunIds, setSelectedRunIds] = useState([]);

    // Column visibility
    const [visibleKeys, toggleColumn, resetColumns] = useColumnPreference('test-runs', COLUMN_DEFS);
    const isVisible = (key) => visibleKeys.has(key);

    // Column widths — drag-to-resize + localStorage persistence
    const { columnWidths, startResize, resetWidths, resetColumnWidth, isResizing } = useColumnWidths('test-runs', COLUMN_DEFS);

    // Combined reset: visibility + widths
    const handleResetAll = useCallback(() => {
        resetColumns();
        resetWidths();
    }, [resetColumns, resetWidths]);

    const loadRuns = useCallback((resetSelection = false) => {
        getTestRuns(filterCategory, filterStatus, sortBy, sortOrder, page, pageSize, selectedFolderId)
            .then(data => {
                const loadedRuns = data.runs || [];
                setRuns(loadedRuns);
                setTotal(data.total || 0);
                if (resetSelection) setSelectedRunIds([]);
                if (onRunsLoaded) onRunsLoaded(loadedRuns);
            })
            .catch(() => {
                setRuns([]);
                setTotal(0);
                if (resetSelection) setSelectedRunIds([]);
                if (onRunsLoaded) onRunsLoaded([]);
            });
    }, [filterCategory, filterStatus, sortBy, sortOrder, page, pageSize, selectedFolderId, onRunsLoaded]);

    useEffect(() => {
        getCategories()
            .then(data => setCategories(Array.isArray(data.categories) ? data.categories : []))
            .catch(() => setCategories([]));
        getRunFolders()
            .then(data => setFolders(data.run_folders || []))
            .catch(() => setFolders([]));
    }, []);

    // 018-websocket-realtime: subscribe to run list updates
    const { registerRefresh, unregisterRefresh } = useWebSocket();
    useSubscription('runs:*', useCallback(() => {
        loadRuns();
    }, [loadRuns]), { debounceMs: 500 });

    useEffect(() => {
        registerRefresh('testRunList', loadRuns);
        return () => unregisterRefresh('testRunList');
    }, [loadRuns, registerRefresh, unregisterRefresh]);

    useEffect(() => {
        loadRuns(true); // Reset selection when filters/page/folder change
    }, [loadRuns]);

    // Reset page when filters change (including folder selection)
    useEffect(() => {
        setPage(1);
    }, [filterCategory, filterStatus, sortBy, sortOrder, selectedFolderId]);

    const handleCreateSuccess = () => {
        setShowModal(false);
    };

    const handleBulkDelete = () => {
        const count = selectedRunIds.length;
        setModal({
            type: 'confirm',
            title: 'Delete Test Runs',
            message: `Delete ${count} test run${count !== 1 ? 's' : ''}? This cannot be undone.`,
            confirmText: 'Delete',
            confirmStyle: 'danger',
            onConfirm: async () => {
                try {
                    const { deleteTestRuns } = await import('../api');
                    await deleteTestRuns(selectedRunIds);
                    setModal(null);
                } catch (err) {
                    toast.error('Failed to delete runs');
                    console.error(err);
                }
            }
        });
    };


    const toggleSelectAll = (e) => {
        if (e.target.checked) {
            setSelectedRunIds(runs.map(r => r.id));
        } else {
            setSelectedRunIds([]);
        }
    };

    const toggleSelect = (id, e) => {
        e.stopPropagation();
        setSelectedRunIds(prev =>
            prev.includes(id) ? prev.filter(i => i !== id) : [...prev, id]
        );
    };

    const handleSort = (col) => {
        if (sortBy === col) {
            setSortOrder(prev => prev === 'ASC' ? 'DESC' : 'ASC');
        } else {
            setSortBy(col);
            setSortOrder('ASC');
        }
    };

    // Sort indicator matching TestGrid style
    const getSortIndicator = (col) => {
        if (sortBy !== col) return <span style={{ opacity: 0.3, marginLeft: 4 }}>↕</span>;
        return <span style={{ marginLeft: 4, color: 'var(--accent-indigo)' }}>{sortOrder === 'ASC' ? '↑' : '↓'}</span>;
    };

    const getRunStats = (run) => {
        const passedAfterRetry = run.retried_count || 0;
        return {
            total: run.total_results || 0,
            passed: (run.passed_results || 0) - passedAfterRetry,
            failed: run.failed_results || 0,
            passedAfterRetry,
            skipped: run.skipped_results || 0,
            pending: run.pending_results || 0,
            toInvestigate: run.to_investigate_count || 0,
            productBug: run.product_bug_count || 0,
            automationBug: run.automation_bug_count || 0,
            systemIssue: run.system_issue_count || 0,
        };
    };

    const getFolderName = (run) => {
        if (!run.run_folder_id) return '—';
        const folder = (folders || []).find(f => f.id === run.run_folder_id);
        return folder ? folder.name : '—';
    };

    // HTML5 drag start: set run ID so folder sidebar can receive it
    const handleDragStart = (e, runId) => {
        e.dataTransfer.setData('runId', runId);
        e.dataTransfer.effectAllowed = 'move';
    };

    // Pagination helpers matching TestGrid
    const totalPages = Math.max(1, Math.ceil(total / pageSize));

    const getPageNumbers = (current, tot) => {
        if (tot <= 7) return Array.from({ length: tot }, (_, i) => i + 1);
        if (current <= 4) return [1, 2, 3, 4, 5, '...', tot];
        if (current >= tot - 3) return [1, '...', tot - 4, tot - 3, tot - 2, tot - 1, tot];
        return [1, '...', current - 1, current, current + 1, '...', tot];
    };

    return (
        <div className="test-grid-container" data-testid="test-run-list-container">
            <header className="grid-header" style={{ marginBottom: selectedRunIds.length > 0 ? 16 : 32 }}>
                <div>
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginBottom: 8, display: 'flex', alignItems: 'center', gap: 6, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                        <span>Execution</span>
                        <span style={{ opacity: 0.5 }}>/</span>
                        <span style={{ color: 'var(--accent-indigo)', fontWeight: 600 }}>Test Runs</span>
                    </div>
                    <h2 className="grid-title">Test Runs</h2>
                </div>
                <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
                    <button
                        className={`action-btn ${showFilters ? 'active' : ''}`}
                        onClick={() => setShowFilters(!showFilters)}
                        style={{ padding: '8px 12px', background: showFilters ? 'var(--bg-tertiary)' : 'transparent' }}
                        title="Column Filters"
                    >
                        {showFilters ? 'Hide Filters' : 'Column Filters'}
                    </button>
                    <ColumnPicker
                        columnDefs={COLUMN_DEFS}
                        visibleKeys={visibleKeys}
                        onToggle={toggleColumn}
                        onReset={handleResetAll}
                    />
                    <button className="primary-btn" onClick={() => setShowModal(true)} data-testid="create-test-run-button">+ New Test Run</button>
                </div>
            </header>

            {selectedRunIds.length > 0 && (
                <div className="bulk-action-bar" style={{
                    display: 'flex', alignItems: 'center', gap: 16, padding: '12px 20px',
                    background: 'rgba(99, 102, 241, 0.1)', border: '1px solid var(--accent-indigo)',
                    borderRadius: 8, marginBottom: 24, animation: 'fadeIn 0.2s ease'
                }}>
                    <div style={{ fontWeight: 600, color: 'var(--accent-indigo)' }}>{selectedRunIds.length} items selected</div>
                    <div style={{ height: 20, width: 1, background: 'var(--border-color)' }}></div>
                    <div style={{ display: 'flex', gap: 8 }}>
                        <button
                            className="action-btn"
                            onClick={handleBulkDelete}
                            style={{ color: 'var(--accent-red)' }}
                            data-testid="bulk-delete-runs-button"
                        >
                            🗑️ Delete
                        </button>
                    </div>
                </div>
            )}

            <div className="table-scroll-x">
                <table className="modern-table resizable">
                    <thead>
                        <tr>
                            <th style={{ width: 40 }}>
                                <input
                                    type="checkbox"
                                    onChange={toggleSelectAll}
                                    checked={runs.length > 0 && selectedRunIds.length === runs.length}
                                    data-testid="select-all-runs-checkbox"
                                />
                            </th>
                            {/* name is mandatory — always rendered */}
                            <th className="col-resize-th" style={{ width: columnWidths['name'], cursor: 'pointer', userSelect: 'none' }} onClick={() => handleSort('name')}>
                                Name {getSortIndicator('name')}
                                <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('name', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('name'); }} />
                            </th>
                            {isVisible('status') && (
                                <th className="col-resize-th" style={{ width: columnWidths['status'], cursor: 'pointer', userSelect: 'none' }} onClick={() => handleSort('status')}>
                                    Status {getSortIndicator('status')}
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('status', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('status'); }} />
                                </th>
                            )}
                            {isVisible('passed') && (
                                <th className="col-resize-th" style={{ width: columnWidths['passed'], cursor: 'pointer', userSelect: 'none' }}>
                                    Passed
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('passed', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('passed'); }} />
                                </th>
                            )}
                            {isVisible('failed') && (
                                <th className="col-resize-th" style={{ width: columnWidths['failed'], cursor: 'pointer', userSelect: 'none' }}>
                                    Failed
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('failed', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('failed'); }} />
                                </th>
                            )}
                            {isVisible('defect_types') && (
                                <th className="col-resize-th" style={{ width: columnWidths['defect_types'], cursor: 'pointer', userSelect: 'none' }}>
                                    Defect Types
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('defect_types', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('defect_types'); }} />
                                </th>
                            )}
                            {isVisible('defect_links') && (
                                <th className="col-resize-th" style={{ width: columnWidths['defect_links'] }}>
                                    Defect Links
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('defect_links', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('defect_links'); }} />
                                </th>
                            )}
                            {isVisible('skipped') && (
                                <th className="col-resize-th" style={{ width: columnWidths['skipped'], cursor: 'pointer', userSelect: 'none' }}>
                                    Skipped
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('skipped', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('skipped'); }} />
                                </th>
                            )}
                            {isVisible('pending') && (
                                <th className="col-resize-th" style={{ width: columnWidths['pending'], cursor: 'pointer', userSelect: 'none' }}>
                                    Pending
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('pending', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('pending'); }} />
                                </th>
                            )}
                            {isVisible('total') && (
                                <th className="col-resize-th" style={{ width: columnWidths['total'], cursor: 'pointer', userSelect: 'none' }}>
                                    Total
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('total', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('total'); }} />
                                </th>
                            )}
                            {isVisible('folder') && (
                                <th className="col-resize-th" style={{ width: columnWidths['folder'], cursor: 'pointer', userSelect: 'none' }}>
                                    Folder
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('folder', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('folder'); }} />
                                </th>
                            )}
                            {isVisible('category') && (
                                <th className="col-resize-th" style={{ width: columnWidths['category'], cursor: 'pointer', userSelect: 'none' }}>
                                    Category
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('category', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('category'); }} />
                                </th>
                            )}
                            {isVisible('comments') && (
                                <th className="col-resize-th" style={{ width: columnWidths['comments'] }}>
                                    Comments
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('comments', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('comments'); }} />
                                </th>
                            )}
                            {isVisible('created_at') && (
                                <th style={{ minWidth: 160, cursor: 'pointer', userSelect: 'none' }} onClick={() => handleSort('created_at')}>
                                    Created At {getSortIndicator('created_at')}
                                </th>
                            )}
                            {isVisible('updated_at') && (
                                <th style={{ minWidth: 160, cursor: 'pointer', userSelect: 'none' }} onClick={() => handleSort('updated_at')}>
                                    Updated At {getSortIndicator('updated_at')}
                                </th>
                            )}
                        </tr>
                        {showFilters && (
                            <tr className="filter-row" style={{ background: 'var(--bg-secondary)' }}>
                                {/* Always: checkbox + name */}
                                <th></th>
                                <th></th>
                                {/* Optional columns — filter inputs only for columns that have filters */}
                                {isVisible('status') && (
                                    <th>
                                        <select
                                            className="col-filter-select"
                                            value={filterStatus}
                                            onChange={e => { setFilterStatus(e.target.value); setPage(1); }}
                                            data-testid="filter-status-select"
                                        >
                                            <option value="">All</option>
                                            <option value="PENDING">Pending</option>
                                            <option value="RUNNING">Running</option>
                                            <option value="PASSED">Passed</option>
                                            <option value="FAILED">Failed</option>
                                        </select>
                                    </th>
                                )}
                                {isVisible('passed')       && <th></th>}
                                {isVisible('failed')       && <th></th>}
                                {isVisible('defect_types') && <th></th>}
                                {isVisible('defect_links') && <th></th>}
                                {isVisible('skipped')      && <th></th>}
                                {isVisible('pending')      && <th></th>}
                                {isVisible('total')   && <th></th>}
                                {isVisible('folder')  && <th></th>}
                                {isVisible('category') && (
                                    <th>
                                        <select
                                            className="col-filter-select"
                                            value={filterCategory}
                                            onChange={e => { setFilterCategory(e.target.value); setPage(1); }}
                                            data-testid="filter-category-select"
                                        >
                                            <option value="">All</option>
                                            {(categories || []).map(s => <option key={s.id} value={s.id}>{s.name}</option>)}
                                        </select>
                                    </th>
                                )}
                                {isVisible('comments') && <th></th>}
                                {isVisible('created_at') && <th></th>}
                                {isVisible('updated_at') && <th></th>}
                            </tr>
                        )}
                    </thead>
                    <tbody>
                        {(runs || []).map(run => {
                            const stats = getRunStats(run);
                            return (
                                <tr
                                    key={run.id}
                                    className={`hover-row ${selectedRunIds.includes(run.id) ? 'selected-row' : ''}`}
                                    onClick={() => navigate(`/runs/run/${run.id}`)}
                                    style={{ cursor: 'pointer' }}
                                    draggable
                                    onDragStart={(e) => handleDragStart(e, run.id)}
                                    data-testid={`run-row-${run.id}`}
                                >
                                    <td onClick={(e) => e.stopPropagation()}>
                                        <input
                                            type="checkbox"
                                            checked={selectedRunIds.includes(run.id)}
                                            onChange={(e) => toggleSelect(run.id, e)}
                                            data-testid={`select-run-checkbox-${run.id}`}
                                        />
                                    </td>
                                    {/* name is mandatory — always rendered */}
                                    <td>
                                        <Link to={`/runs/run/${run.id}`} style={{ fontWeight: 500, color: 'var(--text-primary)', textDecoration: 'none' }}>
                                            {run.name}
                                        </Link>
                                    </td>
                                    {isVisible('status') && (
                                        <td><span className={`status-badge ${run.status.toLowerCase()}`}>{run.status}</span></td>
                                    )}
                                    {isVisible('passed') && (
                                        <td><span data-testid={`run-passed-${run.id}`} style={{ color: 'var(--accent-green)', fontWeight: 600 }}>{stats.passed}</span></td>
                                    )}
                                    {isVisible('failed') && (
                                        <td><span data-testid={`run-failed-${run.id}`} style={{ color: 'var(--accent-red)', fontWeight: 600 }}>{stats.failed}</span></td>
                                    )}
                                    {isVisible('retried') && (
                                        <td><span data-testid={`run-retried-${run.id}`} style={{ color: '#856404', fontWeight: 600 }}>{stats.passedAfterRetry}</span></td>
                                    )}
                                    {isVisible('defect_types') && (
                                        <td>
                                            {stats.failed > 0 ? (
                                                <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
                                                    {stats.toInvestigate > 0 && (
                                                        <span style={{ fontSize: '0.7rem', fontWeight: 600, padding: '1px 6px', borderRadius: 10, background: 'rgba(245,158,11,0.12)', color: '#d97706' }} title="To Investigate">
                                                            🔍 {stats.toInvestigate}
                                                        </span>
                                                    )}
                                                    {stats.productBug > 0 && (
                                                        <span style={{ fontSize: '0.7rem', fontWeight: 600, padding: '1px 6px', borderRadius: 10, background: 'rgba(239,68,68,0.12)', color: '#dc2626' }} title="Product Bug">
                                                            🐞 {stats.productBug}
                                                        </span>
                                                    )}
                                                    {stats.automationBug > 0 && (
                                                        <span style={{ fontSize: '0.7rem', fontWeight: 600, padding: '1px 6px', borderRadius: 10, background: 'rgba(139,92,246,0.12)', color: '#7c3aed' }} title="Automation Bug">
                                                            🤖 {stats.automationBug}
                                                        </span>
                                                    )}
                                                    {stats.systemIssue > 0 && (
                                                        <span style={{ fontSize: '0.7rem', fontWeight: 600, padding: '1px 6px', borderRadius: 10, background: 'rgba(100,116,139,0.12)', color: '#64748b' }} title="System Issue">
                                                            ⚙️ {stats.systemIssue}
                                                        </span>
                                                    )}
                                                </div>
                                            ) : (
                                                <span style={{ color: 'var(--text-secondary)', fontSize: '0.75rem' }}>—</span>
                                            )}
                                        </td>
                                    )}
                                    {isVisible('defect_links') && (() => {
                                        const open = run.open_defect_link_count || 0;
                                        const closed = run.closed_defect_link_count || 0;
                                        const total = open + closed;
                                        return (
                                            <td>
                                                {total > 0 ? (
                                                    <span style={{ display: 'flex', gap: 6, alignItems: 'center', fontSize: '0.8rem' }}>
                                                        {open > 0 && <span style={{ color: 'var(--accent-red)', fontWeight: 600 }}>🐞 {open}</span>}
                                                        {closed > 0 && <span style={{ color: 'var(--accent-green)', fontWeight: 600 }}>✅ {closed}</span>}
                                                    </span>
                                                ) : (
                                                    <span style={{ color: 'var(--text-secondary)', fontSize: '0.8rem', opacity: 0.4 }}>—</span>
                                                )}
                                            </td>
                                        );
                                    })()}
                                    {isVisible('skipped') && (
                                        <td><span data-testid={`run-skipped-${run.id}`} style={{ color: 'var(--text-secondary)', fontWeight: 600 }}>{stats.skipped}</span></td>
                                    )}
                                    {isVisible('pending') && (
                                        <td><span data-testid={`run-pending-${run.id}`} style={{ color: 'var(--warning-color)', fontWeight: 600 }}>{stats.pending}</span></td>
                                    )}
                                    {isVisible('total') && (
                                        <td><span data-testid={`run-total-${run.id}`} style={{ color: 'var(--text-primary)', fontWeight: 600 }}>{stats.total}</span></td>
                                    )}
                                    {isVisible('folder') && (
                                        <td>
                                            <span data-testid={`run-folder-${run.id}`} style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
                                                {getFolderName(run)}
                                            </span>
                                        </td>
                                    )}
                                    {isVisible('category') && (
                                        <td>
                                            <span className="category-tag" style={{ fontSize: '0.75rem', padding: '2px 8px' }}>
                                                {(categories || []).find(s => s.id === run.category_id)?.name || run.category_id}
                                            </span>
                                        </td>
                                    )}
                                    {isVisible('comments') && (
                                        <td>
                                            {run.comment_count > 0 ? (
                                                <span style={{ fontSize: '0.8rem', color: 'var(--accent-indigo)', fontWeight: 600 }}>
                                                    💬 {run.comment_count}
                                                </span>
                                            ) : (
                                                <span style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', opacity: 0.4 }}>—</span>
                                            )}
                                        </td>
                                    )}
                                    {isVisible('created_at') && (
                                        <td style={{ color: 'var(--text-secondary)', fontSize: '0.85rem' }}>{new Date(run.created_at).toLocaleString()}</td>
                                    )}
                                    {isVisible('updated_at') && (
                                        <td style={{ color: 'var(--text-secondary)', fontSize: '0.85rem' }}>{new Date(run.updated_at).toLocaleString()}</td>
                                    )}
                                </tr>
                            );
                        })}
                        {(runs || []).length === 0 && (
                            <tr>
                                <td colSpan={2 + COLUMN_DEFS.filter(c => !c.mandatory && isVisible(c.key)).length} style={{ textAlign: 'center', padding: '60px 0', color: 'var(--text-secondary)' }}>
                                    <div style={{ fontSize: '2rem', marginBottom: 12 }}>🔍</div>
                                    <p>No runs found matching your criteria</p>
                                </td>
                            </tr>
                        )}
                    </tbody>
                </table>
            </div>

            {total > 0 && (
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginTop: 16, padding: '12px 0', flexWrap: 'wrap', gap: 8 }}>
                    {/* Left: count */}
                    <div style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', minWidth: 160 }}>
                        Showing {(page - 1) * pageSize + 1}–{Math.min(page * pageSize, total)} of {total}
                    </div>

                    {/* Center: page buttons */}
                    {totalPages > 1 && (
                        <div style={{ display: 'flex', gap: 4, alignItems: 'center' }}>
                            <button
                                className="action-btn"
                                onClick={() => setPage(p => Math.max(1, p - 1))}
                                disabled={page === 1}
                                style={{ padding: '4px 10px', opacity: page === 1 ? 0.4 : 1 }}
                                data-testid="prev-page"
                            >
                                ‹ Prev
                            </button>

                            {getPageNumbers(page, totalPages).map((pg, i) =>
                                pg === '...' ? (
                                    <span key={`ellipsis-${i}`} style={{ padding: '0 4px', color: 'var(--text-secondary)' }}>…</span>
                                ) : (
                                    <button
                                        key={pg}
                                        className={`action-btn ${page === pg ? 'primary-btn' : ''}`}
                                        onClick={() => setPage(pg)}
                                        style={{ minWidth: 34, padding: '4px 8px' }}
                                    >
                                        {pg}
                                    </button>
                                )
                            )}

                            <button
                                className="action-btn"
                                onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                                disabled={page >= totalPages}
                                style={{ padding: '4px 10px', opacity: page >= totalPages ? 0.4 : 1 }}
                                data-testid="next-page"
                            >
                                Next ›
                            </button>
                        </div>
                    )}

                    {/* Right: page size */}
                    <div style={{ display: 'flex', gap: 8, alignItems: 'center', fontSize: '0.875rem', color: 'var(--text-secondary)' }}>
                        <span>Per page:</span>
                        <select
                            className="modern-select"
                            value={pageSize}
                            onChange={(e) => { setPageSize(Number(e.target.value)); setPage(1); }}
                            style={{ padding: '4px 8px' }}
                            data-testid="page-size-selector"
                        >
                            <option value={10}>10</option>
                            <option value={20}>20</option>
                            <option value={50}>50</option>
                            <option value={100}>100</option>
                        </select>
                    </div>
                </div>
            )}

            {showModal && (
                <CreateRunModal
                    categories={categories}
                    onClose={() => setShowModal(false)}
                    onSuccess={handleCreateSuccess}
                    defaultFolderId={selectedFolderId && selectedFolderId !== 'uncategorised' ? selectedFolderId : null}
                />
            )}

            {modal && (
                <Modal
                    {...modal}
                    onCancel={() => setModal(null)}
                />
            )}
        </div>
    );
}
