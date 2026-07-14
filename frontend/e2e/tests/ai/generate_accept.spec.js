import { test, expect } from '@playwright/test';
import http from 'node:http';
import { API_URL, createRequirementAPI, createFolderAPI } from '../../helpers/api.js';

// Fixed two-test-case envelope returned by the self-hosted fake LLM below. It
// matches the canonical {"test_cases":[...]} contract (pkg/tracker/aigen
// EnvelopeSchemaJSON: name/category/description/source_refs/steps[].action+
// expected_result, category in aigen.KnownCategories) so the backend's
// deterministic draft validator (aigen.ValidateDraft) accepts both drafts
// cleanly — nothing here is provider-specific, so no @needs-mock tag/external
// mock server is required.
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

let fakeLLM;
let fakeLLMURL;
let providerId;
let requirementId;
let folderId;
let folderName;

test.describe('AI generation lifecycle — generate, accept, replay', () => {

    test.beforeAll(async ({ request }) => {
        // 1. Self-hosted fake LLM speaking the OpenAI-compatible chat-completions
        // wire format (backend calls POST {endpoint_url}/v1/chat/completions —
        // see pkg/tracker/llm/openai_compat.go). It ignores the request body and
        // always returns the fixed envelope above, regardless of path.
        fakeLLM = http.createServer((req, res) => {
            res.writeHead(200, { 'Content-Type': 'application/json' });
            res.end(JSON.stringify({
                model: 'fake-model',
                choices: [{ finish_reason: 'stop', message: { content: ENVELOPE } }],
                usage: { prompt_tokens: 50, completion_tokens: 150, total_tokens: 200 },
            }));
        });
        await new Promise(resolve => fakeLLM.listen(0, '127.0.0.1', resolve));
        fakeLLMURL = `http://127.0.0.1:${fakeLLM.address().port}`;

        // 2. Register it as a "local" provider via the admin session from
        // storageState. provider_type "local" uses the SSRF integration guard
        // (safehttp.ValidateIntegrationURL), which allows loopback — the strict
        // guard used for cloud providers would reject 127.0.0.1. is_default:true
        // wins the server-enforced single-default invariant (CreateProvider ->
        // SetDefaultProviderConfig clears every other row's is_default in the
        // same transaction), so the studio deterministically resolves to THIS
        // provider on open regardless of whatever else is configured on this
        // (possibly shared/dirty) dev instance.
        const provider = await request.post(`${API_URL}/settings/llm-providers`, {
            data: {
                label: `e2e-fake-llm-${Date.now()}`, provider_type: 'local',
                endpoint_url: fakeLLMURL, model_name: 'fake-model', enabled: true, is_default: true,
            },
        });
        expect(provider.ok()).toBeTruthy();
        providerId = (await provider.json()).id;

        // 3. Seed a requirement and a destination folder.
        const req = await createRequirementAPI(request, `REQ-E2E-AI-${Date.now()}`, 'E2E login', 'Users log in.');
        requirementId = req.id;

        folderName = `AI E2E ${Date.now()}`;
        const folder = await createFolderAPI(request, folderName);
        folderId = folder.id;
    });

    test.afterAll(async ({ request }) => {
        if (providerId) await request.delete(`${API_URL}/settings/llm-providers/${providerId}`);
        if (fakeLLM) await new Promise(resolve => fakeLLM.close(resolve));
    });

    test('generate and atomically accept a batch via the lifecycle API', async ({ request }) => {
        let run, drafts;

        await test.step('POST /ai-generations creates a completed run with 2 drafts', async () => {
            const created = await request.post(`${API_URL}/ai-generations`, {
                data: { requirement_id: requirementId, provider_id: providerId, idempotency_key: `e2e-${Date.now()}` },
            });
            expect(created.status()).toBe(201);
            ({ run, drafts } = await created.json());
            expect(run.status).toBe('completed');
            expect(drafts).toHaveLength(2);
        });

        await test.step('POST .../accept atomically materializes both drafts into test cases', async () => {
            const accept = await request.post(`${API_URL}/ai-generations/${run.id}/accept`, {
                data: { folder_id: folderId, draft_ids: drafts.map(d => d.id), group_by_category: true },
            });
            expect(accept.status()).toBe(201);
            const accepted = await accept.json();
            expect(accepted.created_ids).toHaveLength(2);
        });

        await test.step('Replaying the same accept is idempotent (no duplicate test cases)', async () => {
            const replay = await request.post(`${API_URL}/ai-generations/${run.id}/accept`, {
                data: { folder_id: folderId, draft_ids: drafts.map(d => d.id), group_by_category: true },
            });
            expect(replay.status()).toBe(200);
            expect((await replay.json()).already_accepted).toBe(true);
        });

        await test.step('Both test cases are linked to the requirement', async () => {
            // There is no /requirements/{id}/test-cases endpoint — RequirementDetailPage
            // itself reads linked tests off GET /traceability's per-requirement row
            // (matrixData.rows[i].linked_test_cases), so verify the same way.
            const matrix = await request.get(`${API_URL}/traceability`);
            expect(matrix.ok()).toBeTruthy();
            const row = (await matrix.json()).rows.find(r => r.requirement_id === requirementId);
            expect(row).toBeTruthy();
            expect(row.linked_test_cases).toHaveLength(2);
            expect(row.linked_test_cases.some(tc => tc.test_case_name.includes('E2E login works'))).toBe(true);
        });
    });

    // Isolated in its own describe so this test gets its own fake LLM + default
    // provider, scoped with a dedicated beforeAll/afterAll. Reusing the outer
    // provider/envelope would regenerate drafts named identically to test 1's
    // already-accepted "[Functional] E2E login works" / "[Negative] E2E wrong
    // password rejected" test cases — Stage 3's existing-test duplicate
    // detection (aigen.SearchDuplicateCandidates / TokenSimilarity) scores a
    // name-identical draft at similarity 1.0, well past aigen.DupHighConfidence
    // (0.9), so Stage 4's "Accept all clean" (isDraftClean) correctly excludes
    // both as high-confidence duplicates — 0 clean drafts, and the
    // commit-summary confirm button permanently renders `disabled`. Giving
    // this test its own provider producing uniquely-named, non-duplicate
    // drafts fixes the test without weakening what "clean" means.
    test.describe('Studio UI (isolated provider)', () => {
        // A per-run token keeps these names from colliding with test 1's cases
        // (zero shared words → similarity 0) and with any previous run's
        // leftovers on a persistent dev DB (the differing token drops any
        // reused wording well below the 0.9 high-confidence threshold).
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

        let uiFakeLLM;
        let uiProviderId;

        test.beforeAll(async ({ request }) => {
            // Same self-hosted fake OpenAI-compatible server as the outer
            // beforeAll, just answering with UI_ENVELOPE instead of ENVELOPE.
            uiFakeLLM = http.createServer((req, res) => {
                res.writeHead(200, { 'Content-Type': 'application/json' });
                res.end(JSON.stringify({
                    model: 'fake-model',
                    choices: [{ finish_reason: 'stop', message: { content: UI_ENVELOPE } }],
                    usage: { prompt_tokens: 50, completion_tokens: 150, total_tokens: 200 },
                }));
            });
            await new Promise(resolve => uiFakeLLM.listen(0, '127.0.0.1', resolve));
            const uiFakeLLMURL = `http://127.0.0.1:${uiFakeLLM.address().port}`;

            // is_default:true so the studio auto-selects this provider on
            // openSession (AIGenerationContext.jsx picks
            // `enabled.find(p => p.is_default) || enabled[0]`), same mechanism
            // as the outer provider. The backend's single-default invariant
            // means this flips off test 1's provider — harmless, since test 1
            // (and its explicit provider_id) has already completed by the time
            // this nested beforeAll runs (workers:1, fullyParallel:false, and
            // Playwright runs a nested describe's beforeAll only once
            // execution reaches its first test).
            const provider = await request.post(`${API_URL}/settings/llm-providers`, {
                data: {
                    label: `e2e-fake-llm-ui-${uiToken}`, provider_type: 'local',
                    endpoint_url: uiFakeLLMURL, model_name: 'fake-model', enabled: true, is_default: true,
                },
            });
            expect(provider.ok()).toBeTruthy();
            uiProviderId = (await provider.json()).id;
        });

        test.afterAll(async ({ request }) => {
            if (uiProviderId) await request.delete(`${API_URL}/settings/llm-providers/${uiProviderId}`);
            if (uiFakeLLM) await new Promise(resolve => uiFakeLLM.close(resolve));
        });

        test('studio UI generates and accepts through the lifecycle endpoints', async ({ page }) => {
            await test.step('Enter the studio from the requirement detail page', async () => {
                // Direct entry button (RequirementDetailPage.jsx ~line 258), simpler and
                // more robust than the requirements-list row kebab menu.
                await page.goto(`/requirements/${requirementId}`);
                await page.getByRole('button', { name: /AI Generate Tests/i }).click();
                await expect(page).toHaveURL(/\/ai-generate/);
            });

            await test.step('Pick the seeded destination folder in the context pane', async () => {
                // The folder picker lives in the Output section, which is collapsed by
                // default (StudioContextPane's `open` state) — expand it first.
                await page.getByRole('button', { name: 'Output' }).click();
                // FolderTreeSelect has no data-testid; its trigger is uniquely identified
                // by this class on this page (the only other consumer, the AI-import
                // modal, isn't open here).
                await page.locator('.folder-tree-trigger').click();
                await page.getByPlaceholder('Search folders…').fill(folderName);
                // Scope to the dropdown row (.folder-tree-row), not just matching text —
                // the trigger button itself already displays the folder name once
                // selected (auto-defaulted to the first folder in the tree, which may
                // already be this one), so a plain text locator matches twice.
                await page.locator('.folder-tree-row', { hasText: folderName }).click();
            });

            await test.step('Generate fires the durable create endpoint and renders drafts', async () => {
                const createResp = page.waitForResponse(r =>
                    r.url().endsWith('/api/ai-generations') && r.request().method() === 'POST');
                await page.getByRole('button', { name: /^generate$/i }).click();
                expect((await createResp).status()).toBe(201);

                // Drafts render from the durable run (appears both in the list row and,
                // since it's auto-selected, the detail pane — hence .first()).
                await expect(page.getByText(`[Functional] Studio-UI clean draft ${uiToken} A`).first()).toBeVisible();
            });

            await test.step('Accept all clean opens the commit-summary confirm modal', async () => {
                // Since Task 7, the header button no longer accepts immediately — it
                // only opens CommitSummaryModal's confirm phase (no network request
                // fires here). "Accept all clean (2)" still matches /accept all/i.
                await page.getByRole('button', { name: /accept all/i }).click();
                await expect(page.getByTestId('commit-summary')).toBeVisible();
            });

            await test.step('Confirming in the modal fires the atomic accept endpoint', async () => {
                const acceptResp = page.waitForResponse(r => /\/api\/ai-generations\/.+\/accept$/.test(r.url()));
                // Confirm-phase button reads "Accept {plan.clean.length} draft(s)" —
                // both drafts are unique (no high-confidence duplicate against test 1's
                // accepted cases, or against each other) and structurally valid, so
                // clean.length is 2.
                await page.getByTestId('commit-summary').getByRole('button', { name: /accept \d+ draft/i }).click();
                expect((await acceptResp).status()).toBe(201);

                // Modal flips from the confirm phase to the summary phase on success.
                await expect(page.getByTestId('commit-summary').getByText(/test case\(s\) created/i)).toBeVisible();
                await page.getByTestId('commit-summary').getByRole('button', { name: 'Done' }).click();
            });
        });
    });
});
