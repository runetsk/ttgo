import React, { useState, useEffect } from 'react';
import { listWebhooks, createWebhook, deleteWebhook, rotateWebhookSecret } from '../api';
import { toast } from '../toast';

export default function WebhookSettings() {
    const [webhooks, setWebhooks] = useState([]);
    const [url, setUrl] = useState('');
    const [desc, setDesc] = useState('');
    const [creating, setCreating] = useState(false);
    const [revealedSecret, setRevealedSecret] = useState(null); // { id, secret } | null

    const load = () => listWebhooks().then(data => setWebhooks(data.webhooks || [])).catch(() => {});
    useEffect(() => { load(); }, []);

    const handleCreate = () => {
        if (!url) return;
        setCreating(true);
        createWebhook(url, desc).then(res => { setUrl(''); setDesc(''); setRevealedSecret({ id: res.id, secret: res.secret }); load(); })
            .catch(err => toast.error(err.response?.data?.error || err.message))
            .finally(() => setCreating(false));
    };

    const handleDelete = (id) => {
        if (!confirm('Delete this webhook?')) return;
        deleteWebhook(id).then(load).catch(err => toast.error(err.response?.data?.error || err.message));
    };

    const handleRotate = (id) => {
        if (!confirm('Rotate this webhook\'s signing secret? The old secret will stop working immediately.')) return;
        rotateWebhookSecret(id)
            .then(res => setRevealedSecret({ id, secret: res.secret }))
            .catch(err => toast.error(err.response?.data?.error || err.message));
    };

    return (
        <div>
            <h3 style={{ marginTop: 0 }}>Outbound Webhooks</h3>
            <p style={{ color: 'var(--text-secondary)', fontSize: '0.9em' }}>Receive notifications when test runs complete. Only HTTPS endpoints are supported.</p>
            <div className="glass-panel" style={{ padding: 16, marginBottom: 24 }}>
                <h4 style={{ marginTop: 0 }}>Add Webhook</h4>
                <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
                    <input className="modern-input" value={url} onChange={e => setUrl(e.target.value)} placeholder="https://example.com/webhook" style={{ flex: 2, minWidth: 200 }} />
                    <input className="modern-input" value={desc} onChange={e => setDesc(e.target.value)} placeholder="Description (optional)" style={{ flex: 1, minWidth: 150 }} />
                    <button className="primary-btn" onClick={handleCreate} disabled={creating || !url}>{creating ? 'Adding...' : 'Add'}</button>
                </div>
            </div>
            {revealedSecret && (
                <div className="glass-panel" style={{ padding: 16, marginBottom: 24, border: '1px solid var(--accent-amber, #d4a017)' }}>
                    <h4 style={{ marginTop: 0 }}>Signing Secret</h4>
                    <p style={{ color: 'var(--text-secondary)', fontSize: '0.9em', marginTop: 0 }}>
                        Save this signing secret now — it won&apos;t be shown again.
                    </p>
                    <code style={{ display: 'block', padding: '8px 12px', background: 'var(--bg-secondary, rgba(0,0,0,0.2))', borderRadius: 4, fontFamily: 'monospace', fontSize: '0.9em', userSelect: 'all', wordBreak: 'break-all', marginBottom: 12 }}>
                        {revealedSecret.secret}
                    </code>
                    <div style={{ display: 'flex', gap: 8 }}>
                        <button className="action-btn" onClick={() => { navigator.clipboard.writeText(revealedSecret.secret); toast.success('Secret copied'); }}>Copy</button>
                        <button className="action-btn" onClick={() => setRevealedSecret(null)}>Dismiss</button>
                    </div>
                </div>
            )}
            <h4>Active Webhooks</h4>
            {webhooks.length === 0 && <div style={{ color: 'var(--text-secondary)' }}>No webhooks configured.</div>}
            {webhooks.map(wh => (
                <div key={wh.id} className="glass-panel" style={{ padding: '12px 16px', marginBottom: 8, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <div>
                        <div style={{ fontWeight: 600, fontFamily: 'monospace', fontSize: '0.9em' }}>{wh.url}</div>
                        <div style={{ fontSize: '0.8em', color: 'var(--text-secondary)' }}>
                            {wh.description && `${wh.description} • `}Event: {wh.event_type} • {wh.is_active ? '🟢 Active' : '🔴 Inactive'}
                        </div>
                    </div>
                    <div style={{ display: 'flex', gap: 8 }}>
                        <button className="action-btn" onClick={() => handleRotate(wh.id)}>Rotate secret</button>
                        <button className="action-btn" style={{ color: 'var(--accent-red)' }} onClick={() => handleDelete(wh.id)}>Delete</button>
                    </div>
                </div>
            ))}
        </div>
    );
}
