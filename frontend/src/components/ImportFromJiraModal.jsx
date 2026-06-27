import React, { useState } from 'react';
import { jira as jiraApi, requirements as reqApi } from '../api';
import { toast } from '../toast';
import ModalShell from './shared/ModalShell';
import ErrorAlert from './shared/ErrorAlert';
import SafeHTML from './shared/SafeHTML';
import { labelStyle } from './shared/styles';

/**
 * ImportFromJiraModal — modal for importing a single requirement from a Jira ticket.
 * 011-jira-confluence-import (T011)
 *
 * Flow: enter ticket key → fetch preview → confirm import → requirement created.
 */
export default function ImportFromJiraModal({ onClose, onImported, jiraEnabled }) {
    const [ticketKey, setTicketKey] = useState('');
    const [preview, setPreview] = useState(null);
    const [fetching, setFetching] = useState(false);
    const [importing, setImporting] = useState(false);
    const [includeChildren, setIncludeChildren] = useState(true);
    const [error, setError] = useState('');

    const handleFetchPreview = () => {
        if (!ticketKey.trim()) return;
        setFetching(true);
        setError('');
        setPreview(null);
        jiraApi.fetchTicket(ticketKey.trim())
            .then(result => {
                if (result.success) {
                    setPreview(result);
                } else {
                    setError(result.error || 'Failed to fetch Jira ticket.');
                }
            })
            .catch(err => setError(err.response?.data?.error || err.message || 'Failed to fetch Jira ticket.'))
            .finally(() => setFetching(false));
    };

    const handleImport = () => {
        setImporting(true);
        setError('');
        reqApi.importSingle('jira', preview.key, includeChildren)
            .then(() => {
                toast.success(`Imported requirement from ${preview.key}`);
                onImported?.();
                onClose();
            })
            .catch(err => {
                const data = err.response?.data;
                if (err.response?.status === 409 && data?.existing_id) {
                    setError(`A requirement from ${preview.key} already exists.`);
                    setPreview(prev => prev ? { ...prev, already_imported: true, existing_requirement_id: data.existing_id } : prev);
                } else {
                    setError(data?.error || err.message || 'Import failed.');
                }
            })
            .finally(() => setImporting(false));
    };

    const handleKeyDown = (e) => {
        if (e.key === 'Enter' && !fetching && ticketKey.trim()) {
            handleFetchPreview();
        }
    };

    if (!jiraEnabled) {
        return (
            <ModalShell title="Import from Jira" width={440} onClose={onClose} footer={<button className="action-btn" onClick={onClose}>Close</button>}>
                <div style={{ textAlign: 'center', padding: '32px 24px' }}>
                    <p style={{ color: 'var(--text-secondary)', marginBottom: 16 }}>
                        Jira integration is not configured. Go to <strong>Settings → Jira Integration</strong> to set it up.
                    </p>
                </div>
            </ModalShell>
        );
    }

    return (
        <ModalShell
            title="Import from Jira"
            width={560}
            onClose={onClose}
            footer={<>
                <button className="action-btn" onClick={onClose} disabled={importing}>Cancel</button>
                {preview && !preview.already_imported && (
                    <button className="primary-btn" onClick={handleImport} disabled={importing}>
                        {importing ? 'Importing...' : 'Import Requirement'}
                    </button>
                )}
            </>}
        >
                    <ErrorAlert message={error} />

                    {/* Ticket key input */}
                    <div style={{ marginBottom: 16 }}>
                        <label style={labelStyle}>Jira Ticket Key</label>
                        <div style={{ display: 'flex', gap: 8 }}>
                            <input
                                className="modern-input"
                                style={{ flex: 1 }}
                                placeholder="e.g. PROJ-123"
                                value={ticketKey}
                                onChange={e => { setTicketKey(e.target.value); setError(''); }}
                                onKeyDown={handleKeyDown}
                                autoFocus
                                disabled={importing}
                            />
                            <button
                                className="primary-btn"
                                onClick={handleFetchPreview}
                                disabled={fetching || !ticketKey.trim() || importing}
                                style={{ whiteSpace: 'nowrap' }}
                            >
                                {fetching ? 'Fetching...' : 'Fetch Preview'}
                            </button>
                        </div>
                    </div>

                    {/* Preview */}
                    {preview && (
                        <div style={{ border: '1px solid var(--border-color)', borderRadius: 8, padding: 16, background: 'rgba(255,255,255,0.02)' }}>
                            {/* Already imported warning */}
                            {preview.already_imported && (
                                <div style={{
                                    background: 'rgba(245,158,11,0.1)', color: '#facc15',
                                    border: '1px solid rgba(245,158,11,0.3)',
                                    padding: '8px 12px', borderRadius: 6, marginBottom: 12, fontSize: '0.85rem'
                                }}>
                                    This ticket has already been imported as a requirement.
                                </div>
                            )}

                            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
                                <span style={sourceBadgeStyle}>{preview.key}</span>
                                {preview.status && (
                                    <span style={{ fontSize: '0.78rem', color: 'var(--text-secondary)', background: 'rgba(255,255,255,0.06)', padding: '2px 8px', borderRadius: 4 }}>
                                        {preview.status}
                                    </span>
                                )}
                                {preview.url && (
                                    <a href={preview.url} target="_blank" rel="noopener noreferrer" style={{ fontSize: '0.78rem', color: 'var(--accent-blue, #60a5fa)', marginLeft: 'auto' }}>
                                        Open in Jira ↗
                                    </a>
                                )}
                            </div>

                            <div style={{ fontWeight: 600, fontSize: '1rem', marginBottom: 8 }}>
                                {preview.title}
                            </div>

                            <div style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', maxHeight: 200, overflow: 'auto', lineHeight: 1.6 }}>
                                {preview.description && preview.description.trim() ? (
                                    <SafeHTML html={preview.description} />
                                ) : (
                                    <em style={{ opacity: 0.5 }}>(empty description)</em>
                                )}
                            </div>
                        </div>
                    )}

                    {/* Include children checkbox */}
                    {preview && !preview.already_imported && (
                        <label style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 14, fontSize: '0.85rem', cursor: 'pointer' }}>
                            <input
                                type="checkbox"
                                checked={includeChildren}
                                onChange={e => setIncludeChildren(e.target.checked)}
                            />
                            Include sub-tickets and child issues
                        </label>
                    )}
        </ModalShell>
    );
}

const sourceBadgeStyle = {
    fontWeight: 700,
    fontSize: '0.82rem',
    color: 'var(--accent-purple, #a78bfa)',
    background: 'rgba(167,139,250,0.1)',
    padding: '3px 8px',
    borderRadius: 4,
    letterSpacing: '0.02em',
};
