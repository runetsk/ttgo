import { test, expect } from '@playwright/test';
import { createFolderAPI, createTestAPI } from '../../helpers/api.js';

test.describe('Create run from picked tests', () => {
    test('picks two of three tests and lands in the run', async ({ page, request }) => {
        const stamp = Date.now();
        const names = [`PickAlpha ${stamp}`, `PickBeta ${stamp}`, `PickGamma ${stamp}`];
        const created = [];

        await test.step('Seed three test cases via API', async () => {
            const folder = await createFolderAPI(request, 'Picker Folder ' + stamp);
            for (const name of names) {
                created.push(await createTestAPI(request, name, folder.id));
            }
        });

        await test.step('Open the modal and switch to Pick tests', async () => {
            await page.goto('/runs');
            await page.getByTestId('create-test-run-button').click();
            await page.getByTestId('create-run-name-input').fill('Picked Run ' + stamp);
            await page.getByTestId('create-run-source-pick').click();
        });

        await test.step('Search narrows the list; select two tests', async () => {
            await page.getByTestId('create-run-test-search').fill(`PickAlpha ${stamp}`);
            await expect(page.getByTestId(`create-run-test-option-${created[2].id}`)).not.toBeVisible();
            await page.getByTestId(`create-run-test-option-${created[0].id}`).click();
            await page.getByTestId('create-run-test-search').fill(`PickBeta ${stamp}`);
            await page.getByTestId(`create-run-test-option-${created[1].id}`).click();
            await expect(page.getByTestId('create-run-selected-count')).toContainText('2');
        });

        await test.step('Create — the new run holds exactly the picked tests', async () => {
            await page.getByTestId('create-run-submit').click();
            await expect(page).toHaveURL(/\/runs\/run\/[a-f0-9-]+$/);
            await expect(page.getByText(names[0])).toBeVisible();
            await expect(page.getByText(names[1])).toBeVisible();
            await expect(page.getByText(names[2])).not.toBeVisible();
        });
    });
});
