import React, { useState, useEffect, useMemo, useRef } from 'react';
import { createTestRun, getRunFolderTree } from '../api';

// Flatten a folder tree into indented options for display
function flattenFolderTree(folders, depth = 0) {
    const result = [];
    for (const f of (folders || [])) {
        result.push({ id: f.id, name: f.name, depth });
        if (f.sub_folders && f.sub_folders.length > 0) {
            result.push(...flattenFolderTree(f.sub_folders, depth + 1));
        }
    }
    return result;
}

export default function CreateRunModal({ categories, onClose, onSuccess, defaultFolderId = null }) {
    const [categoryId, setCategoryId] = useState("");
    const [name, setName] = useState("");
    const [runFolderId, setRunFolderId] = useState(defaultFolderId);
    const [folderTree, setFolderTree] = useState([]);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState(null);
    const nameRef = useRef(null);

    useEffect(() => {
        getRunFolderTree()
            .then(data => setFolderTree(data.run_folders || []))
            .catch(() => setFolderTree([]));
    }, []);

    useEffect(() => {
        setRunFolderId(defaultFolderId);
    }, [defaultFolderId]);

    // Auto-focus the name input after mount
    useEffect(() => {
        setTimeout(() => nameRef.current?.focus(), 100);
    }, []);

    const flatFolders = useMemo(() => flattenFolderTree(folderTree), [folderTree]);

    const selectedCategory = categories.find(s => s.id === categoryId);
    const selectedFolder = flatFolders.find(f => f.id === runFolderId);

    const handleSubmit = (e) => {
        e.preventDefault();
        setLoading(true);
        setError(null);
        createTestRun(categoryId || null, name, runFolderId || null)
            .then(() => {
                setLoading(false);
                onSuccess();
            })
            .catch(err => {
                setLoading(false);
                setError(err?.response?.data?.error || "Failed to create run");
            });
    };

    const handleKeyDown = (e) => {
        if (e.key === 'Escape') onClose();
    };

    return (
        <div className="modal-overlay" onClick={onClose} onKeyDown={handleKeyDown}>
            <div className="modal-content" onClick={e => e.stopPropagation()} style={{ maxWidth: 480, padding: 0, overflow: 'hidden' }}>

                {/* Header */}
                <div style={{
                    padding: '20px 24px 16px',
                    borderBottom: '1px solid var(--border-color)',
                    display: 'flex', alignItems: 'center', gap: 12,
                }}>
                    <div style={{
                        width: 36, height: 36, borderRadius: 9,
                        background: 'linear-gradient(135deg, #6366f1, #8b5cf6)',
                        display: 'flex', alignItems: 'center', justifyContent: 'center',
                        fontSize: '1rem', flexShrink: 0, color: '#fff',
                    }}>&#9654;</div>
                    <div style={{ flex: 1 }}>
                        <h3 style={{ margin: 0, fontSize: '1.05rem', fontWeight: 700, color: 'var(--text-primary)' }}>
                            New Test Run
                        </h3>
                        <p style={{ margin: 0, fontSize: '0.78rem', color: 'var(--text-secondary)', marginTop: 1 }}>
                            Create a new test execution run
                        </p>
                    </div>
                    <button
                        onClick={onClose}
                        style={{
                            background: 'none', border: 'none',
                            color: 'var(--text-secondary)', fontSize: '1.2rem',
                            cursor: 'pointer', padding: '4px 8px', borderRadius: 6,
                            lineHeight: 1,
                        }}
                        title="Close"
                    >&times;</button>
                </div>

                {/* Body */}
                <form onSubmit={handleSubmit}>
                    <div style={{ padding: '20px 24px', display: 'flex', flexDirection: 'column', gap: 16 }}>

                        {error && (
                            <div style={{
                                padding: '10px 14px', borderRadius: 8,
                                background: 'rgba(239,68,68,0.08)',
                                border: '1px solid rgba(239,68,68,0.25)',
                                color: '#ef4444', fontSize: '0.83rem',
                            }}>
                                {error}
                            </div>
                        )}

                        {/* Run Name */}
                        <div>
                            <label style={labelStyle}>
                                Run Name
                            </label>
                            <input
                                ref={nameRef}
                                type="text"
                                className="modern-input"
                                style={{ width: '100%' }}
                                placeholder="e.g. Sprint 42 Regression"
                                value={name}
                                onChange={e => setName(e.target.value)}
                                data-testid="create-run-name-input"
                            />
                            <p style={hintStyle}>Leave blank for auto-generated name</p>
                        </div>

                        {/* Category + Folder — side by side */}
                        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
                            {/* Category */}
                            <div>
                                <label style={labelStyle}>
                                    Category
                                </label>
                                <select
                                    className="modern-select"
                                    value={categoryId}
                                    onChange={e => setCategoryId(e.target.value)}
                                    style={{ width: '100%' }}
                                    data-testid="create-run-category-select"
                                >
                                    <option value="">None (empty run)</option>
                                    {(categories || []).map(s => (
                                        <option key={s.id} value={s.id}>{s.name}</option>
                                    ))}
                                </select>
                                <p style={hintStyle}>
                                    {categoryId ? 'Pre-populates with category tests' : 'Add tests manually later'}
                                </p>
                            </div>

                            {/* Folder */}
                            <div>
                                <label style={labelStyle}>
                                    Folder
                                </label>
                                <select
                                    className="modern-select"
                                    value={runFolderId || ""}
                                    onChange={e => setRunFolderId(e.target.value || null)}
                                    style={{ width: '100%' }}
                                    data-testid="create-run-folder-select"
                                >
                                    <option value="">Uncategorised</option>
                                    {flatFolders.map(f => (
                                        <option key={f.id} value={f.id}>
                                            {'\u00A0\u00A0'.repeat(f.depth)}{f.depth > 0 ? '└ ' : ''}{f.name}
                                        </option>
                                    ))}
                                </select>
                                <p style={hintStyle}>Organise runs into folders</p>
                            </div>
                        </div>
                    </div>

                    {/* Footer */}
                    <div style={{
                        padding: '14px 24px',
                        borderTop: '1px solid var(--border-color)',
                        background: 'var(--bg-secondary)',
                        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                    }}>
                        {/* Summary preview */}
                        <div style={{ fontSize: '0.78rem', color: 'var(--text-secondary)', maxWidth: '55%' }}>
                            {selectedCategory ? (
                                <span>
                                    <strong style={{ color: 'var(--text-primary)' }}>{selectedCategory.name}</strong>
                                    {selectedFolder && (
                                        <span> &middot; {selectedFolder.name}</span>
                                    )}
                                </span>
                            ) : (
                                <span>Empty run{selectedFolder ? <span> &middot; {selectedFolder.name}</span> : null}</span>
                            )}
                        </div>

                        <div style={{ display: 'flex', gap: 8 }}>
                            <button
                                type="button"
                                className="action-btn"
                                onClick={onClose}
                                disabled={loading}
                                style={{ padding: '8px 18px', fontSize: '0.85rem' }}
                                data-testid="create-run-cancel"
                            >
                                Cancel
                            </button>
                            <button
                                type="submit"
                                className="primary-btn"
                                disabled={loading}
                                style={{
                                    padding: '8px 20px', fontSize: '0.85rem',
                                    opacity: loading ? 0.5 : 1,
                                }}
                                data-testid="create-run-submit"
                            >
                                {loading ? 'Creating...' : 'Create Run'}
                            </button>
                        </div>
                    </div>
                </form>
            </div>
        </div>
    );
}

const labelStyle = {
    display: 'block', marginBottom: 6,
    fontSize: '0.78rem', fontWeight: 600,
    color: 'var(--text-secondary)',
    textTransform: 'uppercase', letterSpacing: '0.04em',
};

const hintStyle = {
    margin: '5px 0 0', fontSize: '0.73rem',
    color: 'var(--text-tertiary, var(--text-secondary))',
    opacity: 0.7,
};
