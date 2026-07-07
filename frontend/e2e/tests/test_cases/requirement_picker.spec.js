import { test, expect } from '@playwright/test';
import { createFolderAPI, createTestAPI, createRequirementAPI } from '../../helpers/api.js';

test.describe('Requirement picker on the test case page', () => {
    test('rich options with description preview, match highlight, keyboard nav', async ({ page, request }) => {
        const stamp = Date.now();
        let tc, unique;

        await test.step('Seed a test case and several requirements with descriptions', async () => {
            const folder = await createFolderAPI(request, `RP Folder ${stamp}`);
            tc = await createTestAPI(request, `RP Case ${stamp}`, folder.id);
            unique = await createRequirementAPI(request, `PAY-${stamp}-U`, `Refund ${stamp} handling`,
                'Users can request refunds within 30 days of purchase.');
            await createRequirementAPI(request, `PAY-${stamp}-A`, `Checkout ${stamp} totals`,
                'Cart totals include tax and shipping before payment.');
            await createRequirementAPI(request, `PAY-${stamp}-B`, `Checkout ${stamp} guest`,
                'Guests can complete a purchase without an account.');
            await createRequirementAPI(request, `PAY-${stamp}-C`, `Checkout ${stamp} tax`,
                'Tax is calculated from the shipping destination.');
            await createRequirementAPI(request, `PAY-${stamp}-D`, `Checkout ${stamp} shipping`,
                'Shipping options are chosen before the payment step.');
        });

        const input = page.getByTestId('req-search-input');

        await test.step('An option row shows identifier, title, and a description preview', async () => {
            await page.goto(`/library/tests/${tc.id}`);
            await input.click();
            await input.fill(`Refund ${stamp}`);
            const opt = page.getByTestId(`req-option-${unique.id}`);
            await expect(opt).toContainText(`PAY-${stamp}-U`);
            await expect(opt).toContainText(`Refund ${stamp} handling`);
            await expect(opt).toContainText('request refunds within 30 days');
        });

        await test.step('Enter links the highlighted requirement', async () => {
            await input.press('Enter');
            await expect(page.getByTestId(`linked-req-link-${unique.id}`)).toBeVisible();
        });

        await test.step('Arrow keys move the active row across matches', async () => {
            await input.fill(`Checkout ${stamp}`);
            await expect(page.locator('[data-testid^="req-option-"][data-active="true"]')).toHaveCount(1);
            const first = await page.locator('[data-testid^="req-option-"][data-active="true"]').getAttribute('data-testid');
            await input.press('ArrowDown');
            const second = await page.locator('[data-testid^="req-option-"][data-active="true"]').getAttribute('data-testid');
            expect(second).not.toEqual(first);
        });
    });
});
