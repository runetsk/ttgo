import React, { useState, useEffect, useMemo, useRef } from 'react';
import { defects as defectsApi } from '../api';
import { useSearchParams, Link } from 'react-router-dom';
import DefectModal from '../components/DefectModal';

const SEVERITIES = ['critical', 'major', 'minor', 'trivial'];
const severityColor = { critical: '#ef4444', major: '#f97316', minor: '#eab308', trivial: '#6b7280' };
const severityChip = (sev) => {
    const c = severityColor[sev] || '#94a3b8';
    return { background: c + '22', color: c, border: `1px solid ${c}55`, padding: '1px 9px', borderRadius: 99, fontSize: '0.74rem', fontWeight: 600, textTransform: 'capitalize' };
};
const statusPill = (status) => status === 'closed'
    ? { background: 'rgba(52,211,153,0.15)', color: '#34d399', border: '1px solid rgba(52,211,153,0.35)' }
    : { background: 'rgba(251,191,36,0.15)', color: '#fbbf24', border: '1px solid rgba(251,191,36,0.35)' };

const affMuted = { color: 'var(--text-secondary)', fontSize: '0.85rem' };
const affLabel = { fontSize: '0.72rem', textTransform: 'uppercase', letterSpacing: '0.04em', color: 'var(--text-secondary)', marginBottom: 2 };
const affLink = { fontSize: '0.85rem', color: 'var(--accent-purple, #a78bfa)', textDecoration: 'none' };

// Renders the expanded "affected tests" content for one defect's cache entry.
function renderAffected(entry) {
    if (!entry || entry.loading) return <span style={affMuted}>Loading…</span>;
    if (entry.error) return <span style={{ ...affMuted, color: '#ef4444' }}>Couldn't load affected tests.</span>;
    if (entry.tests.length === 0) return <span style={{ ...affMuted, fontStyle: 'italic' }}>No linked tests.</span>;
    return (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <div style={affLabel}>Affected tests</div>
            {entry.tests.map(t => (
                <Link key={t.id} to={`/library/tests/${t.id}`} style={affLink}>{t.name}</Link>
            ))}
        </div>
    );
}

