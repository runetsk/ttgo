// Pure logic for the Settings → AI save bar (components/aiSettings/SaveBar.jsx and
// AISettingsPage.jsx). No React, no network.
import { SETTING_HELP } from './analysisSettingsHelp.js';

const joinLabels = (labels) => (labels.length <= 1 ? labels.join('')
    : `${labels.slice(0, -1).join(', ')} and ${labels[labels.length - 1]}`);

// saveBarModel summarises the registered sections: which tabs have unsaved changes, which
// invalid settings block saving, and the bar's message. `tabs` gives the order and labels.
export function saveBarModel(sections, tabs) {
    const tabLabel = Object.fromEntries(tabs.map((t) => [t.id, t.label]));
    const order = tabs.map((t) => t.id);
    const dirty = sections.filter((s) => s.dirty);
    const dirtyTabs = order.filter((id) => dirty.some((s) => s.tab === id));
    const problems = dirty
        .filter((s) => s.errors.length > 0)
        .sort((a, b) => order.indexOf(a.tab) - order.indexOf(b.tab))
        .flatMap((s) => s.errors.map((e) => ({ ...e, section: s.key, tab: s.tab, tabLabel: tabLabel[s.tab] })));
    let message = '';
    if (problems.length > 0) message = `Fix ${problems.length} ${problems.length === 1 ? 'setting' : 'settings'} before saving:`;
    else if (dirty.length > 0) message = `Unsaved changes in ${joinLabels(dirtyTabs.map((id) => tabLabel[id]))}`;
    return { dirtyCount: dirty.length, dirtyTabs, problems, canSave: dirty.length > 0 && problems.length === 0, message };
}

// runSaves saves every dirty, valid section at once (they use separate endpoints) and keeps
// what succeeded: a rejection or a synchronous throw only fails its own section.
export async function runSaves(sections) {
    const todo = sections.filter((s) => s.dirty && s.errors.length === 0 && typeof s.save === 'function');
    const results = await Promise.allSettled(todo.map((s) => Promise.resolve().then(() => s.save())));
    const saved = [];
    const failed = [];
    results.forEach((r, i) => {
        const { key, label, tab } = todo[i];
        if (r.status === 'fulfilled') saved.push({ key, label, tab });
        else failed.push({ key, label, tab, message: r.reason?.message || String(r.reason) });
    });
    return { saved, failed };
}

// saveResultMessage names sections, not tabs: one tab can hold a saved and a failed section.
export function saveResultMessage({ saved, failed }) {
    if (failed.length === 0) return 'AI settings saved';
    const parts = [];
    if (saved.length > 0) parts.push(`Saved ${joinLabels(saved.map((s) => s.label))}.`);
    failed.forEach((f) => parts.push(`Couldn't save ${f.label}: ${f.message}`));
    return parts.join(' ');
}

// errorDescriptors turns a validator's { field: message } into the bar's list: the setting's
// jump key (the data-setting anchor) and its human label from SETTING_HELP.
export function errorDescriptors(errors, prefix) {
    return Object.keys(errors || {}).map((field) => {
        const settingKey = `${prefix}.${field}`;
        return { field, label: SETTING_HELP[settingKey]?.title ?? field, settingKey };
    });
}

// saveError is what a section's registered save() throws: the server's message, else a
// fallback (an axios "Request failed with status code 500" is not for people).
export function saveError(err, fallback) {
    return new Error(err?.response?.data?.error || fallback);
}

export function sameSummary(a, b) {
    return !!a && !!b && a.key === b.key && a.label === b.label && a.tab === b.tab && a.dirty === b.dirty
        && JSON.stringify(a.errors) === JSON.stringify(b.errors);
}

const AI_HASH = '#ai-test-generation';

// shouldBlockNavigation says whether a router navigation would leave Settings → AI with unsaved
// changes. Log out is never blocked: the session is already gone when it navigates to /login.
export function shouldBlockNavigation({ dirty, from, to }) {
    if (!dirty || to.pathname === '/login') return false;
    if (to.pathname !== from.pathname) return true;
    return to.pathname === '/settings' && !!to.hash && to.hash !== AI_HASH;
}
