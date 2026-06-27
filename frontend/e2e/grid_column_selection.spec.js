import { test, expect } from '@playwright/test';

const LS_KEY_TESTS = 'ttgo_columns_test-cases';
const LS_KEY_REQS  = 'ttgo_columns_requirements';

// ── Helpers ───────────────────────────────────────────────────────────────────

/** Clear column-selection localStorage so each test starts from defaults. */
async function clearColumnPrefs(page) {
    await page.evaluate((keys) => {
        keys.forEach(k => localStorage.removeItem(k));
    }, [LS_KEY_TESTS, LS_KEY_REQS]);
}

/** Create a folder + one test case via UI and navigate back to the grid. */
async function setupFolderAndTest(page) {
    const ts = Date.now();
    const folderName = `ColPicker ${ts}`;
    const testName   = `TC ${ts}`;

    await page.goto('/');
    await page.getByTestId('create-root-folder-button').click();
    await page.getByTestId('modal-input').fill(folderName);
    await page.getByTestId('modal-confirm-button').click();

    const folder = page.getByTestId('folder-name').filter({ hasText: folderName }).first();
    await expect(folder).toBeVisible();
    await folder.click();

    await page.getByTestId('create-test-button').click();
    await page.getByTestId('modal-input').fill(testName);
    await page.getByTestId('modal-confirm-button').click();

    await expect(page.getByTestId('test-table')).toBeVisible();
    return { folderName, testName };
}

// ── Suite ──────────────────────────────────────────────────────────────────────

test.describe('Grid Column Selection — TestGrid (US1 + US2)', () => {

    test.beforeEach(async ({ page }) => {
        await page.goto('/');
        await clearColumnPrefs(page);
    });

    // ── FR-001 / SC-005 ──────────────────────────────────────────────────────

    test('Columns button is visible in grid header', async ({ page }) => {
        await setupFolderAndTest(page);

        const btn = page.getByRole('button', { name: 'Columns' });
        await expect(btn).toBeVisible();
    });

    // ── FR-001 / FR-005 ──────────────────────────────────────────────────────

    test('column picker opens and lists all columns', async ({ page }) => {
        await setupFolderAndTest(page);

        await page.getByRole('button', { name: 'Columns' }).click();
        const popover = page.getByRole('dialog', { name: 'Column visibility' });
        await expect(popover).toBeVisible();

        // All five columns listed
        await expect(popover.getByText('ID')).toBeVisible();
        await expect(popover.getByText('Test Name')).toBeVisible();
        await expect(popover.getByText('Suites')).toBeVisible();
        await expect(popover.getByText('Created')).toBeVisible();
        await expect(popover.getByText('Updated')).toBeVisible();
    });

    // ── FR-005: mandatory column disabled ────────────────────────────────────

    test('mandatory column (Test Name) is shown as disabled in picker', async ({ page }) => {
        await setupFolderAndTest(page);

        await page.getByRole('button', { name: 'Columns' }).click();
        const popover = page.getByRole('dialog', { name: 'Column visibility' });

        // The Test Name row should have a disabled checkbox
        const testNameRow = popover.locator('[aria-disabled="true"]').filter({ hasText: 'Test Name' });
        await expect(testNameRow).toBeVisible();
    });

    // ── FR-002 / FR-003 / SC-001 ─────────────────────────────────────────────

    test('hiding a column removes it from the grid immediately', async ({ page }) => {
        await setupFolderAndTest(page);

        // "Created" column header is visible by default
        await expect(page.locator('thead th').filter({ hasText: 'Created' })).toBeVisible();

        // Open picker and uncheck "Created"
        await page.getByRole('button', { name: 'Columns' }).click();
        const popover = page.getByRole('dialog', { name: 'Column visibility' });
        await popover.locator('[role="checkbox"]').filter({ hasText: 'Created' }).click();

        // Column header disappears immediately (no reload)
        await expect(page.locator('thead th').filter({ hasText: 'Created' })).not.toBeVisible();
    });

    // ── FR-006 / FR-007 / SC-002 ─────────────────────────────────────────────

    test('column preference is persisted and restored after page reload', async ({ page }) => {
        await setupFolderAndTest(page);

        // Hide "Updated"
        await page.getByRole('button', { name: 'Columns' }).click();
        await page.getByRole('dialog', { name: 'Column visibility' })
            .locator('[role="checkbox"]').filter({ hasText: 'Updated' }).click();

        await expect(page.locator('thead th').filter({ hasText: 'Updated' })).not.toBeVisible();

        // Reload — preference should restore
        await page.reload();
        await expect(page.getByTestId('test-table')).toBeVisible();
        await expect(page.locator('thead th').filter({ hasText: 'Updated' })).not.toBeVisible();

        // "Test Name" and "Suites" columns should still be visible
        await expect(page.locator('thead th').filter({ hasText: 'Test Name' })).toBeVisible();
        await expect(page.locator('thead th').filter({ hasText: 'Suites' })).toBeVisible();
    });

    // ── FR-004 / SC-003 ──────────────────────────────────────────────────────

    test('Test Name column cannot be hidden — always visible', async ({ page }) => {
        await setupFolderAndTest(page);

        // Hide all optional columns
        await page.getByRole('button', { name: 'Columns' }).click();
        const popover = page.getByRole('dialog', { name: 'Column visibility' });
        const optionalCols = ['ID', 'Suites', 'Created', 'Updated'];
        for (const label of optionalCols) {
            await popover.locator('[role="checkbox"]').filter({ hasText: label }).click();
        }

        // Test Name still visible
        await expect(page.locator('thead th').filter({ hasText: 'Test Name' })).toBeVisible();

        // All optional headers gone
        for (const label of optionalCols) {
            await expect(page.locator('thead th').filter({ hasText: label })).not.toBeVisible();
        }
    });

    // ── FR-008 / SC-004 ──────────────────────────────────────────────────────

    test('Reset to default restores all columns and clears preference', async ({ page }) => {
        await setupFolderAndTest(page);

        // Hide "ID" and "Created"
        await page.getByRole('button', { name: 'Columns' }).click();
        const popover = page.getByRole('dialog', { name: 'Column visibility' });
        await popover.locator('[role="checkbox"]').filter({ hasText: 'ID' }).click();
        await popover.locator('[role="checkbox"]').filter({ hasText: 'Created' }).click();

        await expect(page.locator('thead th').filter({ hasText: 'ID' })).not.toBeVisible();

        // Click Reset to default (popover is still open from above)
        await page.getByRole('dialog', { name: 'Column visibility' })
            .getByText('Reset to default').click();

        // All columns restored
        await expect(page.locator('thead th').filter({ hasText: 'ID' })).toBeVisible();
        await expect(page.locator('thead th').filter({ hasText: 'Created' })).toBeVisible();

        // Preference cleared — reload should also show all columns
        await page.reload();
        await expect(page.getByTestId('test-table')).toBeVisible();
        await expect(page.locator('thead th').filter({ hasText: 'ID' })).toBeVisible();
        await expect(page.locator('thead th').filter({ hasText: 'Created' })).toBeVisible();
    });

    // ── Picker closes on Escape ────────────────────────────────────────────

    test('picker closes when Escape is pressed', async ({ page }) => {
        await setupFolderAndTest(page);

        await page.getByRole('button', { name: 'Columns' }).click();
        await expect(page.getByRole('dialog', { name: 'Column visibility' })).toBeVisible();

        await page.keyboard.press('Escape');
        await expect(page.getByRole('dialog', { name: 'Column visibility' })).not.toBeVisible();
    });

    // ── Picker closes on click outside ────────────────────────────────────

    test('picker closes when clicking outside', async ({ page }) => {
        await setupFolderAndTest(page);

        await page.getByRole('button', { name: 'Columns' }).click();
        await expect(page.getByRole('dialog', { name: 'Column visibility' })).toBeVisible();

        // Click outside the popover (the page heading area)
        await page.locator('.grid-header h2').click();
        await expect(page.getByRole('dialog', { name: 'Column visibility' })).not.toBeVisible();
    });
});

