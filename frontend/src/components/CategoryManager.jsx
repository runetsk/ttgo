import React, { useEffect, useState, useRef, useCallback } from 'react';
import { getCategories, createCategory, deleteCategories } from '../api';

/**
 * CategoryManager — /categories
 *
 * Manage test categories with:
 *   - Live server-side search
 *   - Modal-based create form
 *   - Row hover fade-in actions
 *   - Bulk delete with selection bar
 *   - Clean paginated table matching Requirements/Traceability design language
 */
export default function CategoryManager({ onUpdate }) {
    const [categories, setCategories]         = useState([]);
    const [total, setTotal]           = useState(0);
    const [page, setPage]             = useState(1);
    const [pageSize, setPageSize]     = useState(10);
    const [search, setSearch]         = useState('');
    const [searchInput, setSearchInput] = useState('');
    const [loading, setLoading]       = useState(true);
    const [selectedIds, setSelectedIds] = useState([]);

    // Modal state
    const [modal, setModal]           = useState(false);
    const [formName, setFormName]     = useState('');
    const [formDesc, setFormDesc]     = useState('');
    const [formError, setFormError]   = useState('');
    const [formSaving, setFormSaving] = useState(false);

    const searchTimer = useRef(null);

    // ── Data loading ──────────────────────────────────────────────────────────

    const load = useCallback(() => {
        setLoading(true);
        getCategories(page, pageSize, search)
            .then(data => {
                setCategories(data.categories || []);
                setTotal(data.total || 0);
            })
            .catch(() => {})
            .finally(() => setLoading(false));
    }, [page, pageSize, search]);

    // eslint-disable-next-line react-hooks/set-state-in-effect -- load() fetches categories from the server and stores the result; refetches on page/pageSize/search change
    useEffect(() => { load(); }, [load]);

    // Debounced search: update `search` state 300 ms after typing stops
    const handleSearchChange = (val) => {
        setSearchInput(val);
        clearTimeout(searchTimer.current);
        searchTimer.current = setTimeout(() => {
            setSearch(val);
            setPage(1);
            setSelectedIds([]);
        }, 300);
    };

    // ── Selection ─────────────────────────────────────────────────────────────

    const toggleSelect = (id) =>
        setSelectedIds(prev => prev.includes(id) ? prev.filter(i => i !== id) : [...prev, id]);

    const toggleSelectAll = () =>
        setSelectedIds(selectedIds.length === categories.length ? [] : categories.map(s => s.id));

    const allOnPageSelected = categories.length > 0 && selectedIds.length === categories.length;

    // ── CRUD ──────────────────────────────────────────────────────────────────

    const openModal = () => {
        setFormName(''); setFormDesc(''); setFormError('');
        setModal(true);
    };

    const closeModal = () => { setModal(false); setFormError(''); };

    const handleCreate = () => {
        if (!formName.trim()) { setFormError('Category name is required.'); return; }
        setFormSaving(true);
        setFormError('');
        createCategory(formName.trim(), formDesc.trim())
            .then(() => {
                closeModal();
                setPage(1);
                load();
                if (onUpdate) onUpdate();
            })
            .catch(err => setFormError(err.response?.data?.error || err.message || 'Failed to create category.'))
            .finally(() => setFormSaving(false));
    };

    const handleDelete = (ids) => {
        const count = ids.length;
        const label = count === 1
            ? `"${categories.find(s => s.id === ids[0])?.name ?? 'this category'}"`
            : `${count} categories`;
        if (!window.confirm(`Delete ${label}? This will not delete the test cases within them.`)) return;
        deleteCategories(ids)
            .then(() => {
                setSelectedIds([]);
                load();
                if (onUpdate) onUpdate();
            })
            .catch(() => {});
    };

    // ── Pagination ─────────────────────────────────────────────────────────────

    const totalPages = Math.ceil(total / pageSize);

    // ── Render ────────────────────────────────────────────────────────────────

    return (
        <div style={{ padding: '24px 28px', maxWidth: 1100, margin: '0 auto' }} data-testid="category-manager">

            {/* ── Page header ── */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 20 }}>
                <div>
                    <h2 style={{ margin: 0, fontSize: '1.4rem' }}>Category Management</h2>
                    <p style={{ margin: '4px 0 0', fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
                        Organise test cases into named categories — Smoke, Regression, and more.
                    </p>
                </div>
                <button className="primary-btn" onClick={openModal} data-testid="open-create-category-modal">
                    + New Category
                </button>
            </div>

            {/* ── Search + bulk bar ── */}
            <div style={{ display: 'flex', gap: 10, marginBottom: 14, alignItems: 'center', flexWrap: 'wrap' }}>
                <div style={{ position: 'relative', flex: '1 1 240px', maxWidth: 380 }}>
                    <span style={{
                        position: 'absolute', left: 10, top: '50%', transform: 'translateY(-50%)',
                        color: 'var(--text-secondary)', fontSize: '0.88rem', pointerEvents: 'none',
                    }}>🔍</span>
                    <input
                        className="modern-input"
                        style={{ width: '100%', paddingLeft: 30, boxSizing: 'border-box', fontSize: '0.875rem' }}
                        placeholder="Search categories by name or description…"
                        value={searchInput}
                        onChange={e => handleSearchChange(e.target.value)}
                        data-testid="category-search-input"
                    />
                </div>

                {/* Page size selector */}
                <div style={{ display: 'flex', gap: 6, alignItems: 'center', marginLeft: 'auto' }}>
                    <span style={{ fontSize: '0.82rem', color: 'var(--text-secondary)' }}>Show:</span>
                    <select
                        className="modern-select"
                        value={pageSize}
                        onChange={e => { setPageSize(Number(e.target.value)); setPage(1); }}
                        style={{ padding: '4px 8px', fontSize: '0.82rem' }}
                        data-testid="page-size-selector"
                    >
                        <option value={10}>10</option>
                        <option value={20}>20</option>
                        <option value={50}>50</option>
                        <option value={100}>100</option>
                    </select>
                </div>
            </div>

            {/* ── Selection action bar (slides in when rows selected) ── */}
            {selectedIds.length > 0 && (
                <div style={{
                    display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                    background: 'rgba(167,139,250,0.08)', border: '1px solid rgba(167,139,250,0.25)',
                    borderRadius: 8, padding: '10px 16px', marginBottom: 10,
                }}>
                    <span style={{ fontSize: '0.875rem', color: 'var(--text-primary)' }}>
                        <strong>{selectedIds.length}</strong> {selectedIds.length !== 1 ? 'categories' : 'category'} selected
                    </span>
                    <div style={{ display: 'flex', gap: 8 }}>
                        <button
                            className="action-btn"
                            style={{ fontSize: '0.8rem' }}
                            onClick={() => setSelectedIds([])}
                        >
                            Clear
                        </button>
                        <button
                            className="action-btn danger"
                            style={{ fontSize: '0.8rem', color: 'var(--accent-red, #f87171)', borderColor: 'rgba(248,113,113,0.35)' }}
                            onClick={() => handleDelete(selectedIds)}
                            data-testid="bulk-delete-categories-button"
                        >
                            🗑 Delete {selectedIds.length} selected
                        </button>
                    </div>
                </div>
            )}

            {/* ── Category table ── */}
            {loading ? (
                <div className="glass-panel" style={{ padding: 40, textAlign: 'center', color: 'var(--text-secondary)', fontSize: '0.9rem' }}>
                    Loading categories…
                </div>
            ) : categories.length === 0 ? (
                <div className="glass-panel">
                    <EmptyState search={search} onClear={() => { setSearch(''); setSearchInput(''); }} onNew={openModal} />
                </div>
            ) : (
            <div className="glass-panel" style={{ overflowX: 'auto' }}>
                <table className="modern-table">
                    <thead>
                        <tr>
                            <th style={{ width: 40, textAlign: 'center' }}>
                                <input
                                    type="checkbox"
                                    checked={allOnPageSelected}
                                    onChange={toggleSelectAll}
                                    style={{ cursor: 'pointer' }}
                                />
                            </th>
                            <th style={{ width: 220 }}>Name</th>
                            <th>Description</th>
                            <th style={{ width: 160 }}>Created</th>
                            <th style={{ width: 56, textAlign: 'center' }}></th>
                        </tr>
                    </thead>
                    <tbody>
                        {categories.map(s => (
                            <CategoryRow
                                key={s.id}
                                category={s}
                                selected={selectedIds.includes(s.id)}
                                onToggle={toggleSelect}
                                onDelete={handleDelete}
                            />
                        ))}
                    </tbody>
                </table>
            </div>
            )}

            {/* ── Pagination / count footer ── */}
            {!loading && categories.length > 0 && (
                totalPages > 1 ? (
                    <div style={{
                        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                        padding: '10px 4px', marginTop: 4,
                        fontSize: '0.82rem', color: 'var(--text-secondary)',
                    }}>
                        <span>
                            {search
                                ? `${total} result${total !== 1 ? 's' : ''} · page ${page} of ${totalPages}`
                                : `${total} ${total !== 1 ? 'categories' : 'category'} · page ${page} of ${totalPages}`}
                        </span>
                        <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
                            <button
                                className="action-btn"
                                style={{ fontSize: '0.8rem', padding: '3px 12px' }}
                                disabled={page === 1}
                                onClick={() => setPage(p => p - 1)}
                                data-testid="prev-page-button"
                            >
                                ← Prev
                            </button>
                            {/* Page number pills */}
                            {Array.from({ length: Math.min(totalPages, 7) }, (_, i) => {
                                const pg = totalPages <= 7
                                    ? i + 1
                                    : page <= 4 ? i + 1
                                    : page >= totalPages - 3 ? totalPages - 6 + i
                                    : page - 3 + i;
                                return (
                                    <button
                                        key={pg}
                                        className={pg === page ? 'primary-btn' : 'action-btn'}
                                        style={{ fontSize: '0.8rem', padding: '3px 9px', minWidth: 32 }}
                                        onClick={() => setPage(pg)}
                                    >
                                        {pg}
                                    </button>
                                );
                            })}
                            <button
                                className="action-btn"
                                style={{ fontSize: '0.8rem', padding: '3px 12px' }}
                                disabled={page === totalPages}
                                onClick={() => setPage(p => p + 1)}
                                data-testid="next-page-button"
                            >
                                Next →
                            </button>
                        </div>
                    </div>
                ) : (
                    <div style={{ padding: '8px 4px', marginTop: 4, fontSize: '0.78rem', color: 'var(--text-secondary)' }}>
                        {search ? `${total} result${total !== 1 ? 's' : ''} for "${search}"` : `${total} ${total !== 1 ? 'categories' : 'category'} total`}
                    </div>
                )
            )}

            {/* ── Create modal ── */}
            {modal && (
                <div className="modal-overlay" onClick={closeModal}>
                    <div
                        className="modal-content"
                        onClick={e => e.stopPropagation()}
                        style={{ width: 480, maxWidth: '90vw' }}
                    >
                        <header className="modal-header">
                            <h3 style={{ margin: 0 }}>+ New Category</h3>
                            <button className="modal-close-btn" onClick={closeModal}>×</button>
                        </header>
                        <div className="modal-body">
                            {formError && (
                                <div style={{
                                    background: 'rgba(239,68,68,0.1)', color: 'var(--accent-red, #f87171)',
                                    padding: '8px 12px', borderRadius: 6, marginBottom: 14, fontSize: '0.875rem',
                                }}>
                                    {formError}
                                </div>
                            )}
                            <div style={{ marginBottom: 14 }}>
                                <label style={labelStyle}>
                                    Category Name <span style={{ color: 'var(--accent-red, #f87171)', marginLeft: 2 }}>*</span>
                                </label>
                                <input
                                    className="modern-input"
                                    style={{ width: '100%' }}
                                    placeholder="e.g. Smoke Tests"
                                    value={formName}
                                    onChange={e => { setFormName(e.target.value); setFormError(''); }}
                                    autoFocus
                                    data-testid="category-name-input"
                                    onKeyDown={e => e.key === 'Enter' && handleCreate()}
                                />
                            </div>
                            <div>
                                <label style={labelStyle}>
                                    Description <span style={{ fontWeight: 400, opacity: 0.6 }}>(optional)</span>
                                </label>
                                <input
                                    className="modern-input"
                                    style={{ width: '100%' }}
                                    placeholder="e.g. Critical path tests run on every deploy"
                                    value={formDesc}
                                    onChange={e => setFormDesc(e.target.value)}
                                    data-testid="category-description-input"
                                    onKeyDown={e => e.key === 'Enter' && handleCreate()}
                                />
                            </div>
                        </div>
                        <footer className="modal-footer">
                            <button className="action-btn" onClick={closeModal} disabled={formSaving}>Cancel</button>
                            <button
                                className="primary-btn"
                                onClick={handleCreate}
                                disabled={formSaving || !formName.trim()}
                                data-testid="create-category-button"
                            >
                                {formSaving ? 'Creating…' : 'Create Category'}
                            </button>
                        </footer>
                    </div>
                </div>
            )}
        </div>
    );
}

// ── Sub-components ─────────────────────────────────────────────────────────────

function CategoryRow({ category, selected, onToggle, onDelete }) {
    return (
        <tr data-testid={`category-row-${category.id}`}>
            {/* Checkbox */}
            <td style={{ textAlign: 'center' }}>
                <input
                    type="checkbox"
                    checked={selected}
                    onChange={() => onToggle(category.id)}
                    style={{ cursor: 'pointer' }}
                    data-testid={`category-checkbox-${category.id}`}
                />
            </td>

            {/* Name */}
            <td style={{ fontWeight: 600, fontSize: '0.92rem' }}>
                {category.name}
            </td>

            {/* Description */}
            <td style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 0 }}>
                {category.description
                    ? category.description
                    : <span style={{ fontStyle: 'italic', opacity: 0.45 }}>No description</span>
                }
            </td>

            {/* Created at */}
            <td style={{ fontSize: '0.78rem', color: 'var(--text-secondary)', whiteSpace: 'nowrap' }}>
                {formatDate(category.created_at)}
            </td>

            {/* Actions — compact trash icon (matches Tests grid pattern) */}
            <td style={{ textAlign: 'center', padding: '10px 0' }}>
                <button
                    className="action-btn"
                    style={{
                        color: 'var(--accent-red, #f87171)',
                        padding: '4px 8px',
                        opacity: 0.7,
                        display: 'inline-flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                    }}
                    onClick={() => onDelete([category.id])}
                    title="Delete category"
                    data-testid={`delete-category-button-${category.id}`}
                >
                    🗑️
                </button>
            </td>
        </tr>
    );
}

function EmptyState({ search, onClear, onNew }) {
    if (search) {
        return (
            <div style={{ padding: 48, textAlign: 'center', color: 'var(--text-secondary)' }}>
                <div style={{ fontSize: '1.8rem', marginBottom: 10 }}>🔍</div>
                <div style={{ fontWeight: 600, marginBottom: 6, color: 'var(--text-primary)' }}>
                    No categories match <span style={{ color: 'var(--accent-purple, #a78bfa)' }}>"{search}"</span>
                </div>
                <button className="action-btn" style={{ marginTop: 12, fontSize: '0.85rem' }} onClick={onClear}>
                    Clear search
                </button>
            </div>
        );
    }
    return (
        <div style={{ padding: 56, textAlign: 'center' }}>
            <div style={{ fontSize: '2.5rem', marginBottom: 10 }}>🗂</div>
            <div style={{ fontWeight: 600, marginBottom: 6 }}>No categories yet</div>
            <div style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', marginBottom: 20 }}>
                Create your first category to start grouping test cases — e.g. Smoke, Regression, or Edge Cases.
            </div>
            <button className="primary-btn" onClick={onNew}>+ Create first category</button>
        </div>
    );
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function formatDate(iso) {
    if (!iso) return '—';
    const d = new Date(iso);
    return d.toLocaleDateString(undefined, { day: '2-digit', month: 'short', year: 'numeric' });
}

// ── Styles ────────────────────────────────────────────────────────────────────

const labelStyle = {
    display: 'block', marginBottom: 5,
    fontSize: '0.82rem', color: 'var(--text-secondary)', fontWeight: 600,
};
