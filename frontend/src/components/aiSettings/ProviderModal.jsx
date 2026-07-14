import React, { useState, useEffect } from 'react';
import { aiGeneration } from '../../api';
import { toast } from '../../toast';
import { PROVIDER_GROUPS, presetMeta, presetFromConfig } from './constants';
import { m } from './styles';

/* ── Add / Edit Provider Modal ─────────────────────── */
export default function ProviderModal({ provider, onClose, onSaved }) {
    const isEdit = !!provider;
    // Which preset chip is selected. On edit, infer it from the saved config;
    // on add, default to the first preset (OpenAI).
    const initialPresetKey = isEdit ? presetFromConfig(provider).key : 'openai';
    const [presetKey, setPresetKey]         = useState(initialPresetKey);
    const [label, setLabel]                 = useState(provider?.label || '');
    const [endpointURL, setEndpointURL]     = useState(provider?.endpoint_url || presetMeta(initialPresetKey).endpoint);
    const [apiKey, setApiKey]               = useState('');
    const [modelName, setModelName]         = useState(provider?.model_name || '');
    const defaultTimeout = provider?.timeout_seconds || (provider?.provider_type === 'local' ? 600 : 90);
    const [timeoutSeconds, setTimeoutSeconds] = useState(defaultTimeout);
    const [isDefault, setIsDefault]         = useState(provider?.is_default || false);
    const [enabled, setEnabled]             = useState(provider?.enabled !== false);
    const [promptPrice, setPromptPrice]         = useState(provider?.prompt_price_per_mtok ?? '');
    const [completionPrice, setCompletionPrice] = useState(provider?.completion_price_per_mtok ?? '');
    const [saving, setSaving]               = useState(false);

    const preset = presetMeta(presetKey);
    const isLocal = preset.providerType === 'local';
    const endpointRequired = isLocal || presetKey === 'custom';

    // When adding, selecting a preset prefills its default endpoint. Never
    // clobber a saved endpoint while editing.
    useEffect(() => {
        if (!isEdit) setEndpointURL(presetMeta(presetKey).endpoint);
    }, [presetKey, isEdit]);

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
                label, provider_type: preset.providerType, endpoint_url: endpointURL,
                api_key: apiKey, model_name: modelName,
                timeout_seconds: parseInt(timeoutSeconds, 10) || 90,
                is_default: isDefault, enabled,
                prompt_price_per_mtok: promptPrice === '' ? null : parseFloat(promptPrice),
                completion_price_per_mtok: completionPrice === '' ? null : parseFloat(completionPrice),
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

    return (
        <div onClick={onClose} style={m.backdrop}>
            <div onClick={e => e.stopPropagation()} style={m.modal}>
                {/* Accent top bar */}
                <div style={{ ...m.accentBar, background: preset.color }} />

                {/* Header */}
                <div style={m.header}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                        <div style={{ ...m.providerInitial, color: preset.color, background: preset.bg }}>
                            {preset.initial}
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
                    {/* Provider preset selector, grouped by behavior */}
                    <div style={m.field}>
                        <label style={m.fieldLabel}>Provider</label>
                        {PROVIDER_GROUPS.map(group => (
                            <div key={group.key} style={{ marginBottom: 10 }}>
                                <div style={{ fontSize: '0.72rem', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.04em', color: 'var(--text-tertiary)', margin: '4px 0 6px' }}>
                                    {group.label}
                                </div>
                                <div style={m.typeGrid}>
                                    {group.presets.map(pt => (
                                        <button
                                            key={pt.key}
                                            type="button"
                                            onClick={() => setPresetKey(pt.key)}
                                            style={{
                                                ...m.typeCard,
                                                ...(presetKey === pt.key
                                                    ? { borderColor: pt.color, background: pt.bg, color: pt.color }
                                                    : { borderColor: 'var(--border-color)', background: 'rgba(255,255,255,0.02)', color: 'var(--text-secondary)' }
                                                ),
                                            }}
                                        >
                                            <span style={{ fontWeight: 700, fontSize: '0.88rem' }}>{pt.initial}</span>
                                            <span style={{ fontSize: '0.75rem', fontWeight: 500, lineHeight: 1.2, textAlign: 'center' }}>{pt.label}</span>
                                        </button>
                                    ))}
                                </div>
                            </div>
                        ))}
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

                    {/* Model + Timeout */}
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
                        <div style={m.field}>
                            <label style={m.fieldLabel}>
                                Model Name <span style={m.required}>*</span>
                            </label>
                            <input
                                className="modern-input"
                                style={{ width: '100%' }}
                                placeholder={preset.model || 'model-name'}
                                value={modelName}
                                onChange={e => setModelName(e.target.value)}
                                required
                            />
                        </div>
                        <div style={m.field}>
                            <label style={m.fieldLabel}>
                                Timeout (seconds)
                                <span style={{ fontWeight: 400, color: 'var(--text-tertiary)', marginLeft: 6 }}>
                                    {isLocal ? '(600–1800 for local CoT models)' : '(60–300 typical)'}
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
                            Endpoint URL {endpointRequired && <span style={m.required}>*</span>}
                        </label>
                        <input
                            className="modern-input"
                            style={{ width: '100%', fontFamily: 'monospace', fontSize: '0.82rem' }}
                            placeholder={preset.endpoint || 'https://…'}
                            value={endpointURL}
                            onChange={e => setEndpointURL(e.target.value)}
                            required={endpointRequired}
                        />
                    </div>

                    {/* API Key */}
                    <div style={m.field}>
                        <label style={m.fieldLabel}>
                            API Key
                            {isLocal && <span style={{ ...m.chip, marginLeft: 6 }}>Not required</span>}
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
                            placeholder={isEdit ? 'Leave blank to keep existing key' : (isLocal ? 'Not required for local providers' : 'sk-…')}
                            value={apiKey}
                            onChange={e => setApiKey(e.target.value)}
                        />
                    </div>

                    {/* Pricing (optional, for cost analytics) */}
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
                        <div style={m.field}>
                            <label style={m.fieldLabel}>Prompt $ / 1M tokens</label>
                            <input className="modern-input" style={{ width: '100%' }} type="number"
                                min="0" step="0.01" placeholder="unset"
                                value={promptPrice} onChange={e => setPromptPrice(e.target.value)} />
                        </div>
                        <div style={m.field}>
                            <label style={m.fieldLabel}>Completion $ / 1M tokens</label>
                            <input className="modern-input" style={{ width: '100%' }} type="number"
                                min="0" step="0.01" placeholder="unset"
                                value={completionPrice} onChange={e => setCompletionPrice(e.target.value)} />
                        </div>
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
