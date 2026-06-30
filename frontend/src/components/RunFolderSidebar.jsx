import React, { useState, useEffect, useCallback, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import {
    getRunFolderTree,
    createRunFolder,
    updateRunFolder,
    deleteRunFolder,
    moveRunFolder,
    assignRunToFolder,
    getTestRuns,
    copyTestRun,
    copyRunFolder,
    deleteTestRun,
    updateTestRun,
} from '../api';
import Modal from './Modal';
import { ChevronSvg, FolderSvg, AllRunsSvg } from './FolderIcons';
import { toast } from '../toast';
import { useSubscription } from '../hooks/useSubscription';
import { useWebSocket } from '../hooks/useWebSocket';

const STORAGE_KEY = 'runFolderSidebarCollapsed';
const STORAGE_WIDTH_KEY = 'runFolderSidebarWidth';
const STORAGE_EXPANDED_KEY = 'runFolderSidebarExpandedIds';
const STORAGE_UNCAT_KEY = 'runFolderSidebarUncatExpanded';

const STATUS_COLORS = {
    PENDING: '#f59e0b',
    RUNNING: '#3b82f6',
    PASS: '#10b981',
    PASSED: '#10b981',
    FAIL: '#ef4444',
    FAILED: '#ef4444',
    SKIP: '#9ca3af',
    ERROR: '#ef4444',
};

// ── Context menu ──────────────────────────────────────────────────────────────
function ContextMenu({ x, y, items, onClose }) {
    const menuRef = useRef(null);

    useEffect(() => {
        const onKey = (e) => { if (e.key === 'Escape') onClose(); };
        const onDown = (e) => { if (menuRef.current && !menuRef.current.contains(e.target)) onClose(); };
        window.addEventListener('keydown', onKey);
        window.addEventListener('mousedown', onDown);
        return () => {
            window.removeEventListener('keydown', onKey);
            window.removeEventListener('mousedown', onDown);
        };
    }, [onClose]);

    return (
        <div
            ref={menuRef}
            style={{
                position: 'fixed',
                top: y,
                left: x,
                zIndex: 9000,
                background: 'var(--bg-secondary)',
                border: '1px solid var(--border-color)',
                borderRadius: 6,
                boxShadow: '0 8px 24px rgba(0,0,0,0.4)',
                minWidth: 180,
                padding: '4px 0',
            }}
        >
            {items.map((item, i) =>
                item === 'separator' ? (
                    <div key={i} style={{ height: 1, background: 'var(--border-color)', margin: '4px 0' }} />
                ) : (
                    <button
                        key={i}
                        data-testid={item.testId}
                        onClick={() => { onClose(); item.onClick(); }}
                        style={{
                            display: 'block',
                            width: '100%',
                            padding: '7px 14px',
                            background: 'none',
                            border: 'none',
                            cursor: 'pointer',
                            textAlign: 'left',
                            fontSize: '0.875rem',
                            color: item.danger ? 'var(--accent-red, #ef4444)' : 'var(--text-primary)',
                        }}
                        onMouseEnter={e => e.currentTarget.style.background = 'var(--bg-tertiary)'}
                        onMouseLeave={e => e.currentTarget.style.background = 'none'}
                    >
                        {item.label}
                    </button>
                )
            )}
        </div>
    );
}

// ── Run item inside an expanded folder ───────────────────────────────────────
function RunItem({ run, onRunClick, onRunContextMenu, isSelected }) {
    const handleDragStart = (e) => {
        e.stopPropagation();
        e.dataTransfer.setData('runId', run.id);
        e.dataTransfer.effectAllowed = 'move';
    };

    return (
        <div
            className={`run-sidebar-run-item${isSelected ? ' selected' : ''}`}
            onClick={(e) => { e.stopPropagation(); onRunClick(run.id); }}
            onContextMenu={(e) => { if (onRunContextMenu) { e.preventDefault(); e.stopPropagation(); onRunContextMenu(e, run); } }}
            draggable
            onDragStart={handleDragStart}
            title={run.name}
        >
            <span
                style={{
                    width: 7, height: 7, borderRadius: '50%',
                    background: STATUS_COLORS[run.status] || 'var(--text-secondary)',
                    flexShrink: 0,
                }}
            />
            <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: '0.8rem' }}>
                {run.name}
            </span>
        </div>
    );
}

