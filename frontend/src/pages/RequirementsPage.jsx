import React, { useState, useEffect, useMemo, useCallback, useRef, useLayoutEffect } from 'react';
import { createPortal } from 'react-dom';
import { useNavigate } from 'react-router-dom';
import { requirements as reqApi, jira as jiraApi, confluence as confApi, traceability as tracApi } from '../api';
import ColumnPicker from '../components/ColumnPicker';
import ImportFromJiraModal from '../components/ImportFromJiraModal';
import ImportFromConfluenceModal from '../components/ImportFromConfluenceModal';
import BulkImportModal from '../components/BulkImportModal';
import ResyncModal from '../components/ResyncModal';
import ModalShell from '../components/shared/ModalShell';
import ErrorAlert from '../components/shared/ErrorAlert';
import SourceBadge from '../components/shared/SourceBadge';
import { labelStyle } from '../components/shared/styles';
import { useColumnPreference } from '../hooks/useColumnPreference';
import { useColumnWidths } from '../hooks/useColumnWidths';
import { useAIGeneration } from '../contexts/AIGenerationContext';

// T010: Column definitions for the Requirements grid
const COLUMN_DEFS = [
    { key: 'identifier', label: 'Identifier',  mandatory: false, defaultVisible: true, defaultWidth: 120 },
    { key: 'title',      label: 'Requirement', mandatory: true,  defaultVisible: true, defaultWidth: 400 },
    { key: 'coverage',   label: 'Coverage',    mandatory: false, defaultVisible: true, defaultWidth: 160 },
];

/**
 * RequirementsPage — /requirements
 *
 * Full CRUD management for requirements with:
 *   - Coverage summary bar (total / covered / gaps / %)
 *   - Live search filter (identifier + title)
 *   - Per-row coverage badge + linked-test count
 *   - Create / Edit / Delete via modal
 *   - Optional "Fetch from Jira" when Jira integration is enabled (T028)
 */
