import React, { useState, useEffect, useCallback } from 'react';
import { aiGeneration } from '../api';
import { toast } from '../toast';
import { useAuth } from '../contexts/AuthContext';

const PROVIDER_TYPES = [
    { value: 'openai',    label: 'OpenAI',           color: '#10a37f', bg: 'rgba(16,163,127,0.12)',  initial: 'O' },
    { value: 'gemini',    label: 'Google Gemini',    color: '#4285f4', bg: 'rgba(66,133,244,0.12)',  initial: 'G' },
    { value: 'anthropic', label: 'Anthropic Claude', color: '#d97706', bg: 'rgba(217,119,6,0.12)',   initial: 'A' },
    { value: 'local',     label: 'Local / Ollama',   color: '#8b5cf6', bg: 'rgba(139,92,246,0.12)',  initial: '⚙' },
];

const DEFAULT_ENDPOINTS = {
    openai:    'https://api.openai.com',
    gemini:    'https://generativelanguage.googleapis.com/v1beta/openai',
    anthropic: 'https://api.anthropic.com',
    local:     'http://localhost:11434',
};

const TEMPLATE_VARS = ['{{COVERAGE}}', '{{TITLE}}', '{{DESCRIPTION}}', '{{CHILDREN}}', '{{DETAIL_LEVEL}}', '{{ADDITIONAL_INSTRUCTIONS}}'];
const REQUIRED_TEMPLATE_VARS = ['{{COVERAGE}}', '{{TITLE}}', '{{DESCRIPTION}}'];
const PARENT_REQUIRED_VARS = ['{{COVERAGE}}', '{{TITLE}}', '{{CHILDREN}}'];

function providerMeta(type) {
    return PROVIDER_TYPES.find(t => t.value === type) || PROVIDER_TYPES[0];
}

/* ── Add / Edit Provider Modal ─────────────────────── */
function ProviderModal({ provider, onClose, onSaved }) {
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

/* ── Delete Confirm Modal ──────────────────────────── */
function DeleteModal({ provider, onCancel, onConfirm }) {
    return (
        <div onClick={onCancel} style={m.backdrop}>
            <div onClick={e => e.stopPropagation()} style={{ ...m.modal, maxWidth: 380 }}>
                <div style={{ padding: '28px 28px 24px' }}>
                    <div style={m.deleteIcon}>
                        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/>
                        </svg>
                    </div>
                    <h3 style={{ margin: '0 0 8px', fontSize: '1rem', fontWeight: 700 }}>Delete Provider</h3>
                    <p style={{ margin: '0 0 24px', fontSize: '0.875rem', color: 'var(--text-secondary)', lineHeight: 1.6 }}>
                        Delete <strong style={{ color: 'var(--text-primary)' }}>{provider.label}</strong>? This action cannot be undone.
                    </p>
                    <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
                        <button className="action-btn" onClick={onCancel}>Cancel</button>
                        <button
                            className="primary-btn"
                            style={{ background: 'var(--accent-red)', boxShadow: '0 4px 12px rgba(239,68,68,0.25)' }}
                            onClick={onConfirm}
                        >
                            Delete
                        </button>
                    </div>
                </div>
            </div>
        </div>
    );
}

/* ── Provider Card ─────────────────────────────────── */
function ProviderCard({ provider, isAdmin, testingId, testResult, onTest, onSetDefault, onEdit, onDelete }) {
    const meta = providerMeta(provider.provider_type);
    const isTesting = testingId === provider.id;

    return (
        <div style={{
            ...s.providerCard,
            borderLeftColor: provider.enabled ? meta.color : 'var(--border-color)',
        }}>
            <div style={s.cardMain}>
                {/* Left: icon + name + badges */}
                <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12, flex: 1, minWidth: 0 }}>
                    <div style={{ ...s.providerAvatar, color: meta.color, background: meta.bg }}>
                        {meta.initial}
                    </div>
                    <div style={{ flex: 1, minWidth: 0 }}>
                        <div style={s.providerNameRow}>
                            <span style={s.providerName}>{provider.label}</span>
                            {provider.is_default && <span style={s.defaultBadge}>★ Default</span>}
                            {!provider.enabled && <span style={s.disabledBadge}>Disabled</span>}
                        </div>
                        <div style={s.providerMeta}>
                            <span style={s.metaChip}>{meta.label}</span>
                            <span style={s.metaDot}>·</span>
                            <span style={{ ...s.metaChip, fontFamily: 'monospace' }}>{provider.model_name}</span>
                            {provider.timeout_seconds && (
                                <>
                                    <span style={s.metaDot}>·</span>
                                    <span style={s.metaChip}>{provider.timeout_seconds}s</span>
                                </>
                            )}
                            {provider.api_key_masked && (
                                <>
                                    <span style={s.metaDot}>·</span>
                                    <span style={{ ...s.metaChip, fontFamily: 'monospace', opacity: 0.7 }}>key: {provider.api_key_masked}</span>
                                </>
                            )}
                        </div>

                        {/* Test result */}
                        {testResult && (
                            <div style={{
                                ...s.testResult,
                                background: testResult.success ? 'rgba(34,197,94,0.07)' : 'rgba(239,68,68,0.07)',
                                borderColor: testResult.success ? 'rgba(34,197,94,0.25)' : 'rgba(239,68,68,0.25)',
                                color: testResult.success ? '#4ade80' : '#f87171',
                            }}>
                                {testResult.success ? '✓ Connection OK' : `✗ ${testResult.error}`}
                            </div>
                        )}
                    </div>
                </div>

                {/* Right: actions */}
                {isAdmin && (
                    <div style={s.cardActions}>
                        <button
                            style={{ ...s.iconBtn, ...(isTesting ? s.iconBtnActive : {}) }}
                            onClick={() => onTest(provider)}
                            disabled={isTesting}
                            title="Test connection"
                        >
                            {isTesting ? (
                                <span style={s.miniSpinner} />
                            ) : (
                                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                    <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/>
                                </svg>
                            )}
                            <span style={s.iconBtnLabel}>{isTesting ? 'Testing…' : 'Test'}</span>
                        </button>

                        {!provider.is_default && (
                            <button style={s.iconBtn} onClick={() => onSetDefault(provider)} title="Set as default">
                                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                    <polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/>
                                </svg>
                                <span style={s.iconBtnLabel}>Default</span>
                            </button>
                        )}

                        <button style={s.iconBtn} onClick={() => onEdit(provider)} title="Edit provider">
                            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                            </svg>
                            <span style={s.iconBtnLabel}>Edit</span>
                        </button>

                        <button style={{ ...s.iconBtn, ...s.iconBtnDanger }} onClick={() => onDelete(provider)} title="Delete provider">
                            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                <polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/>
                            </svg>
                            <span style={s.iconBtnLabel}>Delete</span>
                        </button>
                    </div>
                )}
            </div>
        </div>
    );
}

