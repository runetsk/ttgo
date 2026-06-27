import React, { useEffect, useMemo, useState } from 'react';
import {
    getFailureAnalysisSettings,
    updateFailureAnalysisSettings,
    resetFailureAnalysisPrompt,
} from '../api';
import { toast } from '../toast';

export default function AIFailureAnalysisSettings({ isAdmin }) {
    const [settings, setSettings] = useState(null);
    const [original, setOriginal] = useState(null);
    const [saving, setSaving] = useState(false);
    const [resetting, setResetting] = useState(false);

    useEffect(() => {
        getFailureAnalysisSettings().then((s) => {
            setSettings(s);
            setOriginal(s);
        }).catch((e) => {
            console.error('Load settings failed', e);
            toast.error('Failed to load AI failure analysis settings');
        });
    }, []);

    const modified = useMemo(() => {
        if (!settings || !original) return false;
        return (
            settings.enabled_on_completion !== original.enabled_on_completion ||
            settings.max_analyses_per_run !== original.max_analyses_per_run ||
            settings.dedup_enabled !== original.dedup_enabled ||
            settings.redaction_enabled !== original.redaction_enabled ||
            settings.prompt_template !== original.prompt_template
        );
    }, [settings, original]);

    if (!settings) {
        return (
            <section style={s.section}>
                <div style={s.loadingState}>
                    <span style={s.loadingSpinner} />
                    Loading AI failure analysis settings…
                </div>
            </section>
        );
    }

    const update = (patch) => setSettings((prev) => ({ ...prev, ...patch }));

    const save = async () => {
        setSaving(true);
        try {
            const next = await updateFailureAnalysisSettings({
                enabled_on_completion: settings.enabled_on_completion,
                max_analyses_per_run:  settings.max_analyses_per_run,
                dedup_enabled:         settings.dedup_enabled,
                redaction_enabled:     settings.redaction_enabled,
                prompt_template:       settings.prompt_template,
            });
            setSettings(next);
            setOriginal(next);
            toast.success('Saved');
        } catch (e) {
            toast.error('Save failed: ' + (e.response?.data?.error || e.message));
        } finally {
            setSaving(false);
        }
    };

    const reset = async () => {
        setResetting(true);
        try {
            const next = await resetFailureAnalysisPrompt();
            setSettings(next);
            setOriginal(next);
            toast.success('Prompt template reset to default');
        } catch (e) {
            toast.error('Reset failed: ' + e.message);
        } finally {
            setResetting(false);
        }
    };

    return (
        <section style={s.section}>
            <div style={s.sectionHead}>
                <div style={s.sectionHeadLeft}>
                    <span style={s.sectionDot} />
                    <h4 style={s.sectionTitle}>AI Failure Analysis</h4>
                    {modified && <span style={s.modifiedBadge}>Unsaved changes</span>}
                </div>
                {isAdmin && (
                    <div style={{ display: 'flex', gap: 8 }}>
                        <button
                            className="primary-btn"
                            onClick={save}
                            disabled={saving || !modified}
                            style={{ fontSize: '0.82rem' }}
                        >
                            {saving ? 'Saving…' : 'Save'}
                        </button>
                    </div>
                )}
            </div>

            <p style={s.desc}>
                Automatically classify failing test results to help triage. All controls admin-only.
            </p>

            {/* Toggles */}
            <div style={s.togglesGrid}>
                <Toggle
                    icon={
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/>
                        </svg>
                    }
                    iconColor="#14b8a6"
                    label="Auto-analyze on run completion"
                    desc="Automatically classify failures whenever a run finishes."
                    checked={settings.enabled_on_completion}
                    disabled={!isAdmin}
                    onChange={(v) => update({ enabled_on_completion: v })}
                />
                <Toggle
                    icon={
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
                            <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
                        </svg>
                    }
                    iconColor="#818cf8"
                    label="Deduplicate similar failures"
                    desc="Group near-identical failures and analyze one representative."
                    checked={settings.dedup_enabled}
                    disabled={!isAdmin}
                    onChange={(v) => update({ dedup_enabled: v })}
                />
                <Toggle
                    icon={
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
                        </svg>
                    }
                    iconColor="#fbbf24"
                    label="Redact secrets"
                    desc="Strip tokens, keys, and credentials before sending to the LLM. Recommended."
                    checked={settings.redaction_enabled}
                    disabled={!isAdmin}
                    onChange={(v) => update({ redaction_enabled: v })}
                />
            </div>

            {/* Max analyses per run */}
            <div style={s.fieldRow}>
                <div style={s.fieldLabelCol}>
                    <label style={s.fieldLabel}>Max analyses per run</label>
                    <p style={s.fieldHint}>
                        Cap on unique failure groups analyzed per run. Keeps cost bounded.
                    </p>
                </div>
                <input
                    className="modern-input"
                    type="number"
                    min={1}
                    max={500}
                    disabled={!isAdmin}
                    value={settings.max_analyses_per_run}
                    onChange={(e) => update({ max_analyses_per_run: parseInt(e.target.value, 10) || 0 })}
                    style={{ width: 110, padding: '8px 10px', fontSize: '0.85rem' }}
                />
            </div>

            {/* Prompt template */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12 }}>
                    <div>
                        <div style={s.subTitle}>Prompt template</div>
                        <p style={s.fieldHint}>
                            Sent to the LLM for each failure. Must return JSON with verdict, confidence, summary, next_action, rationale.
                        </p>
                    </div>
                    {isAdmin && (
                        <button
                            onClick={reset}
                            disabled={resetting}
                            style={s.resetBtn}
                            title="Reset to default prompt"
                        >
                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
                                <polyline points="1 4 1 10 7 10"/>
                                <path d="M3.51 15a9 9 0 1 0 2.13-9.36L1 10"/>
                            </svg>
                            {resetting ? 'Resetting…' : 'Reset to default'}
                        </button>
                    )}
                </div>

                <div style={s.editorWrap}>
                    {!isAdmin && (
                        <div style={s.readOnlyBanner}>
                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
                                <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
                            </svg>
                            View only — admin required to edit
                        </div>
                    )}
                    <textarea
                        className="modern-input"
                        style={{ ...s.editor, opacity: isAdmin ? 1 : 0.7 }}
                        value={settings.prompt_template}
                        onChange={(e) => update({ prompt_template: e.target.value })}
                        disabled={!isAdmin}
                        spellCheck={false}
                        placeholder="Loading template…"
                    />
                    <div style={s.editorFooter}>
                        <span style={s.charCount}>{(settings.prompt_template || '').length} chars</span>
                    </div>
                </div>
            </div>
        </section>
    );
}

