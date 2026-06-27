import React, { useState, useEffect } from 'react';
import { requirements as reqApi } from '../api';
import { toast } from '../toast';
import ModalShell from './shared/ModalShell';
import ErrorAlert from './shared/ErrorAlert';
import SafeHTML from './shared/SafeHTML';

/**
 * ResyncModal — handles re-syncing an imported requirement with its source.
 * 011-jira-confluence-import (T034)
 *
 * Two flows:
 *   1. Auto-update: source unchanged vs local → auto-update, show success
 *   2. Conflict: local edits detected → show side-by-side, let user choose
 */
export default function ResyncModal({ requirement, onClose, onResynced }) {
    const [loading, setLoading] = useState(true);
    const [resolving, setResolving] = useState(false);
    const [data, setData] = useState(null); // { action, requirement?, local?, remote? }
    const [error, setError] = useState('');

    useEffect(() => {
        reqApi.resync(requirement.id)
            .then(result => {
                setData(result);
                if (result.action === 'auto_updated') {
                    toast.success('Requirement auto-updated from source.');
                    onResynced?.();
                }
            })
            .catch(err => setError(err.response?.data?.error || err.message || 'Re-sync failed.'))
            .finally(() => setLoading(false));
    }, [requirement.id]);

    const handleResolve = (resolution) => {
        setResolving(true);
        setError('');
        const remoteTitle = data?.remote?.title || '';
        const remoteDesc = data?.remote?.description || '';
        reqApi.resyncResolve(requirement.id, resolution, remoteTitle, remoteDesc)
            .then(() => {
                toast.success(resolution === 'accept_remote' ? 'Updated from remote source.' : 'Kept local version.');
                onResynced?.();
                onClose();
            })
            .catch(err => setError(err.response?.data?.error || err.message || 'Resolve failed.'))
            .finally(() => setResolving(false));
    };

    const sourceLabel = requirement.source_type === 'jira' ? 'Jira' : requirement.source_type === 'confluence' ? 'Confluence' : 'Source';

    const footer = (
        <>
            <button className="action-btn" onClick={onClose} disabled={resolving}>
                {data?.action === 'auto_updated' ? 'Close' : 'Cancel'}
            </button>
            {data?.action === 'conflict' && (
                <>
                    <button
                        className="action-btn"
                        style={{ borderColor: 'var(--accent-purple, #a78bfa)', color: 'var(--accent-purple, #a78bfa)' }}
                        onClick={() => handleResolve('keep_local')}
                        disabled={resolving}
                    >
                        Keep Local
                    </button>
                    <button
                        className="primary-btn"
                        onClick={() => handleResolve('accept_remote')}
                        disabled={resolving}
                    >
                        {resolving ? 'Updating…' : 'Accept Remote'}
                    </button>
                </>
            )}
        </>
    );

    return (
        <ModalShell title={`Re-sync from ${sourceLabel}`} width={720} onClose={onClose} footer={footer}>
            <ErrorAlert message={error} />

            {loading && (
                <div style={{ textAlign: 'center', padding: 32, color: 'var(--text-secondary)' }}>
                    Fetching latest from {sourceLabel}…
                </div>
            )}

            {/* Auto-updated result */}
            {data?.action === 'auto_updated' && (
                <div style={{ textAlign: 'center', padding: 20 }}>
                    <div style={{ fontSize: '2rem', marginBottom: 10 }}>✓</div>
                    <div style={{ fontWeight: 600, color: 'var(--accent-green, #34d399)', marginBottom: 8 }}>
                        Auto-updated successfully
                    </div>
                    <div style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
                        No local edits were detected, so the requirement was updated automatically from {sourceLabel}.
                    </div>
                </div>
            )}

            {/* Conflict resolution */}
            {data?.action === 'conflict' && (
                <div>
                    <div style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', marginBottom: 14 }}>
                        Local edits were detected. Compare versions below and choose how to resolve.
                    </div>
                    {requirement.source_url && (
                        <div style={{ marginBottom: 14 }}>
                            <a href={requirement.source_url} target="_blank" rel="noopener noreferrer" style={{ fontSize: '0.82rem', color: 'var(--accent-blue, #60a5fa)' }}>
                                Open source in {sourceLabel} ↗
                            </a>
                        </div>
                    )}
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
                        {/* Local */}
                        <div style={{ border: '1px solid var(--border-color)', borderRadius: 8, padding: 14 }}>
                            <div style={{ fontWeight: 600, fontSize: '0.82rem', color: 'var(--accent-purple, #a78bfa)', marginBottom: 8, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                                Local (current)
                            </div>
                            <div style={{ fontWeight: 600, marginBottom: 6 }}>{data.local.title}</div>
                            <div style={{ fontSize: '0.82rem', color: 'var(--text-secondary)', maxHeight: 200, overflow: 'auto', lineHeight: 1.5 }}>
                                <SafeHTML html={data.local.description || '<em>(empty)</em>'} />
                            </div>
                        </div>
                        {/* Remote */}
                        <div style={{ border: '1px solid var(--border-color)', borderRadius: 8, padding: 14 }}>
                            <div style={{ fontWeight: 600, fontSize: '0.82rem', color: 'var(--accent-blue, #60a5fa)', marginBottom: 8, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                                Remote ({sourceLabel})
                            </div>
                            <div style={{ fontWeight: 600, marginBottom: 6 }}>{data.remote.title}</div>
                            <div style={{ fontSize: '0.82rem', color: 'var(--text-secondary)', maxHeight: 200, overflow: 'auto', lineHeight: 1.5 }}>
                                <SafeHTML html={data.remote.description || '<em>(empty)</em>'} />
                            </div>
                        </div>
                    </div>
                </div>
            )}
        </ModalShell>
    );
}
