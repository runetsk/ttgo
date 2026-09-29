import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

// Wave 4 on the run page: a semantically grouped result's AI card opens a panel that says why it
// was merged (probability, asked or remembered) and splits it out of its group, which queues an
// analysis of those results alone; the job banner shows the semantic-grouping line and names a
// split job as such. Analyses, jobs and the semantic endpoints are mocked with page.route: no
// TypeSafe or LLM call is made and nothing is written.

const json = (body, status = 200) => ({ status, contentType: 'application/json', body: JSON.stringify(body) });

const CLONE = {
    id: 'an-sem', version: 1, engine: 'typesafe', model_name: 'jev-1.13.0', confidence: 'high', confidence_score: 0.93,
    decision_status: 'ok', narrative_status: 'ok', narrative_revision: 1, policy_version: 'fa-verdict-v8',
    created_at: '2026-09-29T10:00:00Z', job_id: 'job-w4', verdict: 'infrastructure', suggested_defect_type: 'system_issue',
    suggested_defect_type_confidence: 0.9, summary: 'The payment gateway answered 503.', next_action: 'Retry once the gateway is back.',
    rationale: 'r', source_analysis_id: 'an-rep', dedup_group_key: 'sig-rep', dedup_method: 'semantic', dedup_p_same: 0.86,
    dedup_model: 'jev-1.13.0', signals: '', narrative_fit: 0.9, narrative_split: false,
};

const VIEW = {
    representative: { result_id: 'rr-rep', test_name: 'Payments — authorize a card', failure_type: 'http',
        error_message: 'POST /api/v1/payments/authorize returned 503 Service Unavailable: upstream sandbox-gateway.pay.example circuit open' },
    result: { result_id: 'x', test_name: 'Payments — capture', failure_type: 'http',
        error_message: 'Payment capture failed: sandbox-gateway.pay.example unavailable (HTTP 503), circuit breaker open' },
    pair: { p_same: 0.86, source: 'memory', model: 'jev-latest', answered_model: 'jev-1.13.0', policy_version: 'fa-semantic-v1',
        created_at: '2026-09-29T10:00:00Z', remembered_from: '2026-09-20T10:00:00Z' },
    group_size: 9, split_group_size: 3, other_groups: 1, can_split: true, split_blocked_reason: '',
};

const ENDED = {
    id: 'job-w4', status: 'completed', trigger: 'manual', total_failures: 12, unique_groups: 4, analyzed_count: 4, capped_at: 4,
    retry_failed_only: false, pipeline_label: 'TypeSafe jev-latest, explained by Fake LLM',
    semantic_report: '{"blocks":2,"candidates":12,"asked":6,"remembered":5,"human_blocked":1,"merged":3,"skipped":0,"requests":1}',
    outcomes: { groups: 4, decided: 4, failed: 0, failed_rows: 0 },
};

async function mockRun(page, resultId, { view = VIEW, job = ENDED, afterSplitJob = null } = {}) {
    const calls = { splits: [], jobGets: 0 };
    let current = job;
    await page.route(/\/api\/run-results\/[^/]+\/analyses$/, (route) => {
        const id = route.request().url().split('/run-results/')[1].split('/')[0];
        return route.fulfill(json(id === resultId ? [{ ...CLONE, run_result_id: resultId }] : []));
    });
    await page.route(/\/api\/runs\/[^/]+\/analyses\/current$/, (route) => route.fulfill(json({ [resultId]: { ...CLONE, run_result_id: resultId } })));
    await page.route(/\/api\/runs\/[^/]+\/analysis-job$/, (route) => {
        calls.jobGets++;
        return route.fulfill(json(current));
    });
    await page.route(/\/api\/run-results\/[^/]+\/analyses\/[^/]+\/semantic$/, (route) => route.fulfill(json(view)));
    await page.route(/\/api\/run-results\/[^/]+\/analyses\/[^/]+\/split(\?.*)?$/, (route) => {
        calls.splits.push(route.request().url());
        if (afterSplitJob) current = afterSplitJob;
        return route.fulfill(json(afterSplitJob || { id: 'job-split' }, 201));
    });
    return calls;
}

