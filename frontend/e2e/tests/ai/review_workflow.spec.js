import { test, expect } from '../../fixtures/test.js';
import { ApiClient } from '../../helpers/api.js';
import { startFakeLLM } from '../../helpers/fake-llm.js';
import { TIMEOUTS } from '../../config.js';

// Three-draft envelope: two structurally valid drafts (edit/autosave and
// reject/restore) plus one deliberately INVALID draft (empty `steps`) so
// "Accept all clean" has a real exclusion to prove — aigen ValidateDraft raises
// an error-severity finding whenever len(Steps)==0, which is what makes
// isDraftClean() false on the frontend (src/utils/draftReview.js). The two clean
// names are tokened per run so they never collide with previously accepted test
// cases on a persistent dev DB; the invalid draft keeps a fixed name (never accepted).
let llm, requirementId, folderName;
let cleanDraft1Name, cleanDraft2Name;

test.describe('AI reviewer workflow — edit, reject/restore, accept all clean, resume', () => {
    test.beforeAll(async ({ request }) => {
        const api = new ApiClient(request);
        const token = Date.now();
        cleanDraft1Name = `[Functional] Reviewer happy ${token}`;
        cleanDraft2Name = `[Negative] Reviewer wrong-pass ${token}`;

        const ENVELOPE = JSON.stringify({
            test_cases: [
                {
                    name: cleanDraft1Name,
                    category: 'Functional',
                    description: 'Clean draft.',
                    source_refs: ['AC-1'],
                    steps: [{ action: 'Enter "user@example.com" and sign in', expected_result: 'The dashboard page is displayed' }],
                },
                {
                    name: cleanDraft2Name,
                    category: 'Negative',
                    description: 'Also clean.',
                    source_refs: ['AC-2'],
                    steps: [{ action: 'Enter "wrong-pass-1" and sign in', expected_result: 'An inline "Invalid credentials" error is displayed' }],
                },
                {
                    name: '[Boundary] Broken draft with no steps',
                    category: 'Boundary',
                    description: 'Invalid: no steps.',
                    source_refs: [],
                    steps: [],
                },
            ],
        });

        llm = await startFakeLLM(api, ENVELOPE);

        const req = await api.createRequirement(`REQ-E2E-REV-${token}`, 'Reviewer flow',
            '<h2>Acceptance Criteria</h2><ul><li>User can sign in</li><li>Wrong password shows an error</li></ul>');
        requirementId = req.id;
        folderName = `AI Review E2E ${token}`;
        await api.createFolder(folderName);
    });

    test.afterAll(async () => {
        if (llm) await llm.dispose();
    });

    test('full reviewer loop', async ({ page, aiStudioPage }) => {
        // A real generate round-trip, the 800ms autosave debounce + PATCH, reject,
        // restore, accept-all-clean, and a full reload comfortably exceed the
        // suite's 10s default — extend just this test.
        test.setTimeout(TIMEOUTS.AI_LIFECYCLE);

        await test.step('generate from the requirement page', async () => {
            await aiStudioPage.enterFromRequirement(requirementId);
            await expect(page).toHaveURL(/\/ai-generate/);
            await aiStudioPage.selectOutputFolder(folderName);

            const createResp = page.waitForResponse(r =>
                r.url().endsWith('/api/ai-generations') && r.request().method() === 'POST');
            await aiStudioPage.generate();
            expect((await createResp).status()).toBe(201);
            await expect(page.getByText(cleanDraft1Name).first()).toBeVisible();
        });

        await test.step('edit the selected draft and autosave via PATCH', async () => {
            // Generate auto-selects the first pending draft, but click it explicitly
            // so this step doesn't depend on that default.
            await aiStudioPage.selectDraft(cleanDraft1Name);
            const patchResp = page.waitForResponse(r =>
                /\/api\/ai-generations\/.+\/drafts\/.+$/.test(r.url()) && r.request().method() === 'PATCH');
            await aiStudioPage.draftNameInput.fill(`${cleanDraft1Name} (edited)`);
            expect((await patchResp).status()).toBe(200);
            await expect(aiStudioPage.saveState).toHaveText('Saved');
        });

        await test.step('reject with a structured reason, then restore', async () => {
            await aiStudioPage.selectDraft(cleanDraft2Name);
            await aiStudioPage.openRejectPopover();
            await expect(aiStudioPage.rejectPopover).toBeVisible();
            await aiStudioPage.selectRejectReason('Too vague');

            const rejectResp = page.waitForResponse(r =>
                r.url().includes('/reject') && r.request().method() === 'POST');
            await aiStudioPage.confirmReject();
            expect((await rejectResp).status()).toBe(200);

            const restoreResp = page.waitForResponse(r =>
                r.url().includes('/restore') && r.request().method() === 'POST');
            await aiStudioPage.restoreToPending();
            expect((await restoreResp).status()).toBe(200);
        });

        await test.step('accept all clean excludes the invalid draft', async () => {
            // The header button only opens CommitSummaryModal's confirm phase (no
            // network request fires on this click).
            await aiStudioPage.acceptAllClean();
            const summary = aiStudioPage.commitSummary;
            await expect(summary).toBeVisible();
            await expect(summary).toContainText('2 clean draft(s)');
            await expect(summary).toContainText('1 invalid draft(s) excluded');

            const acceptResp = page.waitForResponse(r => /\/api\/ai-generations\/.+\/accept$/.test(r.url()));
            await aiStudioPage.confirmCommit();
            expect((await acceptResp).status()).toBe(201);

            // Modal flips from the confirm phase to the summary phase on success.
            await expect(summary).toContainText('2 test case(s) created');
            await aiStudioPage.doneCommit();
        });

        await test.step('refresh recovers the session from the persisted run', async () => {
            await page.reload();
            await expect(aiStudioPage.runHistory).toBeVisible();

            // AIGenerateStudio's `filter` state always starts back at "Pending" on
            // a fresh mount, hiding the two drafts accepted above — switch to "All"
            // first. Anchored so it can't match "Accept all clean (1)" (still
            // rendered — the invalid draft is still pending).
            await aiStudioPage.filterDrafts('all');
            await expect(page.getByText(`${cleanDraft1Name} (edited)`).first()).toBeVisible();
        });
    });
});
