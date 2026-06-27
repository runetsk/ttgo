import React, { useEffect, useState, useRef, useCallback, useMemo } from 'react';
import { useLocation } from 'react-router-dom';
import { getFolderTree, createFolder, deleteFolder, deleteFolders, moveFolder, bulkMoveFolders, deleteTests } from '../api';
import Modal from './Modal';
import FolderNode from './FolderNode';
import { toast } from '../toast';

export default function Sidebar({ onSelectFolders, selectedFolderIds }) {
    const [tree, setTree] = useState([]);
    const [modal, setModal] = useState(null);
    const [isLibraryDragOver, setIsLibraryDragOver] = useState(false);
    const location = useLocation();
    const testIdMatch = location.pathname.match(/^\/library\/tests\/([^/]+)/);
    const testId = testIdMatch ? testIdMatch[1] : null;

    // Collapse State
    const [collapsed, setCollapsed] = useState(() => localStorage.getItem('sidebarCollapsed') === 'true');
    const toggleCollapsed = () => setCollapsed(prev => {
        const next = !prev;
        localStorage.setItem('sidebarCollapsed', String(next));
        return next;
    });

    // Resize and Zoom State
    const [width, setWidth] = useState(() => parseInt(localStorage.getItem('sidebarWidth')) || 240);
    const [zoom, setZoom] = useState(() => parseFloat(localStorage.getItem('sidebarZoom')) || 1);
    const isResizing = useRef(false);

    // Lifted state — persisted so expand state survives navigation away from the tests page
    const [expandedIds, setExpandedIds] = useState(() => {
        try { return JSON.parse(localStorage.getItem('sidebarExpandedIds')) || []; }
        catch { return []; }
    });
    const [lastSelectedId, setLastSelectedId] = useState(null);
    const [selectedTestIds, setSelectedTestIds] = useState([]);
    const [lastSelectedTestId, setLastSelectedTestId] = useState(null);
    const [showTests, setShowTests] = useState(() => localStorage.getItem('sidebarShowTests') === 'true');
    const [search, setSearch] = useState('');

    const refresh = useCallback(() => {
        getFolderTree().then(data => setTree(data)).catch(() => {});
    }, []);

    useEffect(() => { refresh(); }, [refresh]);

    // Refresh tree when any component signals a folder mutation (e.g. rename from TestGrid)
    useEffect(() => {
        const handler = () => refresh();
        window.addEventListener('folder-tree-changed', handler);
        return () => window.removeEventListener('folder-tree-changed', handler);
    }, [refresh]);

    const findPath = useCallback((nodes, targetTestId) => {
        if (!nodes) return null;
        for (const node of nodes) {
            if (node.test_cases?.some(t => t.id === targetTestId)) return [node.id];
            if (node.sub_folders) {
                const path = findPath(node.sub_folders, targetTestId);
                if (path) return [node.id, ...path];
            }
        }
        return null;
    }, []);

    useEffect(() => {
        if (testId && tree.length > 0) {
            const path = findPath(tree, testId);
            if (path) setExpandedIds(prev => {
                const next = [...new Set([...prev, ...path])];
                localStorage.setItem('sidebarExpandedIds', JSON.stringify(next));
                return next;
            });
        }
    }, [tree, testId, findPath]);

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
        localStorage.setItem('sidebarWidth', width);
    }, [width]);

    const resize = useCallback((e) => {
        if (isResizing.current) setWidth(Math.max(200, Math.min(520, e.clientX)));
    }, []);

    const toggleShowTests = () => setShowTests(prev => {
        const next = !prev;
        localStorage.setItem('sidebarShowTests', String(next));
        return next;
    });

    const collapseAll = () => {
        setExpandedIds([]);
        localStorage.setItem('sidebarExpandedIds', JSON.stringify([]));
    };

    // Recursive filter: keep any folder whose own name matches, plus ancestors of matches.
    const filteredTree = useMemo(() => {
        const q = search.trim().toLowerCase();
        if (!q) return tree;
        const filter = (nodes) => {
            const out = [];
            for (const n of nodes) {
                const children = n.sub_folders ? filter(n.sub_folders) : [];
                const match = (n.name || '').toLowerCase().includes(q);
                if (match || children.length > 0) {
                    out.push({ ...n, sub_folders: children });
                }
            }
            return out;
        };
        return filter(tree);
    }, [tree, search]);

    // Auto-expand all matching branches while searching
    const searchExpandedIds = useMemo(() => {
        if (!search.trim()) return expandedIds;
        const ids = [];
        const walk = (nodes) => {
            for (const n of nodes) {
                ids.push(n.id);
                if (n.sub_folders) walk(n.sub_folders);
            }
        };
        walk(filteredTree);
        return [...new Set([...expandedIds, ...ids])];
    }, [expandedIds, filteredTree, search]);

    useEffect(() => {
        window.addEventListener('mousemove', resize);
        window.addEventListener('mouseup', stopResizing);
        return () => { window.removeEventListener('mousemove', resize); window.removeEventListener('mouseup', stopResizing); };
    }, [resize, stopResizing]);

    const handleZoom = (delta) => setZoom(prev => {
        const newZoom = Math.max(0.8, Math.min(1.5, prev + delta));
        localStorage.setItem('sidebarZoom', newZoom);
        return newZoom;
    });

    const flattenVisibleTree = useCallback((nodes, expandedList) => {
        let flat = [];
        if (!nodes) return flat;
        nodes.forEach(node => {
            flat.push(node);
            if (node.sub_folders && node.sub_folders.length > 0 && expandedList.includes(node.id))
                flat = flat.concat(flattenVisibleTree(node.sub_folders, expandedList));
        });
        return flat;
    }, []);

    // Flat list of visible test IDs in render order — used for shift-range across folders.
    const flattenVisibleTests = useCallback((nodes, expandedList) => {
        let ids = [];
        if (!nodes) return ids;
        nodes.forEach(node => {
            if (expandedList.includes(node.id)) {
                if (node.sub_folders && node.sub_folders.length > 0)
                    ids = ids.concat(flattenVisibleTests(node.sub_folders, expandedList));
                if (node.test_cases) ids = ids.concat(node.test_cases.map(t => t.id));
            }
        });
        return ids;
    }, []);

    const handleToggle = (id) => setExpandedIds(prev => {
        const next = prev.includes(id) ? prev.filter(i => i !== id) : [...prev, id];
        localStorage.setItem('sidebarExpandedIds', JSON.stringify(next));
        return next;
    });

    const handleSelect = (folder, options = {}) => {
        if (!folder) { onSelectFolders([]); setLastSelectedId(null); return; }
        const type = options.type || 'single';
        if (type === 'range' && lastSelectedId) {
            const flatList = flattenVisibleTree(tree, expandedIds);
            const startIdx = flatList.findIndex(f => f.id === lastSelectedId);
            const endIdx = flatList.findIndex(f => f.id === folder.id);
            if (startIdx !== -1 && endIdx !== -1) {
                onSelectFolders(flatList.slice(Math.min(startIdx, endIdx), Math.max(startIdx, endIdx) + 1));
            } else { onSelectFolders([folder]); setLastSelectedId(folder.id); }
        } else if (type === 'multi') {
            onSelectFolders(prev => { const exists = prev.find(f => f.id === folder.id); return exists ? prev.filter(f => f.id !== folder.id) : [...prev, folder]; });
            setLastSelectedId(folder.id);
        } else { onSelectFolders([folder]); setLastSelectedId(folder.id); }
    };

    const handleSelectTest = (testCase, options = {}) => {
        if (!testCase) { setSelectedTestIds([]); setLastSelectedTestId(null); return; }
        const type = options.type || 'single';
        if (type === 'range' && lastSelectedTestId) {
            const visible = flattenVisibleTests(tree, expandedIds);
            const startIdx = visible.indexOf(lastSelectedTestId);
            const endIdx = visible.indexOf(testCase.id);
            if (startIdx !== -1 && endIdx !== -1) {
                const range = visible.slice(Math.min(startIdx, endIdx), Math.max(startIdx, endIdx) + 1);
                setSelectedTestIds(prev => [...new Set([...prev, ...range])]);
            } else { setSelectedTestIds([testCase.id]); setLastSelectedTestId(testCase.id); }
        } else if (type === 'multi') {
            setSelectedTestIds(prev => prev.includes(testCase.id) ? prev.filter(id => id !== testCase.id) : [...prev, testCase.id]);
            setLastSelectedTestId(testCase.id);
        } else { setSelectedTestIds([testCase.id]); setLastSelectedTestId(testCase.id); }
    };

    const handleDeleteTests = (ids) => {
        if (!ids || ids.length === 0) return;
        setModal({
            type: 'confirm', title: 'Delete Tests', message: `Delete ${ids.length} test${ids.length !== 1 ? 's' : ''}?`, confirmText: 'Delete',
            onConfirm: () => deleteTests(ids).then(() => { setModal(null); setSelectedTestIds([]); refresh(); }).catch(err => toast.error(err.response?.data?.error || 'Failed to delete'))
        });
    };

    const handleCreateRoot = () => setModal({
        type: 'prompt', title: 'New Root Folder', message: 'Create a new top-level library folder', placeholder: 'Folder name...',
        onConfirm: (name) => { if (name) createFolder(name, null).then(() => { setModal(null); refresh(); }); }
    });

    const handleBulkDelete = () => setModal({
        type: 'confirm', title: 'Delete Folders', message: `Delete ${selectedFolderIds.length} folders?`, confirmText: 'Delete',
        onConfirm: () => deleteFolders(selectedFolderIds).then(() => { setModal(null); refresh(); onSelectFolders([]); })
    });

    const handleLibraryDrop = (e) => {
        e.preventDefault(); setIsLibraryDragOver(false);
        const draggedId = e.dataTransfer.getData("folderId");
        const draggedIdsJson = e.dataTransfer.getData("folderIds");
        if (draggedIdsJson) { const ids = JSON.parse(draggedIdsJson); bulkMoveFolders(ids, null).then(refresh).catch(err => toast.error(err.response?.data?.error || "Failed")); }
        else if (draggedId) { moveFolder(draggedId, null).then(refresh).catch(err => toast.error(err.response?.data?.error || "Failed")); }
    };

    if (collapsed) {
        return (
            <aside className="sidebar collapsed" data-testid="sidebar">
                <button
                    className="run-folder-collapse-btn"
                    onClick={toggleCollapsed}
                    title="Expand sidebar"
                    data-testid="sidebar-expand-btn"
                >›</button>
            </aside>
        );
    }

    const effectiveExpandedIds = search.trim() ? searchExpandedIds : expandedIds;

    return (
        <aside className="sidebar" data-testid="sidebar"
            onDragOver={(e) => { e.preventDefault(); setIsLibraryDragOver(true); }}
            onDragLeave={() => setIsLibraryDragOver(false)}
            onDrop={handleLibraryDrop}
            style={{ outline: isLibraryDragOver ? '2px dashed var(--accent-indigo)' : undefined, width }}>
            <div className="resize-handle" onMouseDown={startResizing} />
            <header className="sidebar-header">
                <h2 className="sidebar-title" data-testid="sidebar-title">Library</h2>
                <button
                    className={`sidebar-icon-btn ${showTests ? 'active' : ''}`}
                    onClick={toggleShowTests}
                    title={showTests ? 'Hide tests in tree' : 'Show tests in tree'}
                    data-testid="sidebar-show-tests-toggle"
                >
                    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <line x1="8" y1="6" x2="21" y2="6" />
                        <line x1="8" y1="12" x2="21" y2="12" />
                        <line x1="8" y1="18" x2="21" y2="18" />
                        <line x1="3" y1="6" x2="3.01" y2="6" />
                        <line x1="3" y1="12" x2="3.01" y2="12" />
                        <line x1="3" y1="18" x2="3.01" y2="18" />
                    </svg>
                </button>
                <button
                    className="sidebar-icon-btn"
                    onClick={collapseAll}
                    title="Collapse all"
                    data-testid="sidebar-collapse-all"
                >
                    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <polyline points="4 14 10 14 10 20" />
                        <polyline points="20 10 14 10 14 4" />
                        <line x1="14" y1="10" x2="21" y2="3" />
                        <line x1="3" y1="21" x2="10" y2="14" />
                    </svg>
                </button>
                <button
                    className="sidebar-icon-btn"
                    onClick={handleCreateRoot}
                    title="New root folder"
                    data-testid="create-root-folder-button"
                >
                    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <line x1="12" y1="5" x2="12" y2="19" />
                        <line x1="5" y1="12" x2="19" y2="12" />
                    </svg>
                </button>
                <button
                    className="sidebar-icon-btn"
                    onClick={toggleCollapsed}
                    title="Collapse sidebar"
                    data-testid="sidebar-collapse-btn"
                >
                    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <polyline points="15 18 9 12 15 6" />
                    </svg>
                </button>
            </header>

            <div className="sidebar-search">
                <span className="sidebar-search-icon">
                    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <circle cx="11" cy="11" r="7" />
                        <line x1="21" y1="21" x2="16.65" y2="16.65" />
                    </svg>
                </span>
                <input
                    type="text"
                    className="sidebar-search-input"
                    placeholder="Search folders…"
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    data-testid="sidebar-search"
                />
            </div>

            {selectedFolderIds.length > 1 && (
                <div style={{ padding: '0 10px 8px 10px' }}>
                    <button className="action-btn danger" onClick={handleBulkDelete} style={{ padding: '6px 12px', fontSize: '0.75em', width: '100%' }} data-testid="bulk-delete-folders-button">
                        Delete ({selectedFolderIds.length})
                    </button>
                </div>
            )}

            <nav className="folder-tree" style={{ fontSize: `${zoom}rem` }}>
                {filteredTree && filteredTree.map(f => (
                    <FolderNode key={f.id} folder={f} depth={0} onSelect={handleSelect} selectedIds={selectedFolderIds} onRefresh={refresh} expandedIds={effectiveExpandedIds} onToggle={handleToggle} activeTestId={testId} selectedTestIds={selectedTestIds} onSelectTest={handleSelectTest} onDeleteTests={handleDeleteTests} showTests={showTests} />
                ))}
            </nav>
            {modal && <Modal {...modal} onCancel={() => setModal(null)} />}
        </aside>
    );
}
