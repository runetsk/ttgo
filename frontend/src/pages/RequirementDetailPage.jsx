import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { requirements as reqApi, traceability as tracApi } from '../api';
import ErrorAlert from '../components/shared/ErrorAlert';
import SourceBadge from '../components/shared/SourceBadge';
import { useAIGeneration } from '../contexts/AIGenerationContext';
import { toast } from '../toast';
import { useSubscription } from '../hooks/useSubscription';
import { useWebSocket } from '../hooks/useWebSocket';
import { useAbortController } from '../hooks/useAbortController';
import SafeHTML from '../components/shared/SafeHTML';

const covBadge = (covered) => ({
    display: 'inline-flex',
    alignItems: 'center',
    gap: 4,
    fontSize: '0.75rem',
    fontWeight: 600,
    padding: '3px 10px',
    borderRadius: 20,
    background: covered ? 'rgba(52,211,153,0.12)' : 'rgba(248,113,113,0.12)',
    color: covered ? 'var(--accent-green, #34d399)' : 'var(--accent-red, #f87171)',
    border: `1px solid ${covered ? 'rgba(52,211,153,0.3)' : 'rgba(248,113,113,0.3)'}`,
    whiteSpace: 'nowrap',
});

function formatDate(dateStr) {
    if (!dateStr) return '--';
    try {
        return new Date(dateStr).toLocaleDateString(undefined, {
            year: 'numeric', month: 'short', day: 'numeric',
            hour: '2-digit', minute: '2-digit',
        });
    } catch {
        return dateStr;
    }
}

