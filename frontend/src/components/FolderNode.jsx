import React, { useState, useRef } from 'react';
import { Link } from 'react-router-dom';
import { createFolder, updateFolder, deleteFolder, deleteFolders, moveFolder, bulkMoveFolders, moveTest } from '../api';
import Modal from './Modal';
import ContextMenu from './ContextMenu';
import TestCaseNode from './TestCaseNode';
import { ChevronSvg, FolderSvg } from './FolderIcons';
import { toast } from '../toast';

// Recursively count all test cases under a folder
const countTests = (folder) => {
    const direct = folder.test_cases?.length || 0;
    const nested = folder.sub_folders?.reduce((sum, sub) => sum + countTests(sub), 0) || 0;
    return direct + nested;
};

const FolderNode = ({ folder, depth = 0, onSelect, selectedIds, onRefresh, expandedIds, onToggle, activeTestId, selectedTestIds = [], onSelectTest, onDeleteTests, showTests = false }) => {
    const isSelected = selectedIds.includes(folder.id);
    const isExpanded = expandedIds.includes(folder.id);
    const [contextMenu, setContextMenu] = useState(null);
    const [modal, setModal] = useState(null);
    const [isDragOver, setIsDragOver] = useState(false);
    const menuBtnRef = useRef(null);

    const totalTests = countTests(folder);

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
        }
    };

    const handleAddBtn = (e) => { e.preventDefault(); e.stopPropagation(); handleAction('create'); };
    const handleMenuBtn = (e) => {
        e.preventDefault(); e.stopPropagation();
        const rect = menuBtnRef.current.getBoundingClientRect();
        setContextMenu({ x: rect.left, y: rect.bottom + 4 });
    };

    const hasChildren = (folder.sub_folders && folder.sub_folders.length > 0) || (showTests && folder.test_cases && folder.test_cases.length > 0);

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

            {contextMenu && <ContextMenu x={contextMenu.x} y={contextMenu.y} folderName={folder.name} isMulti={selectedIds.length > 1 && isSelected} onClose={() => setContextMenu(null)} onAction={handleAction} />}
            {modal && <Modal {...modal} onCancel={() => setModal(null)} />}

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
