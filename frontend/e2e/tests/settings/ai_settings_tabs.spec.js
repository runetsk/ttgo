import { test, expect } from '../../fixtures/test.js';

// Settings → AI split into tabs. Every settings endpoint is mocked before navigation and any
// write is refused and counted, so the spec never changes a real setting (safe on a scratch DB).

const TS = {
    id: 'singleton', enabled: true, api_key_masked: '…1234', api_key_status: 'ok', model: 'jev-1.13.0', timeout_seconds: 30,
    verdict_engine_enabled: true, narrative_enabled: true, llm_fallback_enabled: true, escalate_below_pct: 0,
    semantic_dedup_enabled: true, allow_auto_failure_analysis: false,
};
const FA = {
    id: 'singleton', enabled_on_completion: false, max_analyses_per_run: 20, parallel_groups: 4,
    dedup_enabled: true, redaction_enabled: true, prompt_template: 'Classify the failure.',
};
const PROVIDERS = [
    { id: 'p1', label: 'Fake LLM', provider_type: 'openai', model_name: 'fake-1', is_default: true, enabled: true, allow_auto_failure_analysis: false },
];
const TEMPLATE = {
    id: 'singleton',
    content: 'Custom {{COVERAGE}} {{TITLE}} {{DESCRIPTION}}', default_content: 'Default {{COVERAGE}} {{TITLE}} {{DESCRIPTION}}',
    parent_content: 'Parent {{COVERAGE}} {{TITLE}} {{CHILDREN}}', default_parent_content: 'Parent {{COVERAGE}} {{TITLE}} {{CHILDREN}}',
};
const COVERAGE = { essential_max_tokens: 4096, thorough_max_tokens: 8192, comprehensive_max_tokens: 16384 };
const BUDGETS = { id: 'singleton', per_request_usd: 0, monthly_usd: 5, month_spent_usd: 1.25 };

const json = (body, status = 200) => ({ status, contentType: 'application/json', body: JSON.stringify(body) });

async function mockSettings(page) {
    const writes = [];
    // Registered first, so the specific routes below win; it refuses any other settings write.
    await page.route(/\/api\/settings\//, (route) => {
        if (route.request().method() === 'GET') return route.fallback();
        writes.push(`${route.request().method()} ${route.request().url()}`);
        return route.fulfill(json({ error: 'this spec never saves' }, 500));
    });
    const serve = (body) => (route) => (route.request().method() === 'GET' ? route.fulfill(json(body)) : route.fallback());
    await page.route(/\/api\/settings\/typesafe$/, serve(TS));
    await page.route(/\/api\/settings\/ai-failure-analysis$/, serve(FA));
    await page.route(/\/api\/settings\/llm-providers$/, serve(PROVIDERS));
    await page.route(/\/api\/settings\/ai-features$/, serve({ enabled: true }));
    await page.route(/\/api\/settings\/ai-gen-template$/, serve(TEMPLATE));
    await page.route(/\/api\/settings\/ai-gen-coverage$/, serve(COVERAGE));
    await page.route(/\/api\/settings\/ai-budgets$/, serve(BUDGETS));
    return writes;
}

