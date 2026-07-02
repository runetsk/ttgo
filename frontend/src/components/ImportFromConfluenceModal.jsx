import React, { useState, useEffect, useRef, useCallback } from 'react';
import { confluence as confApi, requirements as reqApi } from '../api';
import { toast } from '../toast';
import ModalShell from './shared/ModalShell';
import ErrorAlert from './shared/ErrorAlert';
import SafeHTML from './shared/SafeHTML';
import { labelStyle, alreadyImportedBadgeStyle } from './shared/styles';

/**
 * ImportFromConfluenceModal — multi-step modal for importing a requirement from a Confluence page.
 * 011-jira-confluence-import (T020)
 *
 * Steps:
 *   1. Select a space + optional title/label filter
 *   2. Browse pages in a tree (lazy-loaded children) or flat filtered list
 *   3. Preview page content (sanitized HTML) + confirm import
 */

const STEP_META = [
    { num: 1, label: 'Space' },
    { num: 2, label: 'Page' },
    { num: 3, label: 'Preview' },
];

// ────────────────────────────────────────────────────────────────────
// Step indicator
// ────────────────────────────────────────────────────────────────────

function StepIndicator({ current }) {
    return (
        <div style={{
            display: 'flex', alignItems: 'center', gap: 0,
            padding: '0 0 20px', marginBottom: 4,
            borderBottom: '1px solid var(--border-color)',
        }}>
            {STEP_META.map((s, i) => {
                const isActive = s.num === current;
                const isDone = s.num < current;
                return (
                    <React.Fragment key={s.num}>
                        {i > 0 && (
                            <div style={{
                                flex: 1, height: 2, margin: '0 2px',
                                background: isDone ? 'var(--accent-teal, #14b8a6)' : 'var(--border-color)',
                                borderRadius: 1, transition: 'background 0.3s ease',
                            }} />
                        )}
                        <div style={{
                            display: 'flex', alignItems: 'center', gap: 8,
                            padding: '6px 14px', borderRadius: 20,
                            background: isActive ? 'rgba(99,102,241,0.12)' : isDone ? 'rgba(20,184,166,0.08)' : 'transparent',
                            transition: 'all 0.25s ease',
                        }}>
                            <div style={{
                                width: 24, height: 24, borderRadius: '50%',
                                display: 'flex', alignItems: 'center', justifyContent: 'center',
                                fontSize: '0.72rem', fontWeight: 700,
                                background: isActive ? 'linear-gradient(135deg, var(--accent-indigo), #4f46e5)' : isDone ? 'var(--accent-teal)' : 'var(--bg-tertiary)',
                                color: isActive || isDone ? '#fff' : 'var(--text-secondary)',
                                transition: 'all 0.25s ease',
                                boxShadow: isActive ? '0 2px 8px rgba(99,102,241,0.3)' : 'none',
                            }}>
                                {isDone ? '✓' : s.num}
                            </div>
                            <span style={{
                                fontSize: '0.78rem', fontWeight: isActive ? 600 : 500,
                                color: isActive ? 'var(--text-primary)' : isDone ? 'var(--accent-teal)' : 'var(--text-secondary)',
                                letterSpacing: '0.01em', transition: 'color 0.25s ease',
                            }}>
                                {s.label}
                            </span>
                        </div>
                    </React.Fragment>
                );
            })}
        </div>
    );
}

// ────────────────────────────────────────────────────────────────────
// Space card
// ────────────────────────────────────────────────────────────────────

