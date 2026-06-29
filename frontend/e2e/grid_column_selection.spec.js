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
        await test.step('Set up a folder with one test', async () => {
            await setupFolderAndTest(page);
        });

        await test.step('Verify the Columns button is visible in the grid header', async () => {
            const btn = page.getByRole('button', { name: 'Columns' });
            await expect(btn).toBeVisible();
        });
    });

    // ── FR-001 / FR-005 ──────────────────────────────────────────────────────

    test('column picker opens and lists all columns', async ({ page }) => {
        await test.step('Set up a folder with one test', async () => {
            await setupFolderAndTest(page);
        });

        await test.step('Open the column picker and verify all five columns are listed', async () => {
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
    });

    // ── FR-005: mandatory column disabled ────────────────────────────────────

    test('mandatory column (Test Name) is shown as disabled in picker', async ({ page }) => {
        await test.step('Set up a folder with one test', async () => {
            await setupFolderAndTest(page);
        });

        await test.step('Open the picker and verify the Test Name row is disabled', async () => {
            await page.getByRole('button', { name: 'Columns' }).click();
            const popover = page.getByRole('dialog', { name: 'Column visibility' });

            // The Test Name row should have a disabled checkbox
            const testNameRow = popover.locator('[aria-disabled="true"]').filter({ hasText: 'Test Name' });
            await expect(testNameRow).toBeVisible();
        });
    });

    // ── FR-002 / FR-003 / SC-001 ─────────────────────────────────────────────

    test('hiding a column removes it from the grid immediately', async ({ page }) => {
        await test.step('Set up a folder with one test', async () => {
            await setupFolderAndTest(page);
        });

        await test.step('Verify the Created column header is visible by default', async () => {
            await expect(page.locator('thead th').filter({ hasText: 'Created' })).toBeVisible();
        });

        await test.step('Open the picker and uncheck Created, verifying the header disappears immediately', async () => {
            await page.getByRole('button', { name: 'Columns' }).click();
            const popover = page.getByRole('dialog', { name: 'Column visibility' });
            await popover.locator('[role="checkbox"]').filter({ hasText: 'Created' }).click();

            // Column header disappears immediately (no reload)
            await expect(page.locator('thead th').filter({ hasText: 'Created' })).not.toBeVisible();
        });
    });

    // ── FR-006 / FR-007 / SC-002 ─────────────────────────────────────────────

    test('column preference is persisted and restored after page reload', async ({ page }) => {
        await test.step('Set up a folder with one test', async () => {
            await setupFolderAndTest(page);
        });

        await test.step('Hide the Updated column and verify the header disappears', async () => {
            await page.getByRole('button', { name: 'Columns' }).click();
            await page.getByRole('dialog', { name: 'Column visibility' })
                .locator('[role="checkbox"]').filter({ hasText: 'Updated' }).click();

            await expect(page.locator('thead th').filter({ hasText: 'Updated' })).not.toBeVisible();
        });

        await test.step('Reload and verify the preference is restored with other columns visible', async () => {
            await page.reload();
            await expect(page.getByTestId('test-table')).toBeVisible();
            await expect(page.locator('thead th').filter({ hasText: 'Updated' })).not.toBeVisible();

            // "Test Name" and "Suites" columns should still be visible
            await expect(page.locator('thead th').filter({ hasText: 'Test Name' })).toBeVisible();
            await expect(page.locator('thead th').filter({ hasText: 'Suites' })).toBeVisible();
        });
    });

    // ── FR-004 / SC-003 ──────────────────────────────────────────────────────

    test('Test Name column cannot be hidden — always visible', async ({ page }) => {
        const optionalCols = ['ID', 'Suites', 'Created', 'Updated'];

        await test.step('Set up a folder with one test', async () => {
            await setupFolderAndTest(page);
        });

        await test.step('Hide all optional columns', async () => {
            await page.getByRole('button', { name: 'Columns' }).click();
            const popover = page.getByRole('dialog', { name: 'Column visibility' });
            for (const label of optionalCols) {
                await popover.locator('[role="checkbox"]').filter({ hasText: label }).click();
            }
        });

        await test.step('Verify Test Name is still visible', async () => {
            await expect(page.locator('thead th').filter({ hasText: 'Test Name' })).toBeVisible();
        });

        await test.step('Verify all optional headers are gone', async () => {
            for (const label of optionalCols) {
                await expect(page.locator('thead th').filter({ hasText: label })).not.toBeVisible();
            }
        });
    });

    // ── FR-008 / SC-004 ──────────────────────────────────────────────────────

    test('Reset to default restores all columns and clears preference', async ({ page }) => {
        await test.step('Set up a folder with one test', async () => {
            await setupFolderAndTest(page);
        });

        await test.step('Hide the ID and Created columns', async () => {
            await page.getByRole('button', { name: 'Columns' }).click();
            const popover = page.getByRole('dialog', { name: 'Column visibility' });
            await popover.locator('[role="checkbox"]').filter({ hasText: 'ID' }).click();
            await popover.locator('[role="checkbox"]').filter({ hasText: 'Created' }).click();

            await expect(page.locator('thead th').filter({ hasText: 'ID' })).not.toBeVisible();
        });

        await test.step('Click Reset to default and verify all columns are restored', async () => {
            // Click Reset to default (popover is still open from above)
            await page.getByRole('dialog', { name: 'Column visibility' })
                .getByText('Reset to default').click();

            // All columns restored
            await expect(page.locator('thead th').filter({ hasText: 'ID' })).toBeVisible();
            await expect(page.locator('thead th').filter({ hasText: 'Created' })).toBeVisible();
        });

        await test.step('Reload and verify the cleared preference shows all columns', async () => {
            await page.reload();
            await expect(page.getByTestId('test-table')).toBeVisible();
            await expect(page.locator('thead th').filter({ hasText: 'ID' })).toBeVisible();
            await expect(page.locator('thead th').filter({ hasText: 'Created' })).toBeVisible();
        });
    });

    // ── Picker closes on Escape ────────────────────────────────────────────

    test('picker closes when Escape is pressed', async ({ page }) => {
        await test.step('Set up a folder with one test', async () => {
            await setupFolderAndTest(page);
        });

        await test.step('Open the picker and verify it is visible', async () => {
            await page.getByRole('button', { name: 'Columns' }).click();
            await expect(page.getByRole('dialog', { name: 'Column visibility' })).toBeVisible();
        });

        await test.step('Press Escape and verify the picker closes', async () => {
            await page.keyboard.press('Escape');
            await expect(page.getByRole('dialog', { name: 'Column visibility' })).not.toBeVisible();
        });
    });

    // ── Picker closes on click outside ────────────────────────────────────

    test('picker closes when clicking outside', async ({ page }) => {
        await test.step('Set up a folder with one test', async () => {
            await setupFolderAndTest(page);
        });

        await test.step('Open the picker and verify it is visible', async () => {
            await page.getByRole('button', { name: 'Columns' }).click();
            await expect(page.getByRole('dialog', { name: 'Column visibility' })).toBeVisible();
        });

        await test.step('Click outside the popover and verify the picker closes', async () => {
            // Click outside the popover (the page heading area)
            await page.locator('.grid-header h2').click();
            await expect(page.getByRole('dialog', { name: 'Column visibility' })).not.toBeVisible();
        });
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
        await test.step('Verify the Columns button is visible on the Requirements page', async () => {
            await expect(page.getByRole('button', { name: 'Columns' })).toBeVisible();
        });
    });

    test('column picker lists Requirements columns', async ({ page }) => {
        await test.step('Open the picker and verify the Requirements columns are listed', async () => {
            await page.getByRole('button', { name: 'Columns' }).click();
            const popover = page.getByRole('dialog', { name: 'Column visibility' });
            await expect(popover).toBeVisible();

            await expect(popover.getByText('Identifier')).toBeVisible();
            await expect(popover.getByText('Requirement')).toBeVisible();
            await expect(popover.getByText('Coverage')).toBeVisible();
        });
    });

    test('hiding Coverage column removes it from Requirements table', async ({ page }) => {
        await test.step('Verify the Coverage header is visible by default', async () => {
            await expect(page.locator('thead th').filter({ hasText: 'Coverage' })).toBeVisible();
        });

        await test.step('Hide Coverage and verify it disappears while Requirement stays', async () => {
            await page.getByRole('button', { name: 'Columns' }).click();
            await page.getByRole('dialog', { name: 'Column visibility' })
                .locator('[role="checkbox"]').filter({ hasText: 'Coverage' }).click();

            await expect(page.locator('thead th').filter({ hasText: 'Coverage' })).not.toBeVisible();
            // Requirement column always stays
            await expect(page.locator('thead th').filter({ hasText: 'Requirement' })).toBeVisible();
        });
    });

    test('Requirements column preference persists across navigation', async ({ page }) => {
        await test.step('Hide the Identifier column and verify it disappears', async () => {
            await page.getByRole('button', { name: 'Columns' }).click();
            await page.getByRole('dialog', { name: 'Column visibility' })
                .locator('[role="checkbox"]').filter({ hasText: 'Identifier' }).click();

            await expect(page.locator('thead th').filter({ hasText: 'Identifier' })).not.toBeVisible();
        });

        await test.step('Navigate away and back, verifying the preference persists', async () => {
            await page.goto('/');
            await page.goto('/requirements');

            await expect(page.locator('thead th').filter({ hasText: 'Identifier' })).not.toBeVisible();
        });
    });

    test('Requirements and TestGrid preferences are stored independently', async ({ page }) => {
        let keys;

        await test.step('Hide Identifier on Requirements', async () => {
            await page.getByRole('button', { name: 'Columns' }).click();
            await page.getByRole('dialog', { name: 'Column visibility' })
                .locator('[role="checkbox"]').filter({ hasText: 'Identifier' }).click();
        });

        await test.step('Read the localStorage column keys', async () => {
            keys = await page.evaluate(() => ({
                tests: localStorage.getItem('ttgo_columns_test-cases'),
                reqs:  localStorage.getItem('ttgo_columns_requirements'),
            }));
        });

        await test.step('Verify the Requirements key is set and the test-cases key is not', async () => {
            expect(keys.reqs).not.toBeNull();
            expect(keys.tests).toBeNull();
        });
    });
});
