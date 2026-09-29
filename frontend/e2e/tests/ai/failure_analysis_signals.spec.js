import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

// Wave 3 on the run page and in Settings → AI:
// - TypeSafe's companion-answer chips and the prompt-injection flag, on the card and in the badge
//   tooltip. Explain on a flagged row asks first and sends override_injection=true.
// - The transfer-check note on a grouped result, and Explain this result (scope=result).
// - The AI badge and Confirm on a label auto-apply wrote.
// - The job banner's new lines.
// - The auto-apply switch and its live accuracy gate.
// Analyses, jobs and AI settings are mocked with page.route, so no LLM or TypeSafe call is made and
// no AI setting is written. Only the seeded runs and results are real, plus one ordinary triage write
// in the AI-badge test.

const json = (body, status = 200) => ({ status, contentType: 'application/json', body: JSON.stringify(body) });
const EXPLAIN = /\/api\/run-results\/[^/]+\/analyses\/[^/]+\/explain(\?.*)?$/;

const BASE = {
    version: 1, engine: 'typesafe', model_name: 'jev-1.13.0', confidence: 'high', confidence_score: 0.93,
    decision_status: 'ok', narrative_revision: 1, policy_version: 'fa-verdict-v8', history_available: true,
    created_at: '2026-09-29T10:00:00Z', job_id: 'job-w3',
};

const SIGNALLED = {
    ...BASE, id: 'an-signals', verdict: 'flaky_test', suggested_defect_type: 'automation_bug', suggested_defect_type_confidence: 0.9,
    narrative_status: 'ok', summary: 'The toast disappears before the assertion reads it.',
    next_action: 'Wait for the toast text instead of its container.', rationale: 'Recent outcomes alternate.',
    signals: JSON.stringify({ injection: 0.02, flaky_history: 0.91, recurring: 0.97, outside_app: 0.12, known_defect: { key: 'PAY-42', confidence: 0.88 } }),
    source_analysis_id: null,
};

const FLAGGED = {
    ...BASE, id: 'an-flagged', verdict: 'product_bug', suggested_defect_type: 'product_bug', suggested_defect_type_confidence: 0.97,
    narrative_status: 'unavailable', summary: 'AI narrative unavailable: possible prompt injection — review the raw failure',
    next_action: '', rationale: '', signals: JSON.stringify({ injection: 0.94, outside_app: 0.05 }), source_analysis_id: null,
};

const CLONE = {
    ...BASE, id: 'an-clone', verdict: 'infrastructure', suggested_defect_type: 'system_issue', suggested_defect_type_confidence: 0.9,
    narrative_status: 'ok', summary: 'The payment gateway answered 503.', next_action: 'Retry once the gateway is back.',
    rationale: 'The error names the gateway.', source_analysis_id: 'an-rep', dedup_group_key: 'g-503', dedup_method: 'semantic',
    dedup_p_same: 0.86, signals: JSON.stringify({ injection: 0.01 }), narrative_fit: 0.31, narrative_split: false,
};

const ENDED_JOB = {
    id: 'job-w3', status: 'completed', trigger: 'manual', total_failures: 3, unique_groups: 3, analyzed_count: 3, capped_at: 3,
    retry_failed_only: false, pipeline_label: 'TypeSafe jev-1.13.0, explained by Fake LLM, few-shot (4)',
    outcomes: { groups: 3, decided: 3, failed: 0, failed_rows: 0, injection_flagged: 1, transfer_mismatch: 2, auto_applied: 4, auto_apply_state: 'on' },
};