// ── Requirements grid (US3) ───────────────────────────────────────────────────

test.describe('Grid Column Selection — RequirementsPage (US3)', () => {

    test.beforeEach(async ({ page }) => {
        await page.goto('/requirements');
        await clearColumnPrefs(page);
        await page.reload();
    });

    test('Columns button is visible on Requirements page', async ({ page }) => {
        await expect(page.getByRole('button', { name: 'Columns' })).toBeVisible();
    });

    test('column picker lists Requirements columns', async ({ page }) => {
        await page.getByRole('button', { name: 'Columns' }).click();
        const popover = page.getByRole('dialog', { name: 'Column visibility' });
        await expect(popover).toBeVisible();

        await expect(popover.getByText('Identifier')).toBeVisible();
        await expect(popover.getByText('Requirement')).toBeVisible();
        await expect(popover.getByText('Coverage')).toBeVisible();
    });

    test('hiding Coverage column removes it from Requirements table', async ({ page }) => {
        // Coverage header visible by default
        await expect(page.locator('thead th').filter({ hasText: 'Coverage' })).toBeVisible();

        await page.getByRole('button', { name: 'Columns' }).click();
        await page.getByRole('dialog', { name: 'Column visibility' })
            .locator('[role="checkbox"]').filter({ hasText: 'Coverage' }).click();

        await expect(page.locator('thead th').filter({ hasText: 'Coverage' })).not.toBeVisible();
        // Requirement column always stays
        await expect(page.locator('thead th').filter({ hasText: 'Requirement' })).toBeVisible();
    });

    test('Requirements column preference persists across navigation', async ({ page }) => {
        // Hide "Identifier"
        await page.getByRole('button', { name: 'Columns' }).click();
        await page.getByRole('dialog', { name: 'Column visibility' })
            .locator('[role="checkbox"]').filter({ hasText: 'Identifier' }).click();

        await expect(page.locator('thead th').filter({ hasText: 'Identifier' })).not.toBeVisible();

        // Navigate away and back
        await page.goto('/');
        await page.goto('/requirements');

        await expect(page.locator('thead th').filter({ hasText: 'Identifier' })).not.toBeVisible();
    });

    test('Requirements and TestGrid preferences are stored independently', async ({ page }) => {
        // Hide Identifier on Requirements
        await page.getByRole('button', { name: 'Columns' }).click();
        await page.getByRole('dialog', { name: 'Column visibility' })
            .locator('[role="checkbox"]').filter({ hasText: 'Identifier' }).click();

        // Check localStorage keys are separate
        const keys = await page.evaluate(() => ({
            tests: localStorage.getItem('ttgo_columns_test-cases'),
            reqs:  localStorage.getItem('ttgo_columns_requirements'),
        }));

        // Requirements key should be set, test-cases key should not
        expect(keys.reqs).not.toBeNull();
        expect(keys.tests).toBeNull();
    });
});