/* ── Main Component ────────────────────────────────── */
export default function AIGenSettings() {
    const { user: currentUser } = useAuth();
    const isAdmin = currentUser?.role === 'admin';

    const [providers, setProviders]         = useState([]);
    const [loading, setLoading]             = useState(true);
    const [modal, setModal]                 = useState(null);
    const [testingId, setTestingId]         = useState(null);
    const [testResults, setTestResults]     = useState({});
    const [confirmDelete, setConfirmDelete] = useState(null);

    const [template, setTemplate]           = useState(null);
    const [templateContent, setTemplateContent] = useState('');
    const [savingTemplate, setSavingTemplate]   = useState(false);
    const [resettingTemplate, setResettingTemplate] = useState(false);

    const [parentContent, setParentContent]         = useState('');
    const [savingParent, setSavingParent]           = useState(false);
    const [resettingParent, setResettingParent]     = useState(false);

    const [coverageCfg, setCoverageCfg] = useState(null);
    const [coverageForm, setCoverageForm] = useState({ essential_max_tokens: 4096, thorough_max_tokens: 8192, comprehensive_max_tokens: 16384 });
    const [savingCoverage, setSavingCoverage] = useState(false);

    const loadProviders = useCallback(() => {
        setLoading(true);
        aiGeneration.listProviders()
            .then(data => setProviders(data || []))
            .catch(() => toast.error('Failed to load providers'))
            .finally(() => setLoading(false));
    }, []);

    const loadTemplate = useCallback(() => {
        aiGeneration.getTemplate()
            .then(t => { setTemplate(t); setTemplateContent(t.content || ''); setParentContent(t.parent_content || ''); })
            .catch(() => {});
    }, []);

    const loadCoverageConfig = useCallback(() => {
        aiGeneration.getCoverageConfig()
            .then(cfg => { setCoverageCfg(cfg); setCoverageForm({ essential_max_tokens: cfg.essential_max_tokens, thorough_max_tokens: cfg.thorough_max_tokens, comprehensive_max_tokens: cfg.comprehensive_max_tokens }); })
            .catch(() => {});
    }, []);

    useEffect(() => {
        loadProviders();
        loadTemplate();
        loadCoverageConfig();
    }, [loadProviders, loadTemplate, loadCoverageConfig]);

    const handleTestConnection = async (provider) => {
        setTestingId(provider.id);
        setTestResults(prev => ({ ...prev, [provider.id]: null }));
        try {
            const result = await aiGeneration.testConnection(provider.id);
            setTestResults(prev => ({ ...prev, [provider.id]: result }));
        } catch (err) {
            setTestResults(prev => ({ ...prev, [provider.id]: { success: false, error: err?.response?.data?.error || err.message } }));
        } finally {
            setTestingId(null);
        }
    };

    const handleSetDefault = async (provider) => {
        try {
            await aiGeneration.setDefault(provider.id);
            toast.success(`${provider.label} set as default`);
            loadProviders();
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Failed to set default');
        }
    };

    const handleDeleteExecute = async () => {
        if (!confirmDelete) return;
        setConfirmDelete(null);
        try {
            await aiGeneration.deleteProvider(confirmDelete.id);
            toast.success('Provider deleted');
            loadProviders();
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Failed to delete provider');
        }
    };

    const handleSaveTemplate = async () => {
        setSavingTemplate(true);
        try {
            const t = await aiGeneration.updateTemplate(templateContent);
            setTemplate(t);
            if (t.warnings?.length) {
                t.warnings.forEach(w => toast.error(w));
            } else {
                toast.success('Template saved');
            }
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Failed to save template');
        } finally {
            setSavingTemplate(false);
        }
    };

    const missingRequiredVars = REQUIRED_TEMPLATE_VARS.filter(v => !templateContent.includes(v));

    const handleResetTemplate = async () => {
        if (!window.confirm('Reset template to the built-in default?')) return;
        setResettingTemplate(true);
        try {
            const t = await aiGeneration.resetTemplate();
            setTemplate(t);
            setTemplateContent(t.content || '');
            toast.success('Template reset to default');
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Failed to reset template');
        } finally {
            setResettingTemplate(false);
        }
    };

    const templateModified = template && templateContent !== template.content;
    const parentModified = template && parentContent !== (template.parent_content || '');
    const missingParentVars = PARENT_REQUIRED_VARS.filter(v => !parentContent.includes(v));

    const handleSaveParentTemplate = async () => {
        setSavingParent(true);
        try {
            const t = await aiGeneration.updateParentTemplate(parentContent);
            setTemplate(t);
            if (t.warnings?.length) {
                t.warnings.forEach(w => toast.error(w));
            } else {
                toast.success('Parent template saved');
            }
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Failed to save parent template');
        } finally {
            setSavingParent(false);
        }
    };

    const handleResetParentTemplate = async () => {
        if (!window.confirm('Reset parent template to the built-in default?')) return;
        setResettingParent(true);
        try {
            const t = await aiGeneration.resetParentTemplate();
            setTemplate(t);
            setParentContent(t.parent_content || '');
            toast.success('Parent template reset to default');
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Failed to reset parent template');
        } finally {
            setResettingParent(false);
        }
    };

    const coverageModified = coverageCfg && (
        coverageForm.essential_max_tokens !== coverageCfg.essential_max_tokens ||
        coverageForm.thorough_max_tokens !== coverageCfg.thorough_max_tokens ||
        coverageForm.comprehensive_max_tokens !== coverageCfg.comprehensive_max_tokens
    );

    const handleSaveCoverage = async () => {
        setSavingCoverage(true);
        try {
            const cfg = await aiGeneration.updateCoverageConfig(coverageForm);
            setCoverageCfg(cfg);
            setCoverageForm({ essential_max_tokens: cfg.essential_max_tokens, thorough_max_tokens: cfg.thorough_max_tokens, comprehensive_max_tokens: cfg.comprehensive_max_tokens });
            toast.success('Coverage token limits saved');
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Failed to save coverage config');
        } finally {
            setSavingCoverage(false);
        }
    };

    return (
        <div style={s.page}>
            {/* ── Page Header ── */}
            <div style={s.pageHeader}>
                <div style={s.pageHeaderIcon}>
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <path d="M12 2L2 7l10 5 10-5-10-5z"/><path d="M2 17l10 5 10-5"/><path d="M2 12l10 5 10-5"/>
                    </svg>
                </div>
                <div>
                    <h3 style={s.pageTitle}>AI Test Generation</h3>
                    <p style={s.pageDesc}>
                        Configure LLM providers and the prompt template for AI-powered test case generation.
                        API keys are stored server-side and masked in responses.
                    </p>
                </div>
            </div>

            {/* ── LLM Providers Section ── */}
            <section style={s.section}>
                <div style={s.sectionHead}>
                    <div style={s.sectionHeadLeft}>
                        <span style={s.sectionDot} />
                        <h4 style={s.sectionTitle}>LLM Providers</h4>
                        {providers.length > 0 && (
                            <span style={s.sectionCount}>{providers.length}</span>
                        )}
                    </div>
                    {isAdmin && (
                        <button className="primary-btn" style={s.addBtn} onClick={() => setModal({ type: 'add' })}>
                            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round">
                                <line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>
                            </svg>
                            Add Provider
                        </button>
                    )}
                </div>

                {loading ? (
                    <div style={s.loadingState}>
                        <span style={s.loadingSpinner} />
                        Loading providers…
                    </div>
                ) : providers.length === 0 ? (
                    <div style={s.emptyState}>
                        <div style={s.emptyIcon}>
                            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                                <circle cx="12" cy="12" r="3"/><path d="M12 1v4M12 19v4M4.22 4.22l2.83 2.83M16.95 16.95l2.83 2.83M1 12h4M19 12h4M4.22 19.78l2.83-2.83M16.95 7.05l2.83-2.83"/>
                            </svg>
                        </div>
                        <p style={s.emptyTitle}>No providers configured</p>
                        <p style={s.emptyDesc}>
                            {isAdmin
                                ? 'Add an LLM provider to enable AI-powered test case generation.'
                                : 'An admin needs to configure an LLM provider before you can generate test cases.'}
                        </p>
                        {isAdmin && (
                            <button className="primary-btn" style={{ marginTop: 4 }} onClick={() => setModal({ type: 'add' })}>
                                Add Your First Provider
                            </button>
                        )}
                    </div>
                ) : (
                    <div style={s.providerList}>
                        {providers.map(p => (
                            <ProviderCard
                                key={p.id}
                                provider={p}
                                isAdmin={isAdmin}
                                testingId={testingId}
                                testResult={testResults[p.id]}
                                onTest={handleTestConnection}
                                onSetDefault={handleSetDefault}
                                onEdit={(prov) => setModal({ type: 'edit', provider: prov })}
                                onDelete={(prov) => setConfirmDelete(prov)}
                            />
                        ))}
                    </div>
                )}
            </section>

            {/* ── Standard Prompt Template Section ── */}
            <section style={s.section}>
                <div style={s.sectionHead}>
                    <div style={s.sectionHeadLeft}>
                        <span style={s.sectionDot} />
                        <h4 style={s.sectionTitle}>Standard Prompt Template</h4>
                        {templateModified && <span style={s.modifiedBadge}>Unsaved changes</span>}
                    </div>
                    {isAdmin && (
                        <div style={{ display: 'flex', gap: 8 }}>
                            <button
                                className="action-btn"
                                onClick={handleResetTemplate}
                                disabled={resettingTemplate}
                                style={{ fontSize: '0.82rem' }}
                            >
                                {resettingTemplate ? 'Resetting…' : 'Reset to Default'}
                            </button>
                            <button
                                className="primary-btn"
                                onClick={handleSaveTemplate}
                                disabled={savingTemplate || !templateModified}
                                style={{ fontSize: '0.82rem' }}
                            >
                                {savingTemplate ? 'Saving…' : 'Save Template'}
                            </button>
                        </div>
                    )}
                </div>

                <p style={s.templateDesc}>
                    Used when generating tests for a single requirement (no child issues). Use these placeholders:
                </p>

                {/* Variable chips */}
                <div style={s.varChips}>
                    {TEMPLATE_VARS.map(v => (
                        <code key={v} style={{
                            ...s.varChip,
                            ...(REQUIRED_TEMPLATE_VARS.includes(v) && missingRequiredVars.includes(v)
                                ? { borderColor: 'rgba(239,68,68,0.5)', color: '#f87171', background: 'rgba(239,68,68,0.1)' }
                                : {}),
                        }}>{v}</code>
                    ))}
                </div>

                {/* Missing placeholder warning */}
                {missingRequiredVars.length > 0 && (
                    <div style={{
                        display: 'flex', alignItems: 'flex-start', gap: 10,
                        padding: '10px 14px',
                        borderRadius: 8,
                        background: 'rgba(239,68,68,0.08)',
                        border: '1px solid rgba(239,68,68,0.25)',
                        marginBottom: 8,
                    }}>
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#f87171" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ flexShrink: 0, marginTop: 1 }}>
                            <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
                            <line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/>
                        </svg>
                        <div>
                            <span style={{ fontSize: '0.82rem', color: '#f87171', fontWeight: 600 }}>
                                Missing required placeholders:{' '}
                            </span>
                            <span style={{ fontSize: '0.82rem', color: '#fca5a5', fontFamily: 'monospace' }}>
                                {missingRequiredVars.join(', ')}
                            </span>
                            <p style={{ margin: '3px 0 0', fontSize: '0.78rem', color: 'rgba(252,165,165,0.8)', lineHeight: 1.4 }}>
                                Requirement details will <strong>not</strong> be sent to the LLM without these placeholders.
                            </p>
                        </div>
                    </div>
                )}

                {/* Editor */}
                <div style={s.editorWrap}>
                    {!isAdmin && (
                        <div style={s.readOnlyBanner}>
                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/>
                            </svg>
                            View only — admin required to edit
                        </div>
                    )}
                    <textarea
                        className="modern-input"
                        style={{
                            ...s.editor,
                            opacity: isAdmin ? 1 : 0.7,
                        }}
                        value={templateContent}
                        onChange={e => setTemplateContent(e.target.value)}
                        disabled={!isAdmin}
                        placeholder="Loading template…"
                        spellCheck={false}
                    />
                    <div style={s.editorFooter}>
                        <span style={s.charCount}>{templateContent.length} chars</span>
                    </div>
                </div>
            </section>

            {/* ── Parent Prompt Template Section ── */}
            <section style={s.section}>
                <div style={s.sectionHead}>
                    <div style={s.sectionHeadLeft}>
                        <span style={s.sectionDot} />
                        <h4 style={s.sectionTitle}>Parent Prompt Template</h4>
                        <span style={s.templateTypeBadge}>Children</span>
                        {parentModified && <span style={s.modifiedBadge}>Unsaved changes</span>}
                    </div>
                    {isAdmin && (
                        <div style={{ display: 'flex', gap: 8 }}>
                            <button
                                className="action-btn"
                                onClick={handleResetParentTemplate}
                                disabled={resettingParent}
                                style={{ fontSize: '0.82rem' }}
                            >
                                {resettingParent ? 'Resetting…' : 'Reset to Default'}
                            </button>
                            <button
                                className="primary-btn"
                                onClick={handleSaveParentTemplate}
                                disabled={savingParent || !parentModified}
                                style={{ fontSize: '0.82rem' }}
                            >
                                {savingParent ? 'Saving…' : 'Save Template'}
                            </button>
                        </div>
                    )}
                </div>

                <p style={s.templateDesc}>
                    Used when generating tests for a parent requirement that has child issues.
                    Lighter and focused on coverage across children rather than deep single-requirement rules.
                </p>

                {/* Variable chips */}
                <div style={s.varChips}>
                    {TEMPLATE_VARS.map(v => (
                        <code key={v} style={{
                            ...s.varChip,
                            ...(PARENT_REQUIRED_VARS.includes(v) && missingParentVars.includes(v)
                                ? { borderColor: 'rgba(239,68,68,0.5)', color: '#f87171', background: 'rgba(239,68,68,0.1)' }
                                : {}),
                        }}>{v}</code>
                    ))}
                </div>

                {/* Missing placeholder warning */}
                {missingParentVars.length > 0 && (
                    <div style={{
                        display: 'flex', alignItems: 'flex-start', gap: 10,
                        padding: '10px 14px',
                        borderRadius: 8,
                        background: 'rgba(239,68,68,0.08)',
                        border: '1px solid rgba(239,68,68,0.25)',
                        marginBottom: 8,
                    }}>
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#f87171" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ flexShrink: 0, marginTop: 1 }}>
                            <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
                            <line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/>
                        </svg>
                        <div>
                            <span style={{ fontSize: '0.82rem', color: '#f87171', fontWeight: 600 }}>
                                Missing required placeholders:{' '}
                            </span>
                            <span style={{ fontSize: '0.82rem', color: '#fca5a5', fontFamily: 'monospace' }}>
                                {missingParentVars.join(', ')}
                            </span>
                            <p style={{ margin: '3px 0 0', fontSize: '0.78rem', color: 'rgba(252,165,165,0.8)', lineHeight: 1.4 }}>
                                Child issue context will <strong>not</strong> be sent to the LLM without these placeholders.
                            </p>
                        </div>
                    </div>
                )}

                {/* Editor */}
                <div style={s.editorWrap}>
                    {!isAdmin && (
                        <div style={s.readOnlyBanner}>
                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/>
                            </svg>
                            View only — admin required to edit
                        </div>
                    )}
                    <textarea
                        className="modern-input"
                        style={{
                            ...s.editor,
                            opacity: isAdmin ? 1 : 0.7,
                        }}
                        value={parentContent}
                        onChange={e => setParentContent(e.target.value)}
                        disabled={!isAdmin}
                        placeholder="Loading parent template…"
                        spellCheck={false}
                    />
                    <div style={s.editorFooter}>
                        <span style={s.charCount}>{parentContent.length} chars</span>
                    </div>
                </div>
            </section>

            {/* ── Coverage Token Limits Section ── */}
            <section style={s.section}>
                <div style={s.sectionHead}>
                    <div style={s.sectionHeadLeft}>
                        <span style={s.sectionDot} />
                        <h4 style={s.sectionTitle}>Coverage Token Limits</h4>
                        {coverageModified && <span style={s.modifiedBadge}>Unsaved changes</span>}
                    </div>
                    {isAdmin && (
                        <button
                            className="primary-btn"
                            onClick={handleSaveCoverage}
                            disabled={savingCoverage || !coverageModified}
                            style={{ fontSize: '0.82rem' }}
                        >
                            {savingCoverage ? 'Saving…' : 'Save'}
                        </button>
                    )}
                </div>
                <p style={s.templateDesc}>
                    Maximum tokens the LLM can use per coverage level. Higher values allow more test cases but cost more.
                </p>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 12, marginTop: 8 }}>
                    {[
                        { key: 'essential_max_tokens', label: 'Essential', defaultVal: 4096 },
                        { key: 'thorough_max_tokens', label: 'Thorough', defaultVal: 8192 },
                        { key: 'comprehensive_max_tokens', label: 'Comprehensive', defaultVal: 16384 },
                    ].map(({ key, label, defaultVal }) => (
                        <div key={key} style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                            <label style={{ fontSize: '0.78rem', color: 'var(--text-secondary)', fontWeight: 500 }}>
                                {label}
                            </label>
                            <input
                                className="modern-input"
                                type="number"
                                min={1024}
                                step={1024}
                                value={coverageForm[key]}
                                onChange={e => setCoverageForm(prev => ({ ...prev, [key]: parseInt(e.target.value, 10) || defaultVal }))}
                                disabled={!isAdmin}
                                style={{ padding: '8px 10px', fontSize: '0.85rem' }}
                            />
                        </div>
                    ))}
                </div>
            </section>

            {/* ── Modals ── */}
            {modal && (
                <ProviderModal
                    provider={modal.type === 'edit' ? modal.provider : null}
                    onClose={() => setModal(null)}
                    onSaved={() => { setModal(null); loadProviders(); }}
                />
            )}
            {confirmDelete && (
                <DeleteModal
                    provider={confirmDelete}
                    onCancel={() => setConfirmDelete(null)}
                    onConfirm={handleDeleteExecute}
                />
            )}

            <style>{`
                @keyframes spin { to { transform: rotate(360deg); } }
                @keyframes aigenSettingsFadeIn {
                    from { opacity: 0; transform: translateY(8px); }
                    to   { opacity: 1; transform: translateY(0); }
                }
                .aigen-icon-btn:hover:not(:disabled) {
                    background: rgba(255,255,255,0.08) !important;
                    color: var(--text-primary) !important;
                }
                .aigen-icon-btn-danger:hover:not(:disabled) {
                    background: rgba(239,68,68,0.1) !important;
                    color: #f87171 !important;
                    border-color: rgba(239,68,68,0.25) !important;
                }
                .aigen-provider-card:hover {
                    border-color: rgba(99,102,241,0.3) !important;
                    background: rgba(99,102,241,0.03) !important;
                }
                .aigen-type-card:hover {
                    border-color: var(--accent-indigo) !important;
                    opacity: 0.9;
                }
            `}</style>
        </div>
    );
}

