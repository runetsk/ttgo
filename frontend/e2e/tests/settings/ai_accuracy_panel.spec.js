import { test, expect } from '../../fixtures/test.js';

// The suggestion-accuracy panel on Settings → AI → Failure analysis. Every endpoint the tab reads
// is mocked and installed before navigation, and any write is refused and counted, so the spec
// never changes a real setting and is safe against a scratch backend holding live keys.

const TS = {
    id: 'singleton', enabled: true, api_key_masked: '…1234', api_key_status: 'ok', model: 'jev-1.13.0', timeout_seconds: 30,
    verdict_engine_enabled: true, narrative_enabled: true, llm_fallback_enabled: true, escalate_below_pct: 0,
    semantic_dedup_enabled: true, allow_auto_failure_analysis: false, price_per_mtok: 0.042,
};
const FA = {
    id: 'singleton', enabled_on_completion: false, max_analyses_per_run: 20, parallel_groups: 4,
    dedup_enabled: true, redaction_enabled: true, prompt_template: 'Classify the failure.',
};
const PROVIDERS = [
    { id: 'p1', label: 'Fake LLM', provider_type: 'openai', model_name: 'fake-1', is_default: true, enabled: true, allow_auto_failure_analysis: false },
];
const VERSIONS = ['fa-verdict-v5', 'fa-verdict-v4'];

// All versions: 6 triaged, 4 direct (3 kept) and 2 clones (1 kept). The headline must be the
// direct 75%, not the overall 67%.
const REPORT_ALL = {
    total: 6, agreed: 4, agreement_rate: 4 / 6,
    direct_total: 4, direct_agreed: 3, direct_rate: 0.75, clone_total: 2, clone_agreed: 1,
    unknown_provenance: 0, policy_version: '', policy_versions: VERSIONS,
    by_verdict: [{ verdict: 'flaky_test', total: 6, agreed: 4, rate: 4 / 6 }],
    by_confidence: [{ confidence: 'high', total: 6, agreed: 4, rate: 4 / 6 }],
    by_engine: [{
        engine: 'typesafe', total: 6, agreed: 4, rate: 4 / 6,
        direct_total: 4, direct_agreed: 3, direct_rate: 0.75, clone_total: 2, clone_agreed: 1,
        by_confidence: [{ confidence: 'high', total: 6, agreed: 4, rate: 4 / 6 }],
    }],
    coverage: [{ engine: 'typesafe', analyses: 5, decided: 4, abstained: 1, failed: 0, no_explanation: 2 }],
};
const REPORT_V4 = {
    total: 2, agreed: 1, agreement_rate: 0.5,
    direct_total: 2, direct_agreed: 1, direct_rate: 0.5, clone_total: 0, clone_agreed: 0,
    unknown_provenance: 0, policy_version: 'fa-verdict-v4', policy_versions: VERSIONS,
    by_verdict: [{ verdict: 'environment', total: 2, agreed: 1, rate: 0.5 }],
    by_confidence: [{ confidence: 'medium', total: 2, agreed: 1, rate: 0.5 }],
    by_engine: [{
        engine: 'typesafe', total: 2, agreed: 1, rate: 0.5,
        direct_total: 2, direct_agreed: 1, direct_rate: 0.5, clone_total: 0, clone_agreed: 0,
        by_confidence: [{ confidence: 'medium', total: 2, agreed: 1, rate: 0.5 }],
    }],
    coverage: [{ engine: 'typesafe', analyses: 2, decided: 2, abstained: 0, failed: 0, no_explanation: 0 }],
};

const json = (body, status = 200) => ({ status, contentType: 'application/json', body: JSON.stringify(body) });

async function mockTab(page) {
    const writes = [];
    const accuracyQueries = [];
    const serve = (name, body) => (route) => {
        if (route.request().method() !== 'GET') {
            writes.push(`${route.request().method()} ${name}`);
            return route.fulfill(json({ error: 'the accuracy spec never saves' }, 500));
        }
        return route.fulfill(json(body));
    };
    await page.route(/\/api\/settings\/typesafe$/, serve('typesafe', TS));
    await page.route(/\/api\/settings\/ai-failure-analysis$/, serve('ai-failure-analysis', FA));
    await page.route(/\/api\/settings\/llm-providers$/, serve('llm-providers', PROVIDERS));
    await page.route(/\/api\/settings\/ai-features$/, serve('ai-features', { enabled: true }));
    await page.route(/\/api\/ai\/failure-analysis\/accuracy(\?.*)?$/, (route) => {
        const version = new URL(route.request().url()).searchParams.get('policy_version') || '';
        accuracyQueries.push(version);
        return route.fulfill(json(version === 'fa-verdict-v4' ? REPORT_V4 : REPORT_ALL));
    });
    return { writes, accuracyQueries };
}

test.describe('Settings — failure-analysis accuracy panel', () => {
    test('labels the headline Human agreement and filters by policy version', async ({ page, settingsPage }) => {
        const { writes, accuracyQueries } = await mockTab(page);
        await settingsPage.open();
        await settingsPage.openAccuracyPanel();

        await expect(settingsPage.accuracyHeadlineLabel).toHaveText('Human agreement');
        await expect(settingsPage.accuracyHeadlineLabel).toHaveAttribute('title', /Accepting a suggestion counts as agreement/);
        await expect(settingsPage.accuracyHeadlineRate).toHaveText('75%');
        await expect(settingsPage.accuracyEngine('typesafe')).toContainText('direct 75% of 4 · clones 50% of 2');
        await expect(settingsPage.accuracyEngine('typesafe')).toContainText('4 decided of 5 · 1 abstained · 2 without explanation');

        await expect(settingsPage.accuracyPolicyFilter).toHaveValue('');
        await expect(settingsPage.accuracyPolicyFilter.locator('option')).toHaveText(['All versions', 'fa-verdict-v5', 'fa-verdict-v4']);

        await settingsPage.accuracyPolicyFilter.selectOption('fa-verdict-v4');
        await expect(settingsPage.accuracyHeadlineRate).toHaveText('50%');
        await expect(settingsPage.accuracyPolicyFilter).toHaveValue('fa-verdict-v4');
        await expect(settingsPage.accuracyEngine('typesafe')).toContainText('2 decided of 2');

        expect(accuracyQueries[0]).toBe('');
        expect(accuracyQueries).toContain('fa-verdict-v4');
        expect(writes).toEqual([]);
    });
});
