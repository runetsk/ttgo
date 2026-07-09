import React, { useState, useEffect, useMemo } from 'react';
import { getFolderTree } from '../api';
import { collectTestIds, folderCheckState, toggleFolderSelection } from '../utils/treeSelection';

const EMPTY_SET = new Set();

export default function TestTreePicker({ selectedIds, onChange, lockedIds = EMPTY_SET }) {
    const [tree, setTree] = useState(null); // null = loading
    const [expanded, setExpanded] = useState(() => new Set());
    const [search, setSearch] = useState('');

    useEffect(() => {
        let cancelled = false;
        getFolderTree()
            .then(data => {
                if (cancelled) return;
                const folders = Array.isArray(data) ? data : (data?.folders || []);
                setTree(folders);
                const ids = new Set();
                const walk = (nodes) => (nodes || []).forEach(n => { ids.add(n.id); walk(n.sub_folders); });
                walk(folders);
                setExpanded(ids);
            })
            .catch(() => { if (!cancelled) setTree([]); });
        return () => { cancelled = true; };
    }, []);

    const toggleExpand = (id) => setExpanded(prev => {
        const next = new Set(prev);
        if (next.has(id)) next.delete(id); else next.add(id);
        return next;
    });

    const toggleTest = (id) => {
        if (lockedIds.has(id)) return; // locked tests (already in the run) can't be toggled
        const next = new Set(selectedIds);
        if (next.has(id)) next.delete(id); else next.add(id);
        onChange(next);
    };

    // When searching, collect all tests whose name matches, with their folder path.
    const matches = useMemo(() => {
        const q = search.trim().toLowerCase();
        if (!q || !tree) return null;
        const out = [];
        const walk = (nodes, path) => (nodes || []).forEach(n => {
            const p = [...path, n.name];
            (n.test_cases || []).forEach(t => {
                if ((t.name || '').toLowerCase().includes(q)) out.push({ test: t, path: p });
            });
            walk(n.sub_folders, p);
        });
        walk(tree, []);
        return out;
    }, [search, tree]);

    const rowBase = { display: 'flex', alignItems: 'center', gap: 8, padding: '4px 6px', borderRadius: 6, fontSize: '0.82rem' };

    // Test checkbox shared by the tree and search-result renders: locked tests
    // (already in the run) show checked + disabled and can't be toggled.
    const testCheckbox = (t) => {
        const locked = lockedIds.has(t.id);
        return (
            <input
                type="checkbox"
                data-testid={`test-tree-test-${t.id}`}
                checked={selectedIds.has(t.id) || locked}
                disabled={locked}
                onChange={() => toggleTest(t.id)}
            />
        );
    };
    const testRowStyle = (t, extra) => ({
        ...rowBase,
        ...extra,
        cursor: lockedIds.has(t.id) ? 'default' : 'pointer',
        opacity: lockedIds.has(t.id) ? 0.6 : 1,
        background: (selectedIds.has(t.id) || lockedIds.has(t.id)) ? 'rgba(99,102,241,0.08)' : 'transparent',
    });

    const renderFolder = (node, depth) => {
        const state = folderCheckState(node, selectedIds, lockedIds);
        const isOpen = expanded.has(node.id);
        const hasChildren = (node.sub_folders || []).length > 0 || (node.test_cases || []).length > 0;
        const testIds = collectTestIds(node);
        const allLocked = testIds.length > 0 && testIds.every(id => lockedIds.has(id));
        return (
            <div key={node.id}>
                <div style={{ ...rowBase, paddingLeft: depth * 16 + 6 }}>
                    <button type="button" onClick={() => toggleExpand(node.id)}
                        style={{ width: 14, background: 'none', border: 'none', cursor: 'pointer', color: 'var(--text-secondary)', padding: 0, visibility: hasChildren ? 'visible' : 'hidden' }}>
                        {isOpen ? '▾' : '▸'}
                    </button>
                    <input
                        type="checkbox"
                        data-testid={`test-tree-folder-${node.id}`}
                        checked={state === 'checked'}
                        ref={el => { if (el) el.indeterminate = state === 'partial'; }}
                        onChange={() => onChange(toggleFolderSelection(node, selectedIds, lockedIds))}
                        disabled={testIds.length === 0 || allLocked}
                    />
                    <span style={{ fontWeight: 600, color: 'var(--text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{node.name}</span>
                </div>
                {isOpen && (
                    <>
                        {(node.test_cases || []).map(t => (
                            <label key={t.id} style={testRowStyle(t, { paddingLeft: (depth + 1) * 16 + 24 })}>
                                {testCheckbox(t)}
                                <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{t.name}</span>
                            </label>
                        ))}
                        {(node.sub_folders || []).map(sub => renderFolder(sub, depth + 1))}
                    </>
                )}
            </div>
        );
    };

    return (
        <div data-testid="test-tree-picker">
            <input
                type="text"
                className="modern-input"
                placeholder="Search tests…"
                value={search}
                onChange={e => setSearch(e.target.value)}
                data-testid="test-tree-search"
                style={{ width: '100%', marginBottom: 6 }}
            />
            <div style={{ maxHeight: 340, overflowY: 'auto', border: '1px solid var(--border-color)', borderRadius: 8, padding: 4 }}>
                {tree === null ? (
                    <p style={{ padding: 10, fontSize: '0.82rem', color: 'var(--text-secondary)' }}>Loading tests…</p>
                ) : matches !== null ? (
                    matches.length === 0 ? (
                        <p style={{ padding: 10, fontSize: '0.82rem', color: 'var(--text-secondary)' }}>No tests match.</p>
                    ) : matches.map(({ test, path }) => (
                        <label key={test.id} style={testRowStyle(test)}>
                            {testCheckbox(test)}
                            <span style={{ overflow: 'hidden', minWidth: 0 }}>
                                <span style={{ color: 'var(--text-secondary)', fontSize: '0.72rem' }}>{path.slice(0, -1).join(' / ')}{path.length > 1 ? ' / ' : ''}</span>
                                {test.name}
                            </span>
                        </label>
                    ))
                ) : tree.length === 0 ? (
                    <p style={{ padding: 10, fontSize: '0.82rem', color: 'var(--text-secondary)' }}>No test folders yet.</p>
                ) : tree.map(node => renderFolder(node, 0))}
            </div>
            <p style={{ margin: '6px 0 0', fontSize: '0.73rem', color: 'var(--text-secondary)' }} data-testid="test-tree-selected-count">
                {selectedIds.size} test{selectedIds.size === 1 ? '' : 's'} selected
            </p>
        </div>
    );
}