/* ── Page Styles ─────────────────────────────────── */
const s = {
    page: {
        display: 'flex',
        flexDirection: 'column',
        gap: 32,
        animation: 'aigenSettingsFadeIn 0.2s ease both',
    },
    pageHeader: {
        display: 'flex',
        alignItems: 'flex-start',
        gap: 14,
    },
    pageHeaderIcon: {
        width: 42, height: 42,
        borderRadius: 10,
        background: 'linear-gradient(135deg, rgba(99,102,241,0.2), rgba(20,184,166,0.15))',
        border: '1px solid rgba(99,102,241,0.2)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        color: '#818cf8',
        flexShrink: 0,
        marginTop: 2,
    },
    pageTitle: {
        margin: '0 0 4px',
        fontSize: '1.05rem',
        fontWeight: 700,
        color: 'var(--text-primary)',
    },
    pageDesc: {
        margin: 0,
        fontSize: '0.845rem',
        color: 'var(--text-secondary)',
        lineHeight: 1.6,
        maxWidth: 580,
    },
    section: {
        display: 'flex',
        flexDirection: 'column',
        gap: 14,
    },
    sectionHead: {
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        gap: 12,
    },
    sectionHeadLeft: {
        display: 'flex',
        alignItems: 'center',
        gap: 8,
    },
    sectionDot: {
        width: 6, height: 6,
        borderRadius: '50%',
        background: 'linear-gradient(135deg, #6366f1, #14b8a6)',
        flexShrink: 0,
    },
    sectionTitle: {
        margin: 0,
        fontSize: '0.9rem',
        fontWeight: 700,
        color: 'var(--text-primary)',
    },
    sectionCount: {
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        minWidth: 20, height: 20,
        padding: '0 6px',
        borderRadius: 10,
        background: 'rgba(99,102,241,0.15)',
        border: '1px solid rgba(99,102,241,0.2)',
        fontSize: '0.7rem',
        fontWeight: 700,
        color: '#818cf8',
    },
    addBtn: {
        display: 'flex',
        alignItems: 'center',
        gap: 6,
        fontSize: '0.82rem',
        padding: '5px 12px',
    },
    loadingState: {
        display: 'flex',
        alignItems: 'center',
        gap: 8,
        padding: '20px 0',
        color: 'var(--text-secondary)',
        fontSize: '0.875rem',
    },
    loadingSpinner: {
        display: 'inline-block',
        width: 14, height: 14,
        border: '2px solid var(--border-color)',
        borderTopColor: 'var(--accent-indigo)',
        borderRadius: '50%',
        animation: 'spin 0.7s linear infinite',
    },
    emptyState: {
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        textAlign: 'center',
        padding: '36px 20px',
        borderRadius: 12,
        border: '1px dashed var(--border-color)',
        background: 'rgba(255,255,255,0.01)',
        gap: 8,
    },
    emptyIcon: {
        width: 48, height: 48,
        borderRadius: 12,
        background: 'rgba(99,102,241,0.08)',
        border: '1px solid rgba(99,102,241,0.15)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        color: '#818cf8',
        marginBottom: 4,
    },
    emptyTitle: {
        margin: 0,
        fontSize: '0.925rem',
        fontWeight: 600,
        color: 'var(--text-primary)',
    },
    emptyDesc: {
        margin: 0,
        fontSize: '0.845rem',
        color: 'var(--text-secondary)',
        maxWidth: 340,
        lineHeight: 1.6,
    },
    providerList: {
        display: 'flex',
        flexDirection: 'column',
        gap: 8,
    },
    providerCard: {
        borderRadius: 10,
        border: '1px solid var(--border-color)',
        borderLeft: '3px solid',
        background: 'var(--bg-tertiary)',
        padding: '14px 16px',
        transition: 'all 0.15s',
        className: 'aigen-provider-card',
    },
    cardMain: {
        display: 'flex',
        alignItems: 'flex-start',
        gap: 12,
    },
    providerAvatar: {
        width: 34, height: 34,
        borderRadius: 8,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        fontSize: '0.95rem',
        fontWeight: 800,
        flexShrink: 0,
    },
    providerNameRow: {
        display: 'flex',
        alignItems: 'center',
        gap: 7,
        flexWrap: 'wrap',
        marginBottom: 5,
    },
    providerName: {
        fontWeight: 600,
        fontSize: '0.925rem',
        color: 'var(--text-primary)',
    },
    defaultBadge: {
        fontSize: '0.68rem',
        fontWeight: 700,
        color: '#818cf8',
        background: 'rgba(99,102,241,0.15)',
        border: '1px solid rgba(99,102,241,0.25)',
        padding: '1px 7px',
        borderRadius: 20,
        letterSpacing: '0.02em',
    },
    disabledBadge: {
        fontSize: '0.68rem',
        fontWeight: 700,
        color: '#f87171',
        background: 'rgba(239,68,68,0.1)',
        border: '1px solid rgba(239,68,68,0.2)',
        padding: '1px 7px',
        borderRadius: 20,
    },
    providerMeta: {
        display: 'flex',
        alignItems: 'center',
        gap: 5,
        flexWrap: 'wrap',
    },
    metaChip: {
        fontSize: '0.75rem',
        color: 'var(--text-secondary)',
        background: 'rgba(255,255,255,0.04)',
        border: '1px solid var(--border-color)',
        padding: '1px 7px',
        borderRadius: 4,
    },
    metaDot: {
        color: 'var(--border-color)',
        fontSize: '0.75rem',
        userSelect: 'none',
    },
    testResult: {
        marginTop: 8,
        padding: '5px 10px',
        borderRadius: 6,
        fontSize: '0.78rem',
        border: '1px solid',
        fontWeight: 500,
    },
    cardActions: {
        display: 'flex',
        gap: 4,
        flexShrink: 0,
        alignItems: 'center',
    },
    iconBtn: {
        display: 'flex',
        alignItems: 'center',
        gap: 5,
        padding: '5px 9px',
        background: 'rgba(255,255,255,0.04)',
        border: '1px solid var(--border-color)',
        borderRadius: 6,
        color: 'var(--text-secondary)',
        fontSize: '0.75rem',
        cursor: 'pointer',
        transition: 'all 0.15s',
        fontFamily: 'inherit',
        fontWeight: 500,
        className: 'aigen-icon-btn',
    },
    iconBtnActive: {
        borderColor: 'rgba(99,102,241,0.3)',
        color: '#818cf8',
    },
    iconBtnDanger: {
        className: 'aigen-icon-btn aigen-icon-btn-danger',
    },
    iconBtnLabel: {
        fontSize: '0.75rem',
    },
    miniSpinner: {
        display: 'inline-block',
        width: 11, height: 11,
        border: '1.5px solid var(--border-color)',
        borderTopColor: 'var(--accent-indigo)',
        borderRadius: '50%',
        animation: 'spin 0.7s linear infinite',
        flexShrink: 0,
    },
    modifiedBadge: {
        fontSize: '0.7rem',
        fontWeight: 600,
        color: '#fbbf24',
        background: 'rgba(234,179,8,0.1)',
        border: '1px solid rgba(234,179,8,0.2)',
        padding: '1px 8px',
        borderRadius: 20,
    },
    templateTypeBadge: {
        fontSize: '0.68rem',
        fontWeight: 700,
        color: '#14b8a6',
        background: 'rgba(20,184,166,0.1)',
        border: '1px solid rgba(20,184,166,0.2)',
        padding: '1px 8px',
        borderRadius: 20,
        letterSpacing: '0.02em',
    },
    templateDesc: {
        margin: 0,
        fontSize: '0.845rem',
        color: 'var(--text-secondary)',
        lineHeight: 1.6,
    },
    varChips: {
        display: 'flex',
        flexWrap: 'wrap',
        gap: 6,
    },
    varChip: {
        fontSize: '0.75rem',
        fontFamily: 'monospace',
        color: '#818cf8',
        background: 'rgba(99,102,241,0.1)',
        border: '1px solid rgba(99,102,241,0.2)',
        padding: '2px 8px',
        borderRadius: 5,
        userSelect: 'all',
    },
    editorWrap: {
        borderRadius: 10,
        border: '1px solid var(--border-color)',
        overflow: 'hidden',
        background: 'var(--bg-primary)',
    },
    readOnlyBanner: {
        display: 'flex',
        alignItems: 'center',
        gap: 6,
        padding: '7px 14px',
        background: 'rgba(255,255,255,0.03)',
        borderBottom: '1px solid var(--border-color)',
        fontSize: '0.78rem',
        color: 'var(--text-secondary)',
    },
    editor: {
        width: '100%',
        minHeight: 260,
        fontFamily: '"SF Mono", "Fira Code", "Cascadia Code", monospace',
        fontSize: '0.8rem',
        resize: 'vertical',
        lineHeight: 1.65,
        border: 'none',
        borderRadius: 0,
        background: 'transparent',
        padding: '14px',
        boxSizing: 'border-box',
    },
    editorFooter: {
        display: 'flex',
        justifyContent: 'flex-end',
        padding: '6px 12px',
        borderTop: '1px solid var(--border-color)',
        background: 'rgba(255,255,255,0.02)',
    },
    charCount: {
        fontSize: '0.72rem',
        color: 'var(--text-secondary)',
        fontFamily: 'monospace',
        opacity: 0.6,
    },
};

