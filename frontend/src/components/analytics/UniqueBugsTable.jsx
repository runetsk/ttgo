import React, { useState } from 'react';
import { relativeTime } from './utils';

const PRIORITY_CONFIG = {
    'Highest': { color: '#ef4444', icon: '⬆⬆', bg: 'rgba(239,68,68,0.08)' },
    'High': { color: '#f97316', icon: '⬆', bg: 'rgba(249,115,22,0.08)' },
    'Medium': { color: '#eab308', icon: '—', bg: 'rgba(234,179,8,0.08)' },
    'Low': { color: '#22c55e', icon: '⬇', bg: 'rgba(34,197,94,0.08)' },
    'Lowest': { color: '#6b7280', icon: '⬇⬇', bg: 'rgba(107,114,128,0.08)' },
};

const STATUS_CAT_CONFIG = {
    'todo': { label: 'To Do', color: '#9ca3af', icon: '○' },
    'indeterminate': { label: 'In Progress', color: '#3b82f6', icon: '◐' },
    'done': { label: 'Done', color: '#22c55e', icon: '●' },
};

function StatusBadge({ status, statusCategory }) {
    const cat = STATUS_CAT_CONFIG[statusCategory] || { label: status || 'Unknown', color: '#9ca3af', icon: '○' };
    return (
        <span className="analytics-rate-badge" style={{
            backgroundColor: cat.color + '18',
            color: cat.color,
            border: `1px solid ${cat.color}33`,
            gap: 4,
            display: 'inline-flex',
            alignItems: 'center',
        }}>
            <span style={{ fontSize: '0.7em' }}>{cat.icon}</span>
            {status || cat.label}
        </span>
    );
}

function PriorityCell({ priority }) {
    const cfg = PRIORITY_CONFIG[priority];
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
            {priority}
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
                <div className="analytics-empty-text">No defect links found</div>
                <div className="analytics-empty-hint">Link Jira issues to test cases to see them here</div>
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

    const SortHeader = ({ field, children, align }) => (
        <th
            className="analytics-sortable-th"
            onClick={() => handleSort(field)}
            style={{ textAlign: align || 'left', cursor: 'pointer', userSelect: 'none' }}
        >
            <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
                {children}
                <span style={{ opacity: sortField === field ? 1 : 0.5, fontSize: '0.65em' }}>
                    {sortField === field ? (sortDir === 'asc' ? '▲' : '▼') : '▲'}
                </span>
            </span>
        </th>
    );

    // Summary stats
    const totalOpen = bugs.filter(b => b.status_category !== 'done').length;
    const totalDone = bugs.filter(b => b.status_category === 'done').length;

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
                    <span style={{ fontWeight: 700, color: '#22c55e', fontSize: '1.1rem' }}>{totalDone}</span>
                    <span style={{ color: 'var(--text-secondary)' }}>Resolved</span>
                </div>
            </div>

            <div className="analytics-table-wrap" style={{ maxHeight: 420, overflowY: 'auto' }}>
                <table className="analytics-table">
                    <thead style={{ position: 'sticky', top: 0, background: 'var(--bg-primary)', zIndex: 1 }}>
                        <tr>
                            <SortHeader field="jira_issue_key">Issue</SortHeader>
                            <th>Summary</th>
                            <SortHeader field="status">Status</SortHeader>
                            <SortHeader field="priority">Priority</SortHeader>
                            <th>Assignee</th>
                            <SortHeader field="linked_test_count" align="center">Tests</SortHeader>
                            <SortHeader field="first_linked_at">Linked</SortHeader>
                        </tr>
                    </thead>
                    <tbody>
                        {sorted.map((bug, i) => (
                            <tr key={bug.jira_issue_key || i}>
                                <td style={{ whiteSpace: 'nowrap' }}>
                                    {bug.url ? (
                                        <a href={bug.url} target="_blank" rel="noopener noreferrer"
                                            style={{
                                                color: 'var(--accent-blue)',
                                                textDecoration: 'none',
                                                fontWeight: 600,
                                                display: 'inline-flex',
                                                alignItems: 'center',
                                                gap: 4,
                                            }}>
                                            {bug.jira_issue_key}
                                            <span style={{ fontSize: '0.7em', opacity: 0.6 }}>↗</span>
                                        </a>
                                    ) : (
                                        <span style={{ fontWeight: 600 }}>{bug.jira_issue_key}</span>
                                    )}
                                </td>
                                <td style={{
                                    maxWidth: 280,
                                    overflow: 'hidden',
                                    textOverflow: 'ellipsis',
                                    whiteSpace: 'nowrap',
                                }} title={bug.summary}>
                                    {bug.summary || '—'}
                                </td>
                                <td>
                                    <StatusBadge status={bug.status} statusCategory={bug.status_category} />
                                </td>
                                <td>
                                    <PriorityCell priority={bug.priority} />
                                </td>
                                <td>
                                    {bug.assignee ? (
                                        <span style={{
                                            display: 'inline-flex',
                                            alignItems: 'center',
                                            gap: 6,
                                            fontSize: '0.84rem',
                                        }}>
                                            <span style={{
                                                width: 22,
                                                height: 22,
                                                borderRadius: '50%',
                                                background: 'var(--accent-indigo)',
                                                color: 'white',
                                                display: 'inline-flex',
                                                alignItems: 'center',
                                                justifyContent: 'center',
                                                fontSize: '0.65rem',
                                                fontWeight: 700,
                                                flexShrink: 0,
                                            }}>
                                                {bug.assignee.charAt(0).toUpperCase()}
                                            </span>
                                            {bug.assignee}
                                        </span>
                                    ) : (
                                        <span style={{ color: 'var(--text-secondary)', fontSize: '0.84rem' }}>Unassigned</span>
                                    )}
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