// ── Recursive folder tree node ────────────────────────────────────────────────
function RunFolderNode({
    folder, depth, selectedFolderId, selectedRunId, expandedFolderIds, onToggleExpand,
    onSelectFolder, onCreateSubfolder, onRunClick,
    onContextMenu, onRunContextMenu, onRunDrop, onFolderDrop,
}) {
    const [isDragOver, setIsDragOver] = useState(false);
    const isSelected = selectedFolderId === folder.id;
    const isExpanded = expandedFolderIds.has(folder.id);
    const hasChildren = (folder.sub_folders && folder.sub_folders.length > 0) ||
                        (folder.test_runs && folder.test_runs.length > 0);

    const handleDragStart = (e) => {
        e.stopPropagation();
        e.dataTransfer.setData('runFolderId', folder.id);
        e.dataTransfer.effectAllowed = 'move';
    };

    const handleDragOver = (e) => {
        e.preventDefault();
        e.stopPropagation();
        const hasRunId = e.dataTransfer.types.includes('runid');
        const hasFolderId = e.dataTransfer.types.includes('runfolderid');
        if (hasRunId || hasFolderId) {
            e.dataTransfer.dropEffect = 'move';
            setIsDragOver(true);
        }
    };

    const handleDragLeave = (e) => {
        e.stopPropagation();
        setIsDragOver(false);
    };

    const handleDrop = (e) => {
        e.preventDefault();
        e.stopPropagation();
        setIsDragOver(false);

        const runId = e.dataTransfer.getData('runId');
        const folderId = e.dataTransfer.getData('runFolderId');

        if (runId) {
            onRunDrop(runId, folder.id);
        } else if (folderId && folderId !== folder.id) {
            onFolderDrop(folderId, folder.id);
        }
    };

    return (
        <div className="run-folder-tree-node">
            <div
                className={`run-folder-item ${isSelected ? 'selected' : ''} ${isDragOver ? 'drag-over' : ''}`}
                style={{ paddingLeft: 8 + depth * 14 }}
                onClick={() => onSelectFolder(folder.id)}
                onContextMenu={(e) => onContextMenu(e, folder)}
                draggable
                onDragStart={handleDragStart}
                onDragOver={handleDragOver}
                onDragLeave={handleDragLeave}
                onDrop={handleDrop}
                data-testid={`run-folder-item-${folder.id}`}
                title={folder.name}
            >
                <span
                    className={`expand-toggle ${hasChildren ? 'visible' : ''} ${isExpanded && hasChildren ? 'expanded' : ''}`}
                    onClick={(e) => { e.stopPropagation(); onToggleExpand(folder.id); }}
                    title={isExpanded ? 'Collapse' : 'Expand'}
                >
                    <ChevronSvg />
                </span>
                <span className="folder-icon">
                    <FolderSvg open={isExpanded && hasChildren} />
                </span>
                <span className="run-folder-name">{folder.name}</span>
                <div className="folder-actions" onClick={e => e.stopPropagation()}>
                    <button
                        className="folder-actions-btn"
                        title="New subfolder"
                        onClick={() => onCreateSubfolder(folder)}
                        data-testid={`add-subfolder-${folder.id}`}
                    >+</button>
                    <button
                        className="folder-actions-btn"
                        title="More actions"
                        onClick={(e) => onContextMenu(e, folder)}
                        data-testid={`folder-menu-${folder.id}`}
                    >⋮</button>
                </div>
            </div>

            {isExpanded && (
                <div className="run-sidebar-children">
                    {/* Subfolders */}
                    {(folder.sub_folders || []).map(sub => (
                        <RunFolderNode
                            key={sub.id}
                            folder={sub}
                            depth={depth + 1}
                            selectedFolderId={selectedFolderId}
                            selectedRunId={selectedRunId}
                            expandedFolderIds={expandedFolderIds}
                            onToggleExpand={onToggleExpand}
                            onSelectFolder={onSelectFolder}
                            onCreateSubfolder={onCreateSubfolder}
                            onRunClick={onRunClick}
                            onContextMenu={onContextMenu}
                            onRunContextMenu={onRunContextMenu}
                            onRunDrop={onRunDrop}
                            onFolderDrop={onFolderDrop}
                        />
                    ))}
                    {/* Direct test runs */}
                    {(folder.test_runs || []).map(run => (
                        <div key={run.id} style={{ paddingLeft: (depth + 1) * 14 + 8 }}>
                            <RunItem run={run} onRunClick={onRunClick} onRunContextMenu={onRunContextMenu} isSelected={selectedRunId === run.id} />
                        </div>
                    ))}
                    {!hasChildren && (
                        <div className="run-sidebar-message" style={{ paddingLeft: (depth + 1) * 14 + 8 }}>
                            Empty
                        </div>
                    )}
                </div>
            )}
        </div>
    );
}

