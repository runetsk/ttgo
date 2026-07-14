import { test, expect } from '@playwright/test';
import http from 'node:http';
import { API_URL, createRequirementAPI, createFolderAPI } from '../../helpers/api.js';

// Three-draft envelope for the reviewer-workflow spec: two structurally valid
// drafts (used to exercise edit/autosave and reject/restore) plus one
// deliberately INVALID draft (empty `steps`) so "Accept all clean" has a real
// exclusion to prove. backend/pkg/tracker/aigen/validate.go ValidateDraft
// raises an error-severity finding ("steps"/"no_steps") whenever len(Steps)==0,
// which is exactly what makes draftFlags(...).invalid true and
// isDraftClean(...) false on the frontend (src/utils/draftReview.js) — nothing
// here is provider-specific, so no @needs-mock tag is required (same rationale
// as generate_accept.spec.js).
//
// To ensure idempotency across runs, the two clean drafts' names are tokened
// with Date.now() in beforeAll (mirroring generate_accept.spec.js's UI_ENVELOPE
// pattern) so they never collide with previously accepted test cases in the DB.
// The invalid draft keeps a fixed name (it's never accepted, so no duplicate risk).

let fakeLLM, providerId, requirementId, folderName;
let cleanDraft1Name, cleanDraft2Name;

