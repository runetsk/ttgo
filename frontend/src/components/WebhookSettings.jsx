import React, { useState, useEffect } from 'react';
import { listWebhooks, createWebhook, deleteWebhook } from '../api';
import { toast } from '../toast';

export default function WebhookSettings() {
    const [webhooks, setWebhooks] = useState([]);
    const [url, setUrl] = useState('');
    const [desc, setDesc] = useState('');
    const [creating, setCreating] = useState(false);

    const load = () => listWebhooks().then(data => setWebhooks(data.webhooks || [])).catch(() => {});
    useEffect(() => { load(); }, []);

    const handleCreate = () => {
        if (!url) return;
        setCreating(true);
        createWebhook(url, desc).then(() => { setUrl(''); setDesc(''); load(); })
            .catch(err => toast.error(err.response?.data?.error || err.message))
            .finally(() => setCreating(false));
    };

    const handleDelete = (id) => {
        if (!confirm('Delete this webhook?')) return;
        deleteWebhook(id).then(load).catch(err => toast.error(err.response?.data?.error || err.message));
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
                    <button className="action-btn" style={{ color: 'var(--accent-red)' }} onClick={() => handleDelete(wh.id)}>Delete</button>
                </div>
            ))}
        </div>
    );
}
