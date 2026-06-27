import React, { useState, useEffect } from 'react';
import { resultDefectLinks } from '../api';
import CreateDefectModal from './CreateDefectModal';

// Status category → badge style
const categoryStyle = {
    done:          { background: 'rgba(52,211,153,0.15)', color: '#34d399', border: '1px solid rgba(52,211,153,0.35)' },
    todo:          { background: 'rgba(148,163,184,0.15)', color: '#94a3b8', border: '1px solid rgba(148,163,184,0.35)' },
    indeterminate: { background: 'rgba(251,191,36,0.15)', color: '#fbbf24', border: '1px solid rgba(251,191,36,0.35)' },
};
const defaultBadge = { background: 'rgba(148,163,184,0.10)', color: '#94a3b8', border: '1px solid rgba(148,163,184,0.25)' };

function StatusBadge({ link }) {
    const style = categoryStyle[link.status_category] || defaultBadge;
    const label = link.last_known_status || '—';
    return (
        <span style={{ ...style, padding: '1px 8px', borderRadius: 99, fontSize: '0.75rem', fontWeight: 600, whiteSpace: 'nowrap' }}>
            {label}
        </span>
    );
}

/**
 * DefectLinkPanel
 *
 * Displays Jira defect links for a test case (FR-005, 008-jira-integration).
 * Works both in the test-case editor and inside run result rows.
 *
 * Props:
 *   resultId        {string}  — test case whose defects are managed
 *   createDefectContext {object|null}
 *     When provided, a "New Issue" button appears in the header to open the
 *     Create Defect modal pre-filled with run failure context:
 *       { testName, errorMessage, stackTrace }
 *   containerStyle    {object}  — optional style override for the outer wrapper
 */
