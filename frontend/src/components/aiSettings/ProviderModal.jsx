import React, { useState, useEffect } from 'react';
import { aiGeneration } from '../../api';
import { toast } from '../../toast';
import { PROVIDER_TYPES, DEFAULT_ENDPOINTS, providerMeta } from './constants';
import { m } from './styles';

/* ── Add / Edit Provider Modal ─────────────────────── */
export default function ProviderModal({ provider, onClose, onSaved }) {
    const isEdit = !!provider;
    const [label, setLabel]                 = useState(provider?.label || '');
    const [providerType, setProviderType]   = useState(provider?.provider_type || 'openai');
    const [endpointURL, setEndpointURL]     = useState(provider?.endpoint_url || DEFAULT_ENDPOINTS['openai']);
    const [apiKey, setApiKey]               = useState('');
    const [modelName, setModelName]         = useState(provider?.model_name || '');
    const defaultTimeout = provider?.timeout_seconds || (provider?.provider_type === 'local' ? 600 : 90);
    const [timeoutSeconds, setTimeoutSeconds] = useState(defaultTimeout);
    const [isDefault, setIsDefault]         = useState(provider?.is_default || false);
    const [enabled, setEnabled]             = useState(provider?.enabled !== false);
    const [saving, setSaving]               = useState(false);

    useEffect(() => {
        if (!isEdit) setEndpointURL(DEFAULT_ENDPOINTS[providerType] || '');
    }, [providerType, isEdit]);

    useEffect(() => {
        const onKey = (e) => { if (e.key === 'Escape') onClose(); };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
    }, [onClose]);

    const handleSubmit = async (e) => {
        e.preventDefault();
        setSaving(true);
        try {
            const data = {
                label, provider_type: providerType, endpoint_url: endpointURL,
                api_key: apiKey, model_name: modelName,
                timeout_seconds: parseInt(timeoutSeconds, 10) || 90,
                is_default: isDefault, enabled,
            };
            if (isEdit) {
                await aiGeneration.updateProvider(provider.id, data);
                toast.success('Provider updated');
            } else {
                await aiGeneration.createProvider(data);
                toast.success('Provider added');
            }
            onSaved();
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Failed to save provider');
        } finally {
            setSaving(false);
        }
    };

    const meta = providerMeta(providerType);

    return (
        <div onClick={onClose} style={m.backdrop}>
            <div onClick={e => e.stopPropagation()} style={m.modal}>
                {/* Accent top bar */}
                <div style={{ ...m.accentBar, background: meta.color }} />

                {/* Header */}
                <div style={m.header}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                        <div style={{ ...m.providerInitial, color: meta.color, background: meta.bg }}>
                            {meta.initial}
                        </div>
                        <div>
                            <h3 style={m.modalTitle}>{isEdit ? 'Edit Provider' : 'Add LLM Provider'}</h3>
                            <p style={m.modalSub}>Configure connection settings for AI test case generation</p>
                        </div>
                    </div>
                    <button onClick={onClose} style={m.closeBtn} aria-label="Close">
                        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round">
                            <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
                        </svg>
                    </button>
                </div>

                <form onSubmit={handleSubmit} style={m.formBody}>
                    {/* Provider Type selector */}
                    <div style={m.field}>
                        <label style={m.fieldLabel}>Provider Type</label>
                        <div style={m.typeGrid}>
                            {PROVIDER_TYPES.map(pt => (
                                <button
                                    key={pt.value}
                                    type="button"
                                    onClick={() => setProviderType(pt.value)}
                                    style={{
                                        ...m.typeCard,
                                        ...(providerType === pt.value
                                            ? { borderColor: pt.color, background: pt.bg, color: pt.color }
                                            : { borderColor: 'var(--border-color)', background: 'rgba(255,255,255,0.02)', color: 'var(--text-secondary)' }
                                        ),
                                    }}
                                >
                                    <span style={{ fontWeight: 700, fontSize: '0.88rem' }}>{pt.initial !== '⚙' ? pt.initial : '⚙'}</span>
                                    <span style={{ fontSize: '0.75rem', fontWeight: 500, lineHeight: 1.2, textAlign: 'center' }}>{pt.label}</span>
                                </button>
                            ))}
                        </div>
                    </div>

                    {/* Label */}
                    <div style={m.field}>
                        <label style={m.fieldLabel}>
                            Display Name <span style={m.required}>*</span>
                        </label>
                        <input
                            className="modern-input"
                            style={{ width: '100%' }}
                            placeholder="e.g. GPT-4o Production"
                            value={label}
                            onChange={e => setLabel(e.target.value)}
                            required
                        />
                    </div>

                    {/* Model + Endpoint */}
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
                        <div style={m.field}>
                            <label style={m.fieldLabel}>
                                Model Name <span style={m.required}>*</span>
                            </label>
                            <input
                                className="modern-input"
                                style={{ width: '100%' }}
                                placeholder={providerType === 'openai' ? 'gpt-4o' : providerType === 'gemini' ? 'gemini-2.5-flash' : providerType === 'anthropic' ? 'claude-sonnet-4-5' : 'llama3'}
                                value={modelName}
                                onChange={e => setModelName(e.target.value)}
                                required
                            />
                        </div>
                        <div style={m.field}>
                            <label style={m.fieldLabel}>
                                Timeout (seconds)
                                <span style={{ fontWeight: 400, color: 'var(--text-tertiary)', marginLeft: 6 }}>
                                    {providerType === 'local' ? '(600–1800 for local CoT models)' : '(60–300 typical)'}
                                </span>
                            </label>
                            <input
                                className="modern-input"
                                style={{ width: '100%' }}
                                type="number"
                                min={10}
                                max={7200}
                                value={timeoutSeconds}
                                onChange={e => setTimeoutSeconds(e.target.value)}
                            />
                        </div>
                    </div>

                    {/* Endpoint URL */}
                    <div style={m.field}>
                        <label style={m.fieldLabel}>
                            Endpoint URL {providerType === 'local' && <span style={m.required}>*</span>}
                        </label>
                        <input
                            className="modern-input"
                            style={{ width: '100%', fontFamily: 'monospace', fontSize: '0.82rem' }}
                            placeholder={DEFAULT_ENDPOINTS[providerType] || 'https://…'}
                            value={endpointURL}
                            onChange={e => setEndpointURL(e.target.value)}
                        />
                    </div>

                    {/* API Key */}
                    <div style={m.field}>
                        <label style={m.fieldLabel}>
                            API Key
                            {providerType === 'local' && <span style={{ ...m.chip, marginLeft: 6 }}>Not required</span>}
                        </label>
                        {isEdit && provider.api_key_masked && (
                            <div style={m.currentKeyNote}>
                                Current: <code style={{ fontFamily: 'monospace' }}>{provider.api_key_masked}</code> — leave blank to keep
                            </div>
                        )}
                        <input
                            className="modern-input"
                            style={{ width: '100%' }}
                            type="password"
                            placeholder={isEdit ? 'Leave blank to keep existing key' : (providerType === 'local' ? 'Not required for local providers' : 'sk-…')}
                            value={apiKey}
                            onChange={e => setApiKey(e.target.value)}
                        />
                    </div>

                    {/* Toggles */}
                    <div style={{ display: 'flex', gap: 20, padding: '4px 0' }}>
                        <label style={m.toggle}>
                            <input type="checkbox" checked={enabled} onChange={e => setEnabled(e.target.checked)} style={m.checkbox} />
                            <span style={m.toggleLabel}>Enabled</span>
                        </label>
                        <label style={m.toggle}>
                            <input type="checkbox" checked={isDefault} onChange={e => setIsDefault(e.target.checked)} style={m.checkbox} />
                            <span style={m.toggleLabel}>Set as default</span>
                        </label>
                    </div>

                    {/* Footer */}
                    <div style={m.footer}>
                        <button type="button" className="action-btn" onClick={onClose}>Cancel</button>
                        <button type="submit" className="primary-btn" disabled={saving}>
                            {saving ? 'Saving…' : (isEdit ? 'Update Provider' : 'Add Provider')}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    );
}
