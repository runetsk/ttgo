import React, { useState } from 'react';
import { relativeTime } from './utils';

const SEVERITY_CONFIG = {
    critical: { color: '#ef4444', bg: 'rgba(239,68,68,0.08)' },
    major:    { color: '#f97316', bg: 'rgba(249,115,22,0.08)' },
    minor:    { color: '#eab308', bg: 'rgba(234,179,8,0.08)' },
    trivial:  { color: '#6b7280', bg: 'rgba(107,114,128,0.08)' },
};

const STATUS_CONFIG = {
    closed: { label: 'Closed', color: '#22c55e', icon: '●' },
    open:   { label: 'Open',   color: '#f97316', icon: '○' },
};

function StatusBadge({ status }) {
    const cfg = STATUS_CONFIG[status] || STATUS_CONFIG.open;
    return (
        <span className="analytics-rate-badge" style={{
            backgroundColor: cfg.color + '18',
            color: cfg.color,
            border: `1px solid ${cfg.color}33`,
            gap: 4,
            display: 'inline-flex',
            alignItems: 'center',
        }}>
            <span style={{ fontSize: '0.7em' }}>{cfg.icon}</span>
            {cfg.label}
        </span>
    );
}

function SeverityCell({ severity }) {
    const cfg = SEVERITY_CONFIG[severity];
    if (!cfg) return <span style={{ color: 'var(--text-secondary)' }}>—</span>;
    return (
        <span style={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: 5,
            color: cfg.color,
            fontWeight: 500,
            fontSize: '0.84rem',
        }}>
            <span style={{
                width: 8,
                height: 8,
                borderRadius: '50%',
                backgroundColor: cfg.color,
                flexShrink: 0,
            }} />
            {severity.charAt(0).toUpperCase() + severity.slice(1)}
        </span>
    );
}

export default function UniqueBugsTable({ data }) {
    const bugs = data?.bugs || [];
    const [sortField, setSortField] = useState('linked_test_count');
    const [sortDir, setSortDir] = useState('desc');

    if (bugs.length === 0) {
        return (
            <div className="analytics-empty">
                <div className="analytics-empty-icon">🐛</div>
                <div className="analytics-empty-text">No defects yet</div>
                <div className="analytics-empty-hint">Create or link defects to see them here</div>
            </div>
        );
    }

    const handleSort = (field) => {
        if (sortField === field) {
            setSortDir(prev => prev === 'asc' ? 'desc' : 'asc');
        } else {
            setSortField(field);
            setSortDir('desc');
        }
    };

    const sorted = [...bugs].sort((a, b) => {
        const av = a[sortField], bv = b[sortField];
        const dir = sortDir === 'asc' ? 1 : -1;
        if (typeof av === 'number' && typeof bv === 'number') return (av - bv) * dir;
        return String(av || '').localeCompare(String(bv || '')) * dir;
    });

    const sortTh = (field, label, align) => (
        <th
            key={field}
            className="analytics-sortable-th"
            onClick={() => handleSort(field)}
            style={{ textAlign: align || 'left', cursor: 'pointer', userSelect: 'none' }}
        >
            <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
                {label}
                <span style={{ opacity: sortField === field ? 1 : 0.5, fontSize: '0.65em' }}>
                    {sortField === field ? (sortDir === 'asc' ? '▲' : '▼') : '▲'}
                </span>
            </span>
        </th>
    );

    // Summary stats
    const totalOpen   = bugs.filter(b => b.status === 'open').length;
    const totalClosed = bugs.filter(b => b.status === 'closed').length;

    return (
        <div>
            {/* Summary strip */}
            <div style={{
                display: 'flex',
                gap: 16,
                marginBottom: 16,
                padding: '10px 16px',
                background: 'rgba(255,255,255,0.02)',
                borderRadius: 'var(--radius-md)',
                border: '1px solid var(--border-color)',
                flexWrap: 'wrap',
            }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: '0.84rem' }}>
                    <span style={{ fontWeight: 700, color: 'var(--text-primary)', fontSize: '1.1rem' }}>{bugs.length}</span>
                    <span style={{ color: 'var(--text-secondary)' }}>Total Bugs</span>
                </div>
                <div style={{ width: 1, background: 'var(--border-color)' }} />
                <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: '0.84rem' }}>
                    <span style={{ fontWeight: 700, color: '#f97316', fontSize: '1.1rem' }}>{totalOpen}</span>
                    <span style={{ color: 'var(--text-secondary)' }}>Open</span>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: '0.84rem' }}>
                    <span style={{ fontWeight: 700, color: '#22c55e', fontSize: '1.1rem' }}>{totalClosed}</span>
                    <span style={{ color: 'var(--text-secondary)' }}>Resolved</span>
                </div>
            </div>

            <div className="analytics-table-wrap" style={{ maxHeight: 420, overflowY: 'auto' }}>
                <table className="analytics-table">
                    <thead style={{ position: 'sticky', top: 0, background: 'var(--bg-primary)', zIndex: 1 }}>
                        <tr>
                            {sortTh('title', 'Title')}
                            {sortTh('status', 'Status')}
                            {sortTh('severity', 'Severity')}
                            {sortTh('linked_test_count', 'Tests', 'center')}
                            {sortTh('first_linked_at', 'First Linked')}
                        </tr>
                    </thead>
                    <tbody>
                        {sorted.map((bug, i) => (
                            <tr key={bug.id || i}>
                                <td style={{
                                    maxWidth: 320,
                                    overflow: 'hidden',
                                    textOverflow: 'ellipsis',
                                    whiteSpace: 'nowrap',
                                }} title={bug.title}>
                                    <span style={{ fontWeight: 600 }}>{bug.title || '—'}</span>
                                    {bug.external_url && (
                                        <a
                                            href={bug.external_url}
                                            target="_blank"
                                            rel="noopener noreferrer"
                                            style={{
                                                marginLeft: 6,
                                                color: 'var(--accent-blue)',
                                                textDecoration: 'none',
                                                fontSize: '0.8em',
                                                opacity: 0.8,
                                            }}
                                        >
                                            {bug.external_key || 'link'}
                                            <span style={{ fontSize: '0.7em', opacity: 0.6 }}> ↗</span>
                                        </a>
                                    )}
                                </td>
                                <td>
                                    <StatusBadge status={bug.status} />
                                </td>
                                <td>
                                    <SeverityCell severity={bug.severity} />
                                </td>
                                <td style={{ textAlign: 'center' }}>
                                    <span style={{
                                        display: 'inline-flex',
                                        alignItems: 'center',
                                        justifyContent: 'center',
                                        minWidth: 28,
                                        height: 28,
                                        borderRadius: 'var(--radius-md)',
                                        background: bug.linked_test_count > 3
                                            ? 'rgba(239,68,68,0.1)'
                                            : bug.linked_test_count > 1
                                                ? 'rgba(234,179,8,0.1)'
                                                : 'rgba(255,255,255,0.06)',
                                        color: bug.linked_test_count > 3
                                            ? '#ef4444'
                                            : bug.linked_test_count > 1
                                                ? '#eab308'
                                                : 'var(--text-primary)',
                                        fontWeight: 700,
                                        fontSize: '0.82rem',
                                        padding: '0 6px',
                                    }}>
                                        {bug.linked_test_count}
                                    </span>
                                </td>
                                <td style={{ whiteSpace: 'nowrap', fontSize: '0.82rem' }} title={bug.first_linked_at}>
                                    {relativeTime(bug.first_linked_at)}
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>
        </div>
    );
}
