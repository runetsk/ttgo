import test from 'node:test';
import assert from 'node:assert/strict';
import {
    AI_TABS, tabForSetting, nextTabId, placeholderStatus, insertText, monthMeter, summaryTiles,
} from './aiSettingsTabs.js';

test('AI_TABS lists the five tabs in order', () => {
    assert.deepEqual(AI_TABS.map((t) => t.id), ['providers', 'prompts', 'limits', 'analysis', 'typesafe']);
});

test('tabForSetting routes data-setting keys to their tab', () => {
    assert.equal(tabForSetting('provider.default'), 'providers');
    assert.equal(tabForSetting('fa.dedup_enabled'), 'analysis');
    assert.equal(tabForSetting('fa.prompt_template'), 'analysis');
    assert.equal(tabForSetting('ts.api_key'), 'typesafe');
    assert.equal(tabForSetting('ts.verdict_engine_enabled'), 'typesafe');
    assert.equal(tabForSetting('ai.enabled'), null);
    assert.equal(tabForSetting(''), null);
});

test('nextTabId moves with arrows, wraps, and jumps with Home/End', () => {
    const ids = ['a', 'b', 'c'];
    assert.equal(nextTabId(ids, 'a', 'ArrowRight'), 'b');
    assert.equal(nextTabId(ids, 'c', 'ArrowRight'), 'a');
    assert.equal(nextTabId(ids, 'a', 'ArrowLeft'), 'c');
    assert.equal(nextTabId(ids, 'b', 'Home'), 'a');
    assert.equal(nextTabId(ids, 'b', 'End'), 'c');
    assert.equal(nextTabId(ids, 'b', 'Enter'), null);
    assert.equal(nextTabId(ids, 'x', 'ArrowRight'), null);
});

test('placeholderStatus marks required and present placeholders', () => {
    const rows = placeholderStatus('Hi {{TITLE}}', ['{{TITLE}}', '{{DESCRIPTION}}'], ['{{DESCRIPTION}}']);
    assert.deepEqual(rows, [
        { name: '{{TITLE}}', required: false, present: true },
        { name: '{{DESCRIPTION}}', required: true, present: false },
    ]);
    assert.equal(placeholderStatus(undefined, ['{{TITLE}}'], [])[0].present, false);
});

test('insertText replaces the selection and puts the caret after the insert', () => {
    assert.deepEqual(insertText('ab', 1, 1, 'X'), { value: 'aXb', caret: 2 });
    assert.deepEqual(insertText('abcd', 1, 3, 'X'), { value: 'aXd', caret: 2 });
});

test('monthMeter is off without a budget or a known spend and clamps at 100', () => {
    assert.equal(monthMeter(3, 0), null);
    assert.equal(monthMeter(null, 10), null);
    assert.deepEqual(monthMeter(2.5, 10), { pct: 25, tone: 'neutral' });
    assert.deepEqual(monthMeter(8, 10), { pct: 80, tone: 'warn' });
    assert.deepEqual(monthMeter(15, 10), { pct: 100, tone: 'bad' });
});

const PROVIDER = { label: 'Anthropic Claude', model_name: 'claude-sonnet-4-5', is_default: true, enabled: true };

test('summaryTiles: model tile covers loading, error, none and a default', () => {
    const tile = (args) => summaryTiles({ templates: null, budgets: null, ...args })[0];
    assert.equal(tile({ provider: null, providersStatus: 'loading' }).value, 'Loading…');
    assert.equal(tile({ provider: null, providersStatus: 'error' }).tone, 'bad');
    const none = tile({ provider: null, providersStatus: 'ready' });
    assert.equal(none.value, 'No default model');
    assert.equal(none.tone, 'warn');
    const ok = tile({ provider: PROVIDER, providersStatus: 'ready' });
    assert.deepEqual([ok.id, ok.tab, ok.value, ok.sub, ok.tone], ['model', 'providers', 'Anthropic Claude', 'claude-sonnet-4-5', 'ok']);
    const sameName = tile({ provider: { ...PROVIDER, label: 'claude-sonnet-4-5' }, providersStatus: 'ready' });
    assert.equal(sameName.sub, '', 'the model is not named twice');
});

test('summaryTiles: prompts tile names what is customized and flags unsaved edits', () => {
    const tile = (templates) => summaryTiles({ provider: null, providersStatus: 'ready', templates, budgets: null })[1];
    assert.equal(tile(null).value, 'Loading…');
    const untouched = tile({ standardCustom: false, parentCustom: false, dirty: false });
    assert.deepEqual([untouched.value, untouched.sub], ['Built-in defaults', 'Standard and parent unchanged']);
    const std = tile({ standardCustom: true, parentCustom: false, dirty: false });
    assert.deepEqual([std.value, std.sub, std.tab], ['Standard customized', 'Parent uses the built-in default', 'prompts']);
    assert.equal(tile({ standardCustom: false, parentCustom: true, dirty: false }).sub, 'Standard uses the built-in default');
    assert.equal(tile({ standardCustom: true, parentCustom: true, dirty: false }).value, 'Both customized');
    const dirty = tile({ standardCustom: true, parentCustom: false, dirty: true });
    assert.deepEqual([dirty.sub, dirty.tone], ['Unsaved changes', 'warn']);
});

test('summaryTiles: spend tile shows spend against the budget', () => {
    const tile = (budgets) => summaryTiles({ provider: null, providersStatus: 'ready', templates: null, budgets })[2];
    assert.equal(tile(null).value, 'Loading…');
    const withBudget = tile({ monthlyUsd: 50, spentUsd: 18.4 });
    assert.deepEqual([withBudget.tab, withBudget.value, withBudget.meter], ['limits', '$18.40 of $50.00', { pct: 37, tone: 'neutral' }]);
    assert.equal(withBudget.sub, 'Resets on the 1st (UTC)');
    const over = tile({ monthlyUsd: 10, spentUsd: 12 });
    assert.deepEqual([over.tone, over.sub], ['bad', 'Over budget: generation asks to confirm']);
    const noBudget = tile({ monthlyUsd: 0, spentUsd: 3 });
    assert.deepEqual([noBudget.value, noBudget.sub, noBudget.meter], ['$3.00 this month', 'No monthly budget set', null]);
});
