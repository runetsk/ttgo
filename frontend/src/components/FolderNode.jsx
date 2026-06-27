import React, { useState, useRef, useEffect, useMemo } from 'react';
import { Link } from 'react-router-dom';
import { createFolder, updateFolder, deleteFolder, deleteFolders, moveFolder, bulkMoveFolders, moveTest, qtest } from '../api';
import Modal from './Modal';
import ContextMenu from './ContextMenu';
import TestCaseNode from './TestCaseNode';
import { toast } from '../toast';

// Recursively count all test cases under a folder
const countTests = (folder) => {
    const direct = folder.test_cases?.length || 0;
    const nested = folder.sub_folders?.reduce((sum, sub) => sum + countTests(sub), 0) || 0;
    return direct + nested;
};

function filterModulesFlat(modules, search) {
    if (!modules || !search) return [];
    const lower = search.toLowerCase();
    const results = [];
    const walk = (list) => {
        for (const m of list) {
            if (m.name.toLowerCase().includes(lower)) {
                results.push({ id: m.id, name: m.name, depth: 0, hasChildren: false });
            }
            if (m.children) walk(m.children);
        }
    };
    walk(modules);
    return results;
}

const ChevronSvg = () => (
    <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <polyline points="9 18 15 12 9 6" />
    </svg>
);

const FolderSvg = ({ open }) => (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        {open ? (
            <>
                <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2H3z" />
                <path d="M3 9h18l-2 9a2 2 0 0 1-2 1.5H5a2 2 0 0 1-2-1.5z" />
            </>
        ) : (
            <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
        )}
    </svg>
);