test.describe('Settings — AI tabs', () => {
    test('tiles summarize the setup and open their tab', async ({ page, settingsPage }) => {
        const writes = await mockSettings(page);
        await settingsPage.open();
        await settingsPage.openAISettings();

        await expect(page.getByTestId('ai-tile-model')).toContainText('Fake LLM');
        await expect(page.getByTestId('ai-tile-prompts')).toContainText('Standard customized');
        await expect(page.getByTestId('ai-tile-spend')).toContainText('$1.25 of $5.00');
        await expect(page.getByTestId('ai-panel-providers')).toBeVisible();

        await page.getByTestId('ai-tile-spend').click();
        await expect(page.getByTestId('ai-tab-limits')).toHaveAttribute('aria-selected', 'true');
        await expect(page.getByTestId('budget-meter')).toContainText('$1.25 estimated spend this month of $5.00');
        await expect(page.getByTestId('ai-panel-providers')).toBeHidden();
        expect(writes).toEqual([]);
    });

    test('arrow keys move between tabs', async ({ page, settingsPage }) => {
        await mockSettings(page);
        await settingsPage.open();
        await settingsPage.openAISettings();

        await page.getByTestId('ai-tab-providers').focus();
        await page.keyboard.press('ArrowRight');
        await expect(page.getByTestId('ai-tab-prompts')).toBeFocused();
        await expect(page.getByTestId('ai-panel-prompts')).toBeVisible();
        await page.keyboard.press('End');
        await expect(page.getByTestId('ai-tab-typesafe')).toHaveAttribute('aria-selected', 'true');
        await page.keyboard.press('ArrowRight');
        await expect(page.getByTestId('ai-tab-providers')).toHaveAttribute('aria-selected', 'true');
    });

    test('a diagram chip for a TypeSafe setting opens the TypeSafe.ai tab', async ({ page, settingsPage }) => {
        await mockSettings(page);
        await settingsPage.open();
        await settingsPage.openFailureAnalysis();
        await expect(page.getByTestId('typesafe-settings')).toBeHidden();

        await settingsPage.selectFlowStep('decide');
        await settingsPage.flowChip('ts.timeout_seconds').click();
        await expect(page.getByTestId('ai-tab-typesafe')).toHaveAttribute('aria-selected', 'true');
        await expect(page.getByTestId('typesafe-timeout')).toBeFocused();
    });

    test('the failure-analysis prompt stays folded until opened, and the save bar unfolds it', async ({ page, settingsPage }) => {
        const writes = await mockSettings(page);
        await settingsPage.open();
        await settingsPage.openFailureAnalysis();

        const prompt = page.getByTestId('fa-prompt');
        const editor = page.getByTestId('fa-prompt-editor');
        await expect(prompt).toContainText(`${FA.prompt_template.length} chars`);
        await expect(editor).toHaveCount(0);

        await page.getByTestId('fa-prompt-toggle').click();
        await expect(editor).toHaveValue(FA.prompt_template);
        await editor.fill('');
        await page.getByTestId('fa-prompt-toggle').click();
        await expect(editor).toHaveCount(0);
        await expect(prompt).toContainText('Unsaved changes');
        await expect(page.getByTestId('fa-prompt-error')).toBeVisible();

        await page.getByTestId('ai-tab-providers').click();
        await page.getByTestId('ai-savebar-fix-fa.prompt_template').click();
        await expect(page.getByTestId('ai-tab-analysis')).toHaveAttribute('aria-selected', 'true');
        await expect(editor).toBeVisible();
        await expect(editor).toHaveValue('');
        expect(writes).toEqual([]);
    });

    test('scrolled to the end, the last setting keeps a gap above the window edge', async ({ page, settingsPage }) => {
        await mockSettings(page);
        await page.setViewportSize({ width: 1280, height: 720 });
        await settingsPage.open();
        await settingsPage.openFailureAnalysis();
        await expect(page.getByTestId('fa-prompt')).toBeVisible();

        const gap = await page.evaluate(() => {
            const scroller = document.querySelector('.content-area');
            scroller.scrollTop = scroller.scrollHeight;
            return scroller.getBoundingClientRect().bottom - document.querySelector('[data-testid="fa-prompt"]').getBoundingClientRect().bottom;
        });
        expect(gap).toBeGreaterThanOrEqual(40);
    });

    test('a compact field explains itself from its ⓘ', async ({ page, settingsPage }) => {
        await mockSettings(page);
        await settingsPage.open();
        await settingsPage.openFailureAnalysis();

        const help = page.getByTestId('help-fa.max_analyses_per_run');
        await expect(help).toHaveAttribute('aria-expanded', 'false');
        await help.click();
        await expect(help).toHaveAttribute('aria-expanded', 'true');
        await expect(page.locator('[data-setting="fa.max_analyses_per_run"]')).toContainText('largest groups first');
    });

    test('an unsaved template survives a tab switch and marks its tab', async ({ page, settingsPage }) => {
        const writes = await mockSettings(page);
        await settingsPage.open();
        await settingsPage.openAISettings('Prompts');

        const editor = page.getByTestId('template-editor');
        await expect(editor).toHaveValue(TEMPLATE.content);
        await editor.fill('Edited {{COVERAGE}} {{TITLE}} {{DESCRIPTION}}');
        await expect(page.getByTestId('ai-tab-dirty-prompts')).toBeVisible();
        await expect(page.getByTestId('ai-tile-prompts')).toContainText('Unsaved changes');

        await page.getByTestId('ai-tab-providers').click();
        await page.getByTestId('ai-tab-prompts').click();
        await expect(editor).toHaveValue('Edited {{COVERAGE}} {{TITLE}} {{DESCRIPTION}}');

        await page.getByTestId('template-kind-parent').click();
        await expect(editor).toHaveValue(TEMPLATE.parent_content);
        await page.getByTestId('template-kind-standard').click();
        await expect(editor).toHaveValue('Edited {{COVERAGE}} {{TITLE}} {{DESCRIPTION}}');
        expect(writes).toEqual([]);
    });

    test('clicking a placeholder inserts it at the cursor', async ({ page, settingsPage }) => {
        await mockSettings(page);
        await settingsPage.open();
        await settingsPage.openAISettings('Prompts');

        const editor = page.getByTestId('template-editor');
        await editor.fill('A B');
        await editor.evaluate((el) => el.setSelectionRange(2, 2));
        await page.getByTestId('template-placeholder-TITLE').click();
        await expect(editor).toHaveValue('A {{TITLE}}B');
        await expect(page.getByTestId('ai-panel-prompts').getByRole('alert')).toContainText('{{COVERAGE}}, {{DESCRIPTION}}');
    });

    test('a diagram chip for a provider setting opens the Providers tab', async ({ page, settingsPage }) => {
        await mockSettings(page);
        await settingsPage.open();
        await settingsPage.openFailureAnalysis();

        await settingsPage.flowChip('provider.default').click();
        await expect(page.getByTestId('ai-tab-providers')).toHaveAttribute('aria-selected', 'true');
        await expect(page.locator('[data-setting="provider.default"]')).toBeFocused();
    });
});
