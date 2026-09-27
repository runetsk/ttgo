import React, { useEffect, useMemo, useState } from 'react';
import {
    getFailureAnalysisSettings,
    updateFailureAnalysisSettings,
    resetFailureAnalysisPrompt,
    getFailureAnalysisAccuracy,
} from '../api';
import {
    summarizeAccuracy, confidenceRows, verdictRows, engineRows,
    AGREEMENT_LABEL, AGREEMENT_TOOLTIP, ALL_VERSIONS, splitLabel, unknownNote, coverageRows, policyOptions,
} from './aiSettings/accuracyFormat';
import { ToggleCard, FieldRow, HelpToggle } from './aiSettings/SettingsControls';
import { cs } from './aiSettings/settingsControlStyles';
import { SETTING_HELP } from '../utils/analysisSettingsHelp';
import {
    validateFailureAnalysisDraft, isFailureAnalysisDirty, parseWholeNumber,
    MAX_ANALYSES_MIN, MAX_ANALYSES_MAX, PARALLEL_MIN, PARALLEL_MAX,
} from '../utils/failureAnalysisSettings';
import { toast } from '../toast';
import { errorDescriptors, saveError } from '../utils/saveBar';
import { useSaveSection } from './aiSettings/saveBarContext';

// Rolling window for the accuracy panel. Matches the backend default so the
// panel and the endpoint never describe different periods.
const ACCURACY_WINDOW_DAYS = 30;

const numberValue = (n) => (Number.isFinite(n) ? n : '');
const svgIcon = (paths) => (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">{paths}</svg>
);

