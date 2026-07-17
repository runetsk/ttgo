import { test, expect } from '../../fixtures/test.js';
import { ApiClient } from '../../helpers/api.js';
import { startFakeLLM } from '../../helpers/fake-llm.js';

// Fixed two-test-case envelope the fake LLM returns. It matches the canonical
// {"test_cases":[...]} contract (aigen EnvelopeSchemaJSON: name/category/
// description/source_refs/steps[].action+expected_result, category in
// KnownCategories) so the deterministic draft validator accepts both drafts —
// nothing here is provider-specific, so no @needs-mock tag / external mock.
const ENVELOPE = JSON.stringify({
    test_cases: [
        {
            name: '[Functional] E2E login works',
            category: 'Functional',
            description: 'Happy path.',
            source_refs: [],
            steps: [{ action: 'Log in', expected_result: 'Dashboard shown' }],
        },
        {
            name: '[Negative] E2E wrong password rejected',
            category: 'Negative',
            description: 'Bad creds.',
            source_refs: [],
            steps: [{ action: 'Log in with wrong password', expected_result: 'Error shown' }],
        },
    ],
});

let llm;
let providerId;
let requirementId;
let folderId;
let folderName;

test.describe('AI generation lifecycle — generate, accept, replay', () => {

    test.beforeAll(async ({ request }) => {
        const api = new ApiClient(request);
        llm = await startFakeLLM(api, ENVELOPE);
        providerId = llm.providerId;

        const req = await api.createRequirement(`REQ-E2E-AI-${Date.now()}`, 'E2E login', 'Users log in.');
        requirementId = req.id;

        folderName = `AI E2E ${Date.now()}`;
        const folder = await api.createFolder(folderName);
        folderId = folder.id;
    });

    test.afterAll(async () => {
        if (llm) await llm.dispose();
    });

    test('generate and atomically accept a batch via the lifecycle API', async ({ api }) => {
        let run, drafts;

        await test.step('POST /ai-generations creates a completed run with 2 drafts', async () => {
            const created = await api.createAiGeneration({ requirementId, providerId, idempotencyKey: `e2e-${Date.now()}` });
            expect(created.status()).toBe(201);
            ({ run, drafts } = await created.json());
            expect(run.status).toBe('completed');
            expect(drafts).toHaveLength(2);
        });

        await test.step('POST .../accept atomically materializes both drafts into test cases', async () => {
            const accept = await api.acceptAiGeneration(run.id, { folderId, draftIds: drafts.map(d => d.id) });
            expect(accept.status()).toBe(201);
            const accepted = await accept.json();
            expect(accepted.created_ids).toHaveLength(2);
        });

        await test.step('Replaying the same accept is idempotent (no duplicate test cases)', async () => {
            const replay = await api.acceptAiGeneration(run.id, { folderId, draftIds: drafts.map(d => d.id) });
            expect(replay.status()).toBe(200);
            expect((await replay.json()).already_accepted).toBe(true);
        });

        await test.step('Both test cases are linked to the requirement', async () => {
            // No /requirements/{id}/test-cases endpoint — the detail page reads linked
            // tests off GET /traceability's per-requirement row, so verify the same way.
            const matrix = await api.getTraceability();
            const row = matrix.rows.find(r => r.requirement_id === requirementId);
            expect(row).toBeTruthy();
            expect(row.linked_test_cases).toHaveLength(2);
            expect(row.linked_test_cases.some(tc => tc.test_case_name.includes('E2E login works'))).toBe(true);
        });
    });

    // Isolated in its own describe with its own fake LLM so it generates uniquely
    // named drafts. Reusing the outer provider would regenerate drafts named
    // identically to test 1's already-accepted cases; Stage 3 duplicate detection
    // scores a name-identical draft at similarity 1.0 (past DupHighConfidence 0.9),
    // so "Accept all clean" would exclude both as high-confidence duplicates — 0
    // clean drafts, and the confirm button renders permanently disabled.
    test.describe('Studio UI (isolated provider)', () => {
        // A per-run token keeps these names from colliding with test 1's cases (zero
        // shared words → similarity 0) or any previous run's leftovers on a dirty DB.
        const uiToken = Date.now();
        const UI_ENVELOPE = JSON.stringify({
            test_cases: [
                {
                    name: `[Functional] Studio-UI clean draft ${uiToken} A`,
                    category: 'Functional',
                    description: 'Happy path.',
                    source_refs: [],
                    steps: [{ action: 'Log in', expected_result: 'Dashboard shown' }],
                },
                {
                    name: `[Negative] Studio-UI clean draft ${uiToken} B`,
                    category: 'Negative',
                    description: 'Bad creds.',
                    source_refs: [],
                    steps: [{ action: 'Log in with wrong password', expected_result: 'Error shown' }],
                },
            ],
        });

        let uiLLM;

        test.beforeAll(async ({ request }) => {
            // is_default:true so the studio auto-selects this provider on open. The
            // single-default invariant flips off test 1's provider — harmless, since
            // test 1 (with its explicit provider_id) has already completed.
            uiLLM = await startFakeLLM(new ApiClient(request), UI_ENVELOPE);
        });

        test.afterAll(async () => {
            if (uiLLM) await uiLLM.dispose();
        });

        test('studio UI generates and accepts through the lifecycle endpoints', async ({ page, aiStudioPage }) => {
            await test.step('Enter the studio from the requirement detail page', async () => {
                await aiStudioPage.enterFromRequirement(requirementId);
                await expect(page).toHaveURL(/\/ai-generate/);
            });

            await test.step('Pick the seeded destination folder in the context pane', async () => {
                await aiStudioPage.selectOutputFolder(folderName);
            });

            await test.step('Generate fires the durable create endpoint and renders drafts', async () => {
                const createResp = page.waitForResponse(r =>
                    r.url().endsWith('/api/ai-generations') && r.request().method() === 'POST');
                await aiStudioPage.generate();
                expect((await createResp).status()).toBe(201);

                // Auto-selected, so the draft shows in both the list row and the detail pane.
                await expect(page.getByText(`[Functional] Studio-UI clean draft ${uiToken} A`).first()).toBeVisible();
            });

            await test.step('Accept all clean opens the commit-summary confirm modal', async () => {
                // The header button only opens the confirm modal — no request fires here.
                await aiStudioPage.acceptAllClean();
                await expect(aiStudioPage.commitSummary).toBeVisible();
            });

            await test.step('Confirming in the modal fires the atomic accept endpoint', async () => {
                const acceptResp = page.waitForResponse(r => /\/api\/ai-generations\/.+\/accept$/.test(r.url()));
                await aiStudioPage.confirmCommit();
                expect((await acceptResp).status()).toBe(201);

                await expect(aiStudioPage.commitSummary.getByText(/test case\(s\) created/i)).toBeVisible();
                await aiStudioPage.doneCommit();
            });
        });
    });
});
