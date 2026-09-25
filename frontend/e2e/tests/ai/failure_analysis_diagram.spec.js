import { test, expect } from '../../fixtures/test.js';

// The failure-analysis process diagram on Settings → AI Generation. Every settings endpoint it
// reads is mocked and installed before navigation, and any write is refused and counted, so the
// spec never changes a real setting and is safe against a scratch backend holding live keys.

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

const json = (body, status = 200) => ({ status, contentType: 'application/json', body: JSON.stringify(body) });

// mockSettings serves the four settings the diagram reads; `fail` names endpoints that answer 500.
// It returns the list of refused writes, which every test expects to stay empty.
async function mockSettings(page, { fail = [] } = {}) {
    const writes = [];
    const serve = (name, body) => (route) => {
        if (route.request().method() !== 'GET') {
            writes.push(`${route.request().method()} ${name}`);
            return route.fulfill(json({ error: 'the diagram spec never saves' }, 500));
        }
        return route.fulfill(fail.includes(name) ? json({ error: 'unavailable' }, 500) : json(body));
    };
    await page.route(/\/api\/settings\/typesafe$/, serve('typesafe', TS));
    await page.route(/\/api\/settings\/ai-failure-analysis$/, serve('ai-failure-analysis', FA));
    await page.route(/\/api\/settings\/llm-providers$/, serve('llm-providers', PROVIDERS));
    await page.route(/\/api\/settings\/ai-features$/, serve('ai-features', { enabled: true }));
    return writes;
}

test.describe('Settings — failure-analysis process diagram', () => {
    test('follows the unsaved explanation switch', async ({ page, settingsPage }) => {
        const writes = await mockSettings(page);
        await settingsPage.open();
        await settingsPage.openFailureAnalysis();
        await settingsPage.selectFlowStep('explain');

        const explained = settingsPage.flowPart('explain', 'answers');
        await expect(explained).toContainText('Fake LLM (fake-1) writes the explanation.');
        await expect(page.getByTestId('analysis-flow-unsaved')).toHaveCount(0);

        await page.getByTestId('typesafe-narrative_enabled').uncheck();
        await expect(explained).toHaveAttribute('data-status', 'skip');
        await expect(explained).toContainText('No explanation; Explain on a result writes one on demand.');
        await expect(page.getByTestId('analysis-flow-unsaved')).toBeVisible();
        expect(writes).toEqual([]);
    });

    test('by run completion shows the automatic chips and what blocks it', async ({ page, settingsPage }) => {
        const writes = await mockSettings(page);
        await settingsPage.open();
        await settingsPage.openFailureAnalysis();

        await settingsPage.flowTrigger('auto').click();
        await expect(settingsPage.flowTrigger('auto')).toHaveAttribute('aria-pressed', 'true');
        const start = settingsPage.flowStep('start');
        await expect(start).toHaveAttribute('data-status', 'blocked');
        await expect(settingsPage.flowDetail).toContainText('"Auto-analyze on run completion" is off');
        await expect(settingsPage.flowChip('ts.allow_auto_failure_analysis')).toContainText('Not allowed');

        await page.getByTestId('fa-enabled_on_completion').check();
        await expect(settingsPage.flowDetail).toContainText('Nothing can analyze: the default LLM provider is not approved for automatic analysis, and TypeSafe.ai is not allowed on automatic analysis.');

        await page.getByTestId('typesafe-allow_auto_failure_analysis').check();
        await expect(start).toHaveAttribute('data-status', 'run');
        await settingsPage.selectFlowStep('group');
        await expect(settingsPage.flowPart('group', 'semantic')).toHaveAttribute('data-status', 'run');
        expect(writes).toEqual([]);
    });

    test('removing the stored key sends every decision down the unavailable path', async ({ page, settingsPage }) => {
        const writes = await mockSettings(page);
        await settingsPage.open();
        await settingsPage.openFailureAnalysis();

        await page.getByTestId('typesafe-clear-key').check();
        await expect(settingsPage.flowStep('decide')).toHaveAttribute('data-status', 'warn');
        await expect(settingsPage.flowPart('decide', 'answers')).toHaveAttribute('data-status', 'skip');
        await expect(settingsPage.flowPart('decide', 'unavailable')).toContainText('Fake LLM (fake-1) decides and explains');
        await expect(settingsPage.flowChip('ts.api_key')).toContainText('Being removed');
        expect(writes).toEqual([]);
    });

    test('an invalid cap keeps the saved route and says so', async ({ page, settingsPage }) => {
        const writes = await mockSettings(page);
        await settingsPage.open();
        await settingsPage.openFailureAnalysis();
        await settingsPage.selectFlowStep('group');

        await page.getByTestId('fa-max-analyses').fill('0');
        await expect(settingsPage.flowChip('fa.max_analyses_per_run')).toContainText('invalid');
        await expect(settingsPage.flowPart('group', 'cap')).toContainText('Up to 20 groups, largest first');
        await expect(page.getByTestId('fa-save')).toBeDisabled();
        expect(writes).toEqual([]);
    });

    test('a failed settings load shows an error instead of a route', async ({ page, settingsPage }) => {
        await mockSettings(page, { fail: ['llm-providers'] });
        await settingsPage.open();
        await settingsPage.openFailureAnalysis();

        await expect(page.getByTestId('analysis-flow-error')).toContainText("Couldn't load the LLM providers");
        await expect(settingsPage.flowStep('start')).toHaveCount(0);
    });

    test('a chip jumps to its setting', async ({ page, settingsPage }) => {
        await mockSettings(page);
        await settingsPage.open();
        await settingsPage.openFailureAnalysis();
        await settingsPage.selectFlowStep('group');

        await settingsPage.flowChip('fa.dedup_enabled').click();
        const toggle = page.getByTestId('fa-dedup_enabled');
        await expect(toggle).toBeFocused();
        await expect(toggle).toBeInViewport();
    });

    test('the hint opens without flipping the setting', async ({ page, settingsPage }) => {
        await mockSettings(page);
        await settingsPage.open();
        await settingsPage.openFailureAnalysis();

        const toggle = page.getByTestId('fa-dedup_enabled');
        await expect(toggle).toBeChecked();
        const help = page.getByTestId('help-fa.dedup_enabled');
        await help.click();
        await expect(help).toHaveAttribute('aria-expanded', 'true');
        await expect(page.locator('[data-setting="fa.dedup_enabled"]')).toContainText('only the order number differs');
        await expect(toggle).toBeChecked();
    });

    test('a non-admin reads the diagram and jumps to a read-only setting', async ({ page, settingsPage }) => {
        await mockSettings(page);
        // Show the page as a member would see it; the session stays the admin's.
        await page.route(/\/api\/auth\/me$/, async (route) => {
            const response = await route.fetch();
            const body = await response.json();
            return route.fulfill({ response, json: { ...body, user: { ...body.user, role: 'member' } } });
        });
        await settingsPage.open();
        await settingsPage.openFailureAnalysis();

        await expect(page.getByTestId('typesafe-save')).toHaveCount(0);
        await expect(settingsPage.flowStep('decide')).toHaveAttribute('data-status', 'run');
        await settingsPage.selectFlowStep('group');
        await settingsPage.flowChip('fa.dedup_enabled').click();
        await expect(page.locator('[data-setting="fa.dedup_enabled"]')).toBeFocused();
    });
});