test.describe('AI reviewer workflow — edit, reject/restore, accept all clean, resume', () => {
    test.beforeAll(async ({ request }) => {
        // Create a per-run token to ensure the two clean drafts' names are
        // unique per run, preventing duplicate-detection collisions with prior
        // runs' accepted test cases on a persistent dev DB.
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

        // Self-hosted fake LLM speaking the OpenAI-compatible chat-completions
        // wire format (see pkg/tracker/llm/openai_compat.go); ignores the
        // request body and always returns the envelope above with tokened names.
        fakeLLM = http.createServer((req, res) => {
            res.writeHead(200, { 'Content-Type': 'application/json' });
            res.end(JSON.stringify({
                model: 'fake-model',
                choices: [{ finish_reason: 'stop', message: { content: ENVELOPE } }],
                usage: { prompt_tokens: 40, completion_tokens: 120, total_tokens: 160 },
            }));
        });
        await new Promise(resolve => fakeLLM.listen(0, '127.0.0.1', resolve));

        // "local" provider type uses the loopback-friendly SSRF guard; is_default
        // wins the server-enforced single-default invariant, so the studio
        // deterministically resolves to THIS provider on open.
        const provider = await request.post(`${API_URL}/settings/llm-providers`, {
            data: {
                label: `e2e-review-llm-${Date.now()}`, provider_type: 'local',
                endpoint_url: `http://127.0.0.1:${fakeLLM.address().port}`,
                model_name: 'fake-model', enabled: true, is_default: true,
            },
        });
        expect(provider.ok()).toBeTruthy();
        providerId = (await provider.json()).id;

        const req = await createRequirementAPI(request, `REQ-E2E-REV-${Date.now()}`, 'Reviewer flow',
            '<h2>Acceptance Criteria</h2><ul><li>User can sign in</li><li>Wrong password shows an error</li></ul>');
        requirementId = req.id;
        folderName = `AI Review E2E ${Date.now()}`;
        await createFolderAPI(request, folderName);
    });

    test.afterAll(async ({ request }) => {
        if (providerId) await request.delete(`${API_URL}/settings/llm-providers/${providerId}`);
        if (fakeLLM) await new Promise(resolve => fakeLLM.close(resolve));
    });

    test('full reviewer loop', async ({ page }) => {
        // This test does substantially more than the suite's 10s default
        // (playwright.config.js) budgets for: a real generate round-trip, the
        // 800ms autosave debounce (draftEditor.jsx AUTOSAVE_MS) plus its PATCH,
        // reject, restore, accept-all-clean, and a full page reload that re-runs
        // the app's rehydrate-on-mount flow (two sequential GETs) end to end.
        // Extend just this test rather than the shared default.
        test.setTimeout(45000);

        await test.step('generate from the requirement page', async () => {
            await page.goto(`/requirements/${requirementId}`);
            await page.getByRole('button', { name: /AI Generate Tests/i }).click();
            await expect(page).toHaveURL(/\/ai-generate/);

            // Output section is collapsed by default (StudioContextPane's `open` state).
            await page.getByRole('button', { name: 'Output' }).click();
            // FolderTreeSelect has no data-testid; `.folder-tree-trigger` is the
            // only one on this page (the AI-import modal isn't open here).
            await page.locator('.folder-tree-trigger').click();
            await page.getByPlaceholder('Search folders…').fill(folderName);
            await page.locator('.folder-tree-row', { hasText: folderName }).click();

            const createResp = page.waitForResponse(r =>
                r.url().endsWith('/api/ai-generations') && r.request().method() === 'POST');
            await page.getByRole('button', { name: /^generate$/i }).click();
            expect((await createResp).status()).toBe(201);
            await expect(page.getByText(cleanDraft1Name).first()).toBeVisible();
        });

        await test.step('edit the selected draft and autosave via PATCH', async () => {
            // Generate auto-selects the first pending draft, but click it
            // explicitly so this step doesn't depend on that default.
            await page.getByText(cleanDraft1Name).first().click();
            const nameInput = page.getByTestId('draft-editor').locator('input').first();
            const patchResp = page.waitForResponse(r =>
                /\/api\/ai-generations\/.+\/drafts\/.+$/.test(r.url()) && r.request().method() === 'PATCH');
            await nameInput.fill(`${cleanDraft1Name} (edited)`);
            expect((await patchResp).status()).toBe(200);
            await expect(page.getByTestId('save-state')).toHaveText('Saved');
        });

        await test.step('reject with a structured reason, then restore', async () => {
            await page.getByText(cleanDraft2Name).first().click();

            // "Reject" alone is not a unique accessible name on this page: every
            // pending row in the drafts list (drafts.jsx DraftRow) also renders
            // its own "Reject" button. Scope to the right-hand detail pane — the
            // one <aside> (besides the left context pane and the app's own nav
            // sidebar) that hosts the draft-editor for the currently selected
            // pending draft — to click the right one.
            const detailPane = page.locator('aside').filter({ has: page.getByTestId('draft-editor') });
            await detailPane.getByRole('button', { name: /^reject$/i }).click();

            await expect(page.getByTestId('reject-popover')).toBeVisible();
            await page.getByTestId('reject-popover').getByRole('button', { name: 'Too vague' }).click();
            const rejectResp = page.waitForResponse(r =>
                r.url().includes('/reject') && r.request().method() === 'POST');
            await page.getByTestId('reject-popover').getByRole('button', { name: /^reject$/i }).click();
            expect((await rejectResp).status()).toBe(200);

            // Rejected drafts render with no row-level actions (DraftRow only
            // shows Accept/Reject while status === 'pending'), so once rejected,
            // "Restore to pending" is unambiguous without scoping.
            const restoreResp = page.waitForResponse(r =>
                r.url().includes('/restore') && r.request().method() === 'POST');
            await page.getByRole('button', { name: /restore to pending/i }).click();
            expect((await restoreResp).status()).toBe(200);
        });

        await test.step('accept all clean excludes the invalid draft', async () => {
            // Since Task 7, the header button only opens CommitSummaryModal's
            // confirm phase (no network request fires on this click).
            await page.getByRole('button', { name: /accept all clean/i }).click();
            const summary = page.getByTestId('commit-summary');
            await expect(summary).toBeVisible();
            await expect(summary).toContainText('2 clean draft(s)');
            await expect(summary).toContainText('1 invalid draft(s) excluded');

            const acceptResp = page.waitForResponse(r => /\/api\/ai-generations\/.+\/accept$/.test(r.url()));
            await summary.getByRole('button', { name: /accept 2 draft/i }).click();
            expect((await acceptResp).status()).toBe(201);

            // Modal flips from the confirm phase to the summary phase on success.
            await expect(summary).toContainText('2 test case(s) created');
            await summary.getByRole('button', { name: 'Done' }).click();
        });

        await test.step('refresh recovers the session from the persisted run', async () => {
            await page.reload();
            await expect(page.getByTestId('run-history')).toBeVisible();

            // AIGenerateStudio's `filter` state always starts back at "Pending"
            // on a fresh mount (it isn't persisted) — that would hide the two
            // drafts accepted above. Switch to "All" first. Anchored so it can't
            // match "Accept all clean (1)" (still rendered — the invalid draft
            // is still pending), which also contains the substring "all".
            await page.getByRole('button', { name: /^all\b/i }).click();
            await expect(page.getByText(`${cleanDraft1Name} (edited)`).first()).toBeVisible();
        });
    });
});
