import React, { useState, useEffect, useMemo } from 'react';
import { defects as defectsApi } from '../api';

const SEVERITIES = ['critical', 'major', 'minor', 'trivial'];

export default function DefectsPage() {
    const [rows, setRows] = useState([]);
    const [loading, setLoading] = useState(true);
    const [statusFilter, setStatusFilter] = useState('all');
    const [severityFilter, setSeverityFilter] = useState('all');
    const [search, setSearch] = useState('');

    useEffect(() => {
        const params = {};
        if (statusFilter !== 'all') params.status = statusFilter;
        if (severityFilter !== 'all') params.severity = severityFilter;
        if (search.trim()) params.q = search.trim();
        let cancelled = false;
        defectsApi.list(params).then(d => {
            if (!cancelled) { setRows(d || []); setLoading(false); }
        }).catch(() => { if (!cancelled) setLoading(false); });
        return () => { cancelled = true; };
    }, [statusFilter, severityFilter, search]);

    const stats = useMemo(() => ({
        total: rows.length,
        open: rows.filter(r => r.status === 'open').length,
        closed: rows.filter(r => r.status === 'closed').length,
    }), [rows]);

    const toggleStatus = (d) => {
        const next = d.status === 'open' ? 'closed' : 'open';
        defectsApi.update(d.id, { status: next }).then(upd => setRows(prev => prev.map(r => r.id === d.id ? upd : r))).catch(() => {});
    };
    const remove = (d) => {
        if (!window.confirm(`Delete defect "${d.title}"?`)) return;
        defectsApi.remove(d.id).then(() => setRows(prev => prev.filter(r => r.id !== d.id))).catch(() => {});
    };

    return (
        <div style={{ padding: 24 }}>
            <h2 style={{ marginTop: 0 }}>Defects</h2>
            <div className="glass-panel" style={{ display: 'flex', gap: 16, padding: '10px 16px', marginBottom: 16, flexWrap: 'wrap' }}>
                <span><strong>{stats.total}</strong> Total</span>
                <span style={{ color: '#f97316' }}><strong>{stats.open}</strong> Open</span>
                <span style={{ color: '#34d399' }}><strong>{stats.closed}</strong> Closed</span>
            </div>
            <div style={{ display: 'flex', gap: 8, marginBottom: 12, flexWrap: 'wrap' }}>
                <input className="modern-input" style={{ flex: 1, minWidth: 200 }} placeholder="Search title or external key…" value={search} onChange={e => { setLoading(true); setSearch(e.target.value); }} />
                <select className="modern-input" value={statusFilter} onChange={e => { setLoading(true); setStatusFilter(e.target.value); }}>
                    <option value="all">All statuses</option><option value="open">Open</option><option value="closed">Closed</option>
                </select>
                <select className="modern-input" value={severityFilter} onChange={e => { setLoading(true); setSeverityFilter(e.target.value); }}>
                    <option value="all">All severities</option>
                    {SEVERITIES.map(s => <option key={s} value={s}>{s}</option>)}
                </select>
            </div>
            <div className="glass-panel" style={{ overflowX: 'auto' }}>
                <table className="modern-table">
                    <thead><tr><th>Title</th><th>Status</th><th>Severity</th><th>Tests affected</th><th>External</th><th>Updated</th><th></th></tr></thead>
                    <tbody>
                        {loading && <tr><td colSpan={7} style={{ color: 'var(--text-secondary)' }}>Loading…</td></tr>}
                        {!loading && rows.length === 0 && <tr><td colSpan={7} style={{ color: 'var(--text-secondary)', fontStyle: 'italic' }}>No defects.</td></tr>}
                        {rows.map(d => (
                            <tr key={d.id}>
                                <td>{d.title}</td>
                                <td><button className="action-btn" style={{ fontSize: '0.75rem' }} onClick={() => toggleStatus(d)}>{d.status}</button></td>
                                <td>{d.severity}</td>
                                <td style={{ textAlign: 'center' }}>{d.linked_test_count ?? 0}</td>
                                <td>{d.external_url ? <a href={d.external_url} target="_blank" rel="noopener noreferrer">{d.external_key || 'link'} ↗</a> : <span style={{ color: 'var(--text-secondary)' }}>—</span>}</td>
                                <td style={{ whiteSpace: 'nowrap', fontSize: '0.82rem' }}>{d.updated_at ? new Date(d.updated_at).toLocaleDateString() : '—'}</td>
                                <td><button className="meta-chip-remove" onClick={() => remove(d)} title="Delete">×</button></td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>
        </div>
    );
}
