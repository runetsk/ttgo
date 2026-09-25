import test from 'node:test';
import assert from 'node:assert/strict';
import {
    saveBarModel, runSaves, saveResultMessage, errorDescriptors, saveError, sameSummary, shouldBlockNavigation,
} from './saveBar.js';

const TABS = [
    { id: 'providers', label: 'Providers' }, { id: 'prompts', label: 'Prompts' },
    { id: 'limits', label: 'Limits & budget' }, { id: 'analysis', label: 'Failure analysis' },
];
const sec = (key, tab, dirty, errors = []) => ({ key, label: key, tab, dirty, errors });
const MAX = { field: 'max_analyses_per_run', label: 'Max analyses per run', settingKey: 'fa.max_analyses_per_run' };

test('saveBarModel: nothing dirty', () => {
    const m = saveBarModel([sec('a', 'prompts', false)], TABS);
    assert.deepEqual([m.dirtyCount, m.dirtyTabs, m.canSave, m.message], [0, [], false, '']);
});

test('saveBarModel: names dirty tabs in tab order', () => {
    const m = saveBarModel([sec('fa', 'analysis', true), sec('t', 'prompts', true), sec('c', 'limits', false)], TABS);
    assert.deepEqual(m.dirtyTabs, ['prompts', 'analysis']);
    assert.equal(m.message, 'Unsaved changes in Prompts and Failure analysis');
    assert.equal(m.canSave, true);
    const three = saveBarModel([sec('a', 'analysis', true), sec('b', 'prompts', true), sec('c', 'limits', true)], TABS);
    assert.equal(three.message, 'Unsaved changes in Prompts, Limits & budget and Failure analysis');
});

test('saveBarModel: an invalid dirty section blocks saving and is listed', () => {
    const m = saveBarModel([sec('t', 'prompts', true), sec('fa', 'analysis', true, [MAX])], TABS);
    assert.equal(m.canSave, false);
    assert.equal(m.message, 'Fix 1 setting before saving:');
    assert.deepEqual(m.problems, [{ ...MAX, section: 'fa', tab: 'analysis', tabLabel: 'Failure analysis' }]);
    const two = saveBarModel([sec('fa', 'analysis', true, [MAX, { ...MAX, field: 'x', settingKey: 'fa.x' }])], TABS);
    assert.equal(two.message, 'Fix 2 settings before saving:');
});

test('saveBarModel: errors on a clean section do not block', () => {
    const m = saveBarModel([sec('fa', 'analysis', false, [MAX]), sec('t', 'prompts', true)], TABS);
    assert.deepEqual([m.canSave, m.problems], [true, []]);
});

test('runSaves: saves dirty valid sections in parallel and reports each outcome', async () => {
    const calls = [];
    const ok = (key) => ({ ...sec(key, 'prompts', true), save: async () => { calls.push(key); } });
    const result = await runSaves([
        ok('a'),
        { ...sec('b', 'analysis', true), save: async () => { throw new Error('server said no'); } },
        { ...sec('c', 'analysis', true), save: () => { throw new Error('sync boom'); } },
        { ...sec('d', 'limits', false), save: async () => { calls.push('d'); } },
        { ...sec('e', 'analysis', true, [MAX]), save: async () => { calls.push('e'); } },
    ]);
    assert.deepEqual(calls, ['a']);
    assert.deepEqual(result.saved, [{ key: 'a', label: 'a', tab: 'prompts' }]);
    assert.deepEqual(result.failed, [
        { key: 'b', label: 'b', tab: 'analysis', message: 'server said no' },
        { key: 'c', label: 'c', tab: 'analysis', message: 'sync boom' },
    ]);
});

test('saveResultMessage', () => {
    assert.equal(saveResultMessage({ saved: [{ label: 'A' }], failed: [] }), 'AI settings saved');
    assert.equal(
        saveResultMessage({ saved: [{ label: 'Standard prompt template' }], failed: [{ label: 'TypeSafe.ai', message: 'bad key' }] }),
        "Saved Standard prompt template. Couldn't save TypeSafe.ai: bad key",
    );
    assert.equal(
        saveResultMessage({ saved: [], failed: [{ label: 'A', message: 'x' }, { label: 'B', message: 'y' }] }),
        "Couldn't save A: x Couldn't save B: y",
    );
});

test('errorDescriptors labels validator fields from SETTING_HELP', () => {
    assert.deepEqual(errorDescriptors({ max_analyses_per_run: 'bad' }, 'fa'), [MAX]);
    assert.deepEqual(errorDescriptors({ timeout_seconds: 'bad' }, 'ts')[0].settingKey, 'ts.timeout_seconds');
    assert.equal(errorDescriptors({ timeout_seconds: 'bad' }, 'ts')[0].label, 'Timeout');
    assert.deepEqual(errorDescriptors({ mystery: 'bad' }, 'fa'), [{ field: 'mystery', label: 'mystery', settingKey: 'fa.mystery' }]);
    assert.deepEqual(errorDescriptors(undefined, 'fa'), []);
});

test('saveError prefers the server message', () => {
    assert.equal(saveError({ response: { data: { error: 'nope' } } }, 'fallback').message, 'nope');
    assert.equal(saveError(new Error('Request failed with status code 500'), 'fallback').message, 'fallback');
});

test('sameSummary compares the registered fields', () => {
    const a = { key: 'k', label: 'L', tab: 't', dirty: true, errors: [MAX] };
    assert.equal(sameSummary(a, { ...a, errors: [{ ...MAX }] }), true);
    assert.equal(sameSummary(a, { ...a, dirty: false }), false);
    assert.equal(sameSummary(undefined, a), false);
});

test('shouldBlockNavigation', () => {
    const at = (pathname, hash = '') => ({ pathname, hash });
    const from = at('/settings', '#ai-test-generation');
    const block = (to, dirty = true) => shouldBlockNavigation({ dirty, from, to });
    assert.equal(block(at('/runs'), false), false, 'nothing to lose');
    assert.equal(block(at('/runs')), true);
    assert.equal(block(at('/login')), false, 'log out ends the session before it navigates');
    assert.equal(block(at('/settings', '#custom-fields')), true);
    assert.equal(block(at('/settings', '#ai-test-generation')), false);
    assert.equal(block(at('/settings')), false);
});