function SpaceCard({ space, selected, onClick }) {
    const typeIcon = space.type === 'global' ? '🌐' : '👤';
    return (
        <div
            onClick={onClick}
            style={{
                padding: '12px 16px', borderRadius: 8,
                border: selected ? '1.5px solid var(--accent-indigo)' : '1px solid var(--border-color)',
                background: selected ? 'rgba(99,102,241,0.08)' : 'var(--bg-tertiary)',
                cursor: 'pointer', transition: 'all 0.15s ease',
                display: 'flex', alignItems: 'center', gap: 12,
            }}
            onMouseEnter={e => { if (!selected) { e.currentTarget.style.borderColor = 'rgba(99,102,241,0.4)'; e.currentTarget.style.background = 'rgba(99,102,241,0.04)'; }}}
            onMouseLeave={e => { if (!selected) { e.currentTarget.style.borderColor = 'var(--border-color)'; e.currentTarget.style.background = 'var(--bg-tertiary)'; }}}
        >
            <div style={{
                width: 36, height: 36, borderRadius: 8,
                background: selected ? 'linear-gradient(135deg, var(--accent-indigo), #4f46e5)' : 'rgba(255,255,255,0.06)',
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                fontSize: '1rem', transition: 'all 0.15s ease', flexShrink: 0,
            }}>{typeIcon}</div>
            <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontWeight: 600, fontSize: '0.88rem', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{space.name}</div>
                <div style={{ fontSize: '0.72rem', color: 'var(--text-secondary)', marginTop: 2 }}>{space.key}</div>
            </div>
            {selected && (
                <div style={{
                    width: 20, height: 20, borderRadius: '50%', background: 'var(--accent-indigo)',
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    fontSize: '0.65rem', color: '#fff', fontWeight: 700, flexShrink: 0,
                }}>✓</div>
            )}
        </div>
    );
}

// ────────────────────────────────────────────────────────────────────
// Tree node for a single page (lazy-loads children on expand)
// ────────────────────────────────────────────────────────────────────

