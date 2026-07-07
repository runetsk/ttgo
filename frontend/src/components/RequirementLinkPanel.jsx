import React, { useState, useEffect, useRef } from 'react';
import { Link } from 'react-router-dom';
import { requirements as reqApi } from '../api';

// Emphasise the first case-insensitive occurrence of `query` inside `text`.
function highlightMatch(text, query) {
    const t = text || '';
    const q = (query || '').trim();
    if (!q) return t;
    const idx = t.toLowerCase().indexOf(q.toLowerCase());
    if (idx === -1) return t;
    return (
        <>
            {t.slice(0, idx)}
            <span style={{ background: 'rgba(167,139,250,0.4)', borderRadius: 2 }}>{t.slice(idx, idx + q.length)}</span>
            {t.slice(idx + q.length)}
        </>
    );
}

// Small badge for imported requirements; manually-created ones get none.
// #2563eb reads on both the dark and light dropdown surfaces.
const SOURCE_BADGE = {
    jira: { label: 'Jira', color: '#2563eb' },
    confluence: { label: 'Confluence', color: '#2563eb' },
};

// --aig-tone-purple-fg is a theme-aware colored-text token (readable on both
// the light and dark dropdown backgrounds); the tints stay low-alpha so they
// work over either surface.
const ID_PILL = {
    fontWeight: 700, fontSize: '0.72rem', color: 'var(--aig-tone-purple-fg, #a78bfa)',
    background: 'rgba(139,92,246,0.14)', border: '1px solid rgba(139,92,246,0.3)',
    padding: '1px 7px', borderRadius: 5, flexShrink: 0, letterSpacing: '0.02em', whiteSpace: 'nowrap',
};

