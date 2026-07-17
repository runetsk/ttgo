import { test, expect } from '../../fixtures/test.js';
import { ApiClient } from '../../helpers/api.js';
import { startFakeLLM } from '../../helpers/fake-llm.js';
import { TIMEOUTS } from '../../config.js';

// Two-phase fake LLM driven by a call counter: the studio's first round-trip
// (POST /ai-generations, "Generate") answers with a one-draft BATCH; the second
// (drafts/{id}/regenerate, "Regenerate") answers with a REVISED single case.
// A per-run token keeps draft names from colliding with a previous run's
// leftovers on a persistent dev DB (harmless here — this spec never accepts, so
// duplicate detection, which only scans materialized test cases, can't flag it).
let llm;
let requirementId;

test.describe('AI draft regeneration — compare and choose', () => {
    test.beforeAll(async ({ request }) => {
        const api = new ApiClient(request);
        const token = Date.now();

        const BATCH = JSON.stringify({
            test_cases: [{
                name: `[Functional] Regen target draft ${token}`,
                category: 'Functional',
                description: 'Original version.',
                source_refs: ['AC-1'],
                steps: [{ action: 'Enter "user@example.com" and submit', expected_result: 'The dashboard is displayed' }],
            }],
        });
        const REVISED = JSON.stringify({
            test_cases: [{
                name: `[Functional] Regen target draft ${token} (sharpened)`,
                category: 'Functional',
                description: 'Revised version.',
                source_refs: ['AC-1'],
                steps: [{ action: 'Enter "user@example.com" / "Passw0rd!" and submit', expected_result: 'The dashboard header shows the user menu' }],
            }],
        });

        llm = await startFakeLLM(api, (call) => (call === 0 ? BATCH : REVISED));

        const req = await api.createRequirement(`REQ-E2E-REGEN-${token}`, 'Regen flow',
            '<ul><li>User can sign in</li></ul>');
        requirementId = req.id;

        // Seed a folder so the studio's auto-defaulted folder-tree state stays
        // sane on an instance with zero folders; never used (this spec never accepts).
        await api.createFolder(`AI Regen E2E ${token}`);
    });

    test.afterAll(async () => {
        if (llm) await llm.dispose();
    });

    test('regenerate, compare, choose the new version', async ({ page, aiStudioPage }) => {
        // Two full LLM round-trips (generate + regenerate) plus the compare/choose
        // UI comfortably exceed the suite's 10s default — extend just this test.
        test.setTimeout(TIMEOUTS.AI_LIFECYCLE);

        await test.step('generate the batch', async () => {
            await aiStudioPage.enterFromRequirement(requirementId);
            const createResp = page.waitForResponse(r =>
                r.url().endsWith('/api/ai-generations') && r.request().method() === 'POST');
            await aiStudioPage.generate();
            expect((await createResp).status()).toBe(201);
            // draft.name carries the "[Functional] " prefix and the token suffix;
            // both the list row and the auto-selected detail pane render it, hence .first().
            await expect(page.getByText('Regen target draft').first()).toBeVisible();
        });

        await test.step('regenerate the draft with an instruction', async () => {
            await aiStudioPage.selectDraft('Regen target draft');
            const regenResp = page.waitForResponse(r => r.url().includes('/regenerate'));
            await aiStudioPage.regenerate('sharpen the credentials');
            expect((await regenResp).status()).toBe(201);
            await expect(aiStudioPage.draftCompare).toBeVisible();
        });

        await test.step('toggle split/unified persists the choice', async () => {
            await aiStudioPage.setCompareView('unified');
            expect(await page.evaluate(() => localStorage.getItem('aig.compareView'))).toBe('unified');
            await aiStudioPage.setCompareView('split');
            expect(await page.evaluate(() => localStorage.getItem('aig.compareView'))).toBe('split');
        });

        await test.step('choose the new version — original becomes superseded', async () => {
            const chooseResp = page.waitForResponse(r => r.url().includes('/choose'));
            await aiStudioPage.chooseNewVersion();
            expect((await chooseResp).status()).toBe(200);
            // The alternative (now the only pending draft at this position) renders
            // in the center list, so its "(sharpened)" name shows there.
            await expect(page.getByText('(sharpened)').first()).toBeVisible();
            // selectedDraftId never moves off the original (choosing doesn't
            // reselect), so the detail pane keeps showing it — now marked superseded.
            await expect(page.getByText(/superseded/i).first()).toBeVisible();
        });
    });
});
