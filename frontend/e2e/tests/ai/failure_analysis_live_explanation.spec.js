import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

// A TypeSafe decision is shown before its explanation is written: the card and the grid tooltip
// read "Explanation being written…", and the explanation fills in live when the server broadcasts
// `run_result_analysis.updated`. The analysis endpoints are mocked with page.route and the app's
// WebSocket with page.routeWebSocket (never connected to the server), so no LLM or TypeSafe call is
// made and no AI setting is written. Only the run and its failing result are real.

const PENDING = {
    id: 'an-live-1', version: 1, engine: 'typesafe', model_name: 'jev-1.13.0', verdict: 'flaky_test',
    confidence: 'high', confidence_score: 0.93, suggested_defect_type: 'automation_bug',
    suggested_defect_type_confidence: 0.9, suggestion_source: 'question', decision_status: 'ok',
    narrative_status: 'pending', narrative_revision: 0, summary: '', next_action: '', rationale: '',
    policy_version: 'fa-verdict-v6', history_available: true, source_analysis_id: null,
    created_at: '2026-09-28T10:00:00Z', job_id: 'job-live',
};

const RUNNING_JOB = {
    id: 'job-live', status: 'running', trigger: 'manual', total_failures: 1, unique_groups: 1,
    analyzed_count: 1, capped_at: 1, retry_failed_only: false, pipeline_label: 'TypeSafe jev-1.13.0, explained by Fake LLM',
    outcomes: { groups: 1, decided: 1, explanation_pending: 1 },
};

const json = (body, status = 200) => ({ status, contentType: 'application/json', body: JSON.stringify(body) });

// wsEvent is what the server's shared analysis payload builder broadcasts for a row.
const wsEvent = (type, topic, row) => JSON.stringify({
    type, topic, timestamp: new Date().toISOString(),
    data: { ...row, analysis_id: row.id },
});

test.describe('AI failure analysis — explanation written after the decision', () => {
    test('a pending card fills in live, and a late pending event does not undo it', async ({ page, api, runDetailPage }) => {
        const seed = await api.seedRunWithResults({ statuses: ['FAIL'], label: 'AI Live Explanation' });
        const [row] = seed.rows;
        const resultId = row.result.id;
        const pending = { ...PENDING, run_result_id: resultId };

        await page.route(/\/api\/run-results\/[^/]+\/analyses$/, (route) => route.fulfill(json([pending])));
        await page.route(/\/api\/runs\/[^/]+\/analyses\/current$/, (route) => route.fulfill(json({ [resultId]: pending })));
        await page.route(/\/api\/runs\/[^/]+\/analysis-job$/, (route) => route.fulfill(json({ ...RUNNING_JOB, test_run_id: seed.run.id })));

        // The app's one WebSocket is answered here instead of by the server.
        const topics = new Set();
        let socket = null;
        await page.routeWebSocket(/\/api\/ws$/, (ws) => {
            socket = ws;
            ws.onMessage((message) => {
                const msg = JSON.parse(String(message));
                if (msg.action === 'subscribe') topics.add(msg.topic);
                if (msg.action === 'unsubscribe') topics.delete(msg.topic);
            });
        });
        await runDetailPage.pinColumns({ ai_verdict: true });
        await runDetailPage.open(seed.run.id);

        // The decision is on screen while its explanation is still being written.
        const gridRow = runDetailPage.resultRow(row.tc.name);
        await expect(gridRow.getByTitle(/Explanation being written…/)).toBeVisible({ timeout: TIMEOUTS.HEAVY_GRID });
        await expect(page.getByTestId('run-analysis-explanations-pending')).toContainText('1 explanation in progress');

        await runDetailPage.expandResultRow(row.tc.name);
        await page.getByTestId('result-tab-ai').click();
        const notice = page.getByTestId('analysis-narrative-notice');
        await expect(notice).toHaveText('Explanation being written…');
        await expect(notice).toHaveAttribute('data-status', 'pending');
        await expect(page.getByTestId('analysis-explain')).toHaveCount(0);
        await expect(page.getByTestId('analysis-narrative-reason')).toHaveCount(0);
        await expect(runDetailPage.resultDetail).not.toContainText('Suggested next action');

        // The open AI tab follows its result; the grid follows the run.
        await expect.poll(() => topics.has(`run_result:${resultId}`)).toBe(true);
        await expect.poll(() => topics.has(`run:${seed.run.id}`)).toBe(true);
        const written = {
            ...pending, narrative_status: 'ok', narrative_revision: 1,
            summary: 'The checkout button is clicked before it is enabled.',
            next_action: 'Wait for the button to be enabled before clicking.',
            rationale: 'The timeout names the button.',
        };
        socket.send(wsEvent('run_result_analysis.updated', `run_result:${resultId}`, written));
        socket.send(wsEvent('run_result_analysis.updated', `run:${seed.run.id}`, written));

        await expect(runDetailPage.resultDetail).toContainText('The checkout button is clicked before it is enabled.');
        await expect(runDetailPage.resultDetail).toContainText('Wait for the button to be enabled before clicking.');
        await expect(notice).toHaveCount(0);
        await expect(gridRow.getByTitle(/Explanation being written…/)).toHaveCount(0);

        // A late copy of the pending state arrives, then a new version. Events are applied in
        // order, so once v2 shows, the late v1 event has been handled; v1 must still hold its
        // explanation.
        socket.send(wsEvent('run_result_analysis.updated', `run_result:${resultId}`, pending));
        const v2 = { ...pending, id: 'an-live-2', version: 2 };
        socket.send(wsEvent('run_result_analysis.created', `run_result:${resultId}`, v2));
        const pill = (v) => runDetailPage.resultDetail.getByRole('button', { name: v, exact: true });
        await expect(pill('v2')).toBeVisible();
        await expect(notice).toHaveText('Explanation being written…'); // the newest version is shown
        await pill('v1').click();
        await expect(runDetailPage.resultDetail).toContainText('The checkout button is clicked before it is enabled.');
        await expect(notice).toHaveCount(0);
    });
});
