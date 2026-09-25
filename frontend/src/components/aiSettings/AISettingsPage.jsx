import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useAIGeneration } from '../../contexts/AIGenerationContext';
import { defaultProvider } from '../../utils/typesafeSettings';
import { AI_TABS, nextTabId, summaryTiles, tabForSetting } from '../../utils/aiSettingsTabs';
import AIFeaturesToggle from '../AIFeaturesToggle';
import ProviderManager from './ProviderManager';
import TemplateEditor from './TemplateEditor';
import GenerationDefaults from './GenerationDefaults';
import BudgetSettings from './BudgetSettings';
import FailureAnalysisSection from './FailureAnalysisSection';
import { jumpToSetting, RevealSettingContext } from './jumpToSetting';
import { ps } from './aiSettingsPageStyles';

const TAB_IDS = AI_TABS.map((t) => t.id);

// AISettingsPage is Settings → AI: the AI switch, three summary tiles and four tabs. Every tab's
// sections stay mounted (inactive panels are hidden), so unsaved drafts survive a tab switch;
// each section still saves on its own and reports its state here for the tiles and tab dots.
export default function AISettingsPage({ isAdmin }) {
    const { aiFeaturesEnabled, aiFeaturesStatus, providers, providersStatus } = useAIGeneration();
    const [tab, setTab] = useState('providers');
    const [templates, setTemplates] = useState(null);
    const [budgets, setBudgets] = useState(null);
    const [dirty, setDirty] = useState({});
    const [pendingJump, setPendingJump] = useState(null);

    const markDirty = useCallback((key, value) => {
        setDirty((d) => (d[key] === value ? d : { ...d, [key]: value }));
    }, []);
    const onTemplates = useCallback((st) => { setTemplates(st); markDirty('templates', st.dirty); }, [markDirty]);
    const onCoverageDirty = useCallback((v) => markDirty('coverage', v), [markDirty]);
    const onBudgets = useCallback((st) => { setBudgets(st); markDirty('budgets', st.dirty); }, [markDirty]);
    const onAnalysisDirty = useCallback((v) => markDirty('analysis', v), [markDirty]);
    const tabDirty = { prompts: !!dirty.templates, limits: !!dirty.coverage || !!dirty.budgets, analysis: !!dirty.analysis };

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

    const tiles = useMemo(() => summaryTiles({
        provider: defaultProvider(providers), providersStatus, templates, budgets,
    }), [providers, providersStatus, templates, budgets]);

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
                            {tabDirty[t.id] && <span style={ps.dirtyDot} aria-label="unsaved changes" data-testid={`ai-tab-dirty-${t.id}`} />}
                        </button>
                    ))}
                </div>

                <div {...panel('providers')}><ProviderManager isAdmin={isAdmin} /></div>
                <div {...panel('prompts')}><TemplateEditor isAdmin={isAdmin} onStatusChange={onTemplates} /></div>
                <div {...panel('limits')}>
                    <GenerationDefaults isAdmin={isAdmin} onDirtyChange={onCoverageDirty} />
                    <BudgetSettings isAdmin={isAdmin} onStatusChange={onBudgets} />
                </div>
                <div {...panel('analysis')}><FailureAnalysisSection isAdmin={isAdmin} onDirtyChange={onAnalysisDirty} /></div>

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
        </RevealSettingContext.Provider>
    );
}
