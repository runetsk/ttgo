// Pure helpers for Settings → AI (components/aiSettings/AISettingsPage.jsx and the sections it
// holds). No React, no network.

export const AI_TABS = [
    { id: 'providers', label: 'Providers' },
    { id: 'prompts', label: 'Prompts' },
    { id: 'limits', label: 'Limits & budget' },
    { id: 'analysis', label: 'Failure analysis' },
];

// tabForSetting names the tab holding a data-setting key, or null when the control sits in the
// page header, which is always visible (ai.enabled). ts.* and fa.* live on Failure analysis.
export function tabForSetting(key) {
    if (!key || key.startsWith('ai.')) return null;
    if (key.startsWith('provider.')) return 'providers';
    return 'analysis';
}

// nextTabId implements the tablist keys: arrows move and wrap, Home/End jump to the ends.
export function nextTabId(ids, current, key) {
    const i = ids.indexOf(current);
    if (i < 0) return null;
    switch (key) {
        case 'ArrowRight': return ids[(i + 1) % ids.length];
        case 'ArrowLeft': return ids[(i - 1 + ids.length) % ids.length];
        case 'Home': return ids[0];
        case 'End': return ids[ids.length - 1];
        default: return null;
    }
}

export function placeholderStatus(content, all, required) {
    const text = content || '';
    return all.map((name) => ({ name, required: required.includes(name), present: text.includes(name) }));
}

// insertText replaces [start, end) with text and returns where the caret goes.
export function insertText(value, start, end, text) {
    return { value: value.slice(0, start) + text + value.slice(end), caret: start + text.length };
}

// monthMeter fills the month's spend against the monthly budget: amber from 80%, red at 100%.
export function monthMeter(spentUsd, monthlyUsd) {
    if (!(monthlyUsd > 0) || spentUsd == null) return null;
    const pct = Math.min(100, Math.round((spentUsd / monthlyUsd) * 100));
    return { pct, tone: pct >= 100 ? 'bad' : pct >= 80 ? 'warn' : 'neutral' };
}

const usd = (n) => `$${n.toFixed(2)}`;
const LOADING = { value: 'Loading…', sub: '', tone: 'neutral', meter: null };

function modelTile(provider, providersStatus) {
    const base = { id: 'model', tab: 'providers', label: 'Default model' };
    if (providersStatus === 'loading') return { ...base, ...LOADING };
    if (providersStatus === 'error') return { ...base, value: "Couldn't load providers", sub: 'Reload the page to try again', tone: 'bad', meter: null };
    if (!provider) return { ...base, value: 'No default model', sub: 'Add a provider and make it the default', tone: 'warn', meter: null };
    return { ...base, value: provider.label, sub: provider.model_name || '', tone: 'ok', meter: null };
}

function promptsTile(templates) {
    const base = { id: 'prompts', tab: 'prompts', label: 'Prompt templates' };
    if (!templates) return { ...base, ...LOADING };
    const { standardCustom: std, parentCustom: par, dirty } = templates;
    const value = std && par ? 'Both customized' : std ? 'Standard customized' : par ? 'Parent customized' : 'Built-in defaults';
    const sub = dirty ? 'Unsaved changes'
        : std && !par ? 'Parent uses the built-in default'
            : par && !std ? 'Standard uses the built-in default'
                : std ? 'Standard and parent edited' : 'Standard and parent unchanged';
    return { ...base, value, sub, tone: dirty ? 'warn' : 'neutral', meter: null };
}

function spendTile(budgets) {
    const base = { id: 'spend', tab: 'limits', label: 'Estimated spend this month' };
    if (!budgets) return { ...base, ...LOADING };
    const { monthlyUsd, spentUsd } = budgets;
    const meter = monthMeter(spentUsd, monthlyUsd);
    if (!meter) return { ...base, value: `${usd(spentUsd)} this month`, sub: 'No monthly budget set', tone: 'neutral', meter: null };
    return {
        ...base,
        value: `${usd(spentUsd)} of ${usd(monthlyUsd)}`,
        sub: meter.tone === 'bad' ? 'Over budget: generation asks to confirm' : 'Resets on the 1st (UTC)',
        tone: meter.tone,
        meter,
    };
}

export function summaryTiles({ provider, providersStatus, templates, budgets }) {
    return [modelTile(provider, providersStatus), promptsTile(templates), spendTile(budgets)];
}