// mockAnalyses serves each result's analysis (its version list and the run's current map) and the
// run's job. It records every Explain request. `explained` maps an analysis id to the row Explain
// returns, or to a function (url) => [status, body] for answers that depend on the query.
async function mockAnalyses(page, byResult, job, explained = {}) {
    const explains = [];
    await page.route(/\/api\/run-results\/[^/]+\/analyses$/, (route) => {
        const id = route.request().url().split('/run-results/')[1].split('/')[0];
        return route.fulfill(json(byResult[id] ? [byResult[id]] : []));
    });
    await page.route(/\/api\/runs\/[^/]+\/analyses\/current$/, (route) => route.fulfill(json(byResult)));
    await page.route(/\/api\/runs\/[^/]+\/analysis-job$/, (route) => route.fulfill(json(job)));
    await page.route(EXPLAIN, (route) => {
        const url = new URL(route.request().url());
        explains.push(url);
        const analysisId = url.pathname.split('/analyses/')[1].split('/')[0];
        const answer = explained[analysisId];
        if (typeof answer === 'function') {
            const [status, body] = answer(url);
            return route.fulfill(json(body, status));
        }
        return answer ? route.fulfill(json(answer)) : route.fulfill(json({ error: 'unexpected Explain' }, 500));
    });
    return explains;
}

const INJECTION_409 = { error: 'possible prompt injection: confirm to send this failure to the LLM' };

// Settings mocks, as in failure_analysis_diagram.spec.js: every write is refused and counted.
const TS = {
    id: 'singleton', enabled: true, api_key_masked: '…1234', api_key_status: 'ok', model: 'jev-1.13.0', timeout_seconds: 30,
    verdict_engine_enabled: true, narrative_enabled: true, llm_fallback_enabled: true, escalate_below_pct: 0,
    semantic_dedup_enabled: true, allow_auto_failure_analysis: false,
};
const FA = {
    id: 'singleton', enabled_on_completion: false, max_analyses_per_run: 20, parallel_groups: 4, dedup_enabled: true,
    redaction_enabled: true, prompt_template: 'Classify the failure.', llm_call_timeout_seconds: 45, hedge_after_seconds: 0,
    few_shot_examples: 4, auto_apply_defect_type: false, auto_apply_min_confidence: 95,
};
const PROVIDERS = [
    { id: 'p1', label: 'Fake LLM', provider_type: 'openai', model_name: 'fake-1', is_default: true, enabled: true, allow_auto_failure_analysis: false },
];

async function mockSettings(page) {
    const writes = [];
    const serve = (name, body) => (route) => {
        if (route.request().method() !== 'GET') {
            writes.push(`${route.request().method()} ${name}`);
            return route.fulfill(json({ error: 'this spec never saves' }, 500));
        }
        return route.fulfill(json(body));
    };
    await page.route(/\/api\/settings\/typesafe$/, serve('typesafe', TS));
    await page.route(/\/api\/settings\/ai-failure-analysis$/, serve('ai-failure-analysis', FA));
    await page.route(/\/api\/settings\/llm-providers$/, serve('llm-providers', PROVIDERS));
    await page.route(/\/api\/settings\/ai-features$/, serve('ai-features', { enabled: true }));
    return writes;
}

// The gate is open at 90 % and below, closed above (too few rows).
const gateAt = (pct) => (pct <= 90
    ? { graded: 214, agreed: 210, accuracy: 210 / 214, open: true, policies: ['fa-verdict-v7', 'fa-verdict-v8'], min_confidence: pct / 100 }
    : { graded: 12, agreed: 12, accuracy: 1, open: false, policies: ['fa-verdict-v7'], min_confidence: pct / 100 });