const FolderNode = ({ folder, depth = 0, onSelect, selectedIds, onRefresh, expandedIds, onToggle, activeTestId, selectedTestIds = [], onSelectTest, onDeleteTests, showTests = false }) => {
    const isSelected = selectedIds.includes(folder.id);
    const isExpanded = expandedIds.includes(folder.id);
    const [contextMenu, setContextMenu] = useState(null);
    const [modal, setModal] = useState(null);
    const [isDragOver, setIsDragOver] = useState(false);
    const menuBtnRef = useRef(null);

    // QTest state
    const [qtestEnabled, setQtestEnabled] = useState(false);
    const [qtestModal, setQtestModal] = useState(false);
    const [enabledProjects, setEnabledProjects] = useState([]);
    const [selectedProjectId, setSelectedProjectId] = useState('');
    const [modules, setModules] = useState([]);
    const [selectedModuleId, setSelectedModuleId] = useState('');
    const [loadingModules, setLoadingModules] = useState(false);
    const [uploading, setUploading] = useState(false);
    const [onConflict, setOnConflict] = useState('skip');
    const [recursive, setRecursive] = useState(false);
    const [moduleSearch, setModuleSearch] = useState('');
    const [expandedModuleIds, setExpandedModuleIds] = useState(new Set());

    useEffect(() => {
        qtest.getConfig()
            .then(cfg => setQtestEnabled(!!cfg?.enabled))
            .catch(() => setQtestEnabled(false));
    }, []);

    const totalTests = countTests(folder);
    const directTestCount = folder.test_cases?.length || 0;

    const handleContextMenu = (e) => { e.preventDefault(); e.stopPropagation(); setContextMenu({ x: e.clientX, y: e.clientY }); };
    const handleClick = (e) => {
        if (e.metaKey || e.ctrlKey) { e.preventDefault(); e.stopPropagation(); onSelect(folder, { type: 'multi' }); }
        else if (e.shiftKey) { e.preventDefault(); e.stopPropagation(); onSelect(folder, { type: 'range' }); }
        else onSelect(folder, { type: 'single' });
    };
    const handleDragStart = (e) => {
        e.dataTransfer.setData("folderId", folder.id);
        if (isSelected && selectedIds.length > 0) e.dataTransfer.setData("folderIds", JSON.stringify(selectedIds));
        e.dataTransfer.effectAllowed = "move";
    };
    const handleDragOver = (e) => { e.preventDefault(); e.stopPropagation(); e.dataTransfer.dropEffect = "move"; if (!isDragOver) setIsDragOver(true); };
    const handleDrop = (e) => {
        e.preventDefault(); e.stopPropagation(); setIsDragOver(false);
        const draggedId = e.dataTransfer.getData("folderId");
        const draggedIdsJson = e.dataTransfer.getData("folderIds");
        const testId = e.dataTransfer.getData("testId");
        const testIdsJson = e.dataTransfer.getData("testIds");
        if (draggedIdsJson) {
            const ids = JSON.parse(draggedIdsJson);
            if (!ids.includes(folder.id)) bulkMoveFolders(ids, folder.id).then(onRefresh).catch(err => toast.error(err.response?.data?.error || "Failed to move"));
        } else if (draggedId && draggedId !== folder.id) {
            moveFolder(draggedId, folder.id).then(onRefresh).catch(err => toast.error(err.response?.data?.error || "Failed to move"));
        } else if (testIdsJson) {
            const ids = JSON.parse(testIdsJson);
            Promise.all(ids.map(id => moveTest(id, folder.id)))
                .then(onRefresh)
                .catch(err => toast.error(err.response?.data?.error || "Failed to move tests"));
        } else if (testId) {
            moveTest(testId, folder.id).then(onRefresh);
        }
    };

    // ── QTest folder upload ──
    const openQtestUploadModal = () => {
        setQtestModal(true);
        setSelectedModuleId('');
        setModules([]);
        setOnConflict('skip');
        setModuleSearch('');
        setExpandedModuleIds(new Set());
        qtest.listEnabledProjects()
            .then(projects => {
                setEnabledProjects(projects || []);
                if (projects && projects.length > 0) {
                    const def = projects.find(p => p.is_default) || projects[0];
                    setSelectedProjectId(String(def.project_id));
                    loadModulesForProject(def.project_id);
                }
            })
            .catch(() => toast.error('Failed to load QTest projects'));
    };

    const loadModulesForProject = (projectId) => {
        if (!projectId) return;
        setLoadingModules(true);
        setModules([]);
        setSelectedModuleId('');
        qtest.listModules(parseInt(projectId))
            .then(mods => setModules(mods || []))
            .catch(() => toast.error('Failed to load modules'))
            .finally(() => setLoadingModules(false));
    };

    const handleProjectChange = (val) => {
        setSelectedProjectId(val);
        loadModulesForProject(parseInt(val));
    };

    const toggleModuleExpand = (id) => {
        setExpandedModuleIds(prev => {
            const next = new Set(prev);
            if (next.has(id)) next.delete(id); else next.add(id);
            return next;
        });
    };

    const isSearchingModules = moduleSearch.trim().length > 0;

    const displayModules = useMemo(() => {
        if (isSearchingModules) return filterModulesFlat(modules, moduleSearch);
        const result = [];
        const walk = (list, depth) => {
            if (!list) return;
            for (const m of list) {
                result.push({ id: m.id, name: m.name, depth, hasChildren: !!(m.children && m.children.length > 0) });
                if (m.children && expandedModuleIds.has(m.id)) {
                    walk(m.children, depth + 1);
                }
            }
        };
        walk(modules, 0);
        return result;
    }, [modules, expandedModuleIds, moduleSearch, isSearchingModules]);

    const selectedModuleName = useMemo(() => {
        if (!selectedModuleId) return null;
        const find = (list) => {
            for (const m of (list || [])) {
                if (String(m.id) === String(selectedModuleId)) return m.name;
                const found = find(m.children);
                if (found) return found;
            }
            return null;
        };
        return find(modules);
    }, [modules, selectedModuleId]);

    const handleFolderUpload = () => {
        if (!selectedProjectId) return;
        setUploading(true);
        const parentMod = selectedModuleId ? parseInt(selectedModuleId) : 0;
        qtest.uploadFolder(folder.id, parseInt(selectedProjectId), parentMod, onConflict, recursive)
            .then(resp => {
                const r = resp.result;
                if (r) {
                    const msg = `Created module "${folder.name}". Uploaded: ${r.succeeded || 0}, Skipped: ${r.skipped || 0}, Failed: ${r.failed || 0}`;
                    if (r.failed > 0) toast.error(msg);
                    else toast.success(msg);
                } else {
                    toast.success(`Module "${folder.name}" created in QTest`);
                }
                setQtestModal(false);
            })
            .catch(err => toast.error(err.response?.data?.error || 'Upload failed'))
            .finally(() => setUploading(false));
    };

    // ── Standard folder actions ──
    const handleAction = (action) => {
        setContextMenu(null);
        if (action === 'create') {
            setModal({ type: 'prompt', title: 'New Subfolder', message: `Create a new subfolder in "${folder.name}"`, placeholder: 'Subfolder name...', onConfirm: (name) => {
                if (name) createFolder(name, folder.id).then(() => { setModal(null); onRefresh(); if (!isExpanded) onToggle(folder.id); });
            }});
        } else if (action === 'rename') {
            setModal({ type: 'prompt', title: 'Rename Folder', message: `Rename "${folder.name}"`, placeholder: 'Folder name...', defaultValue: folder.name, onConfirm: (name) => {
                if (name && name !== folder.name) updateFolder(folder.id, name).then(() => { setModal(null); onRefresh(); });
                else setModal(null);
            }});
        } else if (action === 'delete') {
            const isMulti = selectedIds.length > 1 && isSelected;
            const count = isMulti ? selectedIds.length : 1;
            setModal({ type: 'confirm', title: isMulti ? 'Delete Folders' : 'Delete Folder',
                message: isMulti ? `Delete ${count} folders and all their contents?` : `Delete "${folder.name}" and all its contents?`,
                confirmText: 'Delete', onConfirm: () => {
                    const del = isMulti ? deleteFolders(selectedIds) : deleteFolder(folder.id);
                    del.then(() => { setModal(null); onRefresh(); onSelect(null); });
                }});
        } else if (action === 'qtest-upload') {
            openQtestUploadModal();
        } else if (action === 'qtest-unlink') {
            setModal({
                type: 'confirm',
                title: 'Unlink from QTest',
                message: `Remove QTest links for every test under "${folder.name}" (including subfolders)? Test cases in QTest are NOT deleted — only the local link is dropped, so a future upload will create fresh QTest test cases.`,
                confirmText: 'Unlink',
                onConfirm: () => {
                    qtest.unlinkFolder(folder.id, true)
                        .then(({ deleted }) => {
                            setModal(null);
                            toast.success(`Unlinked ${deleted} test case${deleted !== 1 ? 's' : ''} from QTest`);
                            onRefresh();
                        })
                        .catch(err => toast.error(err.response?.data?.error || 'Failed to unlink'));
                },
            });
        }
    };

    const handleAddBtn = (e) => { e.preventDefault(); e.stopPropagation(); handleAction('create'); };
    const handleMenuBtn = (e) => {
        e.preventDefault(); e.stopPropagation();
        const rect = menuBtnRef.current.getBoundingClientRect();
        setContextMenu({ x: rect.left, y: rect.bottom + 4 });
    };

    const hasChildren = (folder.sub_folders && folder.sub_folders.length > 0) || (showTests && folder.test_cases && folder.test_cases.length > 0);

    // Current project name for header subtitle
    const currentProjectName = enabledProjects.find(p => String(p.project_id) === selectedProjectId)?.project_name;

    return (
        <div className="folder-node" data-testid="folder-container">
            <Link to={`/library/folders/${folder.id}`} className={`folder-header ${isSelected ? 'selected' : ''} ${isDragOver ? 'drag-over' : ''}`}
                onClick={handleClick} onContextMenu={handleContextMenu} draggable
                onDragStart={handleDragStart} onDragOver={handleDragOver} onDragLeave={() => setIsDragOver(false)} onDrop={handleDrop}
                data-testid="folder-name"
                style={{ textDecoration: 'none', paddingLeft: 8 + depth * 14 }}>
                <span className={`expand-toggle ${hasChildren ? 'visible' : ''} ${isExpanded && hasChildren ? 'expanded' : ''}`}
                    onClick={(e) => { e.preventDefault(); e.stopPropagation(); onToggle(folder.id); }}>
                    <ChevronSvg />
                </span>
                <span className="folder-icon">
                    <FolderSvg open={isExpanded && hasChildren} />
                </span>
                <div className="folder-label">
                    <span className="folder-name-text">{folder.name}</span>
                </div>
                {totalTests > 0 && <span className="folder-count">{totalTests}</span>}
                <div className="folder-actions">
                    <button className="folder-actions-btn" onClick={handleAddBtn} title="New subfolder">+</button>
                    <button className="folder-actions-btn" ref={menuBtnRef} onClick={handleMenuBtn} title="More actions">⋮</button>
                </div>
            </Link>

            {contextMenu && <ContextMenu x={contextMenu.x} y={contextMenu.y} folderName={folder.name} isMulti={selectedIds.length > 1 && isSelected} onClose={() => setContextMenu(null)} onAction={handleAction} qtestEnabled={qtestEnabled} />}
            {modal && <Modal {...modal} onCancel={() => setModal(null)} />}

            {/* ── QTest Upload Folder Modal ── */}
            {qtestModal && (
                <div className="modal-overlay" onClick={() => !uploading && setQtestModal(false)}>
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
                            }}>📁</div>
                            <div style={{ flex: 1 }}>
                                <h3 style={{ margin: 0, fontSize: '1.2rem', fontWeight: 700, color: 'var(--text-primary)' }}>
                                    Upload Folder to QTest
                                </h3>
                                <p style={{ margin: 0, fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
                                    "{folder.name}" — {directTestCount} test case{directTestCount !== 1 ? 's' : ''}
                                    {currentProjectName && (
                                        <span style={{ marginLeft: 6 }}>
                                            · Project: <span style={{ fontWeight: 600, color: 'var(--text-primary)' }}>{currentProjectName}</span>
                                        </span>
                                    )}
                                </p>
                            </div>
                        </div>

                        {/* Body */}
                        <div style={{ padding: '24px 28px' }}>
                            {/* Info banner */}
                            <div style={{
                                padding: '12px 16px', borderRadius: 10, marginBottom: 20,
                                background: 'rgba(99,102,241,0.06)',
                                border: '1px solid rgba(99,102,241,0.15)',
                                fontSize: '0.85rem', color: 'var(--text-secondary)', lineHeight: 1.5,
                            }}>
                                A new module <strong style={{ color: 'var(--text-primary)' }}>"{folder.name}"</strong> will be created in QTest and all direct test cases uploaded into it.
                            </div>

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
                                        value={selectedProjectId}
                                        onChange={e => handleProjectChange(e.target.value)}
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
                                    No projects enabled — configure projects in QTest Settings first.
                                </div>
                            )}

                            {/* Parent module picker */}
                            <div style={{ marginBottom: 24 }}>
                                <label style={{
                                    display: 'block', marginBottom: 8,
                                    fontSize: '0.8rem', fontWeight: 600,
                                    color: 'var(--text-secondary)',
                                    textTransform: 'uppercase', letterSpacing: '0.05em',
                                }}>
                                    Parent Module
                                    <span style={{ fontWeight: 400, fontStyle: 'italic', marginLeft: 6, textTransform: 'none', letterSpacing: 0 }}>optional — leave empty for root level</span>
                                </label>

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
                                                value={moduleSearch}
                                                onChange={e => setModuleSearch(e.target.value)}
                                                style={{
                                                    flex: 1, border: 'none', outline: 'none',
                                                    background: 'transparent', fontSize: '0.9rem',
                                                    color: 'var(--text-primary)',
                                                }}
                                            />
                                            {moduleSearch && (
                                                <button
                                                    onClick={() => setModuleSearch('')}
                                                    style={{
                                                        background: 'none', border: 'none',
                                                        cursor: 'pointer', color: 'var(--text-secondary)',
                                                        fontSize: '0.85rem', padding: '2px 4px',
                                                    }}
                                                >✕</button>
                                            )}
                                        </div>

                                        {/* Module tree */}
                                        <div style={{ maxHeight: 260, overflowY: 'auto', padding: '4px 0' }}>
                                            {/* Root-level option */}
                                            {!isSearchingModules && (
                                                <div
                                                    onClick={() => setSelectedModuleId('')}
                                                    style={{
                                                        padding: '8px 14px', cursor: 'pointer',
                                                        fontSize: '0.9rem', display: 'flex', alignItems: 'center', gap: 6,
                                                        background: selectedModuleId === ''
                                                            ? 'var(--accent-indigo-subtle, rgba(99,102,241,0.1))'
                                                            : 'transparent',
                                                        color: selectedModuleId === '' ? 'var(--accent-indigo)' : 'var(--text-secondary)',
                                                        fontWeight: selectedModuleId === '' ? 600 : 400,
                                                        fontStyle: 'italic',
                                                        transition: 'background 0.1s',
                                                    }}
                                                    onMouseEnter={e => { if (selectedModuleId !== '') e.currentTarget.style.background = 'var(--bg-hover, rgba(255,255,255,0.05))'; }}
                                                    onMouseLeave={e => { if (selectedModuleId !== '') e.currentTarget.style.background = 'transparent'; }}
                                                >
                                                    <span style={{ width: 20, flexShrink: 0 }} />
                                                    <span>Root level (no parent)</span>
                                                    {selectedModuleId === '' && <span style={{ marginLeft: 'auto', fontSize: '0.8rem' }}>✓</span>}
                                                </div>
                                            )}

                                            {displayModules.length === 0 && isSearchingModules ? (
                                                <div style={{
                                                    padding: '20px 14px', textAlign: 'center',
                                                    color: 'var(--text-secondary)', fontSize: '0.9rem',
                                                }}>
                                                    No modules match your search
                                                </div>
                                            ) : displayModules.map(m => {
                                                const isSel = String(m.id) === String(selectedModuleId);
                                                const isExp = expandedModuleIds.has(m.id);
                                                return (
                                                    <div
                                                        key={m.id}
                                                        style={{
                                                            padding: '8px 14px',
                                                            paddingLeft: isSearchingModules ? 14 : 14 + m.depth * 20,
                                                            cursor: 'pointer', fontSize: '0.9rem',
                                                            display: 'flex', alignItems: 'center', gap: 6,
                                                            background: isSel
                                                                ? 'var(--accent-indigo-subtle, rgba(99,102,241,0.1))'
                                                                : 'transparent',
                                                            color: isSel ? 'var(--accent-indigo)' : 'var(--text-primary)',
                                                            fontWeight: isSel ? 600 : 400,
                                                            transition: 'background 0.1s',
                                                        }}
                                                        onMouseEnter={e => { if (!isSel) e.currentTarget.style.background = 'var(--bg-hover, rgba(255,255,255,0.05))'; }}
                                                        onMouseLeave={e => { if (!isSel) e.currentTarget.style.background = 'transparent'; }}
                                                        onClick={() => setSelectedModuleId(String(m.id))}
                                                    >
                                                        {m.hasChildren && !isSearchingModules ? (
                                                            <span
                                                                onClick={e => { e.stopPropagation(); toggleModuleExpand(m.id); }}
                                                                style={{
                                                                    width: 20, height: 20,
                                                                    display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
                                                                    borderRadius: 4, flexShrink: 0,
                                                                    fontSize: '0.7rem', color: 'var(--text-secondary)',
                                                                    transition: 'transform 0.15s',
                                                                    transform: isExp ? 'rotate(90deg)' : 'rotate(0deg)',
                                                                }}
                                                            >▶</span>
                                                        ) : (
                                                            <span style={{ width: 20, flexShrink: 0 }} />
                                                        )}
                                                        <span style={{ fontSize: '0.85rem', opacity: 0.5 }}>📁</span>
                                                        <span style={{ flex: 1 }}>{m.name}</span>
                                                        {isSel && <span style={{ fontSize: '0.8rem' }}>✓</span>}
                                                    </div>
                                                );
                                            })}
                                        </div>

                                        {/* Selected indicator */}
                                        {selectedModuleName && (
                                            <div style={{
                                                padding: '10px 14px',
                                                borderTop: '1px solid var(--border-color)',
                                                fontSize: '0.85rem', color: 'var(--accent-indigo)',
                                                display: 'flex', alignItems: 'center', gap: 6,
                                            }}>
                                                <span>✓</span>
                                                <span style={{ fontWeight: 600 }}>New module will be created inside: {selectedModuleName}</span>
                                            </div>
                                        )}
                                    </div>
                                )}
                            </div>

                            {/* Include subfolders */}
                            <div style={{ marginBottom: 18 }}>
                                <label
                                    style={{
                                        display: 'flex', alignItems: 'flex-start', gap: 10,
                                        padding: '12px 14px', borderRadius: 10, cursor: 'pointer',
                                        border: `2px solid ${recursive ? 'var(--accent-indigo)' : 'var(--border-color)'}`,
                                        background: recursive
                                            ? 'var(--accent-indigo-subtle, rgba(99,102,241,0.08))'
                                            : 'transparent',
                                        transition: 'all 0.15s',
                                    }}
                                >
                                    <input
                                        type="checkbox"
                                        checked={recursive}
                                        onChange={e => setRecursive(e.target.checked)}
                                        data-testid="qtest-recursive-checkbox"
                                        style={{ marginTop: 3 }}
                                    />
                                    <div>
                                        <div style={{
                                            fontSize: '0.9rem', fontWeight: 600,
                                            color: recursive ? 'var(--accent-indigo)' : 'var(--text-primary)',
                                        }}>Include subfolders</div>
                                        <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginTop: 2 }}>
                                            Mirror the full folder tree as nested QTest modules and upload every test case beneath this folder. Existing modules with the same name are reused.
                                        </div>
                                    </div>
                                </label>
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
                                            onClick={() => setOnConflict(opt.value)}
                                            style={{
                                                flex: 1, padding: '12px 16px',
                                                borderRadius: 10, cursor: 'pointer',
                                                border: `2px solid ${onConflict === opt.value ? 'var(--accent-indigo)' : 'var(--border-color)'}`,
                                                background: onConflict === opt.value
                                                    ? 'var(--accent-indigo-subtle, rgba(99,102,241,0.08))'
                                                    : 'transparent',
                                                transition: 'all 0.15s',
                                            }}
                                        >
                                            <div style={{
                                                fontSize: '0.9rem', fontWeight: 600,
                                                color: onConflict === opt.value ? 'var(--accent-indigo)' : 'var(--text-primary)',
                                            }}>{opt.label}</div>
                                            <div style={{
                                                fontSize: '0.8rem', color: 'var(--text-secondary)', marginTop: 2,
                                            }}>{opt.desc}</div>
                                        </div>
                                    ))}
                                </div>
                            </div>
                        </div>

                        {/* Footer */}
                        <div style={{
                            padding: '16px 28px',
                            borderTop: '1px solid var(--border-color)',
                            display: 'flex', justifyContent: 'flex-end', gap: 10,
                            background: 'var(--bg-secondary)',
                        }}>
                            <button
                                className="action-btn"
                                onClick={() => setQtestModal(false)}
                                disabled={uploading}
                                style={{ padding: '10px 20px', fontSize: '0.9rem' }}
                            >
                                Cancel
                            </button>
                            <button
                                className="primary-btn"
                                onClick={handleFolderUpload}
                                disabled={uploading || !selectedProjectId || enabledProjects.length === 0}
                                style={{ padding: '10px 20px', fontSize: '0.9rem' }}
                            >
                                {uploading
                                    ? 'Uploading...'
                                    : (() => {
                                        const count = recursive ? totalTests : directTestCount;
                                        return `⬆ Upload ${count} test case${count !== 1 ? 's' : ''}`;
                                    })()}
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {isExpanded && hasChildren && (
                <div className="sub-folders">
                    {folder.sub_folders && folder.sub_folders.map(sub => (
                        <FolderNode key={sub.id} folder={sub} depth={depth + 1} onSelect={onSelect} selectedIds={selectedIds} onRefresh={onRefresh} expandedIds={expandedIds} onToggle={onToggle} activeTestId={activeTestId} selectedTestIds={selectedTestIds} onSelectTest={onSelectTest} onDeleteTests={onDeleteTests} showTests={showTests} />
                    ))}
                    {showTests && folder.test_cases && folder.test_cases.map(tc => (
                        <TestCaseNode key={tc.id} testCase={tc} depth={depth + 1} activeTestId={activeTestId} selectedTestIds={selectedTestIds} onSelectTest={onSelectTest} onDeleteTests={onDeleteTests} />
                    ))}
                </div>
            )}
        </div>
    );
};
export default FolderNode;
