import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

// Failure-analysis envelopes — deliberately NOT the `{ test_cases: [...] }` shape
// the other ai/ specs use (those exercise test-case generation). Analyze() parses
// exactly these five keys (failureanalysis/analyzer.go), and the verdict maps to a
// defect_type via models.SuggestedDefectType.
//
// The two rows get DIFFERENT verdicts and confidences on purpose. Identical
// envelopes would still pass if one row's suggestion were smeared onto both, and
// `flaky_test` exercises the LOSSY half of the mapping (verdict `flaky_test` →
// defect_type `automation_bug`), which an identity-mapped `product_bug` cannot.
const FAIL_ENVELOPE = {
    verdict: 'product_bug',
    confidence: 'high',
    summary: 'Checkout total is computed without tax.',
    next_action: 'Raise a defect against the pricing service.',
    rationale: 'The assertion diff is deterministic across attempts, so not flake.',
};

const ERROR_ENVELOPE = {
    verdict: 'flaky_test',
    confidence: 'medium',
    summary: 'The driver session died before the assertion could run.',
    next_action: 'Stabilise the fixture teardown.',
    rationale: 'The stack trace sits in the harness, not in product code.',
};

test.describe('AI failure analysis — suggested defect_type', () => {
    // ERROR rows are the point of the second half: the analyzer has always produced
    // verdicts for them (failing-results queries select status IN ('FAIL','ERROR')),
    // but the chip, the snapshot and the accuracy filter all used to require FAIL, so
    // those verdicts were generated and never calibrated.
    test('accepting the AI suggestion triages FAIL and ERROR rows alike', async ({
        page, api, runDetailPage, fakeLLM,
    }) => {
        // Two real analyze round-trips plus the WS-driven grid refresh after each
        // triage write exceeds the suite's 10s default.
        test.setTimeout(TIMEOUTS.AI_LIFECYCLE);

        let seed, failRow, errorRow;

        await test.step('Seed one run with a FAIL and an ERROR result, and analyze both', async () => {
            // Register the provider first — analyzeSync resolves it through
            // currentProvider(), and is_default wins the single-default invariant.
            await fakeLLM((call) => (call === 0 ? FAIL_ENVELOPE : ERROR_ENVELOPE));
            seed = await api.seedRunWithResults({ statuses: ['FAIL', 'ERROR'], label: 'AI Suggest' });
            [failRow, errorRow] = seed.rows;

            // One analyze call per row, awaited in sequence: POST /run-results/{id}/analyze
            // is LLM rate-limited (burst 8, so two is fine), and the call ORDER is what
            // pairs each row with its envelope.
            const failAnalysis = await api.analyzeRunResult(failRow.result.id);
            expect(failAnalysis.verdict).toBe('product_bug');
            expect(failAnalysis.suggested_defect_type).toBe('product_bug');

            const errorAnalysis = await api.analyzeRunResult(errorRow.result.id);
            expect(errorAnalysis.verdict).toBe('flaky_test');
            expect(errorAnalysis.suggested_defect_type).toBe('automation_bug');
        });

        await test.step('Open the run with the optional defect_type column pinned visible', async () => {
            // The whole defect_type cell — chip included — is gated on
            // isVisible('defect_type'), so leaving this to stored prefs is flaky.
            await runDetailPage.pinColumns({ defect_type: true });
            await runDetailPage.open(seed.run.id);
            await expect(runDetailPage.toolbar).toBeVisible({ timeout: TIMEOUTS.HEAVY_GRID });
        });

        await test.step('Both untriaged failure rows offer their own suggestion', async () => {
            // The ERROR row rendering a defect-type control AT ALL is the fix: the cell
            // used to be gated on status === 'FAIL', which left ERROR rows showing a dash
            // with no way to triage them.
            for (const [row, label] of [[failRow, 'Product bug'], [errorRow, 'Automation bug']]) {
                // to_investigate is the failure auto-default, i.e. nobody has triaged yet.
                await expect(runDetailPage.defectTypeSelect(row.tc.id))
                    .toHaveValue('to_investigate', { timeout: TIMEOUTS.ELEMENT });

                const chip = runDetailPage.defectSuggestion(row.tc.id);
                await expect(chip).toBeVisible({ timeout: TIMEOUTS.ELEMENT });
                await expect(chip).toContainText(label);
            }
        });

        await test.step("Accept writes each row's own suggested defect_type and retires its chip", async () => {
            for (const [row, expected] of [[failRow, 'product_bug'], [errorRow, 'automation_bug']]) {
                const saved = page.waitForResponse(r =>
                    /\/api\/runs\/[^/]+\/results\/[^/]+$/.test(r.url()) && r.request().method() === 'PUT');
                await runDetailPage.defectSuggestionAccept(row.tc.id).click();
                expect((await saved).status()).toBe(200);

                await expect(runDetailPage.defectTypeSelect(row.tc.id))
                    .toHaveValue(expected, { timeout: TIMEOUTS.ELEMENT });
                // Aid, not a nag: the row is decided, so the suggestion retires.
                await expect(runDetailPage.defectSuggestion(row.tc.id))
                    .not.toBeVisible({ timeout: TIMEOUTS.UI_SETTLE });
            }
        });

        await test.step('The calibration record lands on both rows, not just the defect_type', async () => {
            // The point of the whole feature is the snapshot behind the UI: without reading the
            // suggested_* columns back, this spec would still pass if Accept wrote defect_type
            // alone and the rows never entered the accuracy calibration set.
            const run = await api.getRun(seed.run.id);
            const stored = new Map((run.run_results || []).map(r => [r.id, r]));

            const expectations = [
                { row: failRow, status: 'FAIL', defectType: 'product_bug', verdict: 'product_bug', confidence: 'high' },
                { row: errorRow, status: 'ERROR', defectType: 'automation_bug', verdict: 'flaky_test', confidence: 'medium' },
            ];
            for (const e of expectations) {
                const saved = stored.get(e.row.result.id);
                expect(saved, `the seeded ${e.status} result must still be on the run`).toBeTruthy();
                // Accepting a suggestion is a defect_type decision only — it must never rewrite
                // the status that made the row analyzable in the first place.
                expect(saved.status).toBe(e.status);
                expect(saved.defect_type).toBe(e.defectType);
                // suggested_verdict keeps the RAW verdict; suggested_defect_type keeps the mapped
                // one. Collapsing the two would lose which of flaky_test/test_data was proposed.
                expect(saved.suggested_verdict).toBe(e.verdict);
                expect(saved.suggested_defect_type).toBe(e.defectType);
                expect(saved.suggested_confidence).toBe(e.confidence);
                // decided_at is what the accuracy window is measured on — an unset one silently
                // drops the decision out of every report.
                expect(saved.decided_at, `the ${e.status} decision moment must be stamped`).toBeTruthy();
            }
        });
    });
});
