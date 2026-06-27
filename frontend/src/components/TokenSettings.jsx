import React, { useState, useEffect } from 'react';
import { listTokens, createToken, deleteToken } from '../api';
import { toast } from '../toast';

export default function TokenSettings() {
    const [tokens, setTokens] = useState([]);
    const [desc, setDesc] = useState('');
    const [scope, setScope] = useState('read');
    const [creating, setCreating] = useState(false);
    const [newToken, setNewToken] = useState(null);

    const load = () => listTokens().then(data => setTokens(data.tokens || [])).catch(() => {});
    useEffect(() => { load(); }, []);

    const handleCreate = () => {
        if (!desc) return;
        setCreating(true);
        createToken(desc, scope).then(data => {
            setNewToken(data.token);
            setDesc(''); setScope('read');
            load();
        }).catch(err => toast.error(err.response?.data?.error || err.message))
          .finally(() => setCreating(false));
    };

    const handleDelete = (id) => {
        if (!confirm('Revoke this token? This cannot be undone.')) return;
        deleteToken(id).then(load).catch(err => toast.error(err.response?.data?.error || err.message));
    };

    return (
        <div>
            <h3 style={{ marginTop: 0 }}>API Tokens</h3>
            <p style={{ color: 'var(--text-secondary)', fontSize: '0.9em' }}>Create tokens for CI/CD automation. Tokens are shown once at creation.</p>
            {newToken && (
                <div className="glass-panel" style={{ padding: 16, marginBottom: 24, border: '1px solid var(--accent-green)' }}>
                    <strong>New token (copy now — not shown again):</strong>
                    <code style={{ display: 'block', marginTop: 8, padding: 8, background: 'rgba(0,0,0,0.2)', borderRadius: 4, wordBreak: 'break-all' }}>{newToken}</code>
                    <button className="action-btn" style={{ marginTop: 8 }} onClick={() => setNewToken(null)}>Dismiss</button>
                </div>
            )}
            <div className="glass-panel" style={{ padding: 16, marginBottom: 24 }}>
                <h4 style={{ marginTop: 0 }}>Create New Token</h4>
                <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
                    <input className="modern-input" value={desc} onChange={e => setDesc(e.target.value)} placeholder="Description (e.g. GitHub Actions)" style={{ flex: 1, minWidth: 200 }} />
                    <select className="modern-select" value={scope} onChange={e => setScope(e.target.value)} style={{ width: 120 }}>
                        <option value="read">read</option>
                        <option value="write">write</option>
                    </select>
                    <button className="primary-btn" onClick={handleCreate} disabled={creating || !desc}>{creating ? 'Creating...' : 'Create'}</button>
                </div>
            </div>
            <h4>Existing Tokens</h4>
            {tokens.length === 0 && <div style={{ color: 'var(--text-secondary)' }}>No tokens yet.</div>}
            {tokens.map(t => (
                <div key={t.id} className="glass-panel" style={{ padding: '12px 16px', marginBottom: 8, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <div>
                        <div style={{ fontWeight: 600 }}>{t.description}</div>
                        <div style={{ fontSize: '0.8em', color: 'var(--text-secondary)' }}>Scope: {t.scope} • Created: {new Date(t.created_at).toLocaleDateString()}{t.last_used_at && ` • Last used: ${new Date(t.last_used_at).toLocaleDateString()}`}</div>
                    </div>
                    <button className="action-btn" style={{ color: 'var(--accent-red)' }} onClick={() => handleDelete(t.id)}>Revoke</button>
                </div>
            ))}
        </div>
    );
}
