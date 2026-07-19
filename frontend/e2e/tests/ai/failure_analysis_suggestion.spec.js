import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

// Failure-analysis envelope — deliberately NOT the `{ test_cases: [...] }` shape
// the other ai/ specs use (those exercise test-case generation). Analyze() parses
// exactly these five keys (failureanalysis/analyzer.go), and the `product_bug`
// verdict maps to the `product_bug` defect_type via models.SuggestedDefectType.
const ANALYSIS_ENVELOPE = {
    verdict: 'product_bug',
    confidence: 'high',
    summary: 'Checkout total is computed without tax.',
    next_action: 'Raise a defect against the pricing service.',
    rationale: 'The assertion diff is deterministic across attempts, so not flake.',
};

test.describe('AI failure analysis — suggested defect_type', () => {
    test('accepting the AI suggestion triages the row and retires the chip', async ({
        page, api, runDetailPage, fakeLLM,
    }) => {
        // A real analyze round-trip plus the WS-driven grid refresh after the
        // triage write exceeds the suite's 10s default.
        test.setTimeout(TIMEOUTS.AI_LIFECYCLE);

        let seed;

        await test.step('Seed a FAIL result and analyze it against a fake LLM', async () => {
            // Register the provider first — analyzeSync resolves it through
            // currentProvider(), and is_default wins the single-default invariant.
            await fakeLLM(ANALYSIS_ENVELOPE);
            seed = await api.seedRunWithResult({ status: 'FAIL', label: 'AI Suggest' });

            // Exactly one analyze call on purpose: POST /run-results/{id}/analyze
            // is LLM rate-limited, so a burst would earn 429s instead of analyses.
            const analysis = await api.analyzeRunResult(seed.result.id);
            expect(analysis.verdict).toBe('product_bug');
            expect(analysis.suggested_defect_type).toBe('product_bug');
        });

        await test.step('Open the run with the optional defect_type column pinned visible', async () => {
            // The whole defect_type cell — chip included — is gated on
            // isVisible('defect_type'), so leaving this to stored prefs is flaky.
            await runDetailPage.pinColumns({ defect_type: true });
            await runDetailPage.open(seed.run.id);
            await expect(runDetailPage.toolbar).toBeVisible({ timeout: TIMEOUTS.HEAVY_GRID });
        });

        await test.step('The untriaged FAIL row offers the suggestion', async () => {
            // to_investigate is the FAIL auto-default, i.e. nobody has triaged yet.
            await expect(runDetailPage.defectTypeSelect(seed.tc.id))
                .toHaveValue('to_investigate', { timeout: TIMEOUTS.ELEMENT });

            const chip = runDetailPage.defectSuggestion(seed.tc.id);
            await expect(chip).toBeVisible({ timeout: TIMEOUTS.ELEMENT });
            await expect(chip).toContainText('Product bug');
        });

        await test.step('Accept writes the suggested defect_type and the chip disappears', async () => {
            const saved = page.waitForResponse(r =>
                /\/api\/runs\/[^/]+\/results\/[^/]+$/.test(r.url()) && r.request().method() === 'PUT');
            await runDetailPage.defectSuggestionAccept(seed.tc.id).click();
            expect((await saved).status()).toBe(200);

            await expect(runDetailPage.defectTypeSelect(seed.tc.id))
                .toHaveValue('product_bug', { timeout: TIMEOUTS.ELEMENT });
            // Aid, not a nag: the row is decided, so the suggestion retires.
            await expect(runDetailPage.defectSuggestion(seed.tc.id))
                .not.toBeVisible({ timeout: TIMEOUTS.UI_SETTLE });
        });

        await test.step('The calibration record lands on the row, not just the defect_type', async () => {
            // The point of the whole feature is the snapshot behind the UI: without reading the
            // suggested_* columns back, this spec would still pass if Accept wrote defect_type
            // alone and the row never entered the accuracy calibration set.
            const run = await api.getRun(seed.run.id);
            const row = (run.run_results || []).find(r => r.id === seed.result.id);
            expect(row, 'the seeded result must still be on the run').toBeTruthy();
            expect(row.defect_type).toBe('product_bug');
            expect(row.suggested_verdict).toBe('product_bug');
            expect(row.suggested_defect_type).toBe('product_bug');
            expect(row.suggested_confidence).toBe('high');
            // decided_at is what the accuracy window is measured on — an unset one silently
            // drops the decision out of every report.
            expect(row.decided_at, 'the decision moment must be stamped').toBeTruthy();
        });
    });
});