export default function DefectsPage() {
    const [rows, setRows] = useState([]);
    const [loading, setLoading] = useState(true);
    const [statusFilter, setStatusFilter] = useState('all');
    const [severityFilter, setSeverityFilter] = useState('all');
    const [search, setSearch] = useState('');
    const [modal, setModal] = useState(null); // { mode:'create'|'edit', defect } | null
    const [focusId, setFocusId] = useState(null);
    const [searchParams, setSearchParams] = useSearchParams();
    const pendingFocus = useRef(searchParams.get('focus'));

    useEffect(() => {
        const params = {};
        if (statusFilter !== 'all') params.status = statusFilter;
        if (severityFilter !== 'all') params.severity = severityFilter;
        if (search.trim()) params.q = search.trim();
        let cancelled = false;
        defectsApi.list(params).then(d => {
            if (!cancelled) {
                setRows(d || []);
                setLoading(false);
                const fid = pendingFocus.current;
                if (fid) {
                    pendingFocus.current = null;
                    const found = (d || []).find(x => x.id === fid);
                    if (found) {
                        setModal({ mode: 'edit', defect: found });
                        setFocusId(fid);
                        setTimeout(() => { document.getElementById(`defect-row-${fid}`)?.scrollIntoView({ block: 'center' }); }, 60);
                    }
                    setSearchParams({}, { replace: true });
                }
            }
        }).catch(() => { if (!cancelled) setLoading(false); });
        return () => { cancelled = true; };
    }, [statusFilter, severityFilter, search, setSearchParams]);

    const stats = useMemo(() => ({
        total: rows.length,
        open: rows.filter(r => r.status === 'open').length,
        closed: rows.filter(r => r.status === 'closed').length,
    }), [rows]);

    const [expandedId, setExpandedId] = useState(null);
    const [affectedById, setAffectedById] = useState({}); // { [defectId]: { loading, error, tests } }

    const toggleExpand = (d) => {
        const willOpen = expandedId !== d.id;
        setExpandedId(willOpen ? d.id : null);
        if (willOpen && !affectedById[d.id]) {
            setAffectedById(prev => ({ ...prev, [d.id]: { loading: true, error: false, tests: [] } }));
            defectsApi.affectedTests(d.id)
                .then(tests => setAffectedById(prev => ({ ...prev, [d.id]: { loading: false, error: false, tests: tests || [] } })))
                .catch(() => setAffectedById(prev => ({ ...prev, [d.id]: { loading: false, error: true, tests: [] } })));
        }
    };

    const toggleStatus = (d) => {
        const next = d.status === 'open' ? 'closed' : 'open';
        defectsApi.update(d.id, { status: next })
            .then(upd => setRows(prev => prev.map(r => r.id === d.id ? { ...upd, linked_test_count: r.linked_test_count } : r)))
            .catch(() => {});
    };
    const remove = (d) => {
        if (!window.confirm(`Delete defect "${d.title}"?`)) return;
        defectsApi.remove(d.id).then(() => setRows(prev => prev.filter(r => r.id !== d.id))).catch(() => {});
    };
    const handleSaved = (saved) => {
        setRows(prev => modal?.mode === 'edit'
            ? prev.map(r => r.id === saved.id ? { ...saved, linked_test_count: r.linked_test_count } : r)
            : [saved, ...prev]);
    };

    return (
        <div style={{ padding: 24 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 16 }}>
                <h2 style={{ margin: 0 }}>Defects</h2>
                <button className="primary-btn" style={{ marginLeft: 'auto' }} onClick={() => setModal({ mode: 'create', defect: null })}>+ New Defect</button>
            </div>

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
                            <React.Fragment key={d.id}>
                            <tr id={`defect-row-${d.id}`} onClick={() => setModal({ mode: 'edit', defect: d })} style={{ cursor: 'pointer', transition: 'background 0.8s', background: focusId === d.id ? 'rgba(99,102,241,0.14)' : undefined }}>
                                <td>
                                    <div style={{ fontWeight: 600 }}>{d.title}</div>
                                    {d.description && (
                                        <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 460 }}>
                                            {d.description}
                                        </div>
                                    )}
                                </td>
                                <td>
                                    <button onClick={(e) => { e.stopPropagation(); toggleStatus(d); }}
                                        style={{ ...statusPill(d.status), padding: '2px 10px', borderRadius: 99, fontSize: '0.74rem', fontWeight: 600, cursor: 'pointer', fontFamily: 'inherit' }}>
                                        {d.status}
                                    </button>
                                </td>
                                <td><span style={severityChip(d.severity)}>{d.severity}</span></td>
                                <td style={{ textAlign: 'center' }}>
                                    {(d.linked_test_count ?? 0) > 0 ? (
                                        <button onClick={(e) => { e.stopPropagation(); toggleExpand(d); }}
                                            title="Show affected tests"
                                            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--text-primary)', fontFamily: 'inherit', fontSize: '0.85rem', display: 'inline-flex', alignItems: 'center', gap: 4 }}>
                                            <span style={{ fontSize: '0.7rem' }}>{expandedId === d.id ? '▾' : '▸'}</span>
                                            {d.linked_test_count}
                                        </button>
                                    ) : (
                                        <span style={{ color: 'var(--text-secondary)' }}>0</span>
                                    )}
                                </td>
                                <td>{d.external_url
                                    ? <a href={d.external_url} target="_blank" rel="noopener noreferrer" onClick={e => e.stopPropagation()}>{d.external_key || 'link'} ↗</a>
                                    : <span style={{ color: 'var(--text-secondary)' }}>—</span>}</td>
                                <td style={{ whiteSpace: 'nowrap', fontSize: '0.82rem' }}>{d.updated_at ? new Date(d.updated_at).toLocaleDateString() : '—'}</td>
                                <td><button className="meta-chip-remove" onClick={(e) => { e.stopPropagation(); remove(d); }} title="Delete">×</button></td>
                            </tr>
                            {expandedId === d.id && (
                                <tr>
                                    <td colSpan={7} style={{ background: 'rgba(255,255,255,0.02)', padding: '10px 16px' }}>
                                        {renderAffected(affectedById[d.id])}
                                    </td>
                                </tr>
                            )}
                            </React.Fragment>
                        ))}
                    </tbody>
                </table>
            </div>

            {modal && (
                <DefectModal
                    key={modal.defect?.id || 'create'}
                    mode={modal.mode}
                    defect={modal.defect}
                    onClose={() => { setModal(null); if (focusId) setTimeout(() => setFocusId(null), 1200); }}
                    onSaved={handleSaved}
                />
            )}
        </div>
    );
}
