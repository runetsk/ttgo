import { test, expect } from '@playwright/test';
import { API_URL } from '../../config.js';
import { createRunAPI } from '../../helpers/api.js';

test.describe('Run assignees', () => {
    test('assign a run and filter the list by assignee', async ({ page, request }) => {
        const stamp = Date.now();
        const email = `assignee${stamp}@example.com`;
        const displayName = `Assignee ${stamp}`;
        let run;

        await test.step('Seed a user and a run', async () => {
            const res = await request.post(`${API_URL}/users`, {
                data: { email, display_name: displayName, password: 'password123', role: 'member' },
            });
            expect(res.ok()).toBeTruthy();
            run = await createRunAPI(request, `Assign Run ${stamp}`);
        });

        await test.step('Assign the run to the user from the detail header', async () => {
            await page.goto(`/runs/run/${run.id}`);
            await page.getByTestId('run-assignee-picker').selectOption({ label: displayName });
            await expect(page.getByTestId('run-assignee-picker')).toHaveValue(/.+/);
        });

        await test.step('The runs list shows and filters by the assignee', async () => {
            await page.goto('/runs');
            await page.getByRole('button', { name: 'Column Filters' }).click();
            await page.getByTestId('filter-run-assignee').selectOption({ label: displayName });
            await expect(page.getByText(`Assign Run ${stamp}`)).toBeVisible();
        });

        await test.step('Unassigned filter hides the assigned run', async () => {
            await page.getByTestId('filter-run-assignee').selectOption('unassigned');
            await expect(page.getByText(`Assign Run ${stamp}`)).not.toBeVisible();
        });
    });
});
