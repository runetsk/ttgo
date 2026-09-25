import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

// The failure-analysis states a real engine cannot be made to produce on demand: a failed
// attempt, a TypeSafe decision stored without an explanation, and a finished job with failures.
// The analysis endpoints are mocked with page.route, so no LLM or TypeSafe call is made and no
// AI setting is written. Only the run and its two failing results are real. The settings-page
// process diagram has its own spec, failure_analysis_diagram.spec.js.

const FAILED = {
    id: 'an-failed', version: 1, engine: 'typesafe', model_name: 'jev-1.13.0', verdict: 'unknown',
    confidence: 'low', decision_status: 'failed', error_category: 'timeout', narrative_status: 'unavailable',
    summary: 'analysis failed: TypeSafe.ai unavailable and the LLM fallback is off: context deadline exceeded',
    next_action: '', rationale: '', created_at: '2026-09-23T10:00:00Z',
};

const SKIPPED = {
    id: 'an-skipped', version: 1, engine: 'typesafe', model_name: 'jev-1.13.0', verdict: 'flaky_test',
    confidence: 'high', confidence_score: 0.93, suggested_defect_type: 'automation_bug',
    suggested_defect_type_confidence: 0.9, decision_status: 'ok', narrative_status: 'skipped',
    summary: "No explanation: explanations are switched off in the TypeSafe.ai settings; the classification above is TypeSafe's decision.",
    next_action: '', rationale: '', created_at: '2026-09-23T10:00:00Z',
};

const EXPLAINED = {
    ...SKIPPED, narrative_status: 'ok', summary: 'The checkout button is clicked before it is enabled.',
    next_action: 'Wait for the button to be enabled before clicking.', rationale: 'The timeout names the button.',
};

const ENDED_JOB = {
    id: 'job-ended', status: 'completed', trigger: 'manual', total_failures: 2, unique_groups: 2,
    analyzed_count: 2, capped_at: 2, retry_failed_only: false, pipeline_label: 'TypeSafe jev-1.13.0, no LLM',
    outcomes: { groups: 2, decided: 1, failed: 1, unknown: 0, no_explanation: 0, explanation_skipped: 1, taken_over: 0, failed_rows: 1 },
};

const json = (body, status = 200) => ({ status, contentType: 'application/json', body: JSON.stringify(body) });

test.describe('AI failure analysis — failed attempts, explanations and job outcomes', () => {
    let seed, failedRow, skippedRow;

    test.beforeEach(async ({ page, api, runDetailPage }) => {
        seed = await api.seedRunWithResults({ statuses: ['FAIL', 'FAIL'], label: 'AI Outcomes' });
        [failedRow, skippedRow] = seed.rows;
        const byResult = {
            [failedRow.result.id]: [{ ...FAILED, run_result_id: failedRow.result.id }],
            [skippedRow.result.id]: [{ ...SKIPPED, run_result_id: skippedRow.result.id }],
        };
        await page.route(/\/api\/run-results\/[^/]+\/analyses$/, (route) => {
            const id = route.request().url().split('/run-results/')[1].split('/')[0];
            return route.fulfill(json(byResult[id] || []));
        });
        await page.route(/\/api\/runs\/[^/]+\/analyses\/current$/, (route) => route.fulfill(json({
            [failedRow.result.id]: byResult[failedRow.result.id][0],
            [skippedRow.result.id]: byResult[skippedRow.result.id][0],
        })));
        await page.route(/\/api\/runs\/[^/]+\/analysis-job$/, (route) => route.fulfill(json({ ...ENDED_JOB, test_run_id: seed.run.id })));
        await runDetailPage.pinColumns({ ai_verdict: true });
    });

    test('a failed attempt reads as failed, never as an Unknown verdict', async ({ runDetailPage }) => {
        await runDetailPage.open(seed.run.id);
        const row = runDetailPage.resultRow(failedRow.tc.name);
        await expect(row.getByTestId('ai-verdict-failed')).toHaveText('Analysis failed', { timeout: TIMEOUTS.HEAVY_GRID });

        await runDetailPage.expandResultRow(failedRow.tc.name);
        await runDetailPage.page.getByTestId('result-tab-ai').click();
        await expect(runDetailPage.page.getByTestId('analysis-failed-heading')).toContainText('Analysis failed · timed out');
        await expect(runDetailPage.resultDetail).toContainText('context deadline exceeded');
        await expect(runDetailPage.page.getByTestId('analysis-narrative-notice')).toHaveCount(0);
    });

    test('Explain fills in the explanation of a stored TypeSafe decision', async ({ page, runDetailPage }) => {
        let explained = 0;
        await page.route(/\/api\/run-results\/[^/]+\/analyses\/an-skipped\/explain$/, (route) => {
            explained++;
            return route.fulfill(json({ ...EXPLAINED, run_result_id: skippedRow.result.id }));
        });
        await runDetailPage.open(seed.run.id);
        await expect(runDetailPage.resultRow(skippedRow.tc.name)).toBeVisible({ timeout: TIMEOUTS.HEAVY_GRID });
        await runDetailPage.expandResultRow(skippedRow.tc.name);
        await page.getByTestId('result-tab-ai').click();

        const notice = page.getByTestId('analysis-narrative-notice');
        await expect(notice).toContainText('No explanation was written');
        // Compact: no empty Summary / Next-action boxes around the notice.
        await expect(runDetailPage.resultDetail).not.toContainText('Suggested next action');

        await page.getByTestId('analysis-explain').click();
        await expect(runDetailPage.resultDetail).toContainText('The checkout button is clicked before it is enabled.');
        await expect(runDetailPage.resultDetail).toContainText('Wait for the button to be enabled before clicking.');
        await expect(notice).toHaveCount(0);
        expect(explained).toBe(1);
    });

    test('the banner reports an ended job and retries its failed groups', async ({ page, runDetailPage }) => {
        let retried = 0;
        await page.route(/\/api\/runs\/[^/]+\/analysis-job\/retry-failed$/, (route) => {
            retried++;
            return route.fulfill(json({ ...ENDED_JOB, id: 'job-retry', status: 'queued', retry_failed_only: true, analyzed_count: 0, capped_at: 1, outcomes: undefined }, 201));
        });
        await runDetailPage.open(seed.run.id);

        const summary = page.getByTestId('run-analysis-summary');
        await expect(summary).toContainText('AI analysis finished: 1 group decided, 1 failed, 1 with explanations off. 1 failed result has no decision yet.', { timeout: TIMEOUTS.HEAVY_GRID });
        await expect(page.getByTestId('run-analysis-pipeline')).toHaveText('TypeSafe jev-1.13.0, no LLM');

        await page.getByTestId('run-analysis-retry-failed').click();
        await expect(page.getByText('AI retrying failed groups — 0 of 1 groups')).toBeVisible();
        await expect(summary).toHaveCount(0);
        expect(retried).toBe(1);
    });
});
