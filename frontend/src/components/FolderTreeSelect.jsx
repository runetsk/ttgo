import React, { useState, useEffect, useRef } from 'react';

/**
 * FolderTreeSelect — A searchable, hierarchical folder picker dropdown.
 * Extracted from AIGeneratePage so it can be reused across the app.
 *
 * Props:
 *   folders  — flat array of { id, name, depth } from flattenFolderTree()
 *   value    — currently selected folder ID
 *   onChange — (id: string) => void
 *   disabled — boolean
 */
export default function FolderTreeSelect({ folders, value, onChange, disabled }) {
    const [open, setOpen] = useState(false);
    const [search, setSearch] = useState('');
    const [expanded, setExpanded] = useState(() => new Set(folders.map(f => f.id)));
    const containerRef = useRef(null);
    const searchRef = useRef(null);

    useEffect(() => {
        setExpanded(new Set(folders.map(f => f.id)));
    }, [folders.length]);

    useEffect(() => {
        if (!open) return;
        const handler = (e) => {
            if (containerRef.current && !containerRef.current.contains(e.target)) {
                setOpen(false);
                setSearch('');
            }
        };
        document.addEventListener('mousedown', handler);
        return () => document.removeEventListener('mousedown', handler);
    }, [open]);

    useEffect(() => {
        if (open) setTimeout(() => searchRef.current?.focus(), 10);
    }, [open]);

    const hasChildren = (idx) => idx < folders.length - 1 && folders[idx + 1].depth > folders[idx].depth;

    const getPath = (targetId) => {
        const idx = folders.findIndex(f => f.id === targetId);
        if (idx === -1) return [];
        const path = [folders[idx]];
        let depth = folders[idx].depth;
        for (let i = idx - 1; i >= 0 && depth > 0; i--) {
            if (folders[i].depth === depth - 1) { path.unshift(folders[i]); depth--; }
        }
        return path;
    };

    const selectedPath = value ? getPath(value) : [];
    const searchLower = search.toLowerCase().trim();

    const itemsToRender = searchLower
        ? folders.filter(f => f.name.toLowerCase().includes(searchLower))
        : folders.filter((f, idx) => {
            if (f.depth === 0) return true;
            let d = f.depth;
            for (let i = idx - 1; i >= 0 && d > 0; i--) {
                if (folders[i].depth === d - 1) {
                    if (!expanded.has(folders[i].id)) return false;
                    d--;
                }
            }
            return true;
        });

    const toggleExpanded = (id, e) => {
        e.stopPropagation();
        setExpanded(prev => { const n = new Set(prev); n.has(id) ? n.delete(id) : n.add(id); return n; });
    };

    const select = (id) => { onChange(id); setOpen(false); setSearch(''); };

    return (
        <div ref={containerRef} style={{ position: 'relative' }}>
            {/* Trigger */}
            <button
                type="button"
                onClick={() => !disabled && setOpen(o => !o)}
                disabled={disabled}
                className="folder-tree-trigger"
                style={{
                    width: '100%', display: 'flex', alignItems: 'center', gap: 8,
                    padding: '0 10px', height: 36, borderRadius: 7,
                    border: `1px solid ${open ? '#6366f1' : 'var(--border-color)'}`,
                    background: 'var(--bg-secondary)',
                    cursor: disabled ? 'not-allowed' : 'pointer',
                    opacity: disabled ? 0.5 : 1,
                    fontFamily: 'inherit', textAlign: 'left',
                    transition: 'border-color 0.15s', boxSizing: 'border-box', outline: 'none',
                }}
            >
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ color: 'var(--text-secondary)', flexShrink: 0 }}>
                    <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
                </svg>
                <span style={{ flex: 1, display: 'flex', alignItems: 'center', gap: 3, minWidth: 0, overflow: 'hidden' }}>
                    {selectedPath.length > 0 ? selectedPath.map((p, i) => (
                        <React.Fragment key={p.id}>
                            {i > 0 && <svg width="9" height="9" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" style={{ color: 'var(--text-secondary)', flexShrink: 0, opacity: 0.4 }}><polyline points="9 18 15 12 9 6"/></svg>}
                            <span style={{
                                fontSize: '0.85rem',
                                color: i === selectedPath.length - 1 ? 'var(--text-primary)' : 'var(--text-secondary)',
                                fontWeight: i === selectedPath.length - 1 ? 500 : 400,
                                flexShrink: i < selectedPath.length - 1 ? 0 : 1,
                                overflow: i === selectedPath.length - 1 ? 'hidden' : 'visible',
                                textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                            }}>{p.name}</span>
                        </React.Fragment>
                    )) : (
                        <span style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', opacity: 0.5 }}>Select folder…</span>
                    )}
                </span>
                <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" style={{ color: 'var(--text-secondary)', flexShrink: 0, transition: 'transform 0.15s', transform: open ? 'rotate(180deg)' : 'rotate(0deg)' }}>
                    <polyline points="6 9 12 15 18 9"/>
                </svg>
            </button>

            {/* Dropdown panel */}
            {open && (
                <div style={{
                    position: 'absolute', top: 'calc(100% + 4px)', left: 0, right: 0, zIndex: 200,
                    borderRadius: 8, border: '1px solid var(--border-color)',
                    background: 'var(--bg-secondary)', boxShadow: '0 8px 28px rgba(0,0,0,0.28)',
                    overflow: 'hidden',
                }}>
                    {/* Search box */}
                    <div style={{ padding: '7px 8px 6px', borderBottom: '1px solid var(--border-color)' }}>
                        <div style={{ position: 'relative' }}>
                            <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ position: 'absolute', left: 8, top: '50%', transform: 'translateY(-50%)', color: 'var(--text-secondary)', pointerEvents: 'none' }}>
                                <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
                            </svg>
                            <input
                                ref={searchRef}
                                value={search}
                                onChange={e => setSearch(e.target.value)}
                                placeholder="Search folders…"
                                onKeyDown={e => e.key === 'Escape' && (search ? setSearch('') : setOpen(false))}
                                style={{
                                    width: '100%', padding: '5px 8px 5px 27px',
                                    borderRadius: 5, border: '1px solid var(--border-color)',
                                    background: 'rgba(255,255,255,0.04)', color: 'var(--text-primary)',
                                    fontSize: '0.82rem', fontFamily: 'inherit', outline: 'none', boxSizing: 'border-box',
                                }}
                            />
                        </div>
                    </div>

                    {/* Folder list */}
                    <div style={{ maxHeight: 260, overflowY: 'auto', padding: '4px 0' }}>
                        {folders.length === 0 ? (
                            <div style={{ padding: '14px 12px', fontSize: '0.83rem', color: 'var(--text-secondary)', textAlign: 'center' }}>No folders available</div>
                        ) : itemsToRender.length === 0 ? (
                            <div style={{ padding: '14px 12px', fontSize: '0.83rem', color: 'var(--text-secondary)', textAlign: 'center' }}>No folders match "{search}"</div>
                        ) : itemsToRender.map(f => {
                            const folderIdx = folders.indexOf(f);
                            const isLeaf = !hasChildren(folderIdx);
                            const isExpanded = expanded.has(f.id);
                            const isSelected = f.id === value;
                            const breadcrumb = searchLower ? getPath(f.id) : null;

                            return (
                                <div key={f.id} style={{ display: 'flex', alignItems: 'center', paddingLeft: searchLower ? 8 : (f.depth * 18 + 8), paddingRight: 8 }}>
                                    {/* Expand/collapse toggle (tree mode only) */}
                                    {!searchLower && (
                                        <div style={{ width: 16, flexShrink: 0 }}>
                                            {!isLeaf && (
                                                <button
                                                    type="button"
                                                    onClick={e => toggleExpanded(f.id, e)}
                                                    className="folder-tree-expand"
                                                    style={{
                                                        width: 16, height: 16, display: 'flex', alignItems: 'center', justifyContent: 'center',
                                                        background: 'none', border: 'none', cursor: 'pointer',
                                                        color: 'var(--text-secondary)', padding: 0, borderRadius: 3,
                                                    }}
                                                >
                                                    <svg width="8" height="8" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round" style={{ transition: 'transform 0.15s', transform: isExpanded ? 'rotate(90deg)' : 'rotate(0deg)' }}>
                                                        <polyline points="9 18 15 12 9 6"/>
                                                    </svg>
                                                </button>
                                            )}
                                        </div>
                                    )}
                                    {/* Folder row button */}
                                    <button
                                        type="button"
                                        onClick={() => select(f.id)}
                                        className="folder-tree-row"
                                        style={{
                                            flex: 1, display: 'flex', alignItems: 'center', gap: 6,
                                            padding: '5px 6px', borderRadius: 5, border: 'none',
                                            background: isSelected ? 'rgba(99,102,241,0.12)' : 'none',
                                            cursor: 'pointer', fontFamily: 'inherit', textAlign: 'left',
                                            color: isSelected ? '#818cf8' : 'var(--text-primary)',
                                            transition: 'background 0.1s', minWidth: 0,
                                        }}
                                    >
                                        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ flexShrink: 0, color: isSelected ? '#818cf8' : '#6366f1', opacity: 0.75 }}>
                                            <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
                                        </svg>
                                        <span style={{ flex: 1, minWidth: 0 }}>
                                            {searchLower && breadcrumb && breadcrumb.length > 1 ? (
                                                <span style={{ display: 'flex', alignItems: 'baseline', flexWrap: 'wrap', gap: 2 }}>
                                                    {breadcrumb.slice(0, -1).map((p, bi) => (
                                                        <span key={p.id} style={{ fontSize: '0.73rem', color: 'var(--text-secondary)', opacity: 0.65 }}>
                                                            {bi > 0 ? '/ ' : ''}{p.name}{' '}
                                                        </span>
                                                    ))}
                                                    <span style={{ fontSize: '0.73rem', color: 'var(--text-secondary)', opacity: 0.65 }}>/ </span>
                                                    <span style={{ fontSize: '0.83rem', fontWeight: 500 }}>{f.name}</span>
                                                </span>
                                            ) : (
                                                <span style={{ fontSize: '0.83rem', fontWeight: f.depth === 0 ? 600 : 400, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', display: 'block' }}>
                                                    {f.name}
                                                </span>
                                            )}
                                        </span>
                                        {isSelected && (
                                            <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" style={{ flexShrink: 0, color: '#818cf8' }}>
                                                <polyline points="20 6 9 17 4 12"/>
                                            </svg>
                                        )}
                                    </button>
                                </div>
                            );
                        })}
                    </div>
                </div>
            )}

            <style>{`
                .folder-tree-row:hover {
                    background: rgba(99,102,241,0.07) !important;
                }
                .folder-tree-trigger:not(:disabled):hover {
                    border-color: rgba(99,102,241,0.5) !important;
                }
                .folder-tree-expand:hover {
                    background: rgba(255,255,255,0.08) !important;
                }
            `}</style>
        </div>
    );
}
