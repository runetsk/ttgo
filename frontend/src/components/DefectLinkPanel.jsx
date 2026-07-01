import React, { useState, useEffect, useRef } from 'react';
import { resultDefects, defects as defectsApi } from '../api';
import CreateDefectModal from './CreateDefectModal';
import { useNavigate } from 'react-router-dom';

const statusStyle = (status) => status === 'closed'
    ? { background: 'rgba(52,211,153,0.15)', color: '#34d399', border: '1px solid rgba(52,211,153,0.35)' }
    : { background: 'rgba(251,191,36,0.15)', color: '#fbbf24', border: '1px solid rgba(251,191,36,0.35)' };
const severityColor = { critical: '#ef4444', major: '#f97316', minor: '#eab308', trivial: '#6b7280' };

export default function DefectLinkPanel({ resultId, runId, createDefectContext = null, containerStyle }) {
    const navigate = useNavigate();
    const [links, setLinks] = useState([]);
    const [loading, setLoading] = useState(true);
    const [showCreate, setShowCreate] = useState(false);
    const [search, setSearch] = useState('');
    const [matches, setMatches] = useState([]);
    const [searching, setSearching] = useState(false);
    const searchTimer = useRef(null);

    useEffect(() => {
        if (!resultId || !runId) return;
        let cancelled = false;
        resultDefects.list(runId, resultId)
            .then(d => { if (!cancelled) { setLinks(d || []); setLoading(false); } })
            .catch(() => { if (!cancelled) setLoading(false); });
        return () => { cancelled = true; };
    }, [resultId, runId]);

    useEffect(() => {
        const q = search.trim();
        let cancelled = false;
        clearTimeout(searchTimer.current);
        searchTimer.current = setTimeout(() => {
            if (!q) {
                if (!cancelled) setMatches([]);
                return;
            }
            defectsApi.list({ q }).then(rows => {
                if (!cancelled) { setMatches(rows || []); setSearching(false); }
            }).catch(() => { if (!cancelled) setSearching(false); });
        }, 250);
        return () => { cancelled = true; clearTimeout(searchTimer.current); };
    }, [search]);

    const prepend = (defect) => setLinks(prev => prev.some(l => l.id === defect.id) ? prev : [defect, ...prev]);
    const handleLinkExisting = (defect) => resultDefects.link(runId, resultId, defect.id).then(() => { prepend(defect); setSearch(''); setMatches([]); }).catch(() => {});
    const handleUnlink = (defectId) => resultDefects.unlink(runId, resultId, defectId).then(() => setLinks(prev => prev.filter(l => l.id !== defectId))).catch(() => {});

    return (
        <div style={{ marginTop: 16, ...containerStyle }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10 }}>
                <h4 style={{ margin: 0, color: 'var(--text-secondary)', fontSize: '0.85rem', textTransform: 'uppercase', letterSpacing: '0.05em', flex: 1 }}>Linked Defects</h4>
                {createDefectContext && (
                    <button className="action-btn" style={{ fontSize: '0.78rem', padding: '2px 10px' }} onClick={() => setShowCreate(true)}>🐛 New Defect</button>
                )}
            </div>

            {loading && <p style={{ color: 'var(--text-secondary)', fontSize: '0.85em' }}>Loading…</p>}
            {!loading && links.length === 0 && <p style={{ color: 'var(--text-secondary)', fontSize: '0.85em', fontStyle: 'italic' }}>No defects linked yet.</p>}

            {links.map(d => (
                <div key={d.id} style={{ display: 'flex', alignItems: 'flex-start', gap: 10, padding: '8px 10px', borderRadius: 6, marginBottom: 6, background: 'rgba(255,255,255,0.03)', border: '1px solid var(--border-color)' }}>
                    <div style={{ flex: 1, minWidth: 0 }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                            <button onClick={() => navigate(`/defects?focus=${d.id}`)} title="Open in Defects" style={{ fontWeight: 700, fontSize: '0.85rem', color: 'var(--text-primary)', background: 'none', border: 'none', padding: 0, cursor: 'pointer', fontFamily: 'inherit', textAlign: 'left' }}>{d.title}</button>
                            <span style={{ ...statusStyle(d.status), padding: '1px 8px', borderRadius: 99, fontSize: '0.72rem', fontWeight: 600 }}>{d.status}</span>
                            <span style={{ fontSize: '0.72rem', color: severityColor[d.severity] || 'var(--text-secondary)', fontWeight: 600 }}>{d.severity}</span>
                            {d.external_url && <a href={d.external_url} target="_blank" rel="noopener noreferrer" style={{ fontSize: '0.75rem', color: 'var(--accent-purple, #a78bfa)' }}>{d.external_key || 'external'} ↗</a>}
                        </div>
                    </div>
                    <button className="meta-chip-remove" style={{ fontSize: '1rem', lineHeight: 1 }} onClick={() => handleUnlink(d.id)} title="Unlink defect">×</button>
                </div>
            ))}

            <div style={{ marginTop: 10, position: 'relative' }}>
                <input className="modern-input" style={{ width: '100%', fontSize: '0.85rem' }} placeholder="Link existing defect — search by title…" value={search} onChange={e => { const v = e.target.value; setSearch(v); if (v.trim()) setSearching(true); else { setSearching(false); setMatches([]); } }} />
                {search.trim() && (
                    <div className="glass-panel" style={{ position: 'absolute', zIndex: 5, width: '100%', marginTop: 4, maxHeight: 200, overflowY: 'auto' }}>
                        {searching && <div style={{ padding: 8, fontSize: '0.8rem', color: 'var(--text-secondary)' }}>Searching…</div>}
                        {!searching && matches.length === 0 && <div style={{ padding: 8, fontSize: '0.8rem', color: 'var(--text-secondary)' }}>No matches.</div>}
                        {matches.filter(m => !links.some(l => l.id === m.id)).map(m => (
                            <button key={m.id} onClick={() => handleLinkExisting(m)} style={{ display: 'block', width: '100%', textAlign: 'left', padding: '6px 10px', background: 'transparent', border: 'none', cursor: 'pointer', color: 'var(--text-primary)', fontSize: '0.82rem' }}>
                                {m.title} <span style={{ color: 'var(--text-secondary)' }}>· {m.status} · {m.severity}</span>
                            </button>
                        ))}
                    </div>
                )}
            </div>

            {showCreate && createDefectContext && (
                <CreateDefectModal runId={runId} resultId={resultId} testName={createDefectContext.testName}
                    errorMessage={createDefectContext.errorMessage} stackTrace={createDefectContext.stackTrace}
                    onClose={() => setShowCreate(false)} onCreated={prepend} />
            )}
        </div>
    );
}