/**
 * RequirementLinkPanel
 *
 * Displays requirements linked to a test case with the ability to:
 *  - Remove an existing link (× chip button)
 *  - Search all requirements and add a link from a dropdown (rich rows:
 *    identifier pill + title + source badge + description preview, with
 *    match highlighting and keyboard navigation)
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
    const [activeIndex, setActiveIndex] = useState(0); // keyboard highlight

    // Inline-create form state
    const [creating, setCreating] = useState(false);
    const [newId, setNewId] = useState('');
    const [newTitle, setNewTitle] = useState('');
    const [newDesc, setNewDesc] = useState('');
    const [saving, setSaving] = useState(false);

    const searchRef = useRef(null);
    const dropdownRef = useRef(null);
    // Tracks whether requirements have ever loaded successfully, so the load effect
    // below can skip the full-loading spinner on refetches without needing
    // linked/allReqs themselves as dependencies — reading their .length directly
    // would give the effect new "missing deps" every time it sets them, and since
    // they're set inside this same effect, including them would loop.
    const hasLoadedRef = useRef(false);

    // Load linked requirements and all requirements on mount / testCaseId change.
    useEffect(() => {
        if (!testCaseId) return;
        // eslint-disable-next-line react-hooks/set-state-in-effect -- async load result: only shows the spinner on first load, before fetching linked/all requirements
        if (!hasLoadedRef.current) setLoading(true);
        Promise.all([
            reqApi.listByTestCase(testCaseId),
            reqApi.list(),
        ])
            .then(([linkedData, all]) => {
                hasLoadedRef.current = true;
                setLinked(linkedData || []);
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
    const itemCount = filteredReqs.length + (showCreate ? 1 : 0);
    const activeIdx = itemCount === 0 ? -1 : Math.min(activeIndex, itemCount - 1);

    // Keep the keyboard-highlighted row scrolled into view.
    useEffect(() => {
        if (!dropdownOpen) return;
        const el = dropdownRef.current?.querySelector('[data-active="true"]');
        if (el) el.scrollIntoView({ block: 'nearest' });
    }, [activeIdx, dropdownOpen]);

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
                setActiveIndex(0);
                setDropdownOpen(false);
            })
            .catch(() => {});
    };

    const startCreate = () => {
        setNewId(search.trim());
        setCreating(true);
        setDropdownOpen(false);
    };

    const handleKeyDown = (e) => {
        if (e.key === 'Escape') { setDropdownOpen(false); return; }
        if (!dropdownOpen && (e.key === 'ArrowDown' || e.key === 'ArrowUp')) {
            setDropdownOpen(true);
            return;
        }
        if (e.key === 'ArrowDown') {
            e.preventDefault();
            setActiveIndex(i => Math.min(i + 1, itemCount - 1));
        } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            setActiveIndex(i => Math.max(i - 1, 0));
        } else if (e.key === 'Enter') {
            if (activeIdx < 0) return;
            e.preventDefault();
            if (activeIdx < filteredReqs.length) handleAddLink(filteredReqs[activeIdx]);
            else if (showCreate) startCreate();
        }
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
                        <Link
                            to={`/requirements/${req.id}`}
                            data-testid={`linked-req-link-${req.id}`}
                            style={{ display: 'inline-flex', alignItems: 'baseline', gap: 4, textDecoration: 'none', color: 'inherit' }}
                        >
                            <span style={{ fontWeight: 600 }}>{req.identifier}</span>
                            <span style={{ color: 'var(--text-secondary)', fontSize: '0.85em' }}>{req.title}</span>
                        </Link>
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
                <div style={{ position: 'relative', display: 'inline-block', width: '100%', maxWidth: 460 }}>
                    <input
                        ref={searchRef}
                        className="modern-input"
                        style={{ width: '100%', fontSize: '0.85rem' }}
                        placeholder="Search requirements by ID or name…"
                        value={search}
                        data-testid="req-search-input"
                        onChange={e => { setSearch(e.target.value); setActiveIndex(0); setDropdownOpen(true); }}
                        onFocus={() => { setActiveIndex(0); setDropdownOpen(true); }}
                        onKeyDown={handleKeyDown}
                    />
                    {dropdownOpen && (filteredReqs.length > 0 || showCreate) && (
                        <div
                            ref={dropdownRef}
                            data-testid="req-options"
                            style={{
                                position: 'absolute', top: '100%', left: 0, right: 0, zIndex: 50,
                                background: 'var(--bg-secondary)', border: '1px solid var(--border-color)',
                                borderRadius: 8, boxShadow: '0 8px 24px rgba(0,0,0,0.18)', marginTop: 4, maxHeight: 320, overflowY: 'auto',
                            }}
                        >
                            {filteredReqs.map((req, i) => {
                                const active = i === activeIdx;
                                const badge = SOURCE_BADGE[req.source_type];
                                return (
                                    <div
                                        key={req.id}
                                        data-testid={`req-option-${req.id}`}
                                        data-active={active ? 'true' : 'false'}
                                        onMouseDown={() => handleAddLink(req)}
                                        onMouseEnter={() => setActiveIndex(i)}
                                        style={{
                                            padding: '7px 12px', cursor: 'pointer',
                                            display: 'flex', flexDirection: 'column', gap: 2,
                                            background: active ? 'rgba(99,102,241,0.16)' : 'transparent',
                                            borderLeft: `2px solid ${active ? 'var(--accent-indigo)' : 'transparent'}`,
                                        }}
                                    >
                                        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                                            <span style={ID_PILL}>{highlightMatch(req.identifier, search)}</span>
                                            <span style={{ flex: 1, minWidth: 0, color: 'var(--text-primary)', fontWeight: 500, fontSize: '0.85rem', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                                                {highlightMatch(req.title, search)}
                                            </span>
                                            {badge && (
                                                <span style={{
                                                    fontSize: '0.62rem', fontWeight: 700, padding: '1px 6px', borderRadius: 4,
                                                    textTransform: 'uppercase', letterSpacing: '0.03em', flexShrink: 0,
                                                    color: badge.color, background: badge.color + '1c', border: `1px solid ${badge.color}55`,
                                                }}>
                                                    {badge.label}
                                                </span>
                                            )}
                                        </div>
                                        {req.description && (
                                            <div style={{ color: 'var(--text-secondary)', fontSize: '0.76rem', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', paddingLeft: 2 }}>
                                                {req.description}
                                            </div>
                                        )}
                                    </div>
                                );
                            })}
                            {showCreate && (
                                <div
                                    data-testid="req-create-option"
                                    data-active={activeIdx === filteredReqs.length ? 'true' : 'false'}
                                    onMouseDown={startCreate}
                                    onMouseEnter={() => setActiveIndex(filteredReqs.length)}
                                    style={{
                                        padding: '8px 12px', cursor: 'pointer', fontSize: '0.83rem', color: 'var(--aig-tone-green-fg, #34d399)',
                                        borderTop: filteredReqs.length > 0 ? '1px solid var(--border-color)' : 'none',
                                        background: activeIdx === filteredReqs.length ? 'rgba(52,211,153,0.12)' : 'transparent',
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
