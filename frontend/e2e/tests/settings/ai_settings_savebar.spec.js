import { test, expect } from '../../fixtures/test.js';

// The Settings → AI save bar. Every settings endpoint is served by an in-memory mock installed
// before navigation: GETs read its state, PUTs merge into it, are recorded, and can be made to
// fail or to answer late. Any other settings write is refused, so the spec never changes a
// real setting and is safe against a scratch backend holding live keys.

const clone = (v) => JSON.parse(JSON.stringify(v));
const INITIAL = {
    template: {
        id: 'singleton',
        content: 'Custom {{COVERAGE}} {{TITLE}} {{DESCRIPTION}}', default_content: 'Default {{COVERAGE}} {{TITLE}} {{DESCRIPTION}}',
        parent_content: 'Parent {{COVERAGE}} {{TITLE}} {{CHILDREN}}', default_parent_content: 'Parent {{COVERAGE}} {{TITLE}} {{CHILDREN}}',
    },
    coverage: { essential_max_tokens: 4096, thorough_max_tokens: 8192, comprehensive_max_tokens: 16384 },
    budgets: { id: 'singleton', per_request_usd: 0, monthly_usd: 5, month_spent_usd: 1.25 },
    typesafe: {
        id: 'singleton', enabled: true, api_key_masked: '…1234', api_key_status: 'ok', model: 'jev-1.13.0', timeout_seconds: 30,
        verdict_engine_enabled: true, narrative_enabled: true, llm_fallback_enabled: true, escalate_below_pct: 0,
        semantic_dedup_enabled: true, allow_auto_failure_analysis: false,
    },
    fa: {
        id: 'singleton', enabled_on_completion: false, max_analyses_per_run: 20, parallel_groups: 4,
        dedup_enabled: true, redaction_enabled: true, prompt_template: 'Classify the failure.',
    },
};
const PROVIDERS = [
    { id: 'p1', label: 'Fake LLM', provider_type: 'openai', model_name: 'fake-1', is_default: true, enabled: true, allow_auto_failure_analysis: false },
];

const json = (body, status = 200) => ({ status, contentType: 'application/json', body: JSON.stringify(body) });
const sleep = (ms) => new Promise((r) => { setTimeout(r, ms); });

// mockSettings returns a handle: `puts` lists PUT names in arrival order; set `fail` (names that
// answer 500), `delay` (ms to wait before answering) and `staleOther` (each template PUT answers
// with the other template as it was at load, the way a racing response would) at any time.
async function mockSettings(page) {
    const state = clone(INITIAL);
    const mock = { state, puts: [], fail: [], delay: {}, staleOther: false, refused: [] };
    await page.route(/\/api\/settings\//, (route) => {
        if (route.request().method() === 'GET') return route.fallback();
        mock.refused.push(`${route.request().method()} ${route.request().url()}`);
        return route.fulfill(json({ error: 'this spec never saves there' }, 500));
    });
    // name → [state slice, how a PUT body applies to it]
    const endpoints = [
        ['template', /\/api\/settings\/ai-gen-template$/, 'template', (body) => { state.template.content = body.content; }],
        ['parent', /\/api\/settings\/ai-gen-parent-template$/, 'template', (body) => { state.template.parent_content = body.content; }],
        ['coverage', /\/api\/settings\/ai-gen-coverage$/, 'coverage', (body) => Object.assign(state.coverage, body)],
        ['budgets', /\/api\/settings\/ai-budgets$/, 'budgets', (body) => Object.assign(state.budgets, body)],
        ['typesafe', /\/api\/settings\/typesafe$/, 'typesafe', (body) => {
            const { api_key: _key, clear_api_key: _clear, ...rest } = body;
            Object.assign(state.typesafe, rest);
        }],
        ['fa', /\/api\/settings\/ai-failure-analysis$/, 'fa', (body) => Object.assign(state.fa, body)],
    ];
    for (const [name, pattern, slice, apply] of endpoints) {
        await page.route(pattern, async (route) => {
            const method = route.request().method();
            if (method === 'GET') return route.fulfill(json(state[slice]));
            if (method !== 'PUT') return route.fallback();
            mock.puts.push(name);
            if (mock.fail.includes(name)) return route.fulfill(json({ error: `${name} refused` }, 500));
            apply(route.request().postDataJSON());
            const answer = clone(state[slice]);
            if (mock.staleOther && name === 'template') answer.parent_content = INITIAL.template.parent_content;
            if (mock.staleOther && name === 'parent') answer.content = INITIAL.template.content;
            if (mock.delay[name]) await sleep(mock.delay[name]);
            return route.fulfill(json(answer));
        });
    }
    await page.route(/\/api\/settings\/llm-providers$/, (route) => (route.request().method() === 'GET' ? route.fulfill(json(PROVIDERS)) : route.fallback()));
    await page.route(/\/api\/settings\/ai-features$/, (route) => (route.request().method() === 'GET' ? route.fulfill(json({ enabled: true })) : route.fallback()));
    return mock;
}