function PageTreeNode({ page, depth, onSelect }) {
    const [expanded, setExpanded] = useState(false);
    const [children, setChildren] = useState(null); // null = not loaded yet
    const [loading, setLoading] = useState(false);
    const [cursor, setCursor] = useState(null);
    const [hovered, setHovered] = useState(false);
    const imported = page.already_imported;
    // After first load we know definitively whether there are children
    const knownEmpty = children !== null && children.length === 0 && !cursor;

    const toggleExpand = (e) => {
        e.stopPropagation();
        if (knownEmpty) return; // already expanded once, no children
        if (!expanded && children === null) {
            // First expand — lazy-load children
            setLoading(true);
            confApi.listChildPages(page.id, null, 50)
                .then(data => {
                    setChildren(data.pages || []);
                    setCursor(data.next_cursor || null);
                    // Auto-expand only if there are children
                    if ((data.pages || []).length > 0) {
                        setExpanded(true);
                    }
                })
                .catch(() => setChildren([]))
                .finally(() => setLoading(false));
            return; // don't toggle yet — wait for result
        }
        setExpanded(prev => !prev);
    };

    const loadMore = () => {
        if (!cursor || loading) return;
        setLoading(true);
        confApi.listChildPages(page.id, cursor, 50)
            .then(data => {
                setChildren(prev => [...(prev || []), ...(data.pages || [])]);
                setCursor(data.next_cursor || null);
            })
            .finally(() => setLoading(false));
    };

    const indent = depth * 20;

    return (
        <>
            <div
                onMouseEnter={() => setHovered(true)}
                onMouseLeave={() => setHovered(false)}
                style={{
                    display: 'flex', alignItems: 'center', gap: 0,
                    paddingLeft: 8 + indent,
                    paddingRight: 12,
                    minHeight: 36,
                    cursor: imported ? 'default' : 'pointer',
                    opacity: imported ? 0.45 : 1,
                    background: hovered && !imported ? 'rgba(99,102,241,0.05)' : 'transparent',
                    transition: 'background 0.1s ease',
                }}
            >
                {/* Expand/collapse toggle — always visible */}
                <div
                    onClick={toggleExpand}
                    style={{
                        width: 22, height: 22, flexShrink: 0,
                        display: 'flex', alignItems: 'center', justifyContent: 'center',
                        cursor: knownEmpty ? 'default' : 'pointer',
                        borderRadius: 4,
                        color: 'var(--text-secondary)',
                        fontSize: '0.6rem',
                        transition: 'background 0.1s ease',
                    }}
                    onMouseEnter={e => { if (!knownEmpty) e.currentTarget.style.background = 'rgba(255,255,255,0.08)'; }}
                    onMouseLeave={e => { e.currentTarget.style.background = 'transparent'; }}
                >
                    {loading ? (
                        <span style={{ fontSize: '0.65rem', opacity: 0.6 }}>⟳</span>
                    ) : knownEmpty ? (
                        <span style={{ opacity: 0.25, fontSize: '0.5rem' }}>•</span>
                    ) : (
                        <span style={{
                            display: 'inline-block',
                            transition: 'transform 0.15s ease',
                            transform: expanded ? 'rotate(90deg)' : 'rotate(0deg)',
                        }}>▶</span>
                    )}
                </div>

                {/* Page icon */}
                <div style={{
                    width: 18, height: 18, flexShrink: 0,
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    marginRight: 6, fontSize: '0.8rem',
                    opacity: 0.7,
                }}>
                    {expanded && children && children.length > 0 ? '📂' : knownEmpty ? '📄' : '📁'}
                </div>

                {/* Title — clicking selects the page for preview */}
                <div
                    onClick={() => !imported && onSelect(page)}
                    style={{
                        flex: 1, minWidth: 0,
                        padding: '6px 4px',
                        display: 'flex', alignItems: 'center', gap: 8,
                    }}
                >
                    <span style={{
                        fontWeight: 500, fontSize: '0.84rem',
                        whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
                        color: 'var(--text-primary)',
                    }}>
                        {page.title}
                    </span>

                    {imported ? (
                        <span style={{ ...alreadyImportedBadgeStyle, flexShrink: 0 }}>Imported</span>
                    ) : hovered && (
                        <span style={{
                            fontSize: '0.72rem', fontWeight: 600,
                            color: 'var(--accent-indigo)',
                            flexShrink: 0, marginLeft: 'auto',
                        }}>
                            Select →
                        </span>
                    )}
                </div>
            </div>

            {/* Children */}
            {expanded && (
                <div>
                    {loading && children === null && (
                        <div style={{
                            paddingLeft: 8 + indent + 22 + 18 + 6,
                            fontSize: '0.78rem', color: 'var(--text-secondary)',
                            padding: '6px 8px 6px ' + (8 + indent + 22) + 'px',
                        }}>
                            Loading…
                        </div>
                    )}
                    {children && children.map(child => (
                        <PageTreeNode
                            key={child.id}
                            page={child}
                            depth={depth + 1}
                            onSelect={onSelect}
                        />
                    ))}
                    {children && children.length === 0 && !loading && (
                        <div style={{
                            paddingLeft: 8 + indent + 22,
                            fontSize: '0.75rem', color: 'var(--text-secondary)',
                            padding: '4px 8px 4px ' + (8 + indent + 40) + 'px',
                            fontStyle: 'italic', opacity: 0.6,
                        }}>
                            No child pages
                        </div>
                    )}
                    {cursor && (
                        <div style={{ paddingLeft: 8 + indent + 40, padding: '4px 0 4px ' + (8 + indent + 40) + 'px' }}>
                            <button
                                onClick={loadMore}
                                disabled={loading}
                                style={{
                                    background: 'none', border: 'none', cursor: 'pointer',
                                    color: 'var(--accent-indigo)', fontSize: '0.75rem',
                                    fontWeight: 600, fontFamily: 'inherit', padding: '2px 0',
                                }}
                            >
                                {loading ? 'Loading…' : 'Load more…'}
                            </button>
                        </div>
                    )}
                </div>
            )}
        </>
    );
}

// ────────────────────────────────────────────────────────────────────
// Flat page row (used when filters are active — search mode)
// ────────────────────────────────────────────────────────────────────

