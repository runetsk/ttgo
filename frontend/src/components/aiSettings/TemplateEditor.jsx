import React, { useState, useEffect, useCallback } from 'react';
import { aiGeneration } from '../../api';
import { toast } from '../../toast';
import { TEMPLATE_VARS, REQUIRED_TEMPLATE_VARS, PARENT_REQUIRED_VARS } from './constants';
import { s } from './styles';

/* ── Standard + Parent Prompt Template Sections ── */
export default function TemplateEditor({ isAdmin }) {
    const [template, setTemplate]           = useState(null);
    const [templateContent, setTemplateContent] = useState('');
    const [savingTemplate, setSavingTemplate]   = useState(false);
    const [resettingTemplate, setResettingTemplate] = useState(false);

    const [parentContent, setParentContent]         = useState('');
    const [savingParent, setSavingParent]           = useState(false);
    const [resettingParent, setResettingParent]     = useState(false);

    const loadTemplate = useCallback(() => {
        aiGeneration.getTemplate()
            .then(t => { setTemplate(t); setTemplateContent(t.content || ''); setParentContent(t.parent_content || ''); })
            .catch(() => {});
    }, []);

    useEffect(() => {
        loadTemplate();
    }, [loadTemplate]);

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

    return (
        <>
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
        </>
    );
}
