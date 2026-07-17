import { test, expect } from '../../fixtures/test.js';

test.describe('Run assignees', () => {
    test('assign a run and filter the list by assignee', async ({ page, runsPage, runDetailPage, api }) => {
        const stamp = Date.now();
        const email = `assignee${stamp}@example.com`;
        const displayName = `Assignee ${stamp}`;
        let run;

        await test.step('Seed a user and a run', async () => {
            const res = await api.post('/users', { email, display_name: displayName, password: 'password123', role: 'member' });
            expect(res.ok()).toBeTruthy();
            run = await api.createRun(`Assign Run ${stamp}`);
        });

        await test.step('Assign the run to the user from the detail header', async () => {
            await runDetailPage.open(run.id);
            await runDetailPage.assigneePicker.selectOption({ label: displayName });
            await expect(runDetailPage.assigneePicker).toHaveValue(/.+/);
        });

        await test.step('The runs list shows and filters by the assignee', async () => {
            await runsPage.open();
            await runsPage.openColumnFilters();
            await runsPage.assigneeFilter.selectOption({ label: displayName });
            await expect(page.getByText(`Assign Run ${stamp}`)).toBeVisible();
        });

        await test.step('Unassigned filter hides the assigned run', async () => {
            await runsPage.assigneeFilter.selectOption('unassigned');
            await expect(page.getByText(`Assign Run ${stamp}`)).not.toBeVisible();
        });
    });
});
