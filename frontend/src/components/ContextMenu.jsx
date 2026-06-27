import React, { useEffect, useRef } from 'react';

const ContextMenu = ({ x, y, onClose, onAction, folderName, isMulti, qtestEnabled }) => {
    const menuRef = useRef(null);
    useEffect(() => {
        const handleClickOutside = (e) => {
            if (menuRef.current && !menuRef.current.contains(e.target)) onClose();
        };
        document.addEventListener('mousedown', handleClickOutside);
        return () => document.removeEventListener('mousedown', handleClickOutside);
    }, [onClose]);
    return (
        <div className="context-menu" ref={menuRef} style={{ top: y, left: x }} onClick={(e) => e.stopPropagation()}>
            <div style={{ padding: '8px 12px', fontSize: '0.7em', color: 'var(--text-secondary)', textTransform: 'uppercase', letterSpacing: '0.05em', borderBottom: '1px solid var(--border-color)', marginBottom: 4 }}>
                {isMulti ? "Multiple Folders" : folderName}
            </div>
            {!isMulti && (
                <div className="context-menu-item" onClick={() => onAction('create')} data-testid="context-menu-create-subfolder">
                    <span>➕</span> New Subfolder
                </div>
            )}
            {!isMulti && (
                <div className="context-menu-item" onClick={() => onAction('rename')} data-testid="context-menu-rename-folder">
                    <span>✏️</span> Rename
                </div>
            )}
            {!isMulti && qtestEnabled && (
                <div className="context-menu-item" onClick={() => onAction('qtest-upload')} data-testid="context-menu-qtest-upload">
                    <span>⬆</span> Upload to QTest
                </div>
            )}
            {!isMulti && qtestEnabled && (
                <div className="context-menu-item" onClick={() => onAction('qtest-unlink')} data-testid="context-menu-qtest-unlink">
                    <span>🔗</span> Unlink from QTest
                </div>
            )}
            <div className="context-menu-item danger" onClick={() => onAction('delete')} data-testid="context-menu-delete-folder">
                <span>🗑️</span> {isMulti ? "Delete Selected" : "Delete Folder"}
            </div>
        </div>
    );
};
export default ContextMenu;
