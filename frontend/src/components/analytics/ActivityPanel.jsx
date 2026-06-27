import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import { relativeTime } from './utils';

const ACTION_CONFIG = {
    'created': { icon: '＋', color: '#22c55e', bg: 'rgba(34,197,94,0.1)', label: 'Created' },
    'updated': { icon: '✎', color: '#3b82f6', bg: 'rgba(59,130,246,0.1)', label: 'Updated' },
    'deleted': { icon: '✕', color: '#ef4444', bg: 'rgba(239,68,68,0.1)', label: 'Deleted' },
    'defect_linked': { icon: '🔗', color: '#f97316', bg: 'rgba(249,115,22,0.1)', label: 'Defect Linked' },
    'defect_unlinked': { icon: '🔓', color: '#9ca3af', bg: 'rgba(156,163,175,0.1)', label: 'Defect Unlinked' },
    'version': { icon: '⟳', color: '#8b5cf6', bg: 'rgba(139,92,246,0.1)', label: 'Version Created' },
    'restored': { icon: '↺', color: '#8b5cf6', bg: 'rgba(139,92,246,0.1)', label: 'Restored' },
    'default': { icon: '•', color: 'var(--text-secondary)', bg: 'rgba(255,255,255,0.05)', label: 'Action' },
};

function getActionConfig(action) {
    if (!action) return ACTION_CONFIG.default;
    const lower = action.toLowerCase();
    if (lower.startsWith('defect_link:created') || lower.includes('defect_link') && lower.includes('created')) return ACTION_CONFIG.defect_linked;
    if (lower.startsWith('defect_link:deleted') || lower.includes('defect_link') && lower.includes('deleted')) return ACTION_CONFIG.defect_unlinked;
    if (lower.includes('created') || lower.includes('create')) return ACTION_CONFIG.created;
    if (lower.includes('deleted') || lower.includes('delete')) return ACTION_CONFIG.deleted;
    if (lower.includes('restored') || lower.includes('restore')) return ACTION_CONFIG.restored;
    if (lower.includes('version')) return ACTION_CONFIG.version;
    if (lower.includes('updated') || lower.includes('changed') || lower.includes('update')) return ACTION_CONFIG.updated;
    return ACTION_CONFIG.default;
}

function formatActionLabel(action) {
    if (!action) return 'Unknown action';
    if (action.startsWith('defect_link:')) {
        const parts = action.split(':');
        if (parts[1] === 'created') return 'Defect linked';
        if (parts[1] === 'deleted') return 'Defect unlinked';
    }
    // Capitalize first letter, replace underscores
    return action.replace(/_/g, ' ').replace(/^\w/, c => c.toUpperCase());
}

const FILTER_OPTIONS = [
    { value: 'all', label: 'All' },
    { value: 'created', label: 'Created' },
    { value: 'updated', label: 'Updated' },
    { value: 'deleted', label: 'Deleted' },
    { value: 'defect', label: 'Defects' },
];