export default function RequirementsPage() {
    const [reqs, setReqs]     = useState([]);
    const [matrix, setMatrix] = useState(null);
    const [loading, setLoading] = useState(true);
    const [search, setSearch]   = useState('');

    // Modal state
    const [modal, setModal]         = useState(null); // null | { mode: 'create'|'edit', req? }
    const [formId, setFormId]       = useState('');
    const [formTitle, setFormTitle] = useState('');
    const [formDesc, setFormDesc]   = useState('');
    const [formError, setFormError] = useState('');
    const [formSaving, setFormSaving] = useState(false);

    // T011: Column visibility — hook + helper
    const [visibleKeys, toggleColumn, resetColumns] = useColumnPreference('requirements', COLUMN_DEFS);
    const isVisible = (key) => visibleKeys.has(key);

    // Column resizing
    const { columnWidths, startResize, resetWidths, resetColumnWidth, isResizing } = useColumnWidths('requirements', COLUMN_DEFS);

    // Combined reset: visibility + widths
    const handleResetAll = useCallback(() => { resetColumns(); resetWidths(); }, [resetColumns, resetWidths]);

    // Jira integration
    const [jiraEnabled, setJiraEnabled]   = useState(false);

    // Confluence integration
    const [confluenceEnabled, setConfluenceEnabled] = useState(false);

    // 011: Import modal state
    const [showImportJira, setShowImportJira] = useState(false);
    const [showImportConfluence, setShowImportConfluence] = useState(false);
    const [bulkImportSource, setBulkImportSource] = useState(null); // "jira" | "confluence" | null
    const [resyncReq, setResyncReq] = useState(null); // requirement for resync modal

    // Multi-select for bulk operations
    const [selectedIds, setSelectedIds] = useState([]);

    // Import menu dropdown state
    const [importMenuOpen, setImportMenuOpen] = useState(false);
    const importMenuRef = useRef(null);
    useEffect(() => {
        if (!importMenuOpen) return;
        const onDown = (e) => { if (importMenuRef.current && !importMenuRef.current.contains(e.target)) setImportMenuOpen(false); };
        const onEsc = (e) => { if (e.key === 'Escape') setImportMenuOpen(false); };
        document.addEventListener('mousedown', onDown);
        document.addEventListener('keydown', onEsc);
        return () => { document.removeEventListener('mousedown', onDown); document.removeEventListener('keydown', onEsc); };
    }, [importMenuOpen]);

    // AI generation — context-based (persistent across navigation)
    const aiGen = useAIGeneration();
    const navigate = useNavigate();

    // ── Data loading ──────────────────────────────────────────────────────────

    const load = () => {
        setLoading(true);
        Promise.all([reqApi.list(), tracApi.getMatrix()])
            .then(([reqData, matrixData]) => {
                setReqs(reqData || []);
                setMatrix(matrixData);
            })
            .catch(() => {})
            .finally(() => setLoading(false));
    };

    useEffect(() => {
        load();
        jiraApi.getConfig()
            .then(cfg => setJiraEnabled(cfg?.enabled === true))
            .catch(() => setJiraEnabled(false));
        confApi.getConfig()
            .then(cfg => setConfluenceEnabled(cfg?.enabled === true))
            .catch(() => setConfluenceEnabled(false));
        // Register callback so accepting AI-generated drafts refreshes this page
        aiGen.setOnAcceptedCallback(() => load());
    }, []);

    // ── Derived state ─────────────────────────────────────────────────────────

    /** Map reqId → { count: N, covered: bool } built from the traceability matrix */
    const coverageMap = useMemo(() => {
        const map = {};
        if (matrix?.rows) {
            for (const row of matrix.rows) {
                const count = row.linked_test_cases?.length ?? 0;
                map[row.requirement_id] = { count, covered: count > 0 };
            }
        }
        return map;
    }, [matrix]);

    const stats = useMemo(() => {
        const total   = reqs.length;
        const covered = reqs.filter(r => coverageMap[r.id]?.covered).length;
        const gaps    = total - covered;
        const pct     = total > 0 ? ((covered / total) * 100).toFixed(0) : '0';
        return { total, covered, gaps, pct };
    }, [reqs, coverageMap]);

    const filtered = useMemo(() => {
        const q = search.trim().toLowerCase();
        if (!q) return reqs;
        return reqs.filter(r =>
            r.identifier.toLowerCase().includes(q) ||
            r.title.toLowerCase().includes(q) ||
            (r.description || '').toLowerCase().includes(q)
        );
    }, [reqs, search]);

    // ── Modal helpers ─────────────────────────────────────────────────────────

    const openCreate = () => {
        setFormId(''); setFormTitle(''); setFormDesc('');
        setFormError('');
        setModal({ mode: 'create' });
    };

    const openEdit = (req) => {
        setFormId(req.identifier);
        setFormTitle(req.title);
        setFormDesc(req.description || '');
        setFormError('');
        setModal({ mode: 'edit', req });
    };

    const closeModal = () => { setModal(null); setFormError(''); };

    const handleSubmit = () => {
        if (!formId.trim())    { setFormError('Identifier is required.'); return; }
        if (!formTitle.trim()) { setFormError('Title is required.'); return; }
        setFormSaving(true);
        setFormError('');

        const payload = {
            identifier:  formId.trim(),
            title:       formTitle.trim(),
            description: formDesc.trim(),
        };
        const promise = modal.mode === 'create'
            ? reqApi.create(payload)
            : reqApi.update(modal.req.id, payload);

        promise
            .then(() => { closeModal(); load(); })
            .catch(err => setFormError(err.response?.data?.error || err.message || 'An error occurred.'))
            .finally(() => setFormSaving(false));
    };

    const handleDelete = (req) => {
        const count = coverageMap[req.id]?.count ?? 0;
        const warning = count > 0
            ? `"${req.identifier}" is linked to ${count} test case${count > 1 ? 's' : ''}. Deleting it will remove all links. Continue?`
            : `Delete "${req.identifier} – ${req.title}"? This cannot be undone.`;
        if (!window.confirm(warning)) return;

        reqApi.delete(req.id)
            .then(() => { setReqs(prev => prev.filter(r => r.id !== req.id)); load(); })
            .catch(() => {});
    };

    // 011: Unlink handler
    const handleUnlink = (req) => {
        if (!window.confirm('Remove source link? This cannot be undone. The requirement will become standalone.')) return;
        reqApi.unlink(req.id)
            .then(() => load())
            .catch(() => {});
    };

    // Multi-select helpers
    const toggleSelectAll = () => {
        if (selectedIds.length === filtered.length) {
            setSelectedIds([]);
        } else {
            setSelectedIds(filtered.map(r => r.id));
        }
    };
    const toggleSelect = (id) => {
        setSelectedIds(prev => prev.includes(id) ? prev.filter(x => x !== id) : [...prev, id]);
    };
    const handleBulkDelete = () => {
        if (!window.confirm(`Delete ${selectedIds.length} requirement${selectedIds.length > 1 ? 's' : ''}? This cannot be undone.`)) return;
        reqApi.bulkDelete(selectedIds)
            .then(() => { setSelectedIds([]); load(); })
            .catch(() => {});
    };

    // ── Render ────────────────────────────────────────────────────────────────

    if (loading) {
        return (
            <div style={{ padding: 32, textAlign: 'center', color: 'var(--text-secondary)' }}>
                Loading requirements…
            </div>
        );
    }

    return (
        <div style={{ padding: '24px 28px', maxWidth: 1100, margin: '0 auto' }}>

            {/* ── Page header ── */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 20 }}>
                <div>
                    <h2 style={{ margin: 0, fontSize: '1.4rem' }}>Requirements</h2>
                    <p style={{ margin: '4px 0 0', fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
                        Track and manage functional requirements linked to your test cases.
                    </p>
                </div>
                <div style={{ display: 'flex', gap: 10, alignItems: 'center' }}>
                    {/* T012: Column visibility picker */}
                    <ColumnPicker
                        columnDefs={COLUMN_DEFS}
                        visibleKeys={visibleKeys}
                        onToggle={toggleColumn}
                        onReset={handleResetAll}
                    />
                    {/* Import dropdown — consolidates Jira/Confluence single + bulk */}
                    <div style={{ position: 'relative' }} ref={importMenuRef}>
                        <button
                            className="action-btn"
                            style={{ fontSize: '0.82rem', whiteSpace: 'nowrap', display: 'inline-flex', alignItems: 'center', gap: 6 }}
                            onClick={() => setImportMenuOpen(o => !o)}
                            title="Import requirements from an external source"
                        >
                            ⬇ Import <span style={{ fontSize: '0.7rem', opacity: 0.7 }}>▾</span>
                        </button>
                        {importMenuOpen && (
                            <div
                                className="context-menu"
                                style={{ position: 'absolute', top: 'calc(100% + 6px)', right: 0, zIndex: 300, minWidth: 220 }}
                            >
                                <div style={{ padding: '6px 12px', fontSize: '0.68rem', textTransform: 'uppercase', letterSpacing: '0.08em', color: 'var(--text-secondary)' }}>Jira</div>
                                <div className="context-menu-item" onClick={() => { setImportMenuOpen(false); setShowImportJira(true); }}>
                                    <span>⬇</span> Single ticket
                                </div>
                                <div className="context-menu-item" onClick={() => { setImportMenuOpen(false); setBulkImportSource('jira'); }}>
                                    <span>⬇⬇</span> Bulk via JQL
                                </div>
                                <div style={{ padding: '6px 12px 4px', fontSize: '0.68rem', textTransform: 'uppercase', letterSpacing: '0.08em', color: 'var(--text-secondary)', borderTop: '1px solid var(--border-color)', marginTop: 4 }}>Confluence</div>
                                <div className="context-menu-item" onClick={() => { setImportMenuOpen(false); setShowImportConfluence(true); }}>
                                    <span>⬇</span> Single page
                                </div>
                                <div className="context-menu-item" onClick={() => { setImportMenuOpen(false); setBulkImportSource('confluence'); }}>
                                    <span>⬇⬇</span> Bulk from space
                                </div>
                            </div>
                        )}
                    </div>
                    <button className="primary-btn" onClick={openCreate}>+ New Requirement</button>
                </div>
            </div>

            {/* ── Coverage summary strip ── */}
            {reqs.length > 0 && (
                <div
                    className="glass-panel"
                    style={{
                        display: 'flex',
                        flexDirection: 'row',
                        alignItems: 'center',
                        gap: 20,
                        padding: '12px 18px',
                        marginBottom: 16,
                        fontSize: '0.85rem',
                        flexWrap: 'wrap',
                    }}
                >
                    <StatPill label="Total" value={stats.total} />
                    <StatDivider />
                    <StatPill label="Covered" value={stats.covered} valueColor="var(--accent-green, #34d399)" />
                    <StatDivider />
                    <StatPill label="Gaps" value={stats.gaps} valueColor={stats.gaps > 0 ? 'var(--accent-red, #f87171)' : 'var(--text-secondary)'} />
                    <div style={{ flex: 1, minWidth: 180, display: 'flex', alignItems: 'center', gap: 10 }}>
                        <div style={{
                            flex: 1,
                            height: 6,
                            background: 'rgba(255,255,255,0.06)',
                            borderRadius: 3,
                            overflow: 'hidden',
                        }}>
                            <div style={{
                                width: `${stats.pct}%`,
                                height: '100%',
                                background: stats.gaps === 0 ? 'var(--accent-green, #34d399)' : 'var(--accent-purple, #a78bfa)',
                                transition: 'width 0.3s',
                            }} />
                        </div>
                        <span style={{
                            fontWeight: 700,
                            color: stats.gaps === 0 ? 'var(--accent-green, #34d399)' : 'var(--accent-purple, #a78bfa)',
                            minWidth: 42,
                            textAlign: 'right',
                        }}>{stats.pct}%</span>
                    </div>
                </div>
            )}

            {/* ── Search bar ── */}
            {reqs.length > 0 && (
                <div style={{ marginBottom: 14, position: 'relative' }}>
                    <span style={{
                        position: 'absolute', left: 11, top: '50%', transform: 'translateY(-50%)',
                        color: 'var(--text-secondary)', fontSize: '0.9rem', pointerEvents: 'none',
                    }}>🔍</span>
                    <input
                        className="modern-input"
                        style={{ width: '100%', paddingLeft: 32, boxSizing: 'border-box' }}
                        placeholder="Search by identifier, title or description…"
                        value={search}
                        onChange={e => setSearch(e.target.value)}
                    />
                </div>
            )}

            {/* ── Empty state ── */}
            {reqs.length === 0 ? (
                <div className="glass-panel" style={{ padding: 56, textAlign: 'center' }}>
                    <div style={{ fontSize: '2.5rem', marginBottom: 10 }}>📋</div>
                    <div style={{ fontWeight: 600, marginBottom: 6 }}>No requirements yet</div>
                    <div style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', marginBottom: 20 }}>
                        Start by creating a requirement, then link test cases to it from the test case detail view.
                    </div>
                    <button className="primary-btn" onClick={openCreate}>+ Create first requirement</button>
                </div>
            ) : filtered.length === 0 ? (
                <div className="glass-panel" style={{ padding: 40, textAlign: 'center', color: 'var(--text-secondary)' }}>
                    No requirements match <strong>"{search}"</strong>
                </div>
            ) : (
                /* ── Requirements table ── */
                <>
                {selectedIds.length > 0 && (
                    <div style={{
                        display: 'flex', alignItems: 'center', gap: 12,
                        padding: '8px 14px', marginBottom: 8,
                        background: 'rgba(96,165,250,0.08)', borderRadius: 8,
                        fontSize: '0.82rem',
                    }}>
                        <span style={{ fontWeight: 600 }}>{selectedIds.length} selected</span>
                        <button
                            className="action-btn"
                            style={{ fontSize: '0.78rem', padding: '2px 10px', color: 'var(--accent-red, #f87171)', borderColor: 'rgba(248,113,113,0.35)' }}
                            onClick={handleBulkDelete}
                        >
                            Delete Selected
                        </button>
                        <button
                            className="action-btn"
                            style={{ fontSize: '0.78rem', padding: '2px 10px' }}
                            onClick={() => setSelectedIds([])}
                        >
                            Clear Selection
                        </button>
                    </div>
                )}
                <div className="glass-panel" style={{ overflowX: 'auto' }}>
                    <table className="modern-table resizable">
                        <thead>
                            <tr>
                                <th style={{ width: 40, textAlign: 'center' }}>
                                    <input
                                        type="checkbox"
                                        checked={selectedIds.length === filtered.length && filtered.length > 0}
                                        onChange={toggleSelectAll}
                                        onClick={e => e.stopPropagation()}
                                    />
                                </th>
                                {/* T013: Optional columns gated by isVisible() */}
                                {isVisible('identifier') && (
                                    <th className="col-resize-th" style={{ width: columnWidths['identifier'] }}>
                                        Identifier
                                        <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('identifier', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('identifier'); }} />
                                    </th>
                                )}
                                {/* title is mandatory — always rendered */}
                                <th className="col-resize-th" style={{ width: columnWidths['title'] }}>
                                    Requirement
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('title', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('title'); }} />
                                </th>
                                {isVisible('coverage') && (
                                    <th className="col-resize-th" style={{ width: columnWidths['coverage'], textAlign: 'center' }}>
                                        Coverage
                                        <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('coverage', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('coverage'); }} />
                                    </th>
                                )}
                                <th style={{ width: 56, textAlign: 'center' }}></th>
                            </tr>
                        </thead>
                        <tbody>
                            {filtered.map(req => {
                                const cov = coverageMap[req.id];
                                const count   = cov?.count   ?? 0;
                                const covered = cov?.covered ?? false;
                                return (
                                    <RequirementRow
                                        key={req.id}
                                        req={req}
                                        count={count}
                                        covered={covered}
                                        childCount={req.child_count || 0}
                                        selected={selectedIds.includes(req.id)}
                                        onToggleSelect={toggleSelect}
                                        onEdit={openEdit}
                                        onDelete={handleDelete}
                                        onResync={setResyncReq}
                                        onUnlink={handleUnlink}
                                        isVisible={isVisible}
                                        onGenerateTests={() => {
                                            if (aiGen.hasUnsaved && !window.confirm('You have un-accepted AI drafts. Opening a new session will discard them. Continue?')) return;
                                            aiGen.openSession(req);
                                            navigate('/ai-generate');
                                        }}
                                        aiEnabled={aiGen.aiFeaturesEnabled}
                                    />
                                );
                            })}
                        </tbody>
                    </table>
                    {search && (
                        <div style={{ padding: '8px 14px', borderTop: '1px solid var(--border-color)', fontSize: '0.78rem', color: 'var(--text-secondary)' }}>
                            Showing {filtered.length} of {reqs.length} requirements
                        </div>
                    )}
                </div>
                </>
            )}

            {/* ── Import modals (011) ── */}
            {showImportJira && (
                <ImportFromJiraModal
                    jiraEnabled={jiraEnabled}
                    onClose={() => setShowImportJira(false)}
                    onImported={() => load()}
                />
            )}
            {showImportConfluence && (
                <ImportFromConfluenceModal
                    confluenceEnabled={confluenceEnabled}
                    onClose={() => setShowImportConfluence(false)}
                    onImported={() => load()}
                />
            )}
            {bulkImportSource && (
                <BulkImportModal
                    source={bulkImportSource}
                    onClose={() => setBulkImportSource(null)}
                    onImported={() => load()}
                />
            )}
            {resyncReq && (
                <ResyncModal
                    requirement={resyncReq}
                    onClose={() => setResyncReq(null)}
                    onResynced={() => load()}
                />
            )}

            {/* ── Create / Edit modal ── */}
            {modal && (
                <ModalShell
                    title={modal.mode === 'create' ? '+ New Requirement' : 'Edit Requirement'}
                    width={540}
                    onClose={closeModal}
                    footer={
                        <>
                            <button className="action-btn" onClick={closeModal} disabled={formSaving}>Cancel</button>
                            <button className="primary-btn" onClick={handleSubmit} disabled={formSaving}>
                                {formSaving ? 'Saving…' : modal.mode === 'create' ? 'Create' : 'Save Changes'}
                            </button>
                        </>
                    }
                >
                    <ErrorAlert message={formError} />
                    <div style={{ marginBottom: 14 }}>
                        <label style={labelStyle}>Identifier <Required /></label>
                        <input
                            className="modern-input"
                            style={{ width: '100%' }}
                            placeholder="e.g. PROJ-001"
                            value={formId}
                            onChange={e => setFormId(e.target.value)}
                            autoFocus
                        />
                    </div>
                    <div style={{ marginBottom: 14 }}>
                        <label style={labelStyle}>Title <Required /></label>
                        <input
                            className="modern-input"
                            style={{ width: '100%' }}
                            placeholder="Short description of the requirement"
                            value={formTitle}
                            onChange={e => setFormTitle(e.target.value)}
                        />
                    </div>
                    <div style={{ marginBottom: 4 }}>
                        <label style={labelStyle}>Description <span style={{ fontWeight: 400, opacity: 0.75 }}>(optional)</span></label>
                        <textarea
                            className="modern-input"
                            style={{ width: '100%', minHeight: 90, resize: 'vertical' }}
                            placeholder="Detailed acceptance criteria or context"
                            value={formDesc}
                            onChange={e => setFormDesc(e.target.value)}
                        />
                    </div>
                </ModalShell>
            )}
        </div>
    );
}

