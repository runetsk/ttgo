import React, { useState, useEffect, useCallback } from 'react';
import { aiGeneration } from '../api';
import { toast } from '../toast';
import { useAuth } from '../contexts/AuthContext';
import { TEMPLATE_VARS, REQUIRED_TEMPLATE_VARS, PARENT_REQUIRED_VARS } from './aiSettings/constants';
import { s } from './aiSettings/styles';
import ProviderModal from './aiSettings/ProviderModal';
import DeleteModal from './aiSettings/DeleteModal';
import ProviderCard from './aiSettings/ProviderCard';

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

