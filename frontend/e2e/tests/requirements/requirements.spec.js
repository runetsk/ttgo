import { test, expect } from '../../fixtures/test.js';

test.describe('Requirements CRUD', () => {

    test.beforeEach(async ({ api, requirementsPage }) => {
        await api.deleteAllRequirements();
        await requirementsPage.open();
    });

    test('shows empty state when no requirements exist', async ({ page }) => {
        await test.step('Verify the empty state message is shown', async () => {
            await expect(page.getByText('No requirements yet')).toBeVisible();
        });
    });

    test('creates a requirement via modal', async ({ page, requirementsPage }) => {
        const ts = Date.now();
        const identifier = `REQ-${ts}`;
        const title = `Requirement ${ts}`;
        const description = `Description for ${ts}`;

        await test.step('Open the create modal and fill in the requirement fields', async () => {
            await requirementsPage.create({ identifier, title, description });
        });

        await test.step('Verify the new requirement appears in the table', async () => {
            await expect(page.getByText(identifier)).toBeVisible();
            await expect(page.getByText(title)).toBeVisible();
        });
    });

    test('edits a requirement', async ({ page, api, requirementsPage }) => {
        const ts = Date.now();

        await test.step('Create a requirement via API and reload the page', async () => {
            await api.createRequirement(`EDIT-${ts}`, `Original ${ts}`);
            await page.reload();
        });

        await test.step('Open the edit form and update the title', async () => {
            await requirementsPage.rowAction(`EDIT-${ts}`, 'Edit');
            await requirementsPage.titleInput.clear();
            await requirementsPage.titleInput.fill(`Updated ${ts}`);
            await requirementsPage.saveChanges();
        });

        await test.step('Verify the updated title appears', async () => {
            await expect(page.getByText(`Updated ${ts}`)).toBeVisible();
        });
    });

    test('deletes a requirement with confirmation', async ({ page, api, requirementsPage }) => {
        const ts = Date.now();

        await test.step('Create a requirement via API and confirm it is visible', async () => {
            await api.createRequirement(`DEL-${ts}`, `ToDelete ${ts}`);
            await page.reload();
            await expect(page.getByText(`DEL-${ts}`)).toBeVisible();
        });

        await test.step('Accept the confirmation dialog and delete the requirement', async () => {
            page.on('dialog', dialog => dialog.accept());
            await requirementsPage.rowAction(`DEL-${ts}`, 'Delete');
            await expect(page.getByText(`DEL-${ts}`)).not.toBeVisible();
        });
    });

    test('search filters requirements by identifier and title', async ({ page, api, requirementsPage }) => {
        const ts = Date.now();

        await test.step('Create two requirements via API and reload', async () => {
            await api.createRequirement(`ALPHA-${ts}`, `First ${ts}`);
            await api.createRequirement(`BETA-${ts}`, `Second ${ts}`);
            await page.reload();
        });

        await test.step('Filter by identifier and verify only the matching row shows', async () => {
            await requirementsPage.searchInput.fill(`ALPHA-${ts}`);
            await expect(page.getByText(`ALPHA-${ts}`)).toBeVisible();
            await expect(page.getByText(`BETA-${ts}`)).not.toBeVisible();
        });

        await test.step('Clear and filter by title and verify only the matching row shows', async () => {
            await requirementsPage.searchInput.clear();
            await requirementsPage.searchInput.fill(`Second ${ts}`);
            await expect(page.getByText(`Second ${ts}`)).toBeVisible();
            await expect(page.getByText(`First ${ts}`)).not.toBeVisible();
        });
    });

    test('coverage summary cards show correct counts', async ({ page, api }) => {
        const ts = Date.now();

        await test.step('Create two uncovered requirements via API and reload', async () => {
            await api.createRequirement(`COV-A-${ts}`, `CovA ${ts}`);
            await api.createRequirement(`COV-B-${ts}`, `CovB ${ts}`);
            await page.reload();
        });

        await test.step('Verify the coverage summary strip shows the expected counts', async () => {
            // Each stat pill's text content is `${value}${label}` (value span + label span).
            const summary = page.locator('.glass-panel').first();
            await expect(summary.locator('div').filter({ hasText: /^2Total$/ })).toBeVisible();
            await expect(summary.locator('div').filter({ hasText: /^0Covered$/ })).toBeVisible();
            await expect(summary.locator('div').filter({ hasText: /^2Gaps$/ })).toBeVisible();
            await expect(summary.getByText('0%', { exact: true })).toBeVisible();
        });
    });
});
