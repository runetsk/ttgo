import React, { useState, useEffect } from 'react';
import { labelStyle } from './shared/styles';
import { toast } from '../toast';

/**
 * IntegrationSettings -- parameterized integration configuration panel.
 * Used for both Jira and Confluence tabs in SettingsPage.
 *
 * Props:
 *   provider          "jira" | "confluence"
 *   providerLabel     "Jira" | "Confluence"
 *   description       subtitle text
 *   apiGetConfig      () => Promise<config>
 *   apiUpsertConfig   (payload) => Promise<config>
 *   extraFields       [{ key, label, placeholder, defaultValue }]
 *   tokenHintField    response field name indicating token presence
 *   renderTestConnection  (config, enabled) => ReactNode
 */
export default function IntegrationSettings({
    providerLabel,
    description,
    apiGetConfig,
    apiUpsertConfig,
    extraFields = [],
    tokenHintField,
    renderTestConnection,
}) {
    const [config, setConfig] = useState(null);
    const [loading, setLoading] = useState(true);

    // Form state
    const [enabled, setEnabled] = useState(false);
    const [baseUrl, setBaseUrl] = useState('');
    const [email, setEmail] = useState('');
    const [apiToken, setApiToken] = useState('');
    const [extras, setExtras] = useState({});
    const [saving, setSaving] = useState(false);

    useEffect(() => {
        apiGetConfig()
            .then(cfg => {
                setConfig(cfg);
                setEnabled(cfg.enabled ?? false);
                setBaseUrl(cfg.base_url || '');
                setEmail(cfg.email || '');
                // Populate extra fields from config
                const ex = {};
                for (const f of extraFields) {
                    ex[f.key] = cfg[f.key] || f.defaultValue || '';
                }
                setExtras(ex);
            })
            .catch(() => {
                setConfig(null);
                // Initialize extras with defaults
                const ex = {};
                for (const f of extraFields) {
                    ex[f.key] = f.defaultValue || '';
                }
                setExtras(ex);
            })
            .finally(() => setLoading(false));
    }, []); // eslint-disable-line react-hooks/exhaustive-deps

    const handleSave = () => {
        if (!baseUrl.trim()) { toast.error('Base URL is required.'); return; }
        if (!email.trim()) { toast.error('Email is required.'); return; }
        setSaving(true);

        const payload = {
            base_url: baseUrl.trim(),
            email: email.trim(),
            api_token: apiToken,
            enabled,
        };
        for (const f of extraFields) {
            payload[f.key] = (extras[f.key] || '').trim() || f.defaultValue || '';
        }

        apiUpsertConfig(payload)
            .then(cfg => {
                setConfig(cfg);
                setApiToken('');
                toast.success(`${providerLabel} configuration saved.`);
            })
            .catch(err => toast.error(err.response?.data?.error || err.message))
            .finally(() => setSaving(false));
    };

    const setExtra = (key, value) => setExtras(prev => ({ ...prev, [key]: value }));

    if (loading) {
        return <div style={{ color: 'var(--text-secondary)', fontSize: '0.9rem' }}>Loading {providerLabel} configuration...</div>;
    }

    // Token hint display
    const tokenHintValue = config?.[tokenHintField];
    let tokenHintNode = null;
    if (tokenHintValue) {
        // Jira returns a masked string (api_token_masked), Confluence returns a boolean (has_token)
        const hintText = typeof tokenHintValue === 'string'
            ? `current: ${tokenHintValue}`
            : 'token configured';
        tokenHintNode = (
            <span style={{ fontWeight: 400, marginLeft: 8, color: 'var(--text-secondary)' }}>
                ({hintText} — leave blank to keep)
            </span>
        );
    }

    return (
        <div>
            <h3 style={{ marginTop: 0, marginBottom: 4 }}>{providerLabel} Integration</h3>
            <p style={{ color: 'var(--text-secondary)', fontSize: '0.875rem', marginTop: 0, marginBottom: 20 }}>
                {description}
            </p>

            <div className="glass-panel" style={{ padding: 20, marginBottom: 20 }}>
                {/* Enable toggle */}
                <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 18 }}>
                    <label style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer', userSelect: 'none' }}>
                        <input
                            type="checkbox"
                            checked={enabled}
                            onChange={e => setEnabled(e.target.checked)}
                            style={{ width: 16, height: 16, cursor: 'pointer' }}
                        />
                        <span style={{ fontWeight: 600 }}>Enable {providerLabel} integration</span>
                    </label>
                </div>

                <div style={{ display: 'grid', gap: 14, gridTemplateColumns: '1fr 1fr' }}>
                    <div>
                        <label style={labelStyle}>Base URL *</label>
                        <input
                            className="modern-input"
                            style={{ width: '100%' }}
                            placeholder="https://yourcompany.atlassian.net"
                            value={baseUrl}
                            onChange={e => setBaseUrl(e.target.value)}
                        />
                    </div>
                    <div>
                        <label style={labelStyle}>Email *</label>
                        <input
                            className="modern-input"
                            style={{ width: '100%' }}
                            placeholder="you@yourcompany.com"
                            type="email"
                            value={email}
                            onChange={e => setEmail(e.target.value)}
                        />
                    </div>
                    <div style={{ gridColumn: '1 / -1' }}>
                        <label style={labelStyle}>
                            API Token
                            {tokenHintNode}
                        </label>
                        <input
                            className="modern-input"
                            style={{ width: '100%' }}
                            type="password"
                            placeholder={tokenHintValue ? 'Leave blank to keep existing token' : `Your ${providerLabel} API token`}
                            value={apiToken}
                            onChange={e => setApiToken(e.target.value)}
                        />
                    </div>
                    {extraFields.map(f => (
                        <div key={f.key}>
                            <label style={labelStyle}>{f.label}</label>
                            <input
                                className="modern-input"
                                style={{ width: '100%' }}
                                placeholder={f.placeholder}
                                value={extras[f.key] || ''}
                                onChange={e => setExtra(f.key, e.target.value)}
                            />
                        </div>
                    ))}
                </div>

                <div style={{ marginTop: 16 }}>
                    <button className="primary-btn" onClick={handleSave} disabled={saving}>
                        {saving ? 'Saving...' : 'Save Configuration'}
                    </button>
                </div>
            </div>

            {/* Test connection — delegated to parent via render prop */}
            {config && enabled && renderTestConnection && renderTestConnection(config, enabled)}
        </div>
    );
}