export default function AIFailureAnalysisSettings({ isAdmin, onStateChange }) {
    const [settings, setSettings] = useState(null);
    const [original, setOriginal] = useState(null);
    const [loadError, setLoadError] = useState(false);
    const [resetting, setResetting] = useState(false);

    useEffect(() => {
        getFailureAnalysisSettings().then((s) => {
            setSettings(s);
            setOriginal(s);
        }).catch((e) => {
            console.error('Load settings failed', e);
            setLoadError(true);
            toast.error('Failed to load AI failure analysis settings');
        });
    }, []);

    const modified = useMemo(() => !!settings && !!original && isFailureAnalysisDirty(settings, original), [settings, original]);
    const errors = useMemo(() => (settings ? validateFailureAnalysisDraft(settings) : {}), [settings]);

    // Report to the section that draws the process diagram (same contract as TypeSafeSettingsCard).
    useEffect(() => {
        if (!onStateChange) return;
        if (loadError) onStateChange({ status: 'error' });
        else if (!settings || !original) onStateChange({ status: 'loading' });
        else onStateChange({ status: 'ready', saved: original, draft: settings, dirty: modified, errors });
    }, [onStateChange, loadError, settings, original, modified, errors]);

    // Registered with the page save bar before the early returns below (hook order).
    const errorList = useMemo(() => errorDescriptors(errors, 'fa'), [errors]);
    const saveSettings = async () => {
        if (!settings) return;
        let next;
        try {
            next = await updateFailureAnalysisSettings({
                enabled_on_completion: settings.enabled_on_completion,
                max_analyses_per_run:  settings.max_analyses_per_run,
                parallel_groups:       settings.parallel_groups,
                dedup_enabled:         settings.dedup_enabled,
                redaction_enabled:     settings.redaction_enabled,
                prompt_template:       settings.prompt_template,
            });
        } catch (err) {
            throw saveError(err, 'Failed to save the failure-analysis settings');
        }
        setSettings(next);
        setOriginal(next);
    };
    const discardDraft = () => { if (original) setSettings(original); };
    const saving = useSaveSection('failureAnalysis', {
        label: 'AI Failure Analysis', tab: 'analysis', dirty: modified, errors: errorList, save: saveSettings, discard: discardDraft,
    });
    const locked = !isAdmin || saving;

    if (loadError) {
        return (
            <section style={s.section} data-testid="fa-settings">
                <div style={s.loadingState} data-testid="fa-load-error">Couldn&apos;t load the AI failure analysis settings. Reload the page to try again.</div>
            </section>
        );
    }
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
        <section style={s.section} data-testid="fa-settings">
            <div style={s.sectionHead}>
                <div style={s.sectionHeadLeft}>
                    <span style={s.sectionDot} />
                    <h4 style={s.sectionTitle}>AI Failure Analysis</h4>
                    {modified && <span style={s.modifiedBadge}>Unsaved changes</span>}
                </div>
            </div>

            <p style={s.desc}>
                Automatically classify failing test results to help triage. All controls admin-only.
            </p>

            <AccuracyPanel />

            <div style={s.togglesGrid}>
                <ToggleCard
                    icon={svgIcon(<polyline points="22 12 18 12 15 21 9 3 6 12 2 12" />)}
                    iconColor="#14b8a6"
                    label="Auto-analyze on run completion"
                    desc="Queue a run's failures for analysis when the run is completed."
                    checked={settings.enabled_on_completion}
                    disabled={locked}
                    onChange={(v) => update({ enabled_on_completion: v })}
                    testId="fa-enabled_on_completion"
                    setting="fa.enabled_on_completion" help={SETTING_HELP['fa.enabled_on_completion']}
                />
                <ToggleCard
                    icon={svgIcon(<><rect x="9" y="9" width="13" height="13" rx="2" ry="2" /><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" /></>)}
                    iconColor="#818cf8"
                    label="Deduplicate similar failures"
                    desc="Group failures with the same type and error text and analyze one representative."
                    checked={settings.dedup_enabled}
                    disabled={locked}
                    onChange={(v) => update({ dedup_enabled: v })}
                    testId="fa-dedup_enabled"
                    setting="fa.dedup_enabled" help={SETTING_HELP['fa.dedup_enabled']}
                />
                <ToggleCard
                    icon={svgIcon(<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />)}
                    iconColor="#fbbf24"
                    label="Redact secrets"
                    desc="Replace recognized secrets before failure text is sent. Recommended."
                    checked={settings.redaction_enabled}
                    disabled={locked}
                    onChange={(v) => update({ redaction_enabled: v })}
                    testId="fa-redaction_enabled"
                    setting="fa.redaction_enabled" help={SETTING_HELP['fa.redaction_enabled']}
                />
            </div>

            <FieldRow label="Max analyses per run" htmlFor="fa-max-analyses"
                setting="fa.max_analyses_per_run" help={SETTING_HELP['fa.max_analyses_per_run']}
                hint={errors.max_analyses_per_run
                    ? <span style={cs.fieldError}>{errors.max_analyses_per_run}</span>
                    : 'Cap on failure groups analyzed per run, largest first. Keeps cost bounded.'}>
                <input id="fa-max-analyses" data-testid="fa-max-analyses" className="modern-input" type="number"
                    min={MAX_ANALYSES_MIN} max={MAX_ANALYSES_MAX} disabled={locked}
                    value={numberValue(settings.max_analyses_per_run)}
                    onChange={(e) => update({ max_analyses_per_run: parseWholeNumber(e.target.value) })}
                    style={{ width: 110, padding: '8px 10px', fontSize: '0.85rem' }} />
            </FieldRow>

            <FieldRow label="Groups analyzed at once" htmlFor="fa-parallel-groups"
                setting="fa.parallel_groups" help={SETTING_HELP['fa.parallel_groups']}
                hint={errors.parallel_groups
                    ? <span style={cs.fieldError}>{errors.parallel_groups}</span>
                    : 'How many failure groups are analyzed side by side. Lower it if your LLM provider answers with rate-limit errors.'}>
                <input id="fa-parallel-groups" data-testid="fa-parallel-groups" className="modern-input" type="number"
                    min={PARALLEL_MIN} max={PARALLEL_MAX} disabled={locked}
                    value={numberValue(settings.parallel_groups)}
                    onChange={(e) => update({ parallel_groups: parseWholeNumber(e.target.value) })}
                    style={{ width: 110, padding: '8px 10px', fontSize: '0.85rem' }} />
            </FieldRow>

            {/* Prompt template */}
            <div data-setting="fa.prompt_template" tabIndex={-1} style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12 }}>
                    <div>
                        <div style={s.subTitle}>Prompt template</div>
                        <p style={s.fieldHint}>
                            Sent to the LLM for each failure. Must return JSON with verdict, confidence, summary, next_action, rationale.
                        </p>
                        <HelpToggle help={SETTING_HELP['fa.prompt_template']} setting="fa.prompt_template" />
                    </div>
                    {isAdmin && (
                        <button onClick={reset} disabled={resetting || saving} style={s.resetBtn} title="Reset to default prompt">
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
                        disabled={locked}
                        spellCheck={false}
                        placeholder="Loading template…"
                    />
                    <div style={s.editorFooter}>
                        <span style={s.charCount}>{(settings.prompt_template || '').length} chars</span>
                    </div>
                </div>
                {errors.prompt_template && <div style={cs.fieldError} data-testid="fa-prompt-error">{errors.prompt_template}</div>}
            </div>
        </section>
    );
}

// AccuracyPanel reports how often the AI's suggested defect type matched the human's
// triage decision. The by-confidence ladder is the point: a clean descent from high to
// low means confidence is trustworthy, a flat one means it is noise. Self-contained —
// it owns its fetch so a failure here never blocks the settings form.
function AccuracyPanel() {
    const [report, setReport] = useState(null);
    const [status, setStatus] = useState('loading');
    const [policy, setPolicy] = useState(ALL_VERSIONS);

    useEffect(() => {
        let alive = true;
        getFailureAnalysisAccuracy(ACCURACY_WINDOW_DAYS, policy)
            .then((r) => {
                if (!alive) return;
                setReport(r);
                setStatus('ready');
            })
            .catch((e) => {
                console.error('Load AI failure analysis accuracy failed', e);
                if (alive) setStatus('failed');
            });
        return () => { alive = false; };
    }, [policy]);

    // Picking a version refetches. The loading state is set here, in the event, not in the effect.
    const choosePolicy = (value) => {
        setStatus('loading');
        setPolicy(value);
    };

    // All derivations tolerate a null report, so they are safe before the fetch lands.
    const summary = summarizeAccuracy(report);
    const rows = confidenceRows(report);
    const byVerdict = verdictRows(report);
    const byEngine = engineRows(report);
    const options = policyOptions(report, policy);
    const split = splitLabel(report);
    const unknown = policy ? '' : unknownNote(report);
    const coverageOnly = coverageRows(report).filter((c) => !byEngine.some((e) => e.key === c.key));

    return (
        <div style={s.accuracyPanel} data-testid="accuracy-panel">
            <div style={s.accuracyHead}>
                <div style={{ minWidth: 0 }}>
                    <div style={s.subTitle}>Suggestion accuracy</div>
                    <p style={s.fieldHint}>
                        How often people kept the suggested defect type when they triaged, last {ACCURACY_WINDOW_DAYS} days, per engine.
                    </p>
                </div>
                {status === 'ready' && summary.hasData && (
                    <div style={s.accuracyHeadline}>
                        <span style={s.accuracyHeadlineLabel} title={AGREEMENT_TOOLTIP} data-testid="accuracy-headline-label">{AGREEMENT_LABEL}</span>
                        <span style={s.accuracyRate} data-testid="accuracy-headline-rate">{summary.rateLabel}</span>
                        <span style={s.accuracySamples}>{summary.samples}</span>
                    </div>
                )}
            </div>

            {options.length > 1 && (
                <label style={s.accuracyFilter}>
                    <span style={s.accuracyFilterLabel}>Policy version</span>
                    <select
                        className="modern-input"
                        style={s.accuracySelect}
                        value={policy}
                        onChange={(e) => choosePolicy(e.target.value)}
                        data-testid="accuracy-policy-filter"
                    >
                        {options.map((o) => <option key={o.value || 'all'} value={o.value}>{o.label}</option>)}
                    </select>
                </label>
            )}

            {status === 'loading' && <div style={s.accuracyNote}>Loading accuracy…</div>}
            {status === 'failed' && <div style={s.accuracyNote}>Accuracy stats unavailable.</div>}
            {status === 'ready' && !summary.hasData && (
                <div style={s.accuracyNote}>
                    Not enough triaged results yet — accuracy appears once failing results are triaged with a defect type.
                </div>
            )}
            {status === 'ready' && summary.hasData && (split || unknown) && (
                <div style={s.accuracyNote} data-testid="accuracy-split">
                    {[split && `All engines: ${split}.`, unknown].filter(Boolean).join(' ')}
                </div>
            )}
            {status === 'ready' && summary.hasData && byEngine.map((eng) => (
                <div key={eng.key} style={s.ladder} data-testid={`accuracy-engine-${eng.key}`}>
                    <div style={s.ladderCaption}>{eng.label} · {eng.rateLabel} · {eng.samples}</div>
                    {(eng.split || eng.coverage) && (
                        <div style={s.ladderSub}>{[eng.split, eng.coverage].filter(Boolean).join(' — ')}</div>
                    )}
                    {eng.rows.map((row) => (
                        <div key={row.key} style={s.ladderRow}>
                            <span style={s.ladderLabel}>{row.label} confidence</span>
                            <span style={{ ...s.ladderRate, color: row.hasSamples ? 'var(--text-primary)' : 'var(--text-secondary)' }}>{row.rateLabel}</span>
                            <span style={s.ladderSamples}>{row.hasSamples ? row.samples : 'no samples yet'}</span>
                        </div>
                    ))}
                </div>
            ))}

            {status === 'ready' && coverageOnly.length > 0 && (
                <div style={s.ladder} data-testid="accuracy-coverage">
                    <div style={s.ladderCaption}>Analyses with no triaged results yet</div>
                    {coverageOnly.map((c) => (
                        <div key={c.key} style={s.ladderRow}>
                            <span style={s.ladderLabel}>{c.label}</span>
                            <span style={s.coverageText}>{c.text}</span>
                        </div>
                    ))}
                </div>
            )}

            {status === 'ready' && summary.hasData && (
                <div style={s.ladder}>
                    <div style={s.ladderCaption}>All engines</div>
                    {rows.map((row) => (
                        <div key={row.key} style={s.ladderRow}>
                            <span style={s.ladderLabel}>{row.label} confidence</span>
                            <span
                                style={{
                                    ...s.ladderRate,
                                    color: row.hasSamples ? 'var(--text-primary)' : 'var(--text-secondary)',
                                }}
                            >
                                {row.rateLabel}
                            </span>
                            <span style={s.ladderSamples}>{row.hasSamples ? row.samples : 'no samples yet'}</span>
                        </div>
                    ))}
                </div>
            )}

            {/* Per-verdict breakdown: which kind of call the AI actually gets wrong. This is what
                justifies snapshotting the verdict separately — the verdict -> defect_type mapping
                is lossy, so flaky_test and test_data would otherwise merge into one bucket. */}
            {status === 'ready' && byVerdict.length > 0 && (
                <div style={s.ladder}>
                    <div style={s.ladderCaption}>By verdict</div>
                    {byVerdict.map((row) => (
                        <div key={row.key} style={s.ladderRow}>
                            <span style={s.ladderLabel}>{row.label}</span>
                            <span style={{ ...s.ladderRate, color: 'var(--text-primary)' }}>{row.rateLabel}</span>
                            <span style={s.ladderSamples}>{row.samples}</span>
                        </div>
                    ))}
                </div>
            )}
        </div>
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
    accuracyPanel: {
        display: 'flex',
        flexDirection: 'column',
        gap: 10,
        padding: '12px 14px',
        borderRadius: 10,
        border: '1px solid var(--border-color)',
        background: 'var(--bg-secondary)',
    },
    accuracyHead: {
        display: 'flex',
        alignItems: 'flex-start',
        justifyContent: 'space-between',
        gap: 16,
    },
    accuracyHeadline: {
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'flex-end',
        flexShrink: 0,
    },
    accuracyRate: {
        fontSize: '1.35rem',
        fontWeight: 700,
        lineHeight: 1.1,
        color: 'var(--aig-tone-indigo-fg)',
    },
    accuracySamples: {
        fontSize: '0.72rem',
        color: 'var(--text-secondary)',
    },
    accuracyHeadlineLabel: {
        fontSize: '0.72rem',
        fontWeight: 600,
        color: 'var(--text-secondary)',
        textDecoration: 'underline dotted',
        textUnderlineOffset: 2,
        cursor: 'help',
    },
    accuracyFilter: {
        display: 'flex',
        alignItems: 'center',
        gap: 8,
    },
    accuracyFilterLabel: {
        fontSize: '0.78rem',
        color: 'var(--text-secondary)',
    },
    accuracySelect: {
        width: 'auto',
        minWidth: 180,
        padding: '4px 8px',
        fontSize: '0.8rem',
    },
    accuracyNote: {
        fontSize: '0.78rem',
        color: 'var(--text-secondary)',
        lineHeight: 1.5,
    },
    ladder: {
        display: 'flex',
        flexDirection: 'column',
        gap: 2,
        borderTop: '1px solid var(--border-color)',
        paddingTop: 8,
    },
    ladderCaption: {
        fontSize: '0.72rem',
        fontWeight: 700,
        textTransform: 'uppercase',
        letterSpacing: '0.04em',
        color: 'var(--text-secondary)',
        marginBottom: 2,
    },
    ladderSub: {
        fontSize: '0.74rem',
        color: 'var(--text-secondary)',
        marginBottom: 4,
    },
    coverageText: {
        fontSize: '0.74rem',
        color: 'var(--text-secondary)',
        textAlign: 'right',
    },
    ladderRow: {
        display: 'flex',
        alignItems: 'baseline',
        gap: 10,
    },
    ladderLabel: {
        flex: 1,
        minWidth: 0,
        fontSize: '0.8rem',
        color: 'var(--text-primary)',
    },
    ladderRate: {
        width: 46,
        textAlign: 'right',
        fontSize: '0.82rem',
        fontWeight: 700,
        fontVariantNumeric: 'tabular-nums',
    },
    ladderSamples: {
        width: 130,
        textAlign: 'right',
        fontSize: '0.72rem',
        color: 'var(--text-secondary)',
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