test.describe('AI failure analysis — inspect and split a semantic merge', () => {
    test('the grouping note opens the merge panel, and a confirmed split queues its analysis', async ({ page, api, runDetailPage }) => {
        const seed = await api.seedRunWithResults({ statuses: ['FAIL'], label: 'AI Semantic Split' });
        const [row] = seed.rows;
        const calls = await mockRun(page, row.result.id, {
            job: { ...ENDED, test_run_id: seed.run.id },
            afterSplitJob: { id: 'job-split', status: 'running', trigger: 'manual', test_run_id: seed.run.id, split_from_analysis_id: 'an-sem',
                total_failures: 3, unique_groups: 1, analyzed_count: 0, capped_at: 1, outcomes: {} },
        });
        await runDetailPage.pinColumns({ ai_verdict: true });
        await runDetailPage.open(seed.run.id);
        await expect(runDetailPage.resultRow(row.tc.name)).toBeVisible({ timeout: TIMEOUTS.HEAVY_GRID });
        await runDetailPage.expandResultRow(row.tc.name);
        await page.getByTestId('result-tab-ai').click();

        const note = page.getByTestId('analysis-grouping-note');
        await expect(note).toContainText('Grouped semantically (p = 0.86)');
        await expect(page.getByTestId('analysis-semantic-panel')).toHaveCount(0);
        await note.click();
        const panel = page.getByTestId('analysis-semantic-panel');
        await expect(panel).toContainText('Payments — authorize a card');
        await expect(page.getByTestId('analysis-semantic-representative')).toContainText('circuit open');
        await expect(page.getByTestId('analysis-semantic-pair')).toContainText('TypeSafe.ai is 86% sure these failures share a cause.');
        await expect(page.getByTestId('analysis-semantic-pair')).toContainText('Remembered from an analysis on');
        await expect(panel).toContainText('Splitting takes out 3 results with this error');

        const dialogs = [];
        page.once('dialog', (d) => { dialogs.push(d.message()); return d.dismiss(); });
        await page.getByTestId('analysis-semantic-split').click();
        await expect.poll(() => dialogs.length).toBe(1);
        expect(dialogs[0]).toContain('Split 3 results from this group');
        expect(calls.splits).toEqual([]);

        const before = calls.jobGets;
        page.once('dialog', (d) => d.accept());
        await page.getByTestId('analysis-semantic-split').click();
        await expect(page.getByTestId('analysis-semantic-split-done')).toContainText('Split from its group');
        expect(calls.splits).toHaveLength(1);
        expect(calls.splits[0]).toMatch(/\/analyses\/an-sem\/split$/);
        await expect.poll(() => calls.jobGets).toBeGreaterThan(before); // the banner fetched the new job
    });

    test('a split that is not possible now says why', async ({ page, api, runDetailPage }) => {
        const seed = await api.seedRunWithResults({ statuses: ['FAIL'], label: 'AI Semantic Blocked' });
        const [row] = seed.rows;
        await mockRun(page, row.result.id, {
            job: { ...ENDED, test_run_id: seed.run.id },
            view: { ...VIEW, pair: { ...VIEW.pair, source: 'typesafe', remembered_from: null }, can_split: false,
                split_blocked_reason: 'an analysis is running on this run' },
        });
        await runDetailPage.open(seed.run.id);
        await expect(runDetailPage.resultRow(row.tc.name)).toBeVisible({ timeout: TIMEOUTS.HEAVY_GRID });
        await runDetailPage.expandResultRow(row.tc.name);
        await page.getByTestId('result-tab-ai').click();
        await page.getByTestId('analysis-grouping-note').click();
        await expect(page.getByTestId('analysis-semantic-pair')).toContainText('Decided in this analysis.');
        await expect(page.getByTestId('analysis-semantic-split')).toBeDisabled();
        await expect(page.getByTestId('analysis-semantic-split-blocked')).toHaveText('an analysis is running on this run');
    });

    test('the banner shows the semantic grouping line', async ({ page, api, runDetailPage }) => {
        const seed = await api.seedRunWithResults({ statuses: ['FAIL'], label: 'AI Semantic Banner' });
        // An ended job this page did not watch stays up only when it needs a look: auto-apply paused.
        await mockRun(page, seed.rows[0].result.id, { job: { ...ENDED, test_run_id: seed.run.id,
            outcomes: { ...ENDED.outcomes, auto_apply_state: 'paused' } } });
        await runDetailPage.open(seed.run.id);
        await expect(page.getByTestId('run-analysis-semantic'))
            .toHaveText('Semantic grouping: 3 merges from 12 pairs (5 remembered, 1 kept apart by a person)', { timeout: TIMEOUTS.HEAVY_GRID });
    });

    test('a finished split job is named as such in the banner', async ({ page, api, runDetailPage }) => {
        const seed = await api.seedRunWithResults({ statuses: ['FAIL'], label: 'AI Semantic Split Done' });
        await mockRun(page, seed.rows[0].result.id, {
            job: { ...ENDED, test_run_id: seed.run.id, id: 'job-split', split_from_analysis_id: 'an-sem', semantic_report: '',
                outcomes: { groups: 1, decided: 1, failed: 0, failed_rows: 0, auto_apply_state: 'paused' } },
        });
        await runDetailPage.open(seed.run.id);
        await expect(page.getByTestId('run-analysis-summary')).toContainText('Split re-analysis finished: 1 group decided.', { timeout: TIMEOUTS.HEAVY_GRID });
        await expect(page.getByTestId('run-analysis-semantic')).toHaveCount(0);
    });
});