export default function DefectLinkPanel({ resultId, runId, createDefectContext = null, containerStyle }) {
    const [data, setData] = useState({ links: [] });
    const [loading, setLoading] = useState(true);
    const [linkKey, setLinkKey] = useState('');
    const [linking, setLinking] = useState(false);
    const [unlinking, setUnlinking] = useState(null); // jiraKey being unlinked
    const [showCreateModal, setShowCreateModal] = useState(false);

    useEffect(() => {
        if (!resultId || !runId) return;
        setLoading(true);
        setData({ links: [] });
        resultDefectLinks.list(runId, resultId)
            .then(d => setData({ links: d?.links || [] }))
            .catch(() => {})
            .finally(() => setLoading(false));
    }, [resultId, runId]);

    const handleLink = () => {
        const key = linkKey.trim().toUpperCase();
        if (!key) return;
        setLinking(true);
        resultDefectLinks.link(runId, resultId, key)
            .then(link => {
                setData(prev => ({ ...prev, links: [link, ...prev.links] }));
                setLinkKey('');
            })
            .catch(() => {})
            .finally(() => setLinking(false));
    };

    const handleUnlink = (jiraKey) => {
        setUnlinking(jiraKey);
        resultDefectLinks.unlink(runId, resultId, jiraKey)
            .then(() => setData(prev => ({ ...prev, links: prev.links.filter(l => l.jira_issue_key !== jiraKey) })))
            .catch(() => {})
            .finally(() => setUnlinking(null));
    };

    const handleDefectCreated = (newLink) => {
        setData(prev => ({ ...prev, links: [newLink, ...prev.links] }));
    };

    return (
        <div style={{ marginTop: 16, ...containerStyle }}>
            {/* ── Header row ── */}
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10, flexWrap: 'wrap' }}>
                <h4 style={{ margin: 0, color: 'var(--text-secondary)', fontSize: '0.85rem', textTransform: 'uppercase', letterSpacing: '0.05em', flex: 1 }}>
                    Linked Defects
                </h4>
                {createDefectContext && (
                    <button
                        className="action-btn"
                        style={{ fontSize: '0.78rem', padding: '2px 10px' }}
                        onClick={() => setShowCreateModal(true)}
                        title="Create a new Jira issue and link it to this test case"
                    >
                        🐛 New Issue
                    </button>
                )}
            </div>

            {/* ── Defect list ── */}
            {loading && <p style={{ color: 'var(--text-secondary)', fontSize: '0.85em', margin: '4px 0' }}>Loading…</p>}
            {!loading && data.links.length === 0 && (
                <p style={{ color: 'var(--text-secondary)', fontSize: '0.85em', fontStyle: 'italic', margin: '4px 0' }}>No defects linked yet.</p>
            )}

            {data.links.map(link => (
                <div
                    key={link.jira_issue_key}
                    style={{
                        display: 'flex', alignItems: 'flex-start', gap: 10,
                        padding: '8px 10px', borderRadius: 6, marginBottom: 6,
                        background: 'rgba(255,255,255,0.03)', border: '1px solid var(--border-color)',
                        opacity: link.last_known_status?.startsWith('⚠') ? 0.75 : 1,
                    }}
                >
                    <div style={{ flex: 1, minWidth: 0 }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                            <a
                                href={link.last_known_url || '#'}
                                target="_blank"
                                rel="noopener noreferrer"
                                style={{ fontWeight: 700, color: 'var(--accent-purple, #a78bfa)', fontSize: '0.85rem', textDecoration: 'none' }}
                            >
                                {link.jira_issue_key}
                            </a>
                            <StatusBadge link={link} />
                            {link.last_known_priority && (
                                <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                                    {link.last_known_priority}
                                </span>
                            )}
                            {link.comment_pending && (
                                <span style={{ background: 'rgba(251,191,36,0.15)', color: '#fbbf24', border: '1px solid rgba(251,191,36,0.35)', padding: '1px 8px', borderRadius: 99, fontSize: '0.72rem', fontWeight: 600 }}>
                                    Comment Pending
                                </span>
                            )}
                        </div>
                        <div style={{ fontSize: '0.82rem', color: 'var(--text-primary)', marginTop: 2, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                            {link.last_known_summary || <em style={{ color: 'var(--text-secondary)' }}>Issue not found in Jira</em>}
                        </div>
                        {link.last_known_assignee && (
                            <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: 2 }}>
                                Assignee: {link.last_known_assignee}
                            </div>
                        )}
                        {link.last_synced_at && (
                            <div style={{ fontSize: '0.72rem', color: 'var(--text-secondary)', marginTop: 2 }}>
                                Synced: {new Date(link.last_synced_at).toLocaleString()}
                            </div>
                        )}
                    </div>

                    <div style={{ display: 'flex', gap: 4, flexShrink: 0, alignItems: 'center' }}>
                        <button
                            className="meta-chip-remove"
                            style={{ fontSize: '1rem', lineHeight: 1, opacity: unlinking === link.jira_issue_key ? 0.4 : 1 }}
                            onClick={() => handleUnlink(link.jira_issue_key)}
                            disabled={unlinking === link.jira_issue_key}
                            title="Unlink this defect"
                        >
                            ×
                        </button>
                    </div>
                </div>
            ))}

            {/* ── Link by key input ── */}
            <div style={{ display: 'flex', gap: 8, marginTop: data.links.length > 0 ? 10 : 4 }}>
                <input
                    className="modern-input"
                    style={{ flex: 1, maxWidth: 200, fontSize: '0.85rem' }}
                    placeholder="e.g. PROJ-123"
                    value={linkKey}
                    onChange={e => setLinkKey(e.target.value)}
                    onKeyDown={e => e.key === 'Enter' && handleLink()}
                    disabled={linking}
                />
                <button
                    className="primary-btn"
                    style={{ fontSize: '0.82rem', padding: '4px 14px', opacity: linking ? 0.6 : 1 }}
                    onClick={handleLink}
                    disabled={linking || !linkKey.trim()}
                >
                    {linking ? 'Linking…' : 'Link Issue'}
                </button>
            </div>

            {/* ── Create Defect Modal (rendered inline; only when createDefectContext provided) ── */}
            {showCreateModal && createDefectContext && (
                <CreateDefectModal
                    resultId={resultId}
                    testName={createDefectContext.testName}
                    errorMessage={createDefectContext.errorMessage}
                    stackTrace={createDefectContext.stackTrace}
                    onClose={() => setShowCreateModal(false)}
                    onCreated={handleDefectCreated}
                />
            )}
        </div>
    );
}