test.describe('AI failure analysis — signals, injection guard, transfer check, auto-apply', () => {
    test('companion answers show as chips on the card and in the badge tooltip', async ({ page, api, runDetailPage }) => {
        const seed = await api.seedRunWithResults({ statuses: ['FAIL'], label: 'AI Signals' });
        const [row] = seed.rows;
        await mockAnalyses(page, { [row.result.id]: { ...SIGNALLED, run_result_id: row.result.id } }, { ...ENDED_JOB, test_run_id: seed.run.id });
        await runDetailPage.pinColumns({ ai_verdict: true });
        await runDetailPage.open(seed.run.id);

        const gridRow = runDetailPage.resultRow(row.tc.name);
        await expect(gridRow.getByTitle('TypeSafe jev-1.13.0 · confidence 0.93 · Flaky pattern in history · Seen before · Matches PAY-42'))
            .toBeVisible({ timeout: TIMEOUTS.HEAVY_GRID });
        await expect(gridRow.getByTestId('ai-verdict-injection')).toHaveCount(0);

        await runDetailPage.expandResultRow(row.tc.name);
        await page.getByTestId('result-tab-ai').click();
        await expect(page.getByTestId('analysis-signal-flaky_history')).toHaveText('Flaky pattern in history');
        await expect(page.getByTestId('analysis-signal-flaky_history')).toHaveAttribute('title', /91%/);
        await expect(page.getByTestId('analysis-signal-recurring')).toHaveText('Seen before');
        await expect(page.getByTestId('analysis-signal-known_defect')).toHaveText('Matches PAY-42');
        await expect(page.getByTestId('analysis-signal-outside_app')).toHaveCount(0); // 0.12 is below 0.80
        await expect(page.getByTestId('analysis-signal-injection')).toHaveCount(0);
        await expect(runDetailPage.resultDetail).toContainText('The toast disappears before the assertion reads it.');
    });

    test('a failure flagged for possible prompt injection has no explanation, and Explain asks first', async ({ page, api, runDetailPage }) => {
        const seed = await api.seedRunWithResults({ statuses: ['FAIL'], label: 'AI Injection' });
        const [row] = seed.rows;
        const flagged = { ...FLAGGED, run_result_id: row.result.id };
        const explains = await mockAnalyses(page, { [row.result.id]: flagged }, { ...ENDED_JOB, test_run_id: seed.run.id }, {
            'an-flagged': {
                ...flagged, narrative_status: 'ok', narrative_revision: 2,
                summary: 'The log asks the model to ignore its rules; the checkout total assertion still failed.',
                next_action: 'Raise a defect for the checkout total.', rationale: 'The assertion is deterministic.',
            },
        });
        await runDetailPage.pinColumns({ ai_verdict: true });
        await runDetailPage.open(seed.run.id);

        const gridRow = runDetailPage.resultRow(row.tc.name);
        await expect(gridRow.getByTestId('ai-verdict-injection')).toBeVisible({ timeout: TIMEOUTS.HEAVY_GRID });
        await expect(gridRow.getByTitle(/Possible prompt injection — review the raw failure/)).toBeVisible();

        await runDetailPage.expandResultRow(row.tc.name);
        await page.getByTestId('result-tab-ai').click();
        await expect(page.getByTestId('analysis-signal-injection')).toContainText('Possible prompt injection — review the raw failure');
        const notice = page.getByTestId('analysis-narrative-notice');
        await expect(notice).toHaveAttribute('data-status', 'injection');
        await expect(notice).toContainText('possible prompt injection');
        await expect(page.getByTestId('analysis-narrative-reason')).toHaveCount(0);
        const explain = page.getByTestId('analysis-explain');
        await expect(explain).toHaveText('Explain anyway…');

        // Declined: nothing is sent.
        const dialogs = [];
        page.once('dialog', (d) => { dialogs.push(d.message()); return d.dismiss(); });
        await explain.click();
        await expect.poll(() => dialogs.length).toBe(1);
        expect(dialogs[0]).toContain('possible prompt injection');
        expect(explains).toEqual([]);
        await expect(notice).toBeVisible();

        // Confirmed: the request carries the override, and the explanation fills in.
        page.once('dialog', (d) => d.accept());
        await explain.click();
        await expect(runDetailPage.resultDetail).toContainText('The log asks the model to ignore its rules; the checkout total assertion still failed.');
        expect(explains).toHaveLength(1);
        expect(explains[0].searchParams.get('override_injection')).toBe('true');
        expect(explains[0].searchParams.get('scope')).toBeNull();
        await expect(notice).toHaveCount(0);
        await expect(page.getByTestId('analysis-signal-injection')).toBeVisible(); // the flag stays with the decision
    });

    test('a grouped result whose copied explanation may not fit it offers Explain this result', async ({ page, api, runDetailPage }) => {
        const seed = await api.seedRunWithResults({ statuses: ['FAIL'], label: 'AI Transfer' });
        const [row] = seed.rows;
        const clone = { ...CLONE, run_result_id: row.result.id };
        // The group was not flagged, but the clone's own failure trips the injection-only check (R9):
        // the server asks for the override first, then explains.
        const explains = await mockAnalyses(page, { [row.result.id]: clone }, { ...ENDED_JOB, test_run_id: seed.run.id }, {
            'an-clone': (url) => (url.searchParams.get('override_injection') !== 'true'
                ? [409, INJECTION_409]
                : [200, {
                    ...clone, narrative_split: true, narrative_revision: 2,
                    summary: 'This result timed out waiting for the receipt page, not a gateway 503.',
                    next_action: 'Check how long the receipt page takes to load.', rationale: 'Its own error names the receipt page.',
                }]),
        });
        await runDetailPage.pinColumns({ ai_verdict: true });
        await runDetailPage.open(seed.run.id);
        await expect(runDetailPage.resultRow(row.tc.name)).toBeVisible({ timeout: TIMEOUTS.HEAVY_GRID });
        await runDetailPage.expandResultRow(row.tc.name);
        await page.getByTestId('result-tab-ai').click();

        const note = page.getByTestId('analysis-fit-note');
        await expect(note).toHaveAttribute('data-state', 'mismatch');
        await expect(note).toContainText('This explanation may not apply to this result');
        await expect(note).toContainText('31%');
        await expect(runDetailPage.resultDetail).toContainText('The payment gateway answered 503.'); // the group's explanation stays visible

        await expect(page.getByTestId('analysis-explain-result')).toHaveText('Explain this result');

        // Declined: the 409 is neither toasted nor resent.
        const dialogs = [];
        page.once('dialog', (d) => { dialogs.push(d.message()); return d.dismiss(); });
        await page.getByTestId('analysis-explain-result').click();
        await expect.poll(() => dialogs.length).toBe(1);
        expect(dialogs[0]).toContain('possible prompt injection');
        await expect.poll(() => explains.length).toBe(1);
        await expect(page.getByText(INJECTION_409.error)).toHaveCount(0);
        await expect(note).toHaveAttribute('data-state', 'mismatch');

        // Confirmed: the same request again with the override.
        page.once('dialog', (d) => d.accept());
        await page.getByTestId('analysis-explain-result').click();
        await expect(runDetailPage.resultDetail).toContainText('This result timed out waiting for the receipt page, not a gateway 503.');
        await expect(note).toHaveAttribute('data-state', 'split');
        await expect(page.getByTestId('analysis-explain-result')).toHaveCount(0);
        expect(explains).toHaveLength(3);
        for (const u of explains) {
            expect(u.pathname).toMatch(/\/analyses\/an-clone\/explain$/);
            expect(u.searchParams.get('scope')).toBe('result');
        }
        expect(explains.map((u) => u.searchParams.get('override_injection'))).toEqual([null, null, 'true']);
        await expect(page.getByText(INJECTION_409.error)).toHaveCount(0);
    });

    test('a failed explanation of a result alone can be retried for that result', async ({ page, api, runDetailPage }) => {
        const seed = await api.seedRunWithResults({ statuses: ['FAIL'], label: 'AI Split Retry' });
        const [row] = seed.rows;
        const failed = {
            ...CLONE, run_result_id: row.result.id, narrative_split: true, narrative_status: 'unavailable', narrative_revision: 2,
            summary: 'LLM call timed out',
        };
        const explains = await mockAnalyses(page, { [row.result.id]: failed }, { ...ENDED_JOB, test_run_id: seed.run.id }, {
            'an-clone': { ...failed, narrative_status: 'ok', narrative_revision: 3, summary: 'The receipt page loads after the 30 s wait.' },
        });
        await runDetailPage.pinColumns({ ai_verdict: true });
        await runDetailPage.open(seed.run.id);
        await expect(runDetailPage.resultRow(row.tc.name)).toBeVisible({ timeout: TIMEOUTS.HEAVY_GRID });
        await runDetailPage.expandResultRow(row.tc.name);
        await page.getByTestId('result-tab-ai').click();

        await expect(page.getByTestId('analysis-fit-note')).toHaveAttribute('data-state', 'split-failed');
        await expect(page.getByTestId('analysis-explain')).toHaveCount(0); // no group-scope Explain on a split row
        await expect(page.getByTestId('analysis-explain-result')).toHaveText('Retry explanation');
        await page.getByTestId('analysis-explain-result').click();
        await expect(runDetailPage.resultDetail).toContainText('The receipt page loads after the 30 s wait.');
        await expect(page.getByTestId('analysis-fit-note')).toHaveAttribute('data-state', 'split');
        expect(explains).toHaveLength(1);
        expect(explains[0].searchParams.get('scope')).toBe('result');
    });

    test('a label auto-apply wrote shows an AI badge, and Confirm is an ordinary triage write', async ({ page, api, runDetailPage }) => {
        const seed = await api.seedRunWithResults({ statuses: ['FAIL'], label: 'AI Applied' });
        const [row] = seed.rows;
        // The stored row is untriaged; the run as the page first loads it says auto-apply labelled it.
        let aiLabelled = true;
        await page.route(new RegExp(`/api/runs/${seed.run.id}$`), async (route) => {
            if (route.request().method() !== 'GET' || !aiLabelled) return route.fallback();
            const response = await route.fetch();
            const run = await response.json();
            run.run_results = (run.run_results || []).map((r) => (r.id === row.result.id
                ? { ...r, defect_type: 'automation_bug', defect_type_source: 'ai' } : r));
            return route.fulfill({ response, json: run });
        });
        await runDetailPage.pinColumns({ defect_type: true });
        await runDetailPage.open(seed.run.id);

        const badge = page.getByTestId(`defect-type-ai-${row.tc.id}`);
        await expect(badge).toBeVisible({ timeout: TIMEOUTS.HEAVY_GRID });
        await expect(badge).toHaveAttribute('title', /^Set by AI: TypeSafe\.ai suggested "Automation bug"/);
        await expect(runDetailPage.defectTypeSelect(row.tc.id)).toHaveValue('automation_bug');
        await expect(runDetailPage.defectSuggestion(row.tc.id)).toHaveCount(0);

        aiLabelled = false; // from here on the page sees the server's rows as stored
        const put = page.waitForRequest((r) => /\/api\/runs\/[^/]+\/results\/[^/]+$/.test(r.url()) && r.method() === 'PUT');
        await page.getByTestId(`defect-type-ai-confirm-${row.tc.id}`).click();
        expect((await put).postDataJSON()).toEqual({ defect_type: 'automation_bug' });
        // The server's live row update carries defect_type_source "human" (an explicit triage write,
        // Amendment 1), so the badge goes.
        await expect(badge).toHaveCount(0, { timeout: TIMEOUTS.ELEMENT });
        await expect(runDetailPage.defectTypeSelect(row.tc.id)).toHaveValue('automation_bug');

        const run = await api.getRun(seed.run.id);
        const saved = run.run_results.find((r) => r.id === row.result.id);
        expect(saved.defect_type).toBe('automation_bug');
        expect(saved.defect_type_source).toBe('human');
        // The stored row was never really 'ai' (only the page's copy was patched), so this Confirm is
        // not recorded as a confirmation of an AI label; P3's store tests cover suggested_auto_applied.
        expect(saved.suggested_auto_applied ?? false).toBe(false);
    });

    test('the banner reports flagged groups, transfer mismatches and labels set by AI', async ({ page, api, runDetailPage }) => {
        const seed = await api.seedRunWithResults({ statuses: ['FAIL'], label: 'AI Banner' });
        await mockAnalyses(page, {}, { ...ENDED_JOB, test_run_id: seed.run.id });
        await runDetailPage.open(seed.run.id);

        await expect(page.getByTestId('run-analysis-summary')).toContainText('AI analysis finished: 3 groups decided.', { timeout: TIMEOUTS.HEAVY_GRID });
        await expect(page.getByTestId('run-analysis-note-injection'))
            .toHaveText('1 group was flagged for possible prompt injection: no explanation was written. Review the raw failure.');
        await expect(page.getByTestId('run-analysis-note-transfer'))
            .toHaveText("2 grouped results may not match their group's explanation; open them to explain them on their own.");
        await expect(page.getByTestId('run-analysis-note-auto-applied')).toHaveText('4 labels set by AI');
        await expect(page.getByTestId('run-analysis-note-auto-apply-paused')).toHaveCount(0);
    });

    test('a paused auto-apply is reported until dismissed', async ({ page, api, runDetailPage }) => {
        const seed = await api.seedRunWithResults({ statuses: ['FAIL'], label: 'AI Paused' });
        await mockAnalyses(page, {}, {
            ...ENDED_JOB, test_run_id: seed.run.id, auto_apply_state: 'paused',
            outcomes: { groups: 3, decided: 3, failed: 0, failed_rows: 0, auto_applied: 0, auto_apply_state: 'paused' },
        });
        await runDetailPage.open(seed.run.id);

        await expect(page.getByTestId('run-analysis-note-auto-apply-paused'))
            .toHaveText('Auto-apply paused: accuracy gate closed', { timeout: TIMEOUTS.HEAVY_GRID });
        await expect(page.getByTestId('run-analysis-note-auto-applied')).toHaveCount(0);
        await page.getByTestId('run-analysis-dismiss').click();
        await expect(page.getByTestId('run-analysis-summary')).toHaveCount(0);
    });

    test('auto-apply can be switched on only while its accuracy gate is open', async ({ page, settingsPage }) => {
        const writes = await mockSettings(page);
        const gateCalls = [];
        await page.route(/\/api\/settings\/ai-failure-analysis\/auto-apply-gate(\?.*)?$/, (route) => {
            const pct = Number(new URL(route.request().url()).searchParams.get('min_confidence'));
            gateCalls.push(pct);
            return route.fulfill(json(gateAt(pct)));
        });
        await settingsPage.open();
        await settingsPage.openFailureAnalysis();

        const gateLine = page.getByTestId('fa-auto-apply-gate');
        const toggle = page.getByTestId('fa-auto-apply');
        await gateLine.scrollIntoViewIfNeeded();
        await expect(gateLine).toHaveText('100.0 % of 12 rows at ≥ 95 % (50 rows needed) — gate closed');
        await expect(gateLine).toHaveAttribute('data-open', 'false');
        await expect(toggle).toBeDisabled();
        await expect(page.getByTestId('fa-auto-apply-gate-scope')).toContainText('fa-verdict-v7 decisions only');
        await expect(page.getByTestId('fa-auto-apply-gate-scope')).toContainText('confirmations of labels auto-apply set do not count');

        await page.getByTestId('fa-auto-apply-min').fill('90');
        await expect(gateLine).toHaveText('98.1 % of 214 rows at ≥ 90 % — gate open');
        await expect(toggle).toBeEnabled();
        await toggle.check();
        await expect(page.getByTestId('fa-auto-apply-error')).toHaveCount(0);
        await settingsPage.selectFlowStep('store');
        await expect(settingsPage.flowChip('fa.auto_apply_defect_type')).toContainText('At ≥ 90%');

        // Moving the threshold to where the gate is closed blocks saving the switch-on.
        await page.getByTestId('fa-auto-apply-min').fill('95');
        await expect(gateLine).toHaveAttribute('data-open', 'false');
        await expect(page.getByTestId('fa-auto-apply-error')).toContainText('gate is closed');
        await expect(settingsPage.saveBarMessage).toContainText('Fix 1 setting before saving: Set the defect type automatically (Failure analysis)');
        await expect(settingsPage.saveBarSave).toBeDisabled();

        await toggle.uncheck();
        await expect(page.getByTestId('fa-auto-apply-error')).toHaveCount(0);
        expect(gateCalls).toContain(95);
        expect(gateCalls).toContain(90);
        expect(writes).toEqual([]);
    });
});