function PageRow({ page, onSelect }) {
    const [hovered, setHovered] = useState(false);
    const imported = page.already_imported;
    return (
        <div
            onClick={() => !imported && onSelect(page)}
            onMouseEnter={() => setHovered(true)}
            onMouseLeave={() => setHovered(false)}
            style={{
                padding: '11px 16px',
                display: 'flex', alignItems: 'center', gap: 12,
                cursor: imported ? 'default' : 'pointer',
                opacity: imported ? 0.45 : 1,
                background: hovered && !imported ? 'rgba(99,102,241,0.05)' : 'transparent',
                borderBottom: '1px solid var(--border-color)',
                transition: 'background 0.12s ease',
            }}
        >
            <div style={{ fontSize: '0.8rem', opacity: 0.6, flexShrink: 0 }}>📄</div>
            <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{
                    fontWeight: 600, fontSize: '0.88rem',
                    whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
                }}>
                    {page.title}
                </div>
                <div style={{ fontSize: '0.72rem', color: 'var(--text-secondary)', marginTop: 2 }}>
                    {page.status || 'page'}
                </div>
            </div>
            {imported ? (
                <span style={alreadyImportedBadgeStyle}>Imported</span>
            ) : (
                <span style={{
                    fontSize: '0.75rem', fontWeight: 600,
                    color: hovered ? 'var(--accent-indigo)' : 'var(--text-secondary)',
                    transition: 'color 0.12s ease', flexShrink: 0,
                }}>
                    Select →
                </span>
            )}
        </div>
    );
}

// ────────────────────────────────────────────────────────────────────
// Main modal
// ────────────────────────────────────────────────────────────────────