/* ── Modal Styles ─────────────────────────────────── */
const m = {
    backdrop: {
        position: 'fixed', inset: 0,
        background: 'rgba(0,0,0,0.65)',
        backdropFilter: 'blur(6px)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        zIndex: 1000, padding: 16,
    },
    modal: {
        width: '100%',
        maxWidth: 520,
        maxHeight: '92vh',
        display: 'flex',
        flexDirection: 'column',
        background: 'var(--bg-secondary)',
        border: '1px solid var(--border-color)',
        borderRadius: 14,
        boxShadow: '0 32px 80px rgba(0,0,0,0.6)',
        overflow: 'hidden',
        animation: 'aigenSettingsFadeIn 0.2s ease both',
    },
    accentBar: {
        height: 3,
        flexShrink: 0,
    },
    header: {
        display: 'flex',
        alignItems: 'flex-start',
        justifyContent: 'space-between',
        gap: 12,
        padding: '16px 20px 14px',
        borderBottom: '1px solid var(--border-color)',
        flexShrink: 0,
    },
    providerInitial: {
        width: 36, height: 36,
        borderRadius: 8,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        fontSize: '1rem',
        fontWeight: 800,
        flexShrink: 0,
    },
    modalTitle: {
        margin: '0 0 2px',
        fontSize: '0.95rem',
        fontWeight: 700,
    },
    modalSub: {
        margin: 0,
        fontSize: '0.78rem',
        color: 'var(--text-secondary)',
    },
    closeBtn: {
        width: 28, height: 28,
        borderRadius: 6,
        border: 'none',
        background: 'none',
        color: 'var(--text-secondary)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        cursor: 'pointer',
        flexShrink: 0,
        transition: 'all 0.15s',
    },
    formBody: {
        flex: 1,
        overflowY: 'auto',
        padding: '18px 20px',
        display: 'flex',
        flexDirection: 'column',
        gap: 16,
    },
    typeGrid: {
        display: 'grid',
        gridTemplateColumns: 'repeat(4, 1fr)',
        gap: 8,
    },
    typeCard: {
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: 5,
        padding: '10px 6px',
        borderRadius: 8,
        border: '1px solid',
        cursor: 'pointer',
        transition: 'all 0.15s',
        fontFamily: 'inherit',
        className: 'aigen-type-card',
    },
    field: {
        display: 'flex',
        flexDirection: 'column',
        gap: 6,
    },
    fieldLabel: {
        fontSize: '0.78rem',
        fontWeight: 600,
        color: 'var(--text-secondary)',
        textTransform: 'uppercase',
        letterSpacing: '0.05em',
    },
    required: {
        color: 'var(--accent-red)',
        marginLeft: 2,
    },
    chip: {
        fontSize: '0.68rem',
        fontWeight: 600,
        color: 'var(--text-secondary)',
        background: 'rgba(255,255,255,0.05)',
        border: '1px solid var(--border-color)',
        padding: '1px 6px',
        borderRadius: 4,
    },
    currentKeyNote: {
        fontSize: '0.76rem',
        color: 'var(--text-secondary)',
        padding: '5px 10px',
        background: 'rgba(255,255,255,0.03)',
        border: '1px solid var(--border-color)',
        borderRadius: 6,
    },
    toggle: {
        display: 'flex',
        alignItems: 'center',
        gap: 8,
        cursor: 'pointer',
        userSelect: 'none',
    },
    checkbox: {
        width: 16, height: 16,
        cursor: 'pointer',
    },
    toggleLabel: {
        fontSize: '0.875rem',
        fontWeight: 500,
        color: 'var(--text-primary)',
    },
    footer: {
        display: 'flex',
        gap: 8,
        justifyContent: 'flex-end',
        paddingTop: 4,
        borderTop: '1px solid var(--border-color)',
        marginTop: 4,
    },
    deleteIcon: {
        width: 44, height: 44,
        borderRadius: 10,
        background: 'rgba(239,68,68,0.1)',
        border: '1px solid rgba(239,68,68,0.2)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        color: '#f87171',
        marginBottom: 14,
    },
};
