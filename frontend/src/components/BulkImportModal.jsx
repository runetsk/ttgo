import React, { useState, useCallback } from 'react';
import { jira as jiraApi, confluence as confApi, requirements as reqApi } from '../api';
import { toast } from '../toast';
import ModalShell from './shared/ModalShell';
import ErrorAlert from './shared/ErrorAlert';
import { labelStyle, alreadyImportedBadgeStyle } from './shared/styles';

/**
 * BulkImportModal — modal for bulk importing requirements from Jira (JQL) or Confluence (space).
 * 011-jira-confluence-import (T026, T028)
 *
 * Props:
 *   - source: "jira" | "confluence"
 *   - onClose: callback
 *   - onImported: callback after successful import
 */
export default function BulkImportModal({ source, onClose, onImported }) {
    // Jira mode state
    const [jql, setJql] = useState('');
    const [tickets, setTickets] = useState([]);
    const [total, setTotal] = useState(0);
    const [startAt, setStartAt] = useState(0);
    const [maxResults] = useState(25);

    // Confluence mode state
    const [spaces, setSpaces] = useState([]);
    const [selectedSpace, setSelectedSpace] = useState(null);
    const [spacesLoaded, setSpacesLoaded] = useState(false);
    const [labelFilter, setLabelFilter] = useState('');
    const [pages, setPages] = useState([]);
    const [pagesCursor, setPagesCursor] = useState(null);

    // Common state
    const [selected, setSelected] = useState(new Set());
    const [includeChildren, setIncludeChildren] = useState(true);
    const [searching, setSearching] = useState(false);
    const [importing, setImporting] = useState(false);
    const [error, setError] = useState('');
    const [results, setResults] = useState(null); // { imported, skipped, failed }

    // Load spaces for confluence mode
    const loadSpaces = useCallback(() => {
        if (spacesLoaded) return;
        setSearching(true);
        confApi.listSpaces(null, 100)
            .then(data => { setSpaces(data.spaces || []); setSpacesLoaded(true); })
            .catch(err => setError(err.response?.data?.error || err.message))
            .finally(() => setSearching(false));
    }, [spacesLoaded]);

    // Jira: search by JQL
    const handleJiraSearch = (newStartAt = 0) => {
        if (!jql.trim()) return;
        setSearching(true);
        setError('');
        jiraApi.search(jql.trim(), newStartAt, maxResults)
            .then(data => {
                setTickets(data.tickets || []);
                setTotal(data.total || 0);
                setStartAt(data.start_at || 0);
            })
            .catch(err => setError(err.response?.data?.error || err.message))
            .finally(() => setSearching(false));
    };

    // Confluence: search pages in space
    const handleConfluenceSearch = (cursor = null) => {
        if (!selectedSpace) return;
        setSearching(true);
        setError('');
        confApi.listPages(selectedSpace.id, null, labelFilter || null, cursor, 25)
            .then(data => {
                if (cursor) {
                    setPages(prev => [...prev, ...(data.pages || [])]);
                } else {
                    setPages(data.pages || []);
                }
                setPagesCursor(data.next_cursor || null);
            })
            .catch(err => setError(err.response?.data?.error || err.message))
            .finally(() => setSearching(false));
    };

    const items = source === 'jira' ? tickets : pages;
    const getKey = (item) => source === 'jira' ? item.key : item.id;
    const getLabel = (item) => source === 'jira' ? item.key : item.title;

    const nonImportedItems = items.filter(i => !i.already_imported);

    const toggleSelect = (key) => {
        setSelected(prev => {
            const next = new Set(prev);
            if (next.has(key)) next.delete(key);
            else next.add(key);
            return next;
        });
    };

    const selectAll = () => {
        const keys = nonImportedItems.map(i => getKey(i));
        setSelected(prev => {
            const next = new Set(prev);
            keys.forEach(k => next.add(k));
            return next;
        });
    };

    const deselectAll = () => {
        const keys = nonImportedItems.map(i => getKey(i));
        setSelected(prev => {
            const next = new Set(prev);
            keys.forEach(k => next.delete(k));
            return next;
        });
    };

    const handleImport = () => {
        const keys = Array.from(selected);
        if (keys.length === 0) return;
        setImporting(true);
        setError('');
        reqApi.bulkImport(source, keys, source === 'jira' ? includeChildren : false)
            .then(data => {
                setResults(data);
                const count = data.imported?.length || 0;
                if (count > 0) {
                    toast.success(`Imported ${count} requirement${count !== 1 ? 's' : ''}`);
                    onImported?.();
                }
            })
            .catch(err => setError(err.response?.data?.error || err.message))
            .finally(() => setImporting(false));
    };

    const selectedCount = Array.from(selected).filter(k => items.some(i => getKey(i) === k && !i.already_imported)).length;

    return (
        <ModalShell
            title={`Bulk Import from ${source === 'jira' ? 'Jira' : 'Confluence'}`}
            width={680}
            onClose={onClose}
            footer={<>
                <button className="action-btn" onClick={onClose} disabled={importing}>
                    {results ? 'Close' : 'Cancel'}
                </button>
                {!results && items.length > 0 && (
                    <button
                        className="primary-btn"
                        onClick={handleImport}
                        disabled={importing || selectedCount === 0}
                    >
                        {importing ? 'Importing…' : `Import Selected (${selectedCount})`}
                    </button>
                )}
            </>}
        >
                    <ErrorAlert message={error} />

                    {/* Results summary */}
                    {results ? (
                        <div>
                            <h4 style={{ marginTop: 0, marginBottom: 12 }}>Import Results</h4>
                            {results.imported?.length > 0 && (
                                <div style={{ marginBottom: 12 }}>
                                    <div style={{ fontWeight: 600, color: 'var(--accent-green, #34d399)', marginBottom: 4 }}>
                                        ✓ Imported ({results.imported.length})
                                    </div>
                                    {results.imported.map(i => (
                                        <div key={i.source_key} style={{ fontSize: '0.85rem', padding: '2px 0', color: 'var(--text-secondary)' }}>
                                            {i.source_key} — {i.title}
                                        </div>
                                    ))}
                                </div>
                            )}
                            {results.skipped?.length > 0 && (
                                <div style={{ marginBottom: 12 }}>
                                    <div style={{ fontWeight: 600, color: '#facc15', marginBottom: 4 }}>
                                        ⏭ Skipped ({results.skipped.length})
                                    </div>
                                    {results.skipped.map(i => (
                                        <div key={i.source_key} style={{ fontSize: '0.85rem', padding: '2px 0', color: 'var(--text-secondary)' }}>
                                            {i.source_key} — {i.reason}
                                        </div>
                                    ))}
                                </div>
                            )}
                            {results.failed?.length > 0 && (
                                <div style={{ marginBottom: 12 }}>
                                    <div style={{ fontWeight: 600, color: 'var(--accent-red, #f87171)', marginBottom: 4 }}>
                                        ✗ Failed ({results.failed.length})
                                    </div>
                                    {results.failed.map(i => (
                                        <div key={i.source_key} style={{ fontSize: '0.85rem', padding: '2px 0', color: 'var(--text-secondary)' }}>
                                            {i.source_key} — {i.reason}
                                        </div>
                                    ))}
                                </div>
                            )}
                        </div>
                    ) : (
                        <>
                            {/* Search controls */}
                            {source === 'jira' ? (
                                <div style={{ marginBottom: 14 }}>
                                    <label style={labelStyle}>JQL Query</label>
                                    <div style={{ display: 'flex', gap: 8 }}>
                                        <input
                                            className="modern-input"
                                            style={{ flex: 1 }}
                                            placeholder='e.g. project = PROJ AND type = Story'
                                            value={jql}
                                            onChange={e => setJql(e.target.value)}
                                            onKeyDown={e => { if (e.key === 'Enter') handleJiraSearch(); }}
                                            autoFocus
                                            disabled={importing}
                                        />
                                        <button
                                            className="primary-btn"
                                            onClick={() => handleJiraSearch(0)}
                                            disabled={searching || !jql.trim() || importing}
                                        >
                                            {searching ? 'Searching…' : 'Search'}
                                        </button>
                                    </div>
                                </div>
                            ) : (
                                <div style={{ marginBottom: 14 }}>
                                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12, marginBottom: 10 }}>
                                        <div>
                                            <label style={labelStyle}>Space</label>
                                            <select
                                                className="modern-select"
                                                style={{ width: '100%' }}
                                                value={selectedSpace?.id || ''}
                                                onChange={e => {
                                                    const s = spaces.find(sp => sp.id === e.target.value);
                                                    setSelectedSpace(s || null);
                                                }}
                                                onFocus={loadSpaces}
                                            >
                                                <option value="">— Choose a space —</option>
                                                {spaces.map(s => (
                                                    <option key={s.id} value={s.id}>{s.name} ({s.key})</option>
                                                ))}
                                            </select>
                                        </div>
                                        <div>
                                            <label style={labelStyle}>Label <span style={{ fontWeight: 400, opacity: 0.6 }}>(optional)</span></label>
                                            <input
                                                className="modern-input"
                                                style={{ width: '100%' }}
                                                placeholder="e.g. requirements"
                                                value={labelFilter}
                                                onChange={e => setLabelFilter(e.target.value)}
                                            />
                                        </div>
                                    </div>
                                    <button
                                        className="primary-btn"
                                        onClick={() => { setPages([]); setPagesCursor(null); handleConfluenceSearch(); }}
                                        disabled={searching || !selectedSpace || importing}
                                    >
                                        {searching ? 'Loading…' : 'Load Pages'}
                                    </button>
                                </div>
                            )}

                            {/* Include children toggle (Jira only) */}
                            {source === 'jira' && items.length > 0 && (
                                <label style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10, fontSize: '0.85rem', cursor: 'pointer' }}>
                                    <input
                                        type="checkbox"
                                        checked={includeChildren}
                                        onChange={e => setIncludeChildren(e.target.checked)}
                                    />
                                    Include sub-tickets and child issues
                                </label>
                            )}

                            {/* Results table */}
                            {items.length > 0 && (
                                <>
                                    <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 8, fontSize: '0.82rem' }}>
                                        <button className="action-btn" style={{ fontSize: '0.78rem', padding: '2px 8px' }} onClick={selectAll}>Select All</button>
                                        <button className="action-btn" style={{ fontSize: '0.78rem', padding: '2px 8px' }} onClick={deselectAll}>Deselect All</button>
                                        <span style={{ color: 'var(--text-secondary)', marginLeft: 'auto' }}>
                                            {selected.size} selected
                                            {source === 'jira' && ` of ${total} total`}
                                        </span>
                                    </div>
                                    <div style={{ maxHeight: 300, overflow: 'auto', border: '1px solid var(--border-color)', borderRadius: 6 }}>
                                        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
                                            <thead>
                                                <tr style={{ borderBottom: '1px solid var(--border-color)' }}>
                                                    <th style={{ width: 32, padding: '6px 8px' }}></th>
                                                    <th style={{ ...thStyle, width: source === 'jira' ? 100 : undefined }}>
                                                        {source === 'jira' ? 'Key' : 'Title'}
                                                    </th>
                                                    {source === 'jira' && <th style={thStyle}>Summary</th>}
                                                    <th style={{ ...thStyle, width: 100 }}>Status</th>
                                                </tr>
                                            </thead>
                                            <tbody>
                                                {items.map(item => {
                                                    const key = getKey(item);
                                                    const isImported = item.already_imported;
                                                    return (
                                                        <tr
                                                            key={key}
                                                            style={{
                                                                borderBottom: '1px solid rgba(255,255,255,0.04)',
                                                                opacity: isImported ? 0.45 : 1,
                                                                cursor: isImported ? 'default' : 'pointer',
                                                            }}
                                                            onClick={() => !isImported && toggleSelect(key)}
                                                        >
                                                            <td style={{ padding: '6px 8px', textAlign: 'center' }}>
                                                                <input
                                                                    type="checkbox"
                                                                    checked={selected.has(key)}
                                                                    disabled={isImported}
                                                                    onChange={() => toggleSelect(key)}
                                                                    onClick={e => e.stopPropagation()}
                                                                />
                                                            </td>
                                                            <td style={tdStyle}>
                                                                {getLabel(item)}
                                                                {isImported && (
                                                                    <span style={{ marginLeft: 6, ...alreadyImportedBadgeStyle }}>Already imported</span>
                                                                )}
                                                            </td>
                                                            {source === 'jira' && <td style={tdStyle}>{item.summary}</td>}
                                                            <td style={tdStyle}>{item.status || '—'}</td>
                                                        </tr>
                                                    );
                                                })}
                                            </tbody>
                                        </table>
                                    </div>

                                    {/* Pagination */}
                                    {source === 'jira' && total > maxResults && (
                                        <div style={{ display: 'flex', justifyContent: 'center', gap: 8, marginTop: 10 }}>
                                            <button
                                                className="action-btn"
                                                style={{ fontSize: '0.78rem' }}
                                                disabled={startAt === 0 || searching}
                                                onClick={() => handleJiraSearch(Math.max(0, startAt - maxResults))}
                                            >
                                                ← Previous
                                            </button>
                                            <span style={{ fontSize: '0.78rem', color: 'var(--text-secondary)', lineHeight: '28px' }}>
                                                {startAt + 1}–{Math.min(startAt + maxResults, total)} of {total}
                                            </span>
                                            <button
                                                className="action-btn"
                                                style={{ fontSize: '0.78rem' }}
                                                disabled={startAt + maxResults >= total || searching}
                                                onClick={() => handleJiraSearch(startAt + maxResults)}
                                            >
                                                Next →
                                            </button>
                                        </div>
                                    )}
                                    {source === 'confluence' && pagesCursor && (
                                        <div style={{ textAlign: 'center', padding: 10 }}>
                                            <button
                                                className="action-btn"
                                                onClick={() => handleConfluenceSearch(pagesCursor)}
                                                disabled={searching}
                                            >
                                                {searching ? 'Loading…' : 'Load more'}
                                            </button>
                                        </div>
                                    )}
                                </>
                            )}

                            {searching && items.length === 0 && (
                                <div style={{ textAlign: 'center', padding: 20, color: 'var(--text-secondary)' }}>
                                    Searching…
                                </div>
                            )}
                        </>
                    )}
        </ModalShell>
    );
}

const thStyle = { padding: '6px 10px', textAlign: 'left', fontSize: '0.75rem', textTransform: 'uppercase', color: 'var(--text-secondary)', fontWeight: 600 };
const tdStyle = { padding: '8px 10px', verticalAlign: 'middle' };