function Toggle({ icon, iconColor, label, desc, checked, disabled, onChange }) {
    return (
        <label
            style={{
                ...s.toggleCard,
                borderColor: checked ? 'rgba(99,102,241,0.35)' : 'var(--border-color)',
                background: checked ? 'rgba(99,102,241,0.04)' : 'var(--bg-tertiary)',
                cursor: disabled ? 'not-allowed' : 'pointer',
                opacity: disabled ? 0.7 : 1,
            }}
        >
            <div style={{ ...s.toggleIcon, color: iconColor, borderColor: `${iconColor}33`, background: `${iconColor}14` }}>
                {icon}
            </div>
            <div style={{ flex: 1, minWidth: 0 }}>
                <div style={s.toggleLabel}>{label}</div>
                <div style={s.toggleDesc}>{desc}</div>
            </div>
            <span
                style={{
                    ...s.switch,
                    background: checked ? 'var(--accent-indigo)' : 'rgba(148,163,184,0.35)',
                }}
            >
                <span
                    style={{
                        ...s.switchKnob,
                        transform: checked ? 'translateX(16px)' : 'translateX(0)',
                    }}
                />
            </span>
            <input
                type="checkbox"
                checked={checked}
                disabled={disabled}
                onChange={(e) => onChange(e.target.checked)}
                style={{ position: 'absolute', opacity: 0, pointerEvents: 'none' }}
            />
        </label>
    );
}

const s = {
    section: {
        display: 'flex',
        flexDirection: 'column',
        gap: 14,
        marginTop: 32,
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
    modifiedBadge: {
        fontSize: '0.7rem',
        fontWeight: 600,
        color: '#fbbf24',
        background: 'rgba(234,179,8,0.1)',
        border: '1px solid rgba(234,179,8,0.2)',
        padding: '1px 8px',
        borderRadius: 20,
    },
    desc: {
        margin: 0,
        fontSize: '0.845rem',
        color: 'var(--text-secondary)',
        lineHeight: 1.6,
    },
    togglesGrid: {
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))',
        gap: 10,
    },
    toggleCard: {
        position: 'relative',
        display: 'flex',
        alignItems: 'flex-start',
        gap: 10,
        padding: '12px 14px',
        borderRadius: 10,
        borderWidth: 1,
        borderStyle: 'solid',
        borderColor: 'var(--border-color)',
        background: 'var(--bg-tertiary)',
        transition: 'background 0.15s, border-color 0.15s',
    },
    toggleIcon: {
        width: 30, height: 30,
        borderRadius: 8,
        borderWidth: 1,
        borderStyle: 'solid',
        borderColor: 'transparent',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        flexShrink: 0,
    },
    toggleLabel: {
        fontSize: '0.86rem',
        fontWeight: 600,
        color: 'var(--text-primary)',
        marginBottom: 2,
    },
    toggleDesc: {
        fontSize: '0.76rem',
        color: 'var(--text-secondary)',
        lineHeight: 1.5,
    },
    switch: {
        position: 'relative',
        width: 34, height: 18,
        borderRadius: 10,
        transition: 'background 0.15s',
        flexShrink: 0,
        marginTop: 4,
    },
    switchKnob: {
        position: 'absolute',
        top: 2, left: 2,
        width: 14, height: 14,
        borderRadius: '50%',
        background: '#fff',
        transition: 'transform 0.15s',
        boxShadow: '0 1px 2px rgba(0,0,0,0.25)',
    },
    fieldRow: {
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        gap: 16,
        padding: '12px 14px',
        borderRadius: 10,
        border: '1px solid var(--border-color)',
        background: 'var(--bg-tertiary)',
    },
    fieldLabelCol: {
        display: 'flex',
        flexDirection: 'column',
        gap: 2,
        minWidth: 0,
    },
    fieldLabel: {
        fontSize: '0.86rem',
        fontWeight: 600,
        color: 'var(--text-primary)',
    },
    fieldHint: {
        margin: 0,
        fontSize: '0.76rem',
        color: 'var(--text-secondary)',
        lineHeight: 1.5,
    },
    subTitle: {
        fontSize: '0.86rem',
        fontWeight: 700,
        color: 'var(--text-primary)',
        marginBottom: 2,
    },
    resetBtn: {
        display: 'inline-flex',
        alignItems: 'center',
        gap: 6,
        padding: '5px 10px',
        fontSize: '0.78rem',
        borderRadius: 8,
        border: '1px solid var(--border-color)',
        background: 'transparent',
        color: 'var(--text-secondary)',
        cursor: 'pointer',
        transition: 'all 0.15s',
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
};
