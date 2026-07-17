import { test, expect } from '../../fixtures/test.js';

test.describe('Requirement picker on the test case page', () => {
    test('rich options with description preview, match highlight, keyboard nav', async ({ api, testCaseDetailPage }) => {
        const stamp = Date.now();
        let tc, unique;

        await test.step('Seed a test case and several requirements with descriptions', async () => {
            const folder = await api.createFolder(`RP Folder ${stamp}`);
            tc = await api.createTest(`RP Case ${stamp}`, folder.id);
            unique = await api.createRequirement(`PAY-${stamp}-U`, `Refund ${stamp} handling`,
                'Users can request refunds within 30 days of purchase.');
            await api.createRequirement(`PAY-${stamp}-A`, `Checkout ${stamp} totals`,
                'Cart totals include tax and shipping before payment.');
            await api.createRequirement(`PAY-${stamp}-B`, `Checkout ${stamp} guest`,
                'Guests can complete a purchase without an account.');
            await api.createRequirement(`PAY-${stamp}-C`, `Checkout ${stamp} tax`,
                'Tax is calculated from the shipping destination.');
            await api.createRequirement(`PAY-${stamp}-D`, `Checkout ${stamp} shipping`,
                'Shipping options are chosen before the payment step.');
        });

        const input = testCaseDetailPage.reqSearchInput;

        await test.step('An option row shows identifier, title, and a description preview', async () => {
            await testCaseDetailPage.open(tc.id);
            await input.click();
            await input.fill(`Refund ${stamp}`);
            const opt = testCaseDetailPage.reqOption(unique.id);
            await expect(opt).toContainText(`PAY-${stamp}-U`);
            await expect(opt).toContainText(`Refund ${stamp} handling`);
            await expect(opt).toContainText('request refunds within 30 days');
        });

        await test.step('Enter links the highlighted requirement', async () => {
            await input.press('Enter');
            await expect(testCaseDetailPage.linkedReqLink(unique.id)).toBeVisible();
        });

        await test.step('Arrow keys move the active row across matches', async () => {
            await input.fill(`Checkout ${stamp}`);
            await expect(testCaseDetailPage.activeReqOption).toHaveCount(1);
            const first = await testCaseDetailPage.activeReqOption.getAttribute('data-testid');
            await input.press('ArrowDown');
            const second = await testCaseDetailPage.activeReqOption.getAttribute('data-testid');
            expect(second).not.toEqual(first);
        });
    });
});
