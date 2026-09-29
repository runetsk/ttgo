import { test, expect } from '../../fixtures/test.js';

// Wave 5: TypeSafe.ai beyond failure analysis — defect assist in the Defects page's create dialog,
// and the "Other uses of TypeSafe" switches in Settings → AI. Every TypeSafe endpoint and AI
// setting is mocked with page.route; nothing is sent to TypeSafe and nothing is saved.

const json = (body, status = 200) => ({ status, contentType: 'application/json', body: JSON.stringify(body) });
const USES_ON = { import_structure: true, draft_review: true, defect_assist: true, search_rerank: false };

test.describe('TypeSafe.ai — other uses', () => {
    test('defect assist suggests a severity, applies it on click and links a possible duplicate', async ({ page, defectsPage }) => {
        const asked = [];
        await page.route(/\/api\/ai\/typesafe\/features$/, (r) => r.fulfill(json(USES_ON)));
        await page.route(/\/api\/defects\/assist$/, (r) => {
            asked.push(r.request().postDataJSON());
            return r.fulfill(json({
                severity: { value: 'major', confidence: 0.84, probabilities: { critical: 0.05, major: 0.84, minor: 0.09, trivial: 0.02 } },
                duplicates: [{ defect_id: 'def-42', title: 'Checkout total wrong after discount', status: 'open', external_key: 'PAY-42', p_same: 0.91 }],
            }));
        });
        await defectsPage.open();
        await defectsPage.newDefectButton.click();
        await expect(page.getByTestId('defect-assist-check')).toBeDisabled(); // no title yet
        await defectsPage.modalTitle.fill('Checkout total is wrong after a discount');
        await page.getByTestId('defect-assist-check').click();

        await expect(page.getByTestId('defect-assist-severity')).toContainText('Suggested severity: major (84%)');
        expect(asked).toHaveLength(1);
        expect(asked[0].title).toBe('Checkout total is wrong after a discount');
        await expect(defectsPage.modalSeverity).toHaveValue('minor'); // nothing changes without a click
        await page.getByTestId('defect-assist-apply').click();
        await expect(defectsPage.modalSeverity).toHaveValue('major');
        await expect(page.getByTestId('defect-assist-apply')).toHaveCount(0);

        const dup = page.getByTestId('defect-assist-dup-def-42');
        await expect(dup).toHaveText('PAY-42 · Checkout total wrong after discount (91% likely the same problem)');
        await expect(dup).toHaveAttribute('href', '/defects?focus=def-42');
        await defectsPage.modalCancel.click();
    });

    test('without defect assist the form has no TypeSafe check', async ({ page, defectsPage }) => {
        await page.route(/\/api\/ai\/typesafe\/features$/, (r) => r.fulfill(json({ ...USES_ON, defect_assist: false })));
        await defectsPage.open();
        await defectsPage.newDefectButton.click();
        await expect(defectsPage.modalSeverity).toBeVisible();
        await expect(page.getByTestId('defect-assist')).toHaveCount(0);
        await defectsPage.modalCancel.click();
    });

    test('the TypeSafe card lists the other uses with their switches', async ({ page, settingsPage }) => {
        const TS = {
            id: 'singleton', enabled: true, api_key_masked: '…1234', api_key_status: 'ok', model: 'jev-1.13.0', timeout_seconds: 30,
            price_per_mtok: 0.042, verdict_engine_enabled: true, narrative_enabled: true, llm_fallback_enabled: true, escalate_below_pct: 0,
            semantic_dedup_enabled: true, allow_auto_failure_analysis: false,
            import_structure_enabled: true, draft_review_enabled: true, defect_assist_enabled: false, search_rerank_enabled: false,
        };
        await page.route(/\/api\/settings\/typesafe$/, (r) => (r.request().method() === 'GET' ? r.fulfill(json(TS)) : r.fulfill(json({ error: 'no saves' }, 500))));
        await settingsPage.open();
        await settingsPage.openTypeSafeCard();
        for (const [field, on] of Object.entries({ import_structure_enabled: true, draft_review_enabled: true, defect_assist_enabled: false, search_rerank_enabled: false })) {
            const toggle = page.getByTestId(`typesafe-${field}`);
            await expect(toggle).toBeVisible();
            if (on) await expect(toggle).toBeChecked();
            else await expect(toggle).not.toBeChecked();
        }
        await page.getByTestId('typesafe-defect_assist_enabled').check();
        await expect(page.getByTestId('ai-savebar-save')).toBeEnabled(); // the switch counts as an unsaved change
    });
});
