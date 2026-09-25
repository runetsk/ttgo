import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useAIGeneration } from '../../contexts/AIGenerationContext';
import { toast } from '../../toast';
import { defaultProvider } from '../../utils/typesafeSettings';
import { AI_TABS, nextTabId, summaryTiles, tabForSetting } from '../../utils/aiSettingsTabs';
import { saveBarModel, runSaves, saveResultMessage, sameSummary } from '../../utils/saveBar';
import { useUnsavedGuard } from '../../hooks/useUnsavedGuard';
import AIFeaturesToggle from '../AIFeaturesToggle';
import ProviderManager from './ProviderManager';
import TemplateEditor from './TemplateEditor';
import GenerationDefaults from './GenerationDefaults';
import BudgetSettings from './BudgetSettings';
import FailureAnalysisSection from './FailureAnalysisSection';
import SaveBar from './SaveBar';
import { SaveBarContext } from './saveBarContext';
import { jumpToSetting, RevealSettingContext } from './jumpToSetting';
import { ps } from './aiSettingsPageStyles';

const TAB_IDS = AI_TABS.map((t) => t.id);

// AISettingsPage is Settings → AI: the AI switch, three summary tiles, four tabs and one Save
// bar. Every tab's sections stay mounted (inactive panels are hidden), so unsaved drafts survive
// a tab switch. Sections keep their own drafts and API calls and register save/discard handles
// here (saveBarContext.js); the bar saves every dirty section at once and keeps what succeeded.
export default function AISettingsPage({ isAdmin, onDirtyChange }) {
    const { aiFeaturesEnabled, aiFeaturesStatus, providers, providersStatus } = useAIGeneration();
    const [tab, setTab] = useState('providers');
    const [templateStatus, setTemplateStatus] = useState(null); // { standardCustom, parentCustom }
    const [budgets, setBudgets] = useState(null); // { monthlyUsd, spentUsd }
    const [pendingJump, setPendingJump] = useState(null);
    const [sections, setSections] = useState({});
    const [saving, setSaving] = useState(false);
    const [result, setResult] = useState(null); // the message after a partial failure
    const handles = useRef({});

    const registry = useMemo(() => ({
        setHandle: (key, handle) => { handles.current[key] = handle; },
        setSummary: (key, summary) => setSections((prev) => (sameSummary(prev[key], summary) ? prev : { ...prev, [key]: summary })),
        remove: (key) => {
            delete handles.current[key];
            setSections((prev) => {
                if (!(key in prev)) return prev;
                const next = { ...prev };
                delete next[key];
                return next;
            });
        },
    }), []);
    const saveBarValue = useMemo(() => ({ registry, saving }), [registry, saving]);

    const list = useMemo(() => Object.values(sections), [sections]);
    const model = useMemo(() => saveBarModel(list, AI_TABS), [list]);
    const dirtyAny = model.dirtyCount > 0;
    useEffect(() => { onDirtyChange?.(dirtyAny); }, [dirtyAny, onDirtyChange]);
    useEffect(() => () => onDirtyChange?.(false), [onDirtyChange]);
    useUnsavedGuard(dirtyAny, 'Discard unsaved AI settings?');

    const save = useCallback(async () => {
        if (saving || !model.canSave) return;
        setSaving(true);
        setResult(null);
        const outcome = await runSaves(list.map((s) => ({ ...s, save: handles.current[s.key]?.save })));
        setSaving(false);
        if (outcome.failed.length === 0) toast.success(saveResultMessage(outcome));
        else setResult(saveResultMessage(outcome));
    }, [saving, model.canSave, list]);

    const discard = () => {
        if (model.dirtyTabs.length > 1 && !window.confirm(`Discard unsaved changes on ${model.dirtyTabs.length} tabs?`)) return;
        list.filter((s) => s.dirty).forEach((s) => handles.current[s.key]?.discard());
        setResult(null);
    };

    // Ctrl/⌘+S from anywhere while the page is open (focus is often outside it, e.g. on the
    // Settings sidebar). Only claims the shortcut when there is something to save.
    useEffect(() => {
        const onKey = (e) => {
            if (!(e.ctrlKey || e.metaKey) || e.key.toLowerCase() !== 's' || !dirtyAny) return;
            e.preventDefault();
            save();
        };
        document.addEventListener('keydown', onKey);
        return () => document.removeEventListener('keydown', onKey);
    }, [dirtyAny, save]);

    // A setting chip may name a control on another tab: switch first, jump once it has rendered.
    const reveal = useCallback((key) => {
        const target = tabForSetting(key);
        if (target && target !== tab) {
            setTab(target);
            setPendingJump(key);
        } else {
            jumpToSetting(key);
        }
    }, [tab]);
    useEffect(() => {
        if (!pendingJump) return undefined;
        const frame = requestAnimationFrame(() => {
            jumpToSetting(pendingJump);
            setPendingJump(null);
        });
        return () => cancelAnimationFrame(frame);
    }, [pendingJump, tab]);

    const templatesDirty = !!sections['template.standard']?.dirty || !!sections['template.parent']?.dirty;
    const tiles = useMemo(() => summaryTiles({
        provider: defaultProvider(providers),
        providersStatus,
        templates: templateStatus && { ...templateStatus, dirty: templatesDirty },
        budgets,
    }), [providers, providersStatus, templateStatus, templatesDirty, budgets]);

    const selectTab = (id) => {
        setTab(id);
        document.getElementById(`ai-tab-${id}`)?.focus();
    };
    const onTabKey = (e) => {
        const next = nextTabId(TAB_IDS, tab, e.key);
        if (!next) return;
        e.preventDefault();
        selectTab(next);
    };
    // Inactive panels stay mounted (their drafts live in them) and are hidden with display:none;
    // the `hidden` attribute alone would lose to the inline display:flex.
    const panel = (id) => ({
        role: 'tabpanel', id: `ai-panel-${id}`, 'aria-labelledby': `ai-tab-${id}`, hidden: tab !== id,
        'data-testid': `ai-panel-${id}`, style: tab === id ? ps.panel : { display: 'none' },
    });

    return (
        <RevealSettingContext.Provider value={reveal}>
            <SaveBarContext.Provider value={saveBarValue}>
                <div style={ps.page} data-testid="ai-settings">
                    <div style={ps.header}>
                        <div>
                            <h3 style={ps.title}>AI</h3>
                            <p style={ps.desc}>
                                Models, prompts and spending for AI test generation and failure analysis.
                                API keys are stored server-side and masked in responses.
                            </p>
                        </div>
                        <AIFeaturesToggle isAdmin={isAdmin} />
                    </div>

                    {aiFeaturesStatus === 'ready' && !aiFeaturesEnabled && (
                        <div role="status" style={ps.banner} data-testid="ai-off-banner">
                            AI features are off: AI actions are hidden, new failure analyses are refused and completed runs
                            are not queued. An analysis already running finishes. Your settings below are kept.
                        </div>
                    )}

                    <div style={ps.tiles}>
                        {tiles.map((t) => (
                            <button key={t.id} type="button" style={ps.tile} data-testid={`ai-tile-${t.id}`} onClick={() => selectTab(t.tab)}>
                                <span style={ps.tileLabel}>{t.label}</span>
                                <span style={ps.tileValue}>{t.value}</span>
                                {t.sub && <span style={ps.tileSub(t.tone)}>{t.sub}</span>}
                                {t.meter && <span style={ps.meterTrack} aria-hidden="true"><span style={ps.meterFill(t.meter.pct, t.meter.tone)} /></span>}
                            </button>
                        ))}
                    </div>

                    <div role="tablist" aria-label="AI settings" style={ps.tablist} onKeyDown={onTabKey}>
                        {AI_TABS.map((t) => (
                            <button key={t.id} type="button" role="tab" id={`ai-tab-${t.id}`} aria-selected={tab === t.id}
                                aria-controls={`ai-panel-${t.id}`} tabIndex={tab === t.id ? 0 : -1} data-testid={`ai-tab-${t.id}`}
                                onClick={() => setTab(t.id)} style={{ ...ps.tab, ...(tab === t.id ? ps.tabOn : null) }}>
                                {t.label}
                                {model.dirtyTabs.includes(t.id) && <span style={ps.dirtyDot} aria-label="unsaved changes" data-testid={`ai-tab-dirty-${t.id}`} />}
                            </button>
                        ))}
                    </div>

                    <div {...panel('providers')}><ProviderManager isAdmin={isAdmin} /></div>
                    <div {...panel('prompts')}><TemplateEditor isAdmin={isAdmin} onStatusChange={setTemplateStatus} /></div>
                    <div {...panel('limits')}>
                        <GenerationDefaults isAdmin={isAdmin} />
                        <BudgetSettings isAdmin={isAdmin} onStatusChange={setBudgets} />
                    </div>
                    <div {...panel('analysis')}><FailureAnalysisSection isAdmin={isAdmin} /></div>

                    {dirtyAny && (
                        <SaveBar model={model} result={result} saving={saving} onSave={save} onDiscard={discard} onReveal={reveal} />
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
            </SaveBarContext.Provider>
        </RevealSettingContext.Provider>
    );
}
