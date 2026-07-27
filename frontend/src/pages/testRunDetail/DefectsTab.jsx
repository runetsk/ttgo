import React, { useState } from 'react';
import { Link } from 'react-router-dom';

export default function DefectsTab({ runDefectLinks, defectsLoading }) {
    const [expandedDefects, setExpandedDefects] = useState(new Set());

    return (
        <div>
            {defectsLoading && <p style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', padding: 16 }}>Loading defect links...</p>}
            {!defectsLoading && runDefectLinks.length === 0 && (
                <p style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', padding: 16, textAlign: 'center' }}>
                    No defect links in this run yet. Link defects from individual test results.
                </p>
            )}
            {!defectsLoading && runDefectLinks.length > 0 && (() => {
                // Group rows by defect id (RunDefectRow embeds native Defect)
                const grouped = new Map();
                runDefectLinks.forEach(row => {
                    if (!grouped.has(row.id)) {
                        grouped.set(row.id, {
                            id: row.id,
                            title: row.title,
                            status: row.status,
                            severity: row.severity,
                            external_key: row.external_key,
                            external_url: row.external_url,
                            results: [],
                        });
                    }
                    grouped.get(row.id).results.push({
                        id: row.id,
                        test_case_id: row.test_case_id,
                        test_name_snapshot: row.test_name_snapshot,
                        result_status: row.result_status,
                    });
                });
                const severityColor = { critical: '#ef4444', major: '#f97316', minor: '#eab308', trivial: '#6b7280' };
                return (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                        {Array.from(grouped.values()).map(defect => {
                            // Two-tone on purpose (same call as DefectLinkPanel):
                            // `fixed` shares the amber "still outstanding" badge and
                            // prints its own literal status text, so it is not mislabelled.
                            const badgeStyle = defect.status === 'closed'
                                ? { background: 'rgba(52,211,153,0.15)', color: '#34d399', border: '1px solid rgba(52,211,153,0.35)' }
                                : { background: 'rgba(251,191,36,0.15)', color: '#fbbf24', border: '1px solid rgba(251,191,36,0.35)' };
                            return (
                                <div key={defect.id} style={{
                                    border: '1px solid var(--border-color)', borderRadius: 8,
                                    overflow: 'hidden', background: 'var(--bg-primary)',
                                }}>
                                    {/* Defect header — clickable to expand/collapse */}
                                    <div
                                        onClick={() => setExpandedDefects(prev => {
                                            const next = new Set(prev);
                                            next.has(defect.id) ? next.delete(defect.id) : next.add(defect.id);
                                            return next;
                                        })}
                                        style={{
                                            display: 'flex', alignItems: 'center', gap: 12,
                                            padding: '10px 16px', background: 'var(--bg-secondary)',
                                            borderBottom: expandedDefects.has(defect.id) ? '1px solid var(--border-color)' : 'none',
                                            cursor: 'pointer', userSelect: 'none',
                                        }}>
                                        <span style={{ fontSize: '0.68rem', color: 'var(--text-secondary)', flexShrink: 0, width: 12, textAlign: 'center' }}>
                                            {expandedDefects.has(defect.id) ? '▼' : '▶'}
                                        </span>
                                        <span style={{ fontWeight: 700, color: 'var(--text-primary)', fontSize: '0.88rem', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                                            {defect.title}
                                        </span>
                                        <span style={{ ...badgeStyle, padding: '2px 10px', borderRadius: 99, fontSize: '0.72rem', fontWeight: 600, whiteSpace: 'nowrap', flexShrink: 0 }}>
                                            {defect.status}
                                        </span>
                                        <span style={{ fontSize: '0.72rem', color: severityColor[defect.severity] || 'var(--text-secondary)', fontWeight: 600, flexShrink: 0 }}>
                                            {defect.severity}
                                        </span>
                                        {defect.external_url && (
                                            <a href={defect.external_url} target="_blank" rel="noopener noreferrer" onClick={e => e.stopPropagation()}
                                                style={{ fontSize: '0.75rem', color: 'var(--accent-purple, #a78bfa)', flexShrink: 0 }}>
                                                {defect.external_key || 'external'} ↗
                                            </a>
                                        )}
                                        <span style={{
                                            background: 'rgba(239,68,68,0.15)', color: 'var(--accent-red)',
                                            padding: '1px 8px', borderRadius: 99, fontSize: '0.7rem', fontWeight: 700, flexShrink: 0,
                                        }}>
                                            {defect.results.length} test{defect.results.length !== 1 ? 's' : ''}
                                        </span>
                                    </div>
                                    {/* Affected test results — collapsed by default */}
                                    {expandedDefects.has(defect.id) && <div style={{ padding: '6px 16px' }}>
                                        {defect.results.map((res, idx) => (
                                            <div key={`${res.id}-${idx}`} style={{
                                                display: 'flex', alignItems: 'center', gap: 10,
                                                padding: '6px 0',
                                                borderBottom: idx < defect.results.length - 1 ? '1px solid var(--border-color)' : 'none',
                                            }}>
                                                <span style={{ color: 'var(--text-secondary)', fontSize: '0.72rem', width: 16, textAlign: 'center', flexShrink: 0 }}>
                                                    {idx + 1}
                                                </span>
                                                <span style={{ flex: 1, fontSize: '0.82rem' }}>
                                                    {res.test_case_id ? (
                                                        <Link
                                                            to={`/library/tests/${res.test_case_id}`}
                                                            style={{ color: 'var(--accent-indigo)', textDecoration: 'none' }}
                                                        >
                                                            {res.test_name_snapshot}
                                                        </Link>
                                                    ) : (
                                                        res.test_name_snapshot
                                                    )}
                                                </span>
                                                <span className={`status-badge ${(res.result_status || '').toLowerCase()}`} style={{ fontSize: '0.7rem', padding: '2px 8px' }}>
                                                    {res.result_status || '—'}
                                                </span>
                                            </div>
                                        ))}
                                    </div>}
                                </div>
                            );
                        })}
                    </div>
                );
            })()}
        </div>
    );
}
