import { test, expect } from '@playwright/test';
import http from 'node:http';
import { API_URL, createRequirementAPI, createFolderAPI } from '../../helpers/api.js';

// Two-phase fake LLM: the studio's first LLM round-trip (POST /ai-generations,
// fired by the header's "Generate" button) must answer with a one-draft BATCH
// envelope; the second round-trip (RegenSection's "Regenerate" button, which
// hits .../drafts/{id}/regenerate) must answer with a REVISED single-case
// envelope. A shared call counter (closed over by the same fake HTTP server)
// picks the right envelope per call — same OpenAI-compatible chat-completions
// wire shape as generate_accept.spec.js / review_workflow.spec.js.
//
// A per-run token is baked into BOTH names in beforeAll (mirrors those two
// specs' pattern) so re-running this spec against a persistent dev DB never
// collides with a previous run's leftover pending/superseded drafts. This
// spec never calls accept, so SearchDuplicateCandidates (which only scans the
// materialized `test_cases` table) can never actually flag these — but the
// token costs nothing and keeps the pattern consistent with the rest of the
// ai/ e2e suite.
let fakeLLM, providerId, requirementId;

test.describe('AI draft regeneration — compare and choose', () => {
    test.beforeAll(async ({ request }) => {
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

        let calls = 0;
        fakeLLM = http.createServer((req, res) => {
            calls++;
            res.writeHead(200, { 'Content-Type': 'application/json' });
            res.end(JSON.stringify({
                model: 'fake-model',
                choices: [{ finish_reason: 'stop', message: { content: calls === 1 ? BATCH : REVISED } }],
                usage: { prompt_tokens: 30, completion_tokens: 90, total_tokens: 120 },
            }));
        });
        await new Promise(resolve => fakeLLM.listen(0, '127.0.0.1', resolve));

        // "local" provider type uses the loopback-friendly SSRF guard; is_default
        // wins the server-enforced single-default invariant, so the studio
        // deterministically resolves to THIS provider on open (same mechanism
        // as generate_accept.spec.js / review_workflow.spec.js).
        const provider = await request.post(`${API_URL}/settings/llm-providers`, {
            data: {
                label: `e2e-regen-llm-${token}`, provider_type: 'local',
                endpoint_url: `http://127.0.0.1:${fakeLLM.address().port}`,
                model_name: 'fake-model', enabled: true, is_default: true,
            },
        });
        expect(provider.ok()).toBeTruthy();
        providerId = (await provider.json()).id;

        const req = await createRequirementAPI(request, `REQ-E2E-REGEN-${token}`, 'Regen flow',
            '<ul><li>User can sign in</li></ul>');
        requirementId = req.id;

        // Not selected anywhere below — startGeneration's payload has no
        // folder_id (folders only matter at accept time, and this spec never
        // accepts), but seeding one mirrors the sibling specs and keeps the
        // studio's folder-tree state (auto-defaulted to the first folder) sane
        // on a instance that might otherwise have zero folders.
        await createFolderAPI(request, `AI Regen E2E ${token}`);
    });

    test.afterAll(async ({ request }) => {
        if (providerId) await request.delete(`${API_URL}/settings/llm-providers/${providerId}`);
        if (fakeLLM) await new Promise(resolve => fakeLLM.close(resolve));
    });

    test('regenerate, compare, choose the new version', async ({ page }) => {
        // Two full LLM round-trips (generate + regenerate) plus the compare/choose
        // UI interactions comfortably exceed the suite's 10s default
        // (playwright.config.js). Extend just this test, same as
        // review_workflow.spec.js's full reviewer loop.
        test.setTimeout(45000);

        await test.step('generate the batch', async () => {
            // Direct entry button (RequirementDetailPage.jsx "✨ AI Generate Tests").
            await page.goto(`/requirements/${requirementId}`);
            await page.getByRole('button', { name: /AI Generate Tests/i }).click();
            const createResp = page.waitForResponse(r =>
                r.url().endsWith('/api/ai-generations') && r.request().method() === 'POST');
            await page.getByRole('button', { name: /^generate$/i }).click();
            expect((await createResp).status()).toBe(201);
            // Substring match — draft.name carries the "[Functional] " prefix and
            // the per-run token suffix; both list row and (auto-selected) detail
            // pane render it, hence .first().
            await expect(page.getByText('Regen target draft').first()).toBeVisible();
        });

        await test.step('regenerate the draft with an instruction', async () => {
            await page.getByText('Regen target draft').first().click();
            // RegenSection's free-form input (drafts.jsx): placeholder "Optional
            // instruction, e.g. cover the lockout rule".
            await page.getByPlaceholder(/optional instruction/i).fill('sharpen the credentials');
            const regenResp = page.waitForResponse(r => r.url().includes('/regenerate'));
            // Since generation above flips ai.hasGenerated, the STUDIO HEADER's own
            // button now also reads "Regenerate" (StudioHeader: ai.hasGenerated ?
            // 'Regenerate' : 'Generate') — scope to the regen-section testid so this
            // click can't land on that other button.
            await page.getByTestId('regen-section').getByRole('button', { name: /^regenerate$/i }).click();
            expect((await regenResp).status()).toBe(201);
            await expect(page.getByTestId('draft-compare')).toBeVisible();
        });

        await test.step('choose the new version — original becomes superseded', async () => {
            const chooseResp = page.waitForResponse(r => r.url().includes('/choose'));
            await page.getByRole('button', { name: /use new version/i }).click();
            expect((await chooseResp).status()).toBe(200);
            // The alternative (now the only pending draft at this position) is the
            // one rendered in the center list, so its "(sharpened)" name shows there.
            await expect(page.getByText('(sharpened)').first()).toBeVisible();
            // selectedDraftId never moves off the original (choosing doesn't
            // reselect), so the detail pane keeps showing it — now with
            // status "superseded": the StatusPill reads "Superseded" and the
            // detail body adds "This version was superseded."; either satisfies
            // this assertion.
            await expect(page.getByText(/superseded/i).first()).toBeVisible();
        });
    });
});