export default function RequirementDetailPage() {
    const { reqId } = useParams();
    const navigate = useNavigate();
    const aiGen = useAIGeneration();

    const [req, setReq] = useState(null);
    const [linkedTests, setLinkedTests] = useState([]);
    const [children, setChildren] = useState([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState(null);
    const [postingToJira, setPostingToJira] = useState(false);

    // Editable fields
    const [formId, setFormId] = useState('');
    const [formTitle, setFormTitle] = useState('');
    const [formDesc, setFormDesc] = useState('');
    const [editingDesc, setEditingDesc] = useState(false);
    const [formError, setFormError] = useState('');
    const [saving, setSaving] = useState(false);

    // Track original values to detect changes
    const [origId, setOrigId] = useState('');
    const [origTitle, setOrigTitle] = useState('');
    const [origDesc, setOrigDesc] = useState('');
    const getSignal = useAbortController();

    const loadData = useCallback((signal) => {
        if (!req) setLoading(true);
        setError(null);
        Promise.all([reqApi.get(reqId, signal ? { signal } : undefined), tracApi.getMatrix(signal ? { signal } : undefined)])
            .then(([reqData, matrixData]) => {
                if (signal?.aborted) return;
                setReq(reqData);
                const id = reqData.identifier || '';
                const title = reqData.title || '';
                const desc = reqData.description || '';
                setFormId(id);
                setFormTitle(title);
                setFormDesc(desc);
                setOrigId(id);
                setOrigTitle(title);
                setOrigDesc(desc);
                setEditingDesc(false);
                const row = matrixData?.rows?.find(r => String(r.requirement_id) === String(reqId));
                setLinkedTests(row?.linked_test_cases || []);
                // Fetch children if this requirement might be a parent
                if (reqData.child_count > 0 || reqData.parent_id === null || reqData.parent_id === undefined) {
                    reqApi.listChildren(reqId)
                        .then(kids => { if (!signal?.aborted) setChildren(kids || []); })
                        .catch(() => {});
                }
            })
            .catch(err => {
                if (signal?.aborted) return;
                setError(err?.response?.data?.error || err.message || 'Failed to load requirement');
            })
            .finally(() => {
                if (!signal?.aborted) setLoading(false);
            });
    }, [reqId]);

    // 018-websocket-realtime: subscribe to requirement updates
    const { registerRefresh, unregisterRefresh } = useWebSocket();
    useSubscription(reqId ? `requirement:${reqId}` : null, useCallback((event) => {
        if (event.data?.deleted === 'true') {
            navigate('/requirements');
            return;
        }
        loadData();
    }, [loadData, navigate]));

    useEffect(() => {
        const signal = getSignal();
        loadData(signal);
        registerRefresh('requirementDetail', () => loadData());
        return () => unregisterRefresh('requirementDetail');
    }, [loadData, registerRefresh, unregisterRefresh, getSignal]);

    const hasChanges = useMemo(() => {
        return formId !== origId || formTitle !== origTitle || formDesc !== origDesc;
    }, [formId, formTitle, formDesc, origId, origTitle, origDesc]);

    const handleSave = () => {
        if (!formId.trim()) { setFormError('Identifier is required.'); return; }
        if (!formTitle.trim()) { setFormError('Title is required.'); return; }
        setSaving(true);
        setFormError('');
        reqApi.update(reqId, {
            identifier: formId.trim(),
            title: formTitle.trim(),
            description: formDesc,
        })
            .then(() => loadData())
            .catch(err => setFormError(err.response?.data?.error || err.message || 'An error occurred.'))
            .finally(() => setSaving(false));
    };

    const handleDiscard = () => {
        setFormId(origId);
        setFormTitle(origTitle);
        setFormDesc(origDesc);
        setEditingDesc(false);
        setFormError('');
    };

    const handleAIGenerate = () => {
        if (aiGen.hasUnsaved && !window.confirm('You have un-accepted AI drafts. Opening a new session will discard them. Continue?')) return;
        aiGen.openSession(req);
        navigate('/ai-generate');
    };

    const handleUnlink = () => {
        if (!window.confirm('Remove source link? This cannot be undone. The requirement will become standalone.')) return;
        reqApi.unlink(reqId)
            .then(() => loadData())
            .catch(err => setError(err?.response?.data?.error || err.message || 'Failed to unlink'));
    };

    const handlePostToJira = () => {
        setPostingToJira(true);
        reqApi.postToJira(reqId)
            .then(() => toast.success('Test cases posted to Jira ticket.'))
            .catch(err => toast.error(err?.response?.data?.error || err.message || 'Failed to post to Jira'))
            .finally(() => setPostingToJira(false));
    };

    // Resync state
    const [resyncing, setResyncing] = useState(false);
    const [conflictData, setConflictData] = useState(null); // { local, remote }
    const [resolving, setResolving] = useState(false);

    const handleResync = () => {
        setResyncing(true);
        setError(null);
        setConflictData(null);
        reqApi.resync(reqId)
            .then(result => {
                if (result.action === 'auto_updated') {
                    toast.success('Requirement auto-updated from source.');
                    loadData();
                } else if (result.action === 'conflict') {
                    setConflictData({ local: result.local, remote: result.remote });
                }
            })
            .catch(err => setError(err.response?.data?.error || err.message || 'Re-sync failed.'))
            .finally(() => setResyncing(false));
    };

    const handleResyncResolve = (resolution) => {
        setResolving(true);
        setError(null);
        const remoteTitle = conflictData?.remote?.title || '';
        const remoteDesc = conflictData?.remote?.description || '';
        reqApi.resyncResolve(reqId, resolution, remoteTitle, remoteDesc)
            .then(() => {
                toast.success(resolution === 'accept_remote' ? 'Updated from remote source.' : 'Kept local version.');
                setConflictData(null);
                loadData();
            })
            .catch(err => setError(err.response?.data?.error || err.message || 'Resolve failed.'))
            .finally(() => setResolving(false));
    };

    const testCount = linkedTests.length;
    const covered = testCount > 0;

    if (loading) {
        return (
            <div style={{ padding: 32, textAlign: 'center', color: 'var(--text-secondary)' }}>
                Loading requirement...
            </div>
        );
    }

    if (error) {
        return (
            <div style={{ padding: '24px 28px', maxWidth: 960, margin: '0 auto' }}>
                <button className="action-btn" style={{ marginBottom: 16, fontSize: '0.85rem' }} onClick={() => navigate('/requirements')}>
                    ← Back to Requirements
                </button>
                <div className="glass-panel" style={{ padding: 32, textAlign: 'center' }}>
                    <ErrorAlert message={error} />
                </div>
            </div>
        );
    }

    if (!req) return null;

    return (
        <div style={{ padding: '24px 28px', maxWidth: 960, margin: '0 auto' }}>

            {/* Top bar */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 20, gap: 12, flexWrap: 'wrap' }}>
                <button className="action-btn" style={{ fontSize: '0.85rem' }} onClick={() => navigate('/requirements')}>
                    ← Back to Requirements
                </button>

                <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
                    {req.source_type && (
                        <>
                            <button className="action-btn" style={{ fontSize: '0.82rem' }} onClick={handleResync} disabled={resyncing}>
                                {resyncing ? '↻ Syncing…' : '↻ Re-sync'}
                            </button>
                            <button className="action-btn" style={{ fontSize: '0.82rem', color: 'var(--accent-red, #f87171)', borderColor: 'rgba(248,113,113,0.35)' }} onClick={handleUnlink}>⊘ Unlink</button>
                        </>
                    )}
                    {req.source_type === 'jira' && linkedTests.length > 0 && (
                        <button className="action-btn" style={{ fontSize: '0.82rem' }} onClick={handlePostToJira} disabled={postingToJira}>
                            {postingToJira ? 'Posting…' : 'Post Tests to Jira'}
                        </button>
                    )}
                    {aiGen.aiFeaturesEnabled && (
                        <button className="action-btn" style={{ fontSize: '0.82rem' }} onClick={handleAIGenerate}>✨ AI Generate Tests</button>
                    )}
                    {hasChanges && (
                        <>
                            <div style={{ width: 1, height: 20, background: 'var(--border-color)', margin: '0 4px' }} />
                            <button className="action-btn" onClick={handleDiscard} disabled={saving} style={{ fontSize: '0.82rem' }}>Discard</button>
                            <button className="primary-btn" onClick={handleSave} disabled={saving} style={{ fontSize: '0.85rem' }}>
                                {saving ? 'Saving...' : 'Save Changes'}
                            </button>
                        </>
                    )}
                </div>
            </div>

            <ErrorAlert message={formError} />

            {/* Conflict resolution panel */}
            {conflictData && (
                <div className="glass-panel" style={{ padding: '20px 28px', marginBottom: 16, border: '1px solid rgba(248,113,113,0.3)' }}>
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 14 }}>
                        <h3 style={{ margin: 0, fontSize: '0.85rem', fontWeight: 700, color: 'var(--accent-red, #f87171)' }}>
                            Conflict Detected
                        </h3>
                        <div style={{ display: 'flex', gap: 8 }}>
                            <button
                                className="action-btn"
                                style={{ fontSize: '0.82rem', borderColor: 'rgba(248,113,113,0.35)', color: 'var(--accent-red, #f87171)' }}
                                onClick={() => setConflictData(null)}
                                disabled={resolving}
                            >
                                Dismiss
                            </button>
                            <button
                                className="action-btn"
                                style={{ fontSize: '0.82rem', borderColor: 'var(--accent-purple, #a78bfa)', color: 'var(--accent-purple, #a78bfa)' }}
                                onClick={() => handleResyncResolve('keep_local')}
                                disabled={resolving}
                            >
                                Keep Local
                            </button>
                            <button
                                className="primary-btn"
                                style={{ fontSize: '0.82rem' }}
                                onClick={() => handleResyncResolve('accept_remote')}
                                disabled={resolving}
                            >
                                {resolving ? 'Updating…' : 'Accept Remote'}
                            </button>
                        </div>
                    </div>
                    <div style={{ fontSize: '0.82rem', color: 'var(--text-secondary)', marginBottom: 14 }}>
                        Local edits were detected. Compare versions below and choose how to resolve.
                    </div>
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
                        <div style={{ border: '1px solid var(--border-color)', borderRadius: 8, padding: 14 }}>
                            <div style={{ fontWeight: 600, fontSize: '0.78rem', color: 'var(--accent-purple, #a78bfa)', marginBottom: 8, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                                Local (current)
                            </div>
                            <div style={{ fontWeight: 600, marginBottom: 6 }}>{conflictData.local.title}</div>
                            <div style={{ fontSize: '0.82rem', color: 'var(--text-secondary)', maxHeight: 200, overflow: 'auto', lineHeight: 1.5 }}>
                                <SafeHTML html={conflictData.local.description || '<em>(empty)</em>'} />
                            </div>
                        </div>
                        <div style={{ border: '1px solid var(--border-color)', borderRadius: 8, padding: 14 }}>
                            <div style={{ fontWeight: 600, fontSize: '0.78rem', color: 'var(--accent-blue, #60a5fa)', marginBottom: 8, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                                Remote ({req.source_type === 'jira' ? 'Jira' : req.source_type === 'confluence' ? 'Confluence' : 'Source'})
                            </div>
                            <div style={{ fontWeight: 600, marginBottom: 6 }}>{conflictData.remote.title}</div>
                            <div style={{ fontSize: '0.82rem', color: 'var(--text-secondary)', maxHeight: 200, overflow: 'auto', lineHeight: 1.5 }}>
                                <SafeHTML html={conflictData.remote.description || '<em>(empty)</em>'} />
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {/* Header card */}
            <div className="glass-panel" style={{ padding: '24px 28px', marginBottom: 16 }}>
                {/* Badges row */}
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 16, flexWrap: 'wrap' }}>
                    <input
                        className="modern-input"
                        value={formId}
                        onChange={e => setFormId(e.target.value)}
                        placeholder="Identifier"
                        style={{
                            width: 160,
                            fontWeight: 700,
                            fontSize: '0.85rem',
                            color: 'var(--accent-purple, #a78bfa)',
                            letterSpacing: '0.02em',
                            padding: '5px 10px',
                        }}
                    />
                    {req.source_type && <SourceBadge sourceType={req.source_type} sourceUrl={req.source_url} />}
                    <span style={covBadge(covered)}>
                        {covered ? `✓ ${testCount} test${testCount !== 1 ? 's' : ''}` : '✗ Not covered'}
                    </span>
                </div>

                {/* Title */}
                <input
                    className="modern-input"
                    value={formTitle}
                    onChange={e => setFormTitle(e.target.value)}
                    placeholder="Requirement title"
                    style={{
                        width: '100%',
                        boxSizing: 'border-box',
                        fontSize: '1.3rem',
                        fontWeight: 700,
                        padding: '8px 12px',
                        marginBottom: 14,
                    }}
                />

                {/* Metadata */}
                <div style={{ display: 'flex', gap: 20, flexWrap: 'wrap', fontSize: '0.78rem', color: 'var(--text-secondary)' }}>
                    <span><strong>Created:</strong> {formatDate(req.created_at)}</span>
                    <span><strong>Updated:</strong> {formatDate(req.updated_at)}</span>
                    {req.source_type && (
                        <>
                            <span>
                                <strong>Source:</strong>{' '}
                                {req.source_type === 'jira' ? 'Jira' : req.source_type === 'confluence' ? 'Confluence' : req.source_type}
                                {req.source_key && ` (${req.source_key})`}
                            </span>
                            {req.imported_at && <span><strong>Last synced:</strong> {formatDate(req.imported_at)}</span>}
                            {req.source_url && (
                                <a href={req.source_url} target="_blank" rel="noopener noreferrer" style={{ color: 'var(--accent-blue, #60a5fa)', textDecoration: 'none' }}>
                                    Open source →
                                </a>
                            )}
                        </>
                    )}
                </div>
            </div>

            {/* Description */}
            <div className="glass-panel" style={{ padding: '20px 28px', marginBottom: 16 }}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 10 }}>
                    <label style={{
                        fontSize: '0.75rem',
                        color: 'var(--text-secondary)',
                        fontWeight: 600,
                        textTransform: 'uppercase',
                        letterSpacing: '0.05em',
                    }}>
                        Description
                    </label>
                    {!editingDesc && (
                        <button
                            className="action-btn"
                            style={{ fontSize: '0.75rem', padding: '2px 10px' }}
                            onClick={() => setEditingDesc(true)}
                        >
                            Edit
                        </button>
                    )}
                    {editingDesc && (
                        <button
                            className="action-btn"
                            style={{ fontSize: '0.75rem', padding: '2px 10px' }}
                            onClick={() => setEditingDesc(false)}
                        >
                            Preview
                        </button>
                    )}
                </div>
                {editingDesc ? (
                    <textarea
                        className="modern-input"
                        value={formDesc}
                        onChange={e => setFormDesc(e.target.value)}
                        placeholder="Add a description, acceptance criteria, or context..."
                        style={{
                            width: '100%',
                            boxSizing: 'border-box',
                            minHeight: 200,
                            resize: 'vertical',
                            fontSize: '0.88rem',
                            lineHeight: 1.7,
                            fontFamily: 'monospace',
                        }}
                    />
                ) : formDesc ? (
                    <SafeHTML
                        className="req-description-html"
                        style={{ fontSize: '0.9rem', lineHeight: 1.7, color: 'var(--text-primary)' }}
                        html={formDesc}
                    />
                ) : (
                    <div
                        style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', fontStyle: 'italic', cursor: 'pointer', padding: '12px 0' }}
                        onClick={() => setEditingDesc(true)}
                    >
                        No description. Click to add one...
                    </div>
                )}
            </div>

            {/* Linked Test Cases */}
            <div className="glass-panel" style={{ padding: '20px 28px' }}>
                <h3 style={{
                    margin: '0 0 12px',
                    fontSize: '0.75rem',
                    color: 'var(--text-secondary)',
                    fontWeight: 600,
                    textTransform: 'uppercase',
                    letterSpacing: '0.05em',
                }}>
                    Linked Test Cases
                    <span style={{ marginLeft: 6, opacity: 0.7 }}>({testCount})</span>
                </h3>
                {testCount === 0 ? (
                    <div style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', padding: '16px 0' }}>
                        No test cases linked yet. Link test cases from the test case detail view, or use AI Generate to create them.
                    </div>
                ) : (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                        {linkedTests.map(tc => (
                            <div
                                key={tc.test_case_id}
                                onClick={() => navigate(`/library/tests/${tc.test_case_id}`)}
                                style={{
                                    padding: '10px 14px',
                                    borderRadius: 8,
                                    cursor: 'pointer',
                                    fontSize: '0.88rem',
                                    fontWeight: 500,
                                    color: 'var(--text-primary)',
                                    background: 'rgba(99,102,241,0.05)',
                                    border: '1px solid rgba(99,102,241,0.1)',
                                    transition: 'all 0.15s',
                                    display: 'flex',
                                    alignItems: 'center',
                                    gap: 8,
                                }}
                                onMouseEnter={e => {
                                    e.currentTarget.style.background = 'rgba(99,102,241,0.12)';
                                    e.currentTarget.style.borderColor = 'rgba(99,102,241,0.25)';
                                }}
                                onMouseLeave={e => {
                                    e.currentTarget.style.background = 'rgba(99,102,241,0.05)';
                                    e.currentTarget.style.borderColor = 'rgba(99,102,241,0.1)';
                                }}
                            >
                                <span style={{ fontSize: '0.8rem', opacity: 0.6 }}>🧪</span>
                                {tc.test_case_name}
                            </div>
                        ))}
                    </div>
                )}
            </div>

            {/* Child Issues */}
            {children.length > 0 && (
                <div className="glass-panel" style={{ padding: '20px 28px', marginTop: 16 }}>
                    <h3 style={{
                        margin: '0 0 12px',
                        fontSize: '0.75rem',
                        color: 'var(--text-secondary)',
                        fontWeight: 600,
                        textTransform: 'uppercase',
                        letterSpacing: '0.05em',
                    }}>
                        Child Issues
                        <span style={{ marginLeft: 6, opacity: 0.7 }}>({children.length})</span>
                    </h3>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                        {children.map(child => (
                            <div
                                key={child.id}
                                style={{
                                    padding: '10px 14px',
                                    borderRadius: 8,
                                    fontSize: '0.88rem',
                                    background: 'rgba(96,165,250,0.05)',
                                    border: '1px solid rgba(96,165,250,0.1)',
                                    display: 'flex',
                                    alignItems: 'center',
                                    gap: 10,
                                }}
                            >
                                <span style={{
                                    fontWeight: 700,
                                    fontSize: '0.78rem',
                                    color: 'var(--accent-purple, #a78bfa)',
                                    background: 'rgba(167,139,250,0.1)',
                                    padding: '2px 8px',
                                    borderRadius: 4,
                                    flexShrink: 0,
                                }}>
                                    {child.identifier}
                                </span>
                                <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                                    {child.title}
                                </span>
                                {child.source_url && (
                                    <a
                                        href={child.source_url}
                                        target="_blank"
                                        rel="noopener noreferrer"
                                        onClick={e => e.stopPropagation()}
                                        style={{ fontSize: '0.75rem', color: 'var(--accent-blue, #60a5fa)', flexShrink: 0 }}
                                    >
                                        Jira ↗
                                    </a>
                                )}
                            </div>
                        ))}
                    </div>
                </div>
            )}
        </div>
    );
}
