import React, { useState, useEffect, useRef } from 'react';
import { requirements as reqApi } from '../api';

/**
 * RequirementLinkPanel
 *
 * Displays requirements linked to a test case with the ability to:
 *  - Remove an existing link (× chip button)
 *  - Search all requirements and add a link from a dropdown
 *  - Inline-create a new requirement when the search term finds no match
 *
 * Props:
 *   testCaseId {string} — the ID of the test case whose links are managed
 */
export default function RequirementLinkPanel({ testCaseId }) {
    const [linked, setLinked] = useState([]);
    const [allReqs, setAllReqs] = useState([]);
    const [search, setSearch] = useState('');
    const [dropdownOpen, setDropdownOpen] = useState(false);
    const [loading, setLoading] = useState(true);

    // Inline-create form state
    const [creating, setCreating] = useState(false);
    const [newId, setNewId] = useState('');
    const [newTitle, setNewTitle] = useState('');
    const [newDesc, setNewDesc] = useState('');
    const [saving, setSaving] = useState(false);

    const searchRef = useRef(null);
    const dropdownRef = useRef(null);

    // Load linked requirements and all requirements on mount / testCaseId change.
    useEffect(() => {
        if (!testCaseId) return;
        if (!linked.length && !allReqs.length) setLoading(true);
        Promise.all([
            reqApi.listByTestCase(testCaseId),
            reqApi.list(),
        ])
            .then(([linked, all]) => {
                setLinked(linked || []);
                setAllReqs(all || []);
            })
            .catch(() => {})
            .finally(() => setLoading(false));
    }, [testCaseId]);

    // Close dropdown when clicking outside.
    useEffect(() => {
        const handler = (e) => {
            if (
                dropdownRef.current && !dropdownRef.current.contains(e.target) &&
                searchRef.current && !searchRef.current.contains(e.target)
            ) {
                setDropdownOpen(false);
            }
        };
        document.addEventListener('mousedown', handler);
        return () => document.removeEventListener('mousedown', handler);
    }, []);

    const linkedIds = new Set(linked.map(r => r.id));

    // Requirements that match the search term and are not already linked.
    const filteredReqs = allReqs.filter(r =>
        !linkedIds.has(r.id) &&
        (
            r.identifier.toLowerCase().includes(search.toLowerCase()) ||
            r.title.toLowerCase().includes(search.toLowerCase())
        )
    );

    const showCreate = search.trim() !== '' && filteredReqs.length === 0;

    const handleRemoveLink = (reqId) => {
        reqApi.deleteLink(reqId, testCaseId)
            .then(() => setLinked(prev => prev.filter(r => r.id !== reqId)))
            .catch(() => {});
    };

    const handleAddLink = (req) => {
        reqApi.createLink(req.id, testCaseId)
            .then(() => {
                setLinked(prev => [...prev, req]);
                setSearch('');
                setDropdownOpen(false);
            })
            .catch(() => {});
    };

    const handleCreateAndLink = () => {
        if (!newId.trim() || !newTitle.trim()) return;
        setSaving(true);
        reqApi.create({ identifier: newId.trim(), title: newTitle.trim(), description: newDesc.trim() })
            .then(created =>
                reqApi.createLink(created.id, testCaseId).then(() => {
                    setLinked(prev => [...prev, created]);
                    setAllReqs(prev => [...prev, created]);
                    setNewId(''); setNewTitle(''); setNewDesc('');
                    setCreating(false);
                    setSearch('');
                    setDropdownOpen(false);
                })
            )
            .catch(() => {})
            .finally(() => setSaving(false));
    };

    return (
        <div style={{ marginTop: 24 }}>
            <h4 style={{ margin: '0 0 10px', color: 'var(--text-secondary)', fontSize: '0.85rem', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                Linked Requirements
            </h4>

            {/* Linked chips */}
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginBottom: linked.length > 0 ? 10 : 0 }}>
                {loading && <span style={{ color: 'var(--text-secondary)', fontSize: '0.85em' }}>Loading…</span>}
                {!loading && linked.length === 0 && (
                    <span style={{ color: 'var(--text-secondary)', fontSize: '0.85em', fontStyle: 'italic' }}>No requirements linked yet.</span>
                )}
                {linked.map(req => (
                    <span
                        key={req.id}
                        className="meta-chip"
                        style={{ background: 'rgba(99,102,241,0.15)', border: '1px solid rgba(99,102,241,0.35)' }}
                        title={req.title}
                    >
                        <span style={{ fontWeight: 600, marginRight: 4 }}>{req.identifier}</span>
                        <span style={{ color: 'var(--text-secondary)', fontSize: '0.85em' }}>{req.title}</span>
                        <button
                            className="meta-chip-remove"
                            onClick={() => handleRemoveLink(req.id)}
                            title="Remove link"
                        >×</button>
                    </span>
                ))}
            </div>

            {/* Search / add row */}
            {!creating && (
                <div style={{ position: 'relative', display: 'inline-block', width: '100%', maxWidth: 380 }}>
                    <input
                        ref={searchRef}
                        className="modern-input"
                        style={{ width: '100%', fontSize: '0.85rem' }}
                        placeholder="Search or add requirement…"
                        value={search}
                        onChange={e => { setSearch(e.target.value); setDropdownOpen(true); }}
                        onFocus={() => setDropdownOpen(true)}
                    />
                    {dropdownOpen && (filteredReqs.length > 0 || showCreate) && (
                        <div
                            ref={dropdownRef}
                            style={{
                                position: 'absolute', top: '100%', left: 0, right: 0, zIndex: 50,
                                background: 'var(--surface-bg, #1e1e2e)', border: '1px solid var(--border-color)',
                                borderRadius: 6, boxShadow: '0 4px 16px rgba(0,0,0,0.4)', marginTop: 2, maxHeight: 220, overflowY: 'auto',
                            }}
                        >
                            {filteredReqs.map(req => (
                                <div
                                    key={req.id}
                                    style={{ padding: '8px 12px', cursor: 'pointer', fontSize: '0.85rem', display: 'flex', gap: 8, alignItems: 'center' }}
                                    onMouseDown={() => handleAddLink(req)}
                                    onMouseEnter={e => e.currentTarget.style.background = 'rgba(255,255,255,0.05)'}
                                    onMouseLeave={e => e.currentTarget.style.background = 'transparent'}
                                >
                                    <span style={{ fontWeight: 600, color: 'var(--accent-purple, #a78bfa)', minWidth: 80 }}>{req.identifier}</span>
                                    <span style={{ color: 'var(--text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{req.title}</span>
                                </div>
                            ))}
                            {showCreate && (
                                <div
                                    style={{ padding: '8px 12px', cursor: 'pointer', fontSize: '0.85rem', color: 'var(--accent-green, #34d399)', borderTop: filteredReqs.length > 0 ? '1px solid var(--border-color)' : 'none' }}
                                    onMouseDown={() => {
                                        setNewId(search.trim());
                                        setCreating(true);
                                        setDropdownOpen(false);
                                    }}
                                >
                                    + Create new requirement "{search.trim()}"
                                </div>
                            )}
                        </div>
                    )}
                </div>
            )}

            {/* Inline create form */}
            {creating && (
                <div className="glass-panel" style={{ padding: 14, marginTop: 8, border: '1px solid var(--border-color)' }}>
                    <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 8 }}>
                        <input
                            className="modern-input"
                            style={{ width: 130, fontSize: '0.85rem' }}
                            placeholder="Identifier *"
                            value={newId}
                            onChange={e => setNewId(e.target.value)}
                        />
                        <input
                            className="modern-input"
                            style={{ flex: 1, minWidth: 180, fontSize: '0.85rem' }}
                            placeholder="Title *"
                            value={newTitle}
                            onChange={e => setNewTitle(e.target.value)}
                        />
                    </div>
                    <input
                        className="modern-input"
                        style={{ width: '100%', fontSize: '0.85rem', marginBottom: 8 }}
                        placeholder="Description (optional)"
                        value={newDesc}
                        onChange={e => setNewDesc(e.target.value)}
                    />
                    <div style={{ display: 'flex', gap: 8 }}>
                        <button
                            className="primary-btn"
                            style={{ fontSize: '0.82rem', padding: '4px 14px' }}
                            onClick={handleCreateAndLink}
                            disabled={saving || !newId.trim() || !newTitle.trim()}
                        >
                            {saving ? 'Saving…' : 'Create & Link'}
                        </button>
                        <button
                            className="action-btn"
                            style={{ fontSize: '0.82rem', padding: '4px 10px' }}
                            onClick={() => { setCreating(false); setSearch(''); }}
                        >
                            Cancel
                        </button>
                    </div>
                </div>
            )}
        </div>
    );
}