const EDITED = 'Edited {{COVERAGE}} {{TITLE}} {{DESCRIPTION}}';

test.describe('Settings — AI save bar', () => {
    test('one Save sends every changed section and clears the bar', async ({ page, settingsPage }) => {
        const mock = await mockSettings(page);
        await settingsPage.open();
        await settingsPage.openAISettings('Prompts');
        await expect(settingsPage.saveBar).toHaveCount(0);

        await page.getByTestId('template-editor').fill(EDITED);
        await page.getByTestId('ai-tab-analysis').click();
        await page.getByTestId('typesafe-timeout').fill('45');
        await expect(settingsPage.saveBarMessage).toHaveText('Unsaved changes in Prompts and Failure analysis');

        await settingsPage.saveBarSave.click();
        await expect(settingsPage.saveBar).toHaveCount(0);
        expect([...mock.puts].sort()).toEqual(['template', 'typesafe']);
        expect(mock.state.template.content).toBe(EDITED);
        expect(mock.state.typesafe.timeout_seconds).toBe(45);
        await expect(page.getByTestId('ai-tab-dirty-prompts')).toHaveCount(0);
        await expect(page.getByText('AI settings saved')).toBeVisible();
        expect(mock.refused).toEqual([]);
    });

    test('a failed section stays unsaved and Save retries only it', async ({ page, settingsPage }) => {
        const mock = await mockSettings(page);
        mock.fail = ['typesafe'];
        await settingsPage.open();
        await settingsPage.openAISettings('Prompts');
        await page.getByTestId('template-editor').fill(EDITED);
        await page.getByTestId('ai-tab-analysis').click();
        await page.getByTestId('typesafe-timeout').fill('45');

        await settingsPage.saveBarSave.click();
        await expect(settingsPage.saveBarMessage).toHaveText("Saved Standard prompt template. Couldn't save TypeSafe.ai: typesafe refused");
        await expect(page.getByTestId('ai-tab-dirty-prompts')).toHaveCount(0);
        await expect(page.getByTestId('ai-tab-dirty-analysis')).toBeVisible();

        mock.fail = [];
        const before = mock.puts.length;
        await settingsPage.saveBarSave.click();
        await expect(settingsPage.saveBar).toHaveCount(0);
        expect(mock.puts.slice(before)).toEqual(['typesafe']);
    });

    test('an invalid setting blocks saving and the bar jumps to it', async ({ page, settingsPage }) => {
        const mock = await mockSettings(page);
        await settingsPage.open();
        await settingsPage.openAISettings('Failure analysis');
        await page.getByTestId('fa-max-analyses').fill('0');

        await expect(settingsPage.saveBarMessage).toContainText('Fix 1 setting before saving: Max analyses per run (Failure analysis)');
        await expect(settingsPage.saveBarSave).toBeDisabled();

        await page.getByTestId('ai-tab-providers').click();
        await page.getByTestId('ai-savebar-fix-fa.max_analyses_per_run').click();
        await expect(page.getByTestId('ai-tab-analysis')).toHaveAttribute('aria-selected', 'true');
        await expect(page.getByTestId('fa-max-analyses')).toBeFocused();

        await page.keyboard.press('Control+s');
        expect(mock.puts).toEqual([]);
    });

    test('Discard across two tabs asks, then reverts both', async ({ page, settingsPage }) => {
        const mock = await mockSettings(page);
        await settingsPage.open();
        await settingsPage.openAISettings('Prompts');
        const editor = page.getByTestId('template-editor');
        await editor.fill(EDITED);
        await page.getByTestId('ai-tab-limits').click();
        await page.getByTestId('coverage-essential_max_tokens').fill('2048');

        page.once('dialog', (d) => {
            expect(d.message()).toBe('Discard unsaved changes on 2 tabs?');
            return d.accept();
        });
        await settingsPage.saveBarDiscard.click();
        await expect(settingsPage.saveBar).toHaveCount(0);
        await expect(page.getByTestId('coverage-essential_max_tokens')).toHaveValue('4096');
        await page.getByTestId('ai-tab-prompts').click();
        await expect(editor).toHaveValue(INITIAL.template.content);
        expect(mock.puts).toEqual([]);
    });

    test('leaving for another Settings section asks first', async ({ page, settingsPage }) => {
        await mockSettings(page);
        await settingsPage.open();
        await settingsPage.openAISettings('Prompts');
        await page.getByTestId('template-editor').fill(EDITED);

        page.once('dialog', (d) => {
            expect(d.message()).toBe('Discard unsaved AI settings?');
            return d.dismiss();
        });
        await page.getByRole('button', { name: 'Jira', exact: true }).click();
        await expect(page.getByTestId('ai-settings')).toBeVisible();
        await expect(page.getByTestId('template-editor')).toHaveValue(EDITED);
    });

    test('a hash change to another section asks first and keeps the AI section', async ({ page, settingsPage }) => {
        await mockSettings(page);
        await settingsPage.open();
        await settingsPage.openAISettings('Prompts');
        await page.getByTestId('template-editor').fill(EDITED);

        page.once('dialog', (d) => d.dismiss());
        await page.evaluate(() => { window.location.hash = '#custom-fields'; });
        await expect(page).toHaveURL(/#ai-test-generation$/);
        await expect(page.getByTestId('ai-settings')).toBeVisible();
        await expect(page.getByTestId('template-editor')).toHaveValue(EDITED);
    });

    test('Ctrl+S saves, also with focus outside the page', async ({ page, settingsPage }) => {
        const mock = await mockSettings(page);
        await settingsPage.open();
        await settingsPage.openAISettings('Prompts');
        await page.getByTestId('template-editor').fill(EDITED);
        await page.keyboard.press('Control+s');
        await expect(settingsPage.saveBar).toHaveCount(0);
        expect(mock.puts).toEqual(['template']);

        await page.getByTestId('ai-tab-limits').click();
        await page.getByTestId('coverage-essential_max_tokens').fill('2048');
        await page.getByRole('button', { name: 'AI', exact: true }).click();
        await page.keyboard.press('Control+s');
        await expect(settingsPage.saveBar).toHaveCount(0);
        expect(mock.puts).toEqual(['template', 'coverage']);
    });

    test('saving both templates keeps both saved, whatever order the answers arrive in', async ({ page, settingsPage }) => {
        const mock = await mockSettings(page);
        // Both PUTs return the whole row; each answer carries the other template as it was before
        // the save, so whichever arrives last would reset the other one if taken whole.
        mock.staleOther = true;
        await settingsPage.open();
        await settingsPage.openAISettings('Prompts');
        const editor = page.getByTestId('template-editor');
        await editor.fill(EDITED);
        await page.getByTestId('template-kind-parent').click();
        await editor.fill('New parent {{COVERAGE}} {{TITLE}} {{CHILDREN}}');

        await settingsPage.saveBarSave.click();
        await expect(settingsPage.saveBar).toHaveCount(0);
        expect([...mock.puts].sort()).toEqual(['parent', 'template']);
        await expect(page.getByTestId('ai-tab-dirty-prompts')).toHaveCount(0);
        await expect(page.getByTestId('template-unsaved')).toHaveCount(0);
        await expect(editor).toHaveValue('New parent {{COVERAGE}} {{TITLE}} {{CHILDREN}}');
    });

    test('inputs are locked while saving', async ({ page, settingsPage }) => {
        const mock = await mockSettings(page);
        mock.delay = { typesafe: 1500 };
        await settingsPage.open();
        await settingsPage.openAISettings('Failure analysis');
        const timeout = page.getByTestId('typesafe-timeout');
        await timeout.fill('45');

        await settingsPage.saveBarSave.click();
        await expect(settingsPage.saveBarMessage).toHaveText('Saving…');
        await expect(timeout).toBeDisabled();
        await expect(page.getByTestId('fa-max-analyses')).toBeDisabled();
        await expect(settingsPage.saveBar).toHaveCount(0);
        await expect(timeout).toBeEnabled();
    });
});

const FAILURES = [
    { name: 'template', tab: 'Prompts', tabId: 'prompts', label: 'Standard prompt template', edit: (page) => page.getByTestId('template-editor').fill(EDITED) },
    { name: 'coverage', tab: 'Limits & budget', tabId: 'limits', label: 'Output tokens per coverage level', edit: (page) => page.getByTestId('coverage-essential_max_tokens').fill('2048') },
    { name: 'budgets', tab: 'Limits & budget', tabId: 'limits', label: 'Soft cost budgets', edit: (page) => page.getByTestId('budget-monthly').fill('9') },
    { name: 'typesafe', tab: 'Failure analysis', tabId: 'analysis', label: 'TypeSafe.ai', edit: (page) => page.getByTestId('typesafe-timeout').fill('45') },
    { name: 'fa', tab: 'Failure analysis', tabId: 'analysis', label: 'AI Failure Analysis', edit: (page) => page.getByTestId('fa-max-analyses').fill('30') },
];

test.describe('Settings — AI save bar, a failing endpoint', () => {
    for (const f of FAILURES) {
        test(`${f.label} stays unsaved when its save fails`, async ({ page, settingsPage }) => {
            const mock = await mockSettings(page);
            mock.fail = [f.name];
            await settingsPage.open();
            await settingsPage.openAISettings(f.tab);
            await f.edit(page);

            await settingsPage.saveBarSave.click();
            await expect(settingsPage.saveBarMessage).toHaveText(`Couldn't save ${f.label}: ${f.name} refused`);
            await expect(page.getByTestId(`ai-tab-dirty-${f.tabId}`)).toBeVisible();
            await expect(page.getByText('AI settings saved')).toHaveCount(0);
        });
    }
});