export default function ActivityPanel({ data }) {
    const activities = data?.activities || [];
    const [filter, setFilter] = useState('all');

    if (activities.length === 0) {
        return (
            <div className="analytics-empty">
                <div className="analytics-empty-icon">📋</div>
                <div className="analytics-empty-text">No recent activity</div>
                <div className="analytics-empty-hint">Activity will appear here as changes are made</div>
            </div>
        );
    }

    const filtered = filter === 'all' ? activities : activities.filter(a => {
        const lower = (a.action || '').toLowerCase();
        switch (filter) {
            case 'created': return lower.includes('created') && !lower.startsWith('defect');
            case 'updated': return lower.includes('updated') || lower.includes('changed');
            case 'deleted': return lower.includes('deleted') && !lower.startsWith('defect');
            case 'defect': return lower.startsWith('defect_link');
            default: return true;
        }
    });

    return (
        <div>
            {/* Filter bar */}
            <div style={{
                display: 'flex',
                gap: 4,
                marginBottom: 14,
                flexWrap: 'wrap',
            }}>
                {FILTER_OPTIONS.map(opt => (
                    <button
                        key={opt.value}
                        type="button"
                        className={`analytics-preset-btn${filter === opt.value ? ' active' : ''}`}
                        onClick={() => setFilter(opt.value)}
                        style={{ padding: '4px 12px', fontSize: '0.76rem' }}
                    >
                        {opt.label}
                    </button>
                ))}
                <span style={{
                    marginLeft: 'auto',
                    fontSize: '0.78rem',
                    color: 'var(--text-secondary)',
                    alignSelf: 'center',
                }}>
                    {filtered.length} of {activities.length} activities
                </span>
            </div>

            {/* Timeline */}
            <div style={{
                maxHeight: 450,
                overflowY: 'auto',
                paddingRight: 4,
            }}>
                {filtered.length === 0 ? (
                    <div className="analytics-empty" style={{ padding: '24px 16px' }}>
                        <div className="analytics-empty-text">No matching activities</div>
                    </div>
                ) : (
                    <div style={{ position: 'relative', paddingLeft: 28 }}>
                        {/* Timeline line */}
                        <div style={{
                            position: 'absolute',
                            left: 13,
                            top: 16,
                            bottom: 16,
                            width: 2,
                            background: 'var(--border-color)',
                            borderRadius: 1,
                        }} />

                        {filtered.map((a, i) => {
                            const cfg = getActionConfig(a.action);
                            return (
                                <div key={a.id || i} style={{
                                    position: 'relative',
                                    padding: '10px 14px 10px 20px',
                                    marginBottom: 2,
                                    borderRadius: 'var(--radius-md)',
                                    transition: 'background 0.1s',
                                }}
                                    onMouseEnter={e => e.currentTarget.style.background = 'rgba(255,255,255,0.03)'}
                                    onMouseLeave={e => e.currentTarget.style.background = 'transparent'}
                                >
                                    {/* Timeline dot */}
                                    <div style={{
                                        position: 'absolute',
                                        left: -20,
                                        top: 14,
                                        width: 26,
                                        height: 26,
                                        borderRadius: '50%',
                                        background: cfg.bg,
                                        display: 'flex',
                                        alignItems: 'center',
                                        justifyContent: 'center',
                                        fontSize: '0.72rem',
                                        border: '2px solid var(--bg-primary)',
                                        zIndex: 1,
                                    }}>
                                        <span style={{ color: cfg.color, lineHeight: 1 }}>{cfg.icon}</span>
                                    </div>

                                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', gap: 8, flexWrap: 'wrap' }}>
                                        <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, minWidth: 0 }}>
                                            <span style={{
                                                display: 'inline-block',
                                                padding: '2px 8px',
                                                borderRadius: 4,
                                                fontSize: '0.76rem',
                                                fontWeight: 600,
                                                color: cfg.color,
                                                background: cfg.bg,
                                                whiteSpace: 'nowrap',
                                            }}>
                                                {formatActionLabel(a.action)}
                                            </span>
                                            {a.test_case_id && (
                                                <Link
                                                    to={`/library/tests/${a.test_case_id}`}
                                                    style={{
                                                        fontSize: '0.84rem',
                                                        color: 'var(--accent-indigo)',
                                                        textDecoration: 'none',
                                                        overflow: 'hidden',
                                                        textOverflow: 'ellipsis',
                                                        whiteSpace: 'nowrap',
                                                    }}
                                                >
                                                    {a.test_case_name || a.test_case_id}
                                                </Link>
                                            )}
                                        </div>
                                        <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, flexShrink: 0 }}>
                                            {a.user_id && (
                                                <span style={{
                                                    display: 'inline-flex',
                                                    alignItems: 'center',
                                                    gap: 4,
                                                    fontSize: '0.76rem',
                                                    color: 'var(--text-secondary)',
                                                }}>
                                                    <span style={{
                                                        width: 18,
                                                        height: 18,
                                                        borderRadius: '50%',
                                                        background: 'var(--bg-tertiary)',
                                                        border: '1px solid var(--border-color)',
                                                        display: 'inline-flex',
                                                        alignItems: 'center',
                                                        justifyContent: 'center',
                                                        fontSize: '0.6rem',
                                                        fontWeight: 700,
                                                    }}>
                                                        {a.user_id.charAt(0).toUpperCase()}
                                                    </span>
                                                    {a.user_id}
                                                </span>
                                            )}
                                            <span
                                                style={{
                                                    fontSize: '0.76rem',
                                                    color: 'var(--text-secondary)',
                                                    whiteSpace: 'nowrap',
                                                }}
                                                title={a.timestamp}
                                            >
                                                {relativeTime(a.timestamp)}
                                            </span>
                                        </div>
                                    </div>
                                </div>
                            );
                        })}
                    </div>
                )}
            </div>
        </div>
    );
}