// ── Collect all folder IDs from a tree (for "select all descendants" on delete) ──
function collectFolderIds(folder) {
    const ids = [folder.id];
    (folder.sub_folders || []).forEach(sub => ids.push(...collectFolderIds(sub)));
    return ids;
}

// ── RunFolderSidebar ─────────────────────────────────────────────────────────
export default function RunFolderSidebar({
    selectedFolderId,
    selectedRunId,
    onSelectFolder,
    onRunDropped,
}) {
    const navigate = useNavigate();
    const [folderTree, setFolderTree] = useState([]);
    const [collapsed, setCollapsed] = useState(
        () => localStorage.getItem(STORAGE_KEY) === 'true'
    );

    // Resize state
    const [width, setWidth] = useState(() => parseInt(localStorage.getItem(STORAGE_WIDTH_KEY)) || 220);
    const isResizing = useRef(false);

    // Zoom state
    const [zoom, setZoom] = useState(() => parseFloat(localStorage.getItem('runFolderSidebarZoom')) || 1);
    const handleZoom = (delta) => setZoom(prev => {
        const next = Math.max(0.8, Math.min(1.5, prev + delta));
        localStorage.setItem('runFolderSidebarZoom', next);
        return next;
    });

    const startResizing = useCallback(() => {
        isResizing.current = true;
        document.body.style.cursor = 'col-resize';
        document.body.style.userSelect = 'none';
    }, []);

    const stopResizing = useCallback(() => {
        if (!isResizing.current) return;
        isResizing.current = false;
        document.body.style.cursor = 'default';
        document.body.style.userSelect = 'auto';
        localStorage.setItem(STORAGE_WIDTH_KEY, width);
    }, [width]);

    const resize = useCallback((e) => {
        if (isResizing.current) setWidth(Math.max(180, Math.min(600, e.clientX)));
    }, []);

    useEffect(() => {
        window.addEventListener('mousemove', resize);
        window.addEventListener('mouseup', stopResizing);
        return () => {
            window.removeEventListener('mousemove', resize);
            window.removeEventListener('mouseup', stopResizing);
        };
    }, [resize, stopResizing]);

    const [modal, setModal] = useState(null);
    const [createError, setCreateError] = useState('');
    const [renameError, setRenameError] = useState('');

    // ── Context menu ─────────────────────────────────────────────────────────
    const [contextMenu, setContextMenu] = useState(null);
    const closeContextMenu = useCallback(() => setContextMenu(null), []);

    // ── Expand/collapse ───────────────────────────────────────────────────────
    const [expandedFolderIds, setExpandedFolderIds] = useState(() => {
        try { return new Set(JSON.parse(localStorage.getItem(STORAGE_EXPANDED_KEY)) || []); }
        catch { return new Set(); }
    });

    const [uncatExpanded, setUncatExpanded] = useState(
        () => localStorage.getItem(STORAGE_UNCAT_KEY) === 'true'
    );
    const [uncatRuns, setUncatRuns] = useState({ runs: [], loading: false });

    // ── Load folder tree ──────────────────────────────────────────────────────
    const loadFolderTree = useCallback(() => {
        getRunFolderTree()
            .then(data => setFolderTree(data.run_folders || []))
            .catch(() => setFolderTree([]));
    }, []);

    useEffect(() => { loadFolderTree(); }, [loadFolderTree]);

    // 018-websocket-realtime: subscribe to folder and run updates
    const { registerRefresh, unregisterRefresh } = useWebSocket();
    useSubscription('folders:*', useCallback(() => {
        loadFolderTree();
    }, [loadFolderTree]));
    // Refresh sidebar when run status changes so status dots stay current
    useSubscription('runs:*', useCallback(() => {
        loadFolderTree();
    }, [loadFolderTree]), { debounceMs: 500 });

    useEffect(() => {
        registerRefresh('folderTree', loadFolderTree);
        return () => unregisterRefresh('folderTree');
    }, [loadFolderTree, registerRefresh, unregisterRefresh]);

    const loadUncatRuns = useCallback(() => {
        setUncatRuns({ runs: [], loading: true });
        getTestRuns(null, null, 'created_at', 'DESC', 1, 100, 'uncategorised')
            .then(data => setUncatRuns({ runs: data.runs || [], loading: false }))
            .catch(() => setUncatRuns({ runs: [], loading: false }));
    }, []);

    const handleToggleExpand = useCallback((folderId) => {
        setExpandedFolderIds(prev => {
            const next = new Set(prev);
            if (next.has(folderId)) {
                next.delete(folderId);
            } else {
                next.add(folderId);
            }
            localStorage.setItem(STORAGE_EXPANDED_KEY, JSON.stringify([...next]));
            return next;
        });
    }, []);

    const handleToggleUncat = () => {
        if (!uncatExpanded) loadUncatRuns();
        setUncatExpanded(prev => {
            const next = !prev;
            localStorage.setItem(STORAGE_UNCAT_KEY, String(next));
            return next;
        });
    };

    useEffect(() => {
        if (uncatExpanded) loadUncatRuns();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const handleRunClick = useCallback((runId) => {
        navigate(`/runs/run/${runId}`);
    }, [navigate]);

    const toggleCollapsed = () => {
        setCollapsed(prev => {
            const next = !prev;
            localStorage.setItem(STORAGE_KEY, String(next));
            return next;
        });
    };

    // ── Create (root or subfolder) ────────────────────────────────────────────
    const handleCreateConfirm = (name) => {
        if (!name || !name.trim()) { setCreateError('Name cannot be empty'); return; }
        const parentId = modal.parentId || null;
        createRunFolder(name.trim(), parentId)
            .then(() => {
                setModal(null);
                setCreateError('');
                loadFolderTree();
                // Auto-expand parent if creating subfolder
                if (parentId) {
                    setExpandedFolderIds(prev => {
                        const next = new Set(prev);
                        next.add(parentId);
                        localStorage.setItem(STORAGE_EXPANDED_KEY, JSON.stringify([...next]));
                        return next;
                    });
                }
            })
            .catch(err => setCreateError(err?.response?.data?.error || 'Failed to create folder'));
    };

    // ── Rename ───────────────────────────────────────────────────────────────
    const handleRenameConfirm = (name) => {
        if (!name || !name.trim()) { setRenameError('Name cannot be empty'); return; }
        updateRunFolder(modal.folder.id, name.trim())
            .then(() => { setModal(null); setRenameError(''); loadFolderTree(); })
            .catch(err => setRenameError(err?.response?.data?.error || 'Failed to rename folder'));
    };

    // ── Delete ────────────────────────────────────────────────────────────────
    const handleDeleteConfirm = () => {
        const folder = modal.folder;
        deleteRunFolder(folder.id)
            .then(() => {
                setModal(null);
                // If selected folder was deleted (or is a descendant), clear selection
                const deletedIds = collectFolderIds(folder);
                if (deletedIds.includes(selectedFolderId)) onSelectFolder(null);
                setExpandedFolderIds(prev => {
                    const next = new Set(prev);
                    deletedIds.forEach(id => next.delete(id));
                    return next;
                });
                loadFolderTree();
                if (onRunDropped) onRunDropped();
            })
            .catch(() => setModal(null));
    };

    // ── Drop run onto folder ──────────────────────────────────────────────────
    const handleRunDrop = useCallback((runId, folderId) => {
        assignRunToFolder(runId, folderId)
            .then(() => {
                if (onRunDropped) onRunDropped();
                loadFolderTree();
            })
            .catch(err => console.error('Failed to assign run to folder:', err));
    }, [onRunDropped, loadFolderTree]);

    // ── Drop folder onto folder (move) ────────────────────────────────────────
    const handleFolderDrop = useCallback((folderId, newParentId) => {
        moveRunFolder(folderId, newParentId)
            .then(() => loadFolderTree())
            .catch(err => {
                const msg = err?.response?.data?.error || 'Failed to move folder';
                console.error(msg);
                // Show error briefly if it's a circular reference
                if (msg.includes('circular')) {
                    toast.error(msg);
                }
            });
    }, [loadFolderTree]);

    // ── Drop handlers for All Runs / Uncategorised ────────────────────────────
    const [dragOverAll, setDragOverAll] = useState(false);
    const [dragOverUncat, setDragOverUncat] = useState(false);

    const makeDropHandlers = (setter, onDropCallback) => ({
        onDragOver: (e) => {
            const hasRunId = e.dataTransfer.types.includes('runid');
            const hasFolderId = e.dataTransfer.types.includes('runfolderid');
            if (hasRunId || hasFolderId) {
                e.preventDefault();
                e.dataTransfer.dropEffect = 'move';
                setter(true);
            }
        },
        onDragLeave: () => setter(false),
        onDrop: (e) => {
            e.preventDefault();
            setter(false);
            const runId = e.dataTransfer.getData('runId');
            const folderId = e.dataTransfer.getData('runFolderId');
            if (runId) onDropCallback(runId, 'run');
            else if (folderId) onDropCallback(folderId, 'folder');
        },
    });

    const allRunsDropHandlers = makeDropHandlers(setDragOverAll, (id, type) => {
        if (type === 'run') {
            assignRunToFolder(id, null)
                .then(() => { if (onRunDropped) onRunDropped(); if (uncatExpanded) loadUncatRuns(); loadFolderTree(); })
                .catch(err => console.error('Failed to unassign run:', err));
        } else if (type === 'folder') {
            // Move folder to root
            moveRunFolder(id, null)
                .then(() => loadFolderTree())
                .catch(err => console.error('Failed to move folder to root:', err));
        }
    });

    const uncatDropHandlers = makeDropHandlers(setDragOverUncat, (id, type) => {
        if (type === 'run') {
            assignRunToFolder(id, null)
                .then(() => { if (onRunDropped) onRunDropped(); if (uncatExpanded) loadUncatRuns(); loadFolderTree(); })
                .catch(err => console.error('Failed to unassign run:', err));
        } else if (type === 'folder') {
            moveRunFolder(id, null)
                .then(() => loadFolderTree())
                .catch(err => console.error('Failed to move folder to root:', err));
        }
    });

    // Always show the Uncategorised section — its visibility shouldn't depend
    // on the currently-filtered run list (which changes when a folder is selected).
    const hasUncategorised = true;

    // ── Context menu items ────────────────────────────────────────────────────
    const handleContextMenu = useCallback((e, folder) => {
        e.preventDefault();
        e.stopPropagation();
        setContextMenu({ x: e.clientX, y: e.clientY, folder, targetType: 'folder' });
    }, []);

    const handleRunContextMenu = useCallback((e, run) => {
        e.preventDefault();
        e.stopPropagation();
        setContextMenu({ x: e.clientX, y: e.clientY, run, targetType: 'run' });
    }, []);

    // ── Copy handlers ────────────────────────────────────────────────────────
    const handleCopyRun = useCallback((run) => {
        copyTestRun(run.id, `Copy of ${run.name}`, run.run_folder_id || null)
            .then(() => {
                loadFolderTree();
                if (onRunDropped) onRunDropped();
            })
            .catch(err => console.error('Failed to copy run:', err));
    }, [loadFolderTree, onRunDropped]);

    const handleRenameRun = useCallback((run) => {
        const newName = window.prompt('Rename run:', run.name);
        if (newName && newName !== run.name) {
            updateTestRun(run.id, newName)
                .then(() => {
                    loadFolderTree();
                    if (onRunDropped) onRunDropped();
                })
                .catch(err => console.error('Failed to rename run:', err));
        }
    }, [loadFolderTree, onRunDropped]);

    const handleDeleteRun = useCallback((run) => {
        if (window.confirm(`Delete run "${run.name}"? This cannot be undone.`)) {
            deleteTestRun(run.id)
                .then(() => {
                    loadFolderTree();
                    if (onRunDropped) onRunDropped();
                })
                .catch(err => console.error('Failed to delete run:', err));
        }
    }, [loadFolderTree, onRunDropped]);

    const handleCopyFolder = useCallback((folder) => {
        copyRunFolder(folder.id, `Copy of ${folder.name}`)
            .then(() => loadFolderTree())
            .catch(err => console.error('Failed to copy folder:', err));
    }, [loadFolderTree]);

    const contextMenuItems = contextMenu ? (
        contextMenu.targetType === 'run' ? [
            {
                label: 'Rename',
                onClick: () => handleRenameRun(contextMenu.run),
            },
            {
                label: '📋 Copy',
                onClick: () => handleCopyRun(contextMenu.run),
            },
            'separator',
            {
                label: 'Delete',
                danger: true,
                onClick: () => handleDeleteRun(contextMenu.run),
            },
        ] : [
            {
                label: 'New Subfolder',
                onClick: () => { setCreateError(''); setModal({ type: 'create', parentId: contextMenu.folder.id }); },
            },
            {
                label: 'Rename',
                testId: `rename-folder-${contextMenu.folder.id}`,
                onClick: () => { setRenameError(''); setModal({ type: 'rename', folder: contextMenu.folder }); },
            },
            'separator',
            {
                label: '📋 Copy Folder',
                onClick: () => handleCopyFolder(contextMenu.folder),
            },
            {
                label: 'Move to Root',
                onClick: () => {
                    moveRunFolder(contextMenu.folder.id, null)
                        .then(() => loadFolderTree())
                        .catch(err => console.error('Failed to move to root:', err));
                },
            },
            'separator',
            {
                label: 'Delete',
                danger: true,
                testId: `delete-folder-${contextMenu.folder.id}`,
                onClick: () => setModal({ type: 'delete', folder: contextMenu.folder }),
            },
        ]
    ) : [];

    // ── Render ────────────────────────────────────────────────────────────────
    if (collapsed) {
        return (
            <div className="run-folder-sidebar collapsed" data-testid="run-folder-sidebar-collapsed">
                <button
                    className="run-folder-collapse-btn"
                    onClick={toggleCollapsed}
                    title="Expand sidebar"
                    data-testid="sidebar-expand-btn"
                >&rsaquo;</button>
            </div>
        );
    }

    return (
        <div className="run-folder-sidebar" data-testid="run-folder-sidebar" style={{ width }}>
            <div className="resize-handle" onMouseDown={startResizing} />
            {/* Header */}
            <div className="run-folder-sidebar-header">
                <span className="run-folder-sidebar-title">Folders</span>
                <div style={{ display: 'flex', gap: 4, alignItems: 'center' }}>
                    <button style={{ background: 'none', border: 'none', color: 'var(--text-secondary)', padding: 0, fontSize: '0.8em', cursor: 'pointer' }} onClick={() => handleZoom(-0.1)} title="Zoom Out">A-</button>
                    <button style={{ background: 'none', border: 'none', color: 'var(--text-secondary)', padding: 0, fontSize: '0.9em', cursor: 'pointer' }} onClick={() => handleZoom(0.1)} title="Zoom In">A+</button>
                    <button
                        className="run-folder-action-btn"
                        onClick={() => { setCreateError(''); setModal({ type: 'create', parentId: null }); }}
                        title="New Folder"
                        data-testid="add-folder-btn"
                    >+ Folder</button>
                    <button
                        className="run-folder-collapse-btn"
                        onClick={toggleCollapsed}
                        title="Collapse sidebar"
                        data-testid="sidebar-collapse-btn"
                    >&lsaquo;</button>
                </div>
            </div>

            {/* Zoomable content — scrolls internally so its scrollbar clears the resize-handle */}
            <div className="run-folder-sidebar-scroll" style={{ fontSize: `${zoom}rem` }}>

            {/* All Runs */}
            <div
                className={`run-folder-item all-runs ${!selectedFolderId ? 'selected' : ''} ${dragOverAll ? 'drag-over' : ''}`}
                onClick={() => onSelectFolder(null)}
                data-testid="all-runs-entry"
                {...allRunsDropHandlers}
            >
                <span className="expand-toggle" />
                <span className="folder-icon"><AllRunsSvg /></span>
                <span className="run-folder-name">All Runs</span>
            </div>

            {/* Uncategorised */}
            {hasUncategorised && (
                <div>
                    <div
                        className={`run-folder-item uncategorised ${selectedFolderId === 'uncategorised' ? 'selected' : ''} ${dragOverUncat ? 'drag-over' : ''}`}
                        onClick={() => onSelectFolder('uncategorised')}
                        data-testid="uncategorised-entry"
                        {...uncatDropHandlers}
                    >
                        <span
                            className={`expand-toggle visible ${uncatExpanded ? 'expanded' : ''}`}
                            onClick={(e) => { e.stopPropagation(); handleToggleUncat(); }}
                        >
                            <ChevronSvg />
                        </span>
                        <span className="folder-icon"><FolderSvg open={uncatExpanded} /></span>
                        <span className="run-folder-name">Uncategorised</span>
                    </div>
                    {uncatExpanded && (
                        <div className="run-sidebar-children">
                            {uncatRuns.loading ? (
                                <div className="run-sidebar-message">Loading&hellip;</div>
                            ) : uncatRuns.runs.length === 0 ? (
                                <div className="run-sidebar-message">No runs</div>
                            ) : (
                                uncatRuns.runs.map(run => (
                                    <RunItem key={run.id} run={run} onRunClick={handleRunClick} onRunContextMenu={handleRunContextMenu} isSelected={selectedRunId === run.id} />
                                ))
                            )}
                        </div>
                    )}
                </div>
            )}

            {/* Recursive folder tree */}
            {folderTree.map(folder => (
                <RunFolderNode
                    key={folder.id}
                    folder={folder}
                    depth={0}
                    selectedFolderId={selectedFolderId}
                    selectedRunId={selectedRunId}
                    expandedFolderIds={expandedFolderIds}
                    onToggleExpand={handleToggleExpand}
                    onSelectFolder={(id) => onSelectFolder(id)}
                    onCreateSubfolder={(f) => { setCreateError(''); setModal({ type: 'create', parentId: f.id }); }}
                    onRunClick={handleRunClick}
                    onContextMenu={handleContextMenu}
                    onRunContextMenu={handleRunContextMenu}
                    onRunDrop={handleRunDrop}
                    onFolderDrop={handleFolderDrop}
                />
            ))}

            {folderTree.length === 0 && (
                <div className="run-folder-empty" data-testid="folder-list-empty">No folders yet</div>
            )}

            </div>{/* end zoomable content */}

            {/* Context menu */}
            {contextMenu && (
                <ContextMenu
                    x={contextMenu.x}
                    y={contextMenu.y}
                    items={contextMenuItems}
                    onClose={closeContextMenu}
                />
            )}

            {/* Modals */}
            {modal?.type === 'create' && (
                <Modal
                    title={modal.parentId ? 'New Subfolder' : 'New Folder'}
                    placeholder="e.g. Smoke Tests"
                    confirmText="Create"
                    onConfirm={handleCreateConfirm}
                    onCancel={() => { setModal(null); setCreateError(''); }}
                />
            )}
            {createError && modal?.type === 'create' && (
                <div className="run-folder-error" data-testid="folder-create-error">{createError}</div>
            )}

            {modal?.type === 'rename' && (
                <Modal
                    title="Rename Folder"
                    defaultValue={modal.folder.name}
                    placeholder="Folder name"
                    confirmText="Rename"
                    onConfirm={handleRenameConfirm}
                    onCancel={() => { setModal(null); setRenameError(''); }}
                />
            )}
            {renameError && modal?.type === 'rename' && (
                <div className="run-folder-error" data-testid="folder-rename-error">{renameError}</div>
            )}

            {modal?.type === 'delete' && (
                <Modal
                    title="Delete Folder"
                    type="confirm"
                    message={
                        (modal.folder.sub_folders?.length > 0)
                            ? `Delete "${modal.folder.name}" and all its subfolders? Runs inside will be moved to Uncategorised.`
                            : `Delete "${modal.folder.name}"? Runs inside will be moved to Uncategorised.`
                    }
                    confirmText="Delete"
                    confirmStyle="danger"
                    onConfirm={handleDeleteConfirm}
                    onCancel={() => setModal(null)}
                />
            )}
        </div>
    );
}
