import React, { useState, useEffect, useMemo, useRef } from 'react';
import { createTestRun, getRunFolderTree } from '../api';
import TestTreePicker from './TestTreePicker';

// Flatten a run-folder tree into indented options for the placement dropdown.
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

export default function CreateRunModal({ onClose, onSuccess, defaultFolderId = null }) {
    const [name, setName] = useState("");
    const [runFolderId, setRunFolderId] = useState(defaultFolderId);
    const [folderTree, setFolderTree] = useState([]);
    const [selectedTestIds, setSelectedTestIds] = useState(new Set());
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState(null);
    const nameRef = useRef(null);

    useEffect(() => {
        getRunFolderTree()
            .then(data => setFolderTree(data.run_folders || []))
            .catch(() => setFolderTree([]));
    }, []);

    useEffect(() => { setRunFolderId(defaultFolderId); }, [defaultFolderId]);
    useEffect(() => { setTimeout(() => nameRef.current?.focus(), 100); }, []);

    const flatFolders = useMemo(() => flattenFolderTree(folderTree), [folderTree]);

    const create = (mode) => {
        setLoading(true);
        setError(null);
        const pickedIds = Array.from(selectedTestIds);
        createTestRun(null, name, runFolderId || null, pickedIds)
            .then((run) => { setLoading(false); onSuccess(run, mode); })
            .catch(err => { setLoading(false); setError(err?.response?.data?.error || "Failed to create run"); });
    };

    const handleKeyDown = (e) => { if (e.key === 'Escape') onClose(); };
    const noneSelected = selectedTestIds.size === 0;

    return (
        <div className="modal-overlay" onClick={onClose} onKeyDown={handleKeyDown}>
            <div className="modal-content" onClick={e => e.stopPropagation()} style={{ maxWidth: 680, width: '92vw', padding: 0, overflow: 'hidden', display: 'flex', flexDirection: 'column', maxHeight: '86vh' }}>

                {/* Header */}
                <div style={{ padding: '20px 24px 16px', borderBottom: '1px solid var(--border-color)', display: 'flex', alignItems: 'center', gap: 12 }}>
                    <div style={{ width: 36, height: 36, borderRadius: 9, background: 'linear-gradient(135deg, #6366f1, #8b5cf6)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '1rem', flexShrink: 0, color: '#fff' }}>&#9654;</div>
                    <div style={{ flex: 1 }}>
                        <h3 style={{ margin: 0, fontSize: '1.05rem', fontWeight: 700, color: 'var(--text-primary)' }}>New Test Run</h3>
                        <p style={{ margin: 0, fontSize: '0.78rem', color: 'var(--text-secondary)', marginTop: 1 }}>Pick tests to run, or create an empty run</p>
                    </div>
                    <button onClick={onClose} style={{ background: 'none', border: 'none', color: 'var(--text-secondary)', fontSize: '1.2rem', cursor: 'pointer', padding: '4px 8px', borderRadius: 6, lineHeight: 1 }} title="Close">&times;</button>
                </div>

                {/* Body */}
                <div style={{ padding: '20px 24px', display: 'flex', flexDirection: 'column', gap: 16, overflowY: 'auto' }}>
                    {error && (
                        <div style={{ padding: '10px 14px', borderRadius: 8, background: 'rgba(239,68,68,0.08)', border: '1px solid rgba(239,68,68,0.25)', color: '#ef4444', fontSize: '0.83rem' }}>{error}</div>
                    )}

                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
                        <div>
                            <label style={labelStyle}>Run Name</label>
                            <input ref={nameRef} type="text" className="modern-input" style={{ width: '100%' }} placeholder="e.g. Sprint 42 Regression" value={name} onChange={e => setName(e.target.value)} data-testid="create-run-name-input" />
                            <p style={hintStyle}>Leave blank for auto-generated name</p>
                        </div>
                        <div>
                            <label style={labelStyle}>Folder</label>
                            <select className="modern-select" value={runFolderId || ""} onChange={e => setRunFolderId(e.target.value || null)} style={{ width: '100%' }} data-testid="create-run-folder-select">
                                <option value="">Uncategorised</option>
                                {flatFolders.map(f => (
                                    <option key={f.id} value={f.id}>{'  '.repeat(f.depth)}{f.depth > 0 ? '└ ' : ''}{f.name}</option>
                                ))}
                            </select>
                            <p style={hintStyle}>Organise runs into folders</p>
                        </div>
                    </div>

                    <div>
                        <label style={labelStyle}>Tests</label>
                        <TestTreePicker selectedIds={selectedTestIds} onChange={setSelectedTestIds} />
                    </div>
                </div>

                {/* Footer */}
                <div style={{ padding: '14px 24px', borderTop: '1px solid var(--border-color)', background: 'var(--bg-secondary)', display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 8 }}>
                    <button type="button" className="action-btn" onClick={onClose} disabled={loading} style={{ padding: '8px 18px', fontSize: '0.85rem', marginRight: 'auto' }} data-testid="create-run-cancel">Cancel</button>
                    <button type="button" className="action-btn" onClick={() => create('detail')} disabled={loading} style={{ padding: '8px 20px', fontSize: '0.85rem', opacity: loading ? 0.5 : 1 }} data-testid="create-run-submit">
                        {loading ? 'Creating…' : 'Create Run'}
                    </button>
                    <button type="button" className="primary-btn" onClick={() => create('execute')} disabled={loading || noneSelected} title={noneSelected ? 'Select at least one test' : 'Create and start executing'} style={{ padding: '8px 20px', fontSize: '0.85rem', opacity: (loading || noneSelected) ? 0.5 : 1 }} data-testid="create-run-execute">
                        ▶ Manual execute
                    </button>
                </div>
            </div>
        </div>
    );
}

const labelStyle = { display: 'block', marginBottom: 6, fontSize: '0.78rem', fontWeight: 600, color: 'var(--text-secondary)', textTransform: 'uppercase', letterSpacing: '0.04em' };
const hintStyle = { margin: '5px 0 0', fontSize: '0.73rem', color: 'var(--text-tertiary, var(--text-secondary))', opacity: 0.7 };