export default function ImportFromConfluenceModal({ onClose, onImported, confluenceEnabled }) {
    const [step, setStep] = useState(1);

    // Step 1: Spaces
    const [spaces, setSpaces] = useState([]);
    const [spacesLoading, setSpacesLoading] = useState(false);
    const [selectedSpace, setSelectedSpace] = useState(null);
    const [spaceSearch, setSpaceSearch] = useState('');
    const [showFilters, setShowFilters] = useState(false);
    const [titleFilter, setTitleFilter] = useState('');

    // Step 2: Pages — all pages fetched from API (flat), tree built client-side
    const [pages, setPages] = useState([]);
    const [pagesLoading, setPagesLoading] = useState(false);
    const [pagesCursor, setPagesCursor] = useState(null);
    const isFilteredMode = !!titleFilter;

    // Step 3: Preview
    const [preview, setPreview] = useState(null);
    const [previewLoading, setPreviewLoading] = useState(false);

    // Import
    const [importing, setImporting] = useState(false);
    const [error, setError] = useState('');

    const spaceSearchRef = useRef(null);

    // Load spaces on mount
    useEffect(() => {
        if (!confluenceEnabled) return;
        // eslint-disable-next-line react-hooks/set-state-in-effect -- async load result: fetches Confluence spaces list
        setSpacesLoading(true);
        confApi.listSpaces(null, 100)
            .then(data => setSpaces(data.spaces || []))
            .catch(err => setError(err.response?.data?.error || err.message))
            .finally(() => setSpacesLoading(false));
    }, [confluenceEnabled]);

    useEffect(() => {
        if (step === 1 && spaceSearchRef.current) spaceSearchRef.current.focus();
    }, [step]);

    const filteredSpaces = spaces.filter(s => {
        if (!spaceSearch) return true;
        const q = spaceSearch.toLowerCase();
        return s.name.toLowerCase().includes(q) || s.key.toLowerCase().includes(q);
    });

    // In tree mode, derive root pages (no parent_id) from the flat list
    const rootPages = isFilteredMode
        ? pages
        : pages.filter(p => !p.parent_id);

    // Fetch pages
    const handleSearchPages = useCallback((cursor = null) => {
        if (!selectedSpace) return;
        setPagesLoading(true);
        setError('');
        confApi.listPages(selectedSpace.id, titleFilter || null, null, cursor, 250)
            .then(data => {
                if (cursor) {
                    setPages(prev => [...prev, ...(data.pages || [])]);
                } else {
                    setPages(data.pages || []);
                }
                setPagesCursor(data.next_cursor || null);
            })
            .catch(err => setError(err.response?.data?.error || err.message))
            .finally(() => setPagesLoading(false));
    }, [selectedSpace, titleFilter]);

    const handleSelectSpace = () => {
        if (!selectedSpace) return;
        setStep(2);
        setPages([]);
        setPagesCursor(null);
        handleSearchPages();
    };

    const handleSelectPage = (page) => {
        setStep(3);
        setPreviewLoading(true);
        setPreview(null);
        setError('');
        confApi.getPage(page.id)
            .then(data => setPreview(data))
            .catch(err => setError(err.response?.data?.error || err.message))
            .finally(() => setPreviewLoading(false));
    };

    const handleImport = () => {
        if (!preview) return;
        setImporting(true);
        setError('');
        reqApi.importSingle('confluence', preview.id)
            .then(() => {
                toast.success(`Imported requirement from Confluence page "${preview.title}"`);
                onImported?.();
                onClose();
            })
            .catch(err => {
                const data = err.response?.data;
                if (err.response?.status === 409 && data?.existing_id) {
                    setError('A requirement from this page already exists.');
                    setPreview(prev => prev ? { ...prev, already_imported: true, existing_requirement_id: data.existing_id } : prev);
                } else {
                    setError(data?.error || err.message || 'Import failed.');
                }
            })
            .finally(() => setImporting(false));
    };

    const goBack = () => {
        setError('');
        if (step === 3) setStep(2);
        else if (step === 2) setStep(1);
    };

    // ── Not configured state ──
    if (!confluenceEnabled) {
        return (
            <ModalShell title="Import from Confluence" width={440} onClose={onClose}
                footer={<button className="action-btn" onClick={onClose}>Close</button>}
            >
                <div style={{ textAlign: 'center', padding: '32px 24px' }}>
                    <div style={{
                        width: 48, height: 48, borderRadius: 12, margin: '0 auto 16px',
                        background: 'rgba(255,255,255,0.04)',
                        display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '1.4rem',
                    }}>🔗</div>
                    <p style={{ color: 'var(--text-secondary)', marginBottom: 16, lineHeight: 1.6 }}>
                        Confluence integration is not configured.<br />
                        Go to <strong style={{ color: 'var(--text-primary)' }}>Settings → Confluence Integration</strong> to set it up.
                    </p>
                </div>
            </ModalShell>
        );
    }

    return (
        <ModalShell
            title="Import from Confluence"
            width={step === 3 ? 860 : 640}
            maxHeight={step === 3 ? '85vh' : undefined}
            onClose={onClose}
            footer={<>
                {step > 1 && (
                    <button className="action-btn" onClick={goBack} disabled={importing}>← Back</button>
                )}
                <div style={{ flex: 1 }} />
                <button className="action-btn" onClick={onClose} disabled={importing}>Cancel</button>
                {step === 1 && (
                    <button className="primary-btn" onClick={handleSelectSpace} disabled={!selectedSpace}>Browse Pages</button>
                )}
                {step === 3 && preview && !preview.already_imported && (
                    <button className="primary-btn" onClick={handleImport} disabled={importing}>
                        {importing ? 'Importing…' : 'Import Requirement'}
                    </button>
                )}
            </>}
        >
            <StepIndicator current={step} />
            <ErrorAlert message={error} />

            {/* ── Step 1: Space selection ── */}
            {step === 1 && (
                <div>
                    <div style={{ position: 'relative', marginBottom: 14 }}>
                        <input
                            ref={spaceSearchRef}
                            className="modern-input"
                            style={{ width: '100%', paddingLeft: 36 }}
                            placeholder="Search spaces by name or key…"
                            value={spaceSearch}
                            onChange={e => setSpaceSearch(e.target.value)}
                        />
                        <span style={{
                            position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)',
                            fontSize: '0.85rem', opacity: 0.4, pointerEvents: 'none',
                        }}>🔍</span>
                    </div>

                    {spacesLoading ? (
                        <div style={{ textAlign: 'center', padding: '32px 20px', color: 'var(--text-secondary)', fontSize: '0.85rem' }}>
                            <div style={{ marginBottom: 8, fontSize: '1.2rem' }}>⏳</div>Loading spaces…
                        </div>
                    ) : filteredSpaces.length === 0 ? (
                        <div style={{ textAlign: 'center', padding: '28px 20px', color: 'var(--text-secondary)', fontSize: '0.85rem' }}>
                            {spaceSearch ? <>No spaces matching "<em>{spaceSearch}</em>"</> : 'No spaces found in your Confluence instance.'}
                        </div>
                    ) : (
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 6, maxHeight: 280, overflowY: 'auto', paddingRight: 4 }}>
                            {filteredSpaces.map(s => (
                                <SpaceCard key={s.id} space={s} selected={selectedSpace?.id === s.id} onClick={() => setSelectedSpace(s)} />
                            ))}
                        </div>
                    )}

                    {/* Collapsible filters */}
                    <div style={{ marginTop: 16 }}>
                        <button
                            onClick={() => setShowFilters(!showFilters)}
                            style={{
                                background: 'none', border: 'none', cursor: 'pointer',
                                color: 'var(--text-secondary)', fontSize: '0.78rem',
                                display: 'flex', alignItems: 'center', gap: 6,
                                padding: '4px 0', fontFamily: 'inherit', fontWeight: 500,
                            }}
                        >
                            <span style={{
                                display: 'inline-block', transition: 'transform 0.2s ease',
                                transform: showFilters ? 'rotate(90deg)' : 'rotate(0deg)', fontSize: '0.65rem',
                            }}>▶</span>
                            Advanced filters
                            {titleFilter && (
                                <span style={{
                                    background: 'var(--accent-indigo)', color: '#fff', fontSize: '0.6rem', fontWeight: 700,
                                    width: 16, height: 16, borderRadius: '50%',
                                    display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
                                }}>
                                    1
                                </span>
                            )}
                        </button>
                        {showFilters && (
                            <div style={{
                                marginTop: 10, padding: '14px',
                                background: 'rgba(255,255,255,0.02)',
                                border: '1px solid var(--border-color)', borderRadius: 8,
                            }}>
                                <label style={labelStyle}>Title filter</label>
                                <input className="modern-input" style={{ width: '100%' }} placeholder="Filter pages by exact title…"
                                    value={titleFilter} onChange={e => setTitleFilter(e.target.value)} />
                                <div style={{ fontSize: '0.72rem', color: 'var(--text-secondary)', marginTop: 6, opacity: 0.7 }}>
                                    When set, pages matching the title are shown as a flat list instead of a tree.
                                </div>
                            </div>
                        )}
                    </div>
                </div>
            )}

            {/* ── Step 2: Page tree / filtered list ── */}
            {step === 2 && (
                <div>
                    {/* Space context header */}
                    <div style={{
                        display: 'flex', alignItems: 'center', gap: 8,
                        marginBottom: 12, fontSize: '0.82rem',
                    }}>
                        <span style={{
                            background: 'rgba(99,102,241,0.1)', color: 'var(--accent-indigo)',
                            padding: '3px 10px', borderRadius: 6, fontWeight: 600, fontSize: '0.75rem',
                        }}>{selectedSpace?.key}</span>
                        <span style={{ color: 'var(--text-secondary)' }}>{selectedSpace?.name}</span>
                        {isFilteredMode && (
                            <span style={{ color: 'var(--text-secondary)', marginLeft: 'auto', fontSize: '0.75rem', fontStyle: 'italic' }}>
                                🔍 search results
                            </span>
                        )}
                        {!isFilteredMode && (
                            <span style={{ color: 'var(--text-secondary)', marginLeft: 'auto', fontSize: '0.75rem', opacity: 0.6 }}>
                                Click ▶ to expand
                            </span>
                        )}
                    </div>

                    {/* Page tree / list container */}
                    <div style={{
                        border: '1px solid var(--border-color)',
                        borderRadius: 8, overflow: 'hidden',
                    }}>
                        {pagesLoading && pages.length === 0 ? (
                            <div style={{ textAlign: 'center', padding: '32px 20px', color: 'var(--text-secondary)', fontSize: '0.85rem' }}>
                                Loading pages…
                            </div>
                        ) : rootPages.length === 0 && !pagesLoading ? (
                            <div style={{ textAlign: 'center', padding: '32px 20px', color: 'var(--text-secondary)', fontSize: '0.85rem' }}>
                                <div style={{ marginBottom: 6, fontSize: '1.1rem', opacity: 0.5 }}>📄</div>
                                No pages found.
                                {isFilteredMode && (
                                    <div style={{ fontSize: '0.78rem', marginTop: 4 }}>
                                        Try removing the title filter or searching a different space.
                                    </div>
                                )}
                            </div>
                        ) : (
                            <div style={{ maxHeight: 400, overflowY: 'auto' }}>
                                {isFilteredMode ? (
                                    // Flat search results
                                    rootPages.map(p => (
                                        <PageRow key={p.id} page={p} onSelect={handleSelectPage} />
                                    ))
                                ) : (
                                    // Tree view — root pages with expandable children
                                    rootPages.map(p => (
                                        <PageTreeNode key={p.id} page={p} depth={0} onSelect={handleSelectPage} />
                                    ))
                                )}
                            </div>
                        )}
                    </div>

                    {/* Load more (for root level pagination) */}
                    {pagesCursor && (
                        <div style={{ textAlign: 'center', marginTop: 10 }}>
                            <button className="action-btn" onClick={() => handleSearchPages(pagesCursor)}
                                disabled={pagesLoading} style={{ fontSize: '0.8rem' }}>
                                {pagesLoading ? 'Loading…' : 'Load more pages'}
                            </button>
                        </div>
                    )}

                    {rootPages.length > 0 && (
                        <div style={{ fontSize: '0.72rem', color: 'var(--text-secondary)', textAlign: 'right', marginTop: 8 }}>
                            {rootPages.length} {isFilteredMode ? 'result' : 'root page'}{rootPages.length !== 1 ? 's' : ''}
                            {!isFilteredMode && pages.length > rootPages.length && (
                                <span> ({pages.length} total loaded)</span>
                            )}
                        </div>
                    )}
                </div>
            )}

            {/* ── Step 3: Page preview ── */}
            {step === 3 && (
                <div style={{ display: 'flex', flexDirection: 'column', minHeight: 0, flex: 1 }}>
                    {previewLoading ? (
                        <div style={{ textAlign: 'center', padding: '48px 20px', color: 'var(--text-secondary)', fontSize: '0.85rem' }}>
                            Loading page preview…
                        </div>
                    ) : preview && (
                        <div style={{ display: 'flex', flexDirection: 'column', minHeight: 0, flex: 1 }}>
                            {preview.already_imported && (
                                <div style={{
                                    background: 'rgba(245,158,11,0.08)', color: '#facc15',
                                    border: '1px solid rgba(245,158,11,0.25)',
                                    padding: '10px 14px', borderRadius: 8, marginBottom: 14, fontSize: '0.84rem',
                                    display: 'flex', alignItems: 'center', gap: 8, flexShrink: 0,
                                }}>
                                    <span>⚠</span> This page has already been imported as a requirement.
                                </div>
                            )}

                            {/* Page header card — fixed height */}
                            <div style={{
                                background: 'var(--bg-tertiary)', border: '1px solid var(--border-color)',
                                borderRadius: 10, padding: '14px 20px', marginBottom: 14, flexShrink: 0,
                            }}>
                                <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
                                    <span style={{
                                        fontWeight: 700, fontSize: '0.72rem', color: 'var(--accent-teal, #2dd4bf)',
                                        background: 'rgba(45,212,191,0.1)', padding: '3px 10px', borderRadius: 5,
                                        letterSpacing: '0.03em', textTransform: 'uppercase',
                                    }}>Confluence</span>
                                    {preview.url && (
                                        <a href={preview.url} target="_blank" rel="noopener noreferrer" style={{
                                            fontSize: '0.78rem', color: 'var(--accent-indigo)', marginLeft: 'auto',
                                            textDecoration: 'none', fontWeight: 500, display: 'flex', alignItems: 'center', gap: 4,
                                        }}>
                                            Open in Confluence <span style={{ fontSize: '0.7rem' }}>↗</span>
                                        </a>
                                    )}
                                </div>
                                <div style={{ fontWeight: 700, fontSize: '1.05rem', lineHeight: 1.35 }}>{preview.title}</div>
                            </div>

                            {/* Page body — fills remaining space */}
                            <div style={{
                                border: '1px solid var(--border-color)', borderRadius: 10, overflow: 'hidden',
                                display: 'flex', flexDirection: 'column', flex: 1, minHeight: 200,
                            }}>
                                <div style={{
                                    padding: '6px 16px', background: 'rgba(255,255,255,0.02)',
                                    borderBottom: '1px solid var(--border-color)',
                                    fontSize: '0.72rem', fontWeight: 600, color: 'var(--text-secondary)',
                                    textTransform: 'uppercase', letterSpacing: '0.06em', flexShrink: 0,
                                }}>Page Content</div>
                                <div className="confluence-preview-body" style={{
                                    padding: '18px 20px', overflowY: 'auto', flex: 1,
                                    fontSize: '0.88rem', color: 'var(--text-primary)', lineHeight: 1.7,
                                }}>
                                    {preview.body_html && preview.body_html.trim() ? (
                                        <SafeHTML html={preview.body_html} />
                                    ) : (
                                        <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--text-secondary)', fontStyle: 'italic', opacity: 0.6 }}>
                                            This page has no content.
                                        </div>
                                    )}
                                </div>
                            </div>
                        </div>
                    )}
                </div>
            )}

            <style>{`
                .confluence-preview-body table { width: 100%; border-collapse: collapse; margin: 10px 0; font-size: 0.84rem; }
                .confluence-preview-body th, .confluence-preview-body td { padding: 8px 12px; border: 1px solid var(--border-color); text-align: left; vertical-align: top; }
                .confluence-preview-body th { background: rgba(255,255,255,0.04); font-weight: 600; font-size: 0.8rem; color: var(--text-secondary); text-transform: uppercase; letter-spacing: 0.02em; }
                .confluence-preview-body tr:hover td { background: rgba(255,255,255,0.02); }
                .confluence-preview-body p { margin: 6px 0; }
                .confluence-preview-body ul, .confluence-preview-body ol { padding-left: 20px; margin: 6px 0; }
                .confluence-preview-body li { margin: 3px 0; }
                .confluence-preview-body h1, .confluence-preview-body h2, .confluence-preview-body h3, .confluence-preview-body h4 { margin: 14px 0 6px; color: var(--text-primary); line-height: 1.3; }
                .confluence-preview-body h1 { font-size: 1.15rem; }
                .confluence-preview-body h2 { font-size: 1.05rem; }
                .confluence-preview-body h3 { font-size: 0.95rem; }
                .confluence-preview-body a { color: var(--accent-indigo); text-decoration: none; }
                .confluence-preview-body a:hover { text-decoration: underline; }
                .confluence-preview-body code { background: rgba(255,255,255,0.06); padding: 2px 6px; border-radius: 4px; font-size: 0.82em; }
                .confluence-preview-body pre { background: rgba(255,255,255,0.04); padding: 12px 16px; border-radius: 6px; overflow-x: auto; font-size: 0.82em; }
                .confluence-preview-body img { max-width: 100%; border-radius: 4px; }
            `}</style>
        </ModalShell>
    );
}