// ── Sub-components ────────────────────────────────────────────────────────────

function StatPill({ label, value, valueColor = 'var(--text-primary)' }) {
    return (
        <div style={{ display: 'inline-flex', alignItems: 'baseline', gap: 6, whiteSpace: 'nowrap' }}>
            <span style={{ fontWeight: 700, fontSize: '1rem', color: valueColor }}>{value}</span>
            <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>{label}</span>
        </div>
    );
}

function StatDivider() {
    return <span style={{ color: 'var(--text-secondary)', opacity: 0.4 }}>·</span>;
}

function RequirementRow({ req, count, covered, childCount, selected, onToggleSelect, onEdit, onDelete, onResync, onUnlink, isVisible, onGenerateTests, aiEnabled }) {
    const navigate = useNavigate();
    const [menuOpen, setMenuOpen] = useState(false);
    const [menuPos, setMenuPos] = useState({ top: 0, left: 0 });
    const btnRef = useRef(null);
    const menuRef = useRef(null);

    // Position the portal menu relative to the button, flipping up/left as needed to stay on-screen.
    useLayoutEffect(() => {
        if (!menuOpen || !btnRef.current) return;
        const r = btnRef.current.getBoundingClientRect();
        const MENU_W = 180;
        const MENU_H_EST = 220; // worst-case with all items
        const vw = window.innerWidth;
        const vh = window.innerHeight;
        let top = r.bottom + 4;
        if (top + MENU_H_EST > vh - 8) top = Math.max(8, r.top - MENU_H_EST - 4);
        let left = r.right - MENU_W;
        if (left < 8) left = 8;
        if (left + MENU_W > vw - 8) left = vw - MENU_W - 8;
        setMenuPos({ top, left });
    }, [menuOpen]);

    // Close on outside click / Escape / scroll
    useEffect(() => {
        if (!menuOpen) return;
        const onDown = (e) => {
            if (btnRef.current?.contains(e.target)) return;
            if (menuRef.current?.contains(e.target)) return;
            setMenuOpen(false);
        };
        const onEsc = (e) => { if (e.key === 'Escape') setMenuOpen(false); };
        const onScroll = () => setMenuOpen(false);
        document.addEventListener('mousedown', onDown);
        document.addEventListener('keydown', onEsc);
        window.addEventListener('scroll', onScroll, true);
        return () => {
            document.removeEventListener('mousedown', onDown);
            document.removeEventListener('keydown', onEsc);
            window.removeEventListener('scroll', onScroll, true);
        };
    }, [menuOpen]);

    const coverageBadge = covered
        ? <span style={covBadge(true)}>✓ {count} test{count !== 1 ? 's' : ''}</span>
        : <span style={covBadge(false)}>✗ Not covered</span>;

    return (
        <tr
            onClick={() => navigate(`/requirements/${req.id}`)}
            style={{ cursor: 'pointer' }}
        >
            <td style={{ textAlign: 'center', width: 40 }}>
                <input
                    type="checkbox"
                    checked={selected}
                    onChange={() => onToggleSelect(req.id)}
                    onClick={e => e.stopPropagation()}
                />
            </td>
            {/* T013: Optional cells gated by isVisible() */}
            {isVisible('identifier') && (
                <td style={{ whiteSpace: 'normal', verticalAlign: 'top', overflow: 'visible', textOverflow: 'clip' }}>
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4, alignItems: 'center' }}>
                        <span style={{
                            fontWeight: 700,
                            fontSize: '0.82rem',
                            color: 'var(--accent-purple, #a78bfa)',
                            background: 'rgba(167,139,250,0.1)',
                            padding: '3px 8px',
                            borderRadius: 4,
                            letterSpacing: '0.02em',
                        }}>
                            {req.identifier}
                        </span>
                        <SourceBadge sourceType={req.source_type} sourceUrl={req.source_url} />
                        {childCount > 0 && (
                            <span
                                onClick={(e) => { e.stopPropagation(); navigate(`/requirements/${req.id}`); }}
                                style={{
                                    fontSize: '0.72rem',
                                    color: 'var(--accent-blue, #60a5fa)',
                                    background: 'rgba(96,165,250,0.1)',
                                    padding: '2px 8px',
                                    borderRadius: 3,
                                    cursor: 'pointer',
                                    whiteSpace: 'nowrap',
                                }}
                                title="View child issues"
                            >
                                {childCount} children
                            </span>
                        )}
                    </div>
                </td>
            )}

            {/* title is mandatory — always rendered */}
            <td style={{ verticalAlign: 'top' }}>
                <div style={{ fontWeight: 600, marginBottom: req.description ? 3 : 0 }}>
                    {req.title}
                </div>
                {req.description && (() => {
                    // Extract a plain-text preview WITHOUT touching the live DOM.
                    // DOMParser produces an inert document — it neither executes
                    // scripts nor loads resources (e.g. <img onerror>), so a
                    // malicious/legacy-unsanitized description cannot run code (F-011).
                    const parsed = new DOMParser().parseFromString(req.description, 'text/html');
                    const text = (parsed.body.textContent || '').replace(/\s+/g, ' ');
                    return text.trim() ? (
                        <div style={{
                            fontSize: '0.8rem',
                            color: 'var(--text-secondary)',
                            lineHeight: 1.5,
                            display: '-webkit-box',
                            WebkitLineClamp: 2,
                            WebkitBoxOrient: 'vertical',
                            overflow: 'hidden',
                        }}>
                            {text.trim()}
                        </div>
                    ) : null;
                })()}
            </td>

            {/* Coverage badge */}
            {isVisible('coverage') && (
                <td style={{ textAlign: 'center', verticalAlign: 'middle' }}>
                    {coverageBadge}
                </td>
            )}

            {/* Actions — single kebab, portal-positioned so the menu never clips */}
            <td style={{ textAlign: 'center', whiteSpace: 'nowrap', padding: 0, verticalAlign: 'middle' }} onClick={(e) => e.stopPropagation()}>
                <button
                    ref={btnRef}
                    className="action-btn"
                    style={{
                        fontSize: '1rem',
                        padding: '2px 10px',
                        lineHeight: 1,
                        opacity: menuOpen ? 1 : 0.7,
                        transition: 'opacity 0.15s',
                    }}
                    onClick={(e) => { e.stopPropagation(); setMenuOpen(prev => !prev); }}
                    title="Actions"
                    aria-label="Actions"
                >
                    ⋮
                </button>
                {menuOpen && createPortal(
                    <div
                        ref={menuRef}
                        className="context-menu"
                        style={{ position: 'fixed', top: menuPos.top, left: menuPos.left, zIndex: 1000, minWidth: 180 }}
                        onClick={(e) => e.stopPropagation()}
                    >
                        <div className="context-menu-item" onClick={() => { setMenuOpen(false); onEdit(req); }}>
                            <span>✎</span> Edit
                        </div>
                        {aiEnabled && (
                            <div className="context-menu-item" onClick={() => { setMenuOpen(false); onGenerateTests(req); }}>
                                <span>✨</span> AI Gen
                            </div>
                        )}
                        {req.source_type && (
                            <>
                                <div className="context-menu-item" onClick={() => { setMenuOpen(false); onResync(req); }}>
                                    <span>↻</span> Re-sync
                                </div>
                                <div className="context-menu-item danger" onClick={() => { setMenuOpen(false); onUnlink(req); }}>
                                    <span>⊘</span> Unlink
                                </div>
                            </>
                        )}
                        <div style={{ height: 1, background: 'var(--border-color)', margin: '4px 0' }} />
                        <div className="context-menu-item danger" onClick={() => { setMenuOpen(false); onDelete(req); }}>
                            <span>🗑</span> Delete
                        </div>
                    </div>,
                    document.body
                )}
            </td>
        </tr>
    );
}

function Required() {
    return <span style={{ color: 'var(--accent-red, #f87171)', marginLeft: 2 }}>*</span>;
}

// ── Styles ────────────────────────────────────────────────────────────────────

const covBadge = (covered) => ({
    display: 'inline-flex',
    alignItems: 'center',
    gap: 4,
    fontSize: '0.75rem',
    fontWeight: 600,
    padding: '3px 10px',
    borderRadius: 20,
    background: covered ? 'rgba(52,211,153,0.12)' : 'rgba(248,113,113,0.12)',
    color: covered ? 'var(--accent-green, #34d399)' : 'var(--accent-red, #f87171)',
    border: `1px solid ${covered ? 'rgba(52,211,153,0.3)' : 'rgba(248,113,113,0.3)'}`,
    whiteSpace: 'nowrap',
});
