import { test, expect } from '@playwright/test';
import { deleteAllRequirements, createRequirementAPI } from './helpers.js';

test.describe('Requirements CRUD', () => {

    test.beforeEach(async ({ request, page }) => {
        await deleteAllRequirements(request);
        await page.goto('/requirements');
    });

    // ── 1. Empty state ────────────────────────────────────────────────────────

    test('shows empty state when no requirements exist', async ({ page }) => {
        await test.step('Verify the empty state message is shown', async () => {
            await expect(page.getByText('No requirements yet')).toBeVisible();
        });
    });

    // ── 2. Create ─────────────────────────────────────────────────────────────

    test('creates a requirement via modal', async ({ page }) => {
        const ts = Date.now();
        const identifier = `REQ-${ts}`;
        const title = `Requirement ${ts}`;
        const description = `Description for ${ts}`;

        await test.step('Open the create modal and fill in the requirement fields', async () => {
            await page.getByRole('button', { name: '+ New Requirement' }).click();

            // Fill the create modal
            await page.getByPlaceholder('e.g. PROJ-001').fill(identifier);
            await page.getByPlaceholder('Short description of the requirement').fill(title);
            await page.locator('textarea').fill(description);
            await page.getByRole('button', { name: 'Create', exact: true }).click();
        });

        await test.step('Verify the new requirement appears in the table', async () => {
            // Verify the requirement appears in the table
            await expect(page.getByText(identifier)).toBeVisible();
            await expect(page.getByText(title)).toBeVisible();
        });
    });

    // ── 3. Edit ───────────────────────────────────────────────────────────────

    test('edits a requirement', async ({ request, page }) => {
        const ts = Date.now();

        await test.step('Create a requirement via API and reload the page', async () => {
            await createRequirementAPI(request, `EDIT-${ts}`, `Original ${ts}`);
            await page.reload();
        });

        await test.step('Open the edit form and update the title', async () => {
            // Click the edit button on the row
            const row = page.locator('tr').filter({ hasText: `EDIT-${ts}` });
            await row.getByRole('button', { name: 'Edit' }).click();

            // Update the title
            const titleInput = page.getByPlaceholder('Short description of the requirement');
            await titleInput.clear();
            await titleInput.fill(`Updated ${ts}`);
            await page.getByRole('button', { name: 'Save Changes' }).click();
        });

        await test.step('Verify the updated title appears', async () => {
            // Verify updated title appears
            await expect(page.getByText(`Updated ${ts}`)).toBeVisible();
        });
    });

    // ── 4. Delete ─────────────────────────────────────────────────────────────

    test('deletes a requirement with confirmation', async ({ request, page }) => {
        const ts = Date.now();

        await test.step('Create a requirement via API and confirm it is visible', async () => {
            await createRequirementAPI(request, `DEL-${ts}`, `ToDelete ${ts}`);
            await page.reload();

            await expect(page.getByText(`DEL-${ts}`)).toBeVisible();
        });

        await test.step('Accept the confirmation dialog and delete the requirement', async () => {
            // Accept the confirmation dialog
            page.on('dialog', dialog => dialog.accept());

            const row = page.locator('tr').filter({ hasText: `DEL-${ts}` });
            await row.getByRole('button', { name: 'Delete' }).click();

            // Requirement should disappear
            await expect(page.getByText(`DEL-${ts}`)).not.toBeVisible();
        });
    });

    // ── 5. Search ─────────────────────────────────────────────────────────────

    test('search filters requirements by identifier and title', async ({ request, page }) => {
        const ts = Date.now();
        let searchInput;

        await test.step('Create two requirements via API and reload', async () => {
            await createRequirementAPI(request, `ALPHA-${ts}`, `First ${ts}`);
            await createRequirementAPI(request, `BETA-${ts}`, `Second ${ts}`);
            await page.reload();

            searchInput = page.getByPlaceholder('Search by identifier, title or description…');
        });

        await test.step('Filter by identifier and verify only the matching row shows', async () => {
            // Filter by identifier
            await searchInput.fill(`ALPHA-${ts}`);
            await expect(page.getByText(`ALPHA-${ts}`)).toBeVisible();
            await expect(page.getByText(`BETA-${ts}`)).not.toBeVisible();
        });

        await test.step('Clear and filter by title and verify only the matching row shows', async () => {
            // Clear and filter by title
            await searchInput.clear();
            await searchInput.fill(`Second ${ts}`);
            await expect(page.getByText(`Second ${ts}`)).toBeVisible();
            await expect(page.getByText(`First ${ts}`)).not.toBeVisible();
        });
    });

    // ── 6. Coverage summary cards ─────────────────────────────────────────────

    test('coverage summary cards show correct counts', async ({ request, page }) => {
        const ts = Date.now();

        await test.step('Create two uncovered requirements via API and reload', async () => {
            // Create two requirements, neither linked to test cases → 0 coverage
            await createRequirementAPI(request, `COV-A-${ts}`, `CovA ${ts}`);
            await createRequirementAPI(request, `COV-B-${ts}`, `CovB ${ts}`);
            await page.reload();
        });

        await test.step('Verify the coverage summary cards show the expected counts', async () => {
            // Summary cards: glass-panel divs with value + label
            const cards = page.locator('.glass-panel');
            await expect(cards.filter({ hasText: /^2Total$/ })).toBeVisible();
            await expect(cards.filter({ hasText: /^0Covered$/ })).toBeVisible();
            await expect(cards.filter({ hasText: /^2Gaps$/ })).toBeVisible();
            await expect(cards.filter({ hasText: /^0%Coverage$/ })).toBeVisible();
        });
    });
});
