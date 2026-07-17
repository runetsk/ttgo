import { test, expect } from '../../fixtures/test.js';

test.describe('Grid Column Selection — TestGrid (US1 + US2)', () => {

    test.beforeEach(async ({ libraryPage }) => {
        await libraryPage.open();
        await libraryPage.clearColumnPrefs();
    });

    test('Columns button is visible in grid header', async ({ libraryPage }) => {
        await test.step('Set up a folder with one test', async () => {
            await libraryPage.setupFolderAndTest('ColPicker');
        });

        await test.step('Verify the Columns button is visible in the grid header', async () => {
            await expect(libraryPage.columnsButton).toBeVisible();
        });
    });

    test('column picker opens and lists all columns', async ({ libraryPage }) => {
        await test.step('Set up a folder with one test', async () => {
            await libraryPage.setupFolderAndTest('ColPicker');
        });

        await test.step('Open the column picker and verify all five columns are listed', async () => {
            await libraryPage.openColumnPicker();
            const popover = libraryPage.columnPicker;
            await expect(popover).toBeVisible();

            await expect(popover.getByText('ID')).toBeVisible();
            await expect(popover.getByText('Test Name')).toBeVisible();
            await expect(popover.getByText('Categories')).toBeVisible();
            await expect(popover.getByText('Created')).toBeVisible();
            await expect(popover.getByText('Updated')).toBeVisible();
        });
    });

    test('mandatory column (Test Name) is shown as disabled in picker', async ({ libraryPage }) => {
        await test.step('Set up a folder with one test', async () => {
            await libraryPage.setupFolderAndTest('ColPicker');
        });

        await test.step('Open the picker and verify the Test Name row is disabled', async () => {
            await libraryPage.openColumnPicker();
            const testNameRow = libraryPage.columnPicker.locator('[aria-disabled="true"]').filter({ hasText: 'Test Name' });
            await expect(testNameRow).toBeVisible();
        });
    });

    test('hiding a column removes it from the grid immediately', async ({ libraryPage }) => {
        await test.step('Set up a folder with one test', async () => {
            await libraryPage.setupFolderAndTest('ColPicker');
        });

        await test.step('Verify the Created column header is visible by default', async () => {
            await expect(libraryPage.columnHeader('Created')).toBeVisible();
        });

        await test.step('Open the picker and uncheck Created, verifying the header disappears immediately', async () => {
            await libraryPage.openColumnPicker();
            await libraryPage.toggleColumn('Created');
            await expect(libraryPage.columnHeader('Created')).not.toBeVisible();
        });
    });

    test('column preference is persisted and restored after page reload', async ({ page, libraryPage }) => {
        await test.step('Set up a folder with one test', async () => {
            await libraryPage.setupFolderAndTest('ColPicker');
        });

        await test.step('Hide the Updated column and verify the header disappears', async () => {
            await libraryPage.openColumnPicker();
            await libraryPage.toggleColumn('Updated');
            await expect(libraryPage.columnHeader('Updated')).not.toBeVisible();
        });

        await test.step('Reload and verify the preference is restored with other columns visible', async () => {
            await page.reload();
            await expect(libraryPage.testTable).toBeVisible();
            await expect(libraryPage.columnHeader('Updated')).not.toBeVisible();
            await expect(libraryPage.columnHeader('Test Name')).toBeVisible();
            await expect(libraryPage.columnHeader('Categories')).toBeVisible();
        });
    });

    test('Test Name column cannot be hidden — always visible', async ({ libraryPage }) => {
        const optionalCols = ['ID', 'Categories', 'Created', 'Updated'];

        await test.step('Set up a folder with one test', async () => {
            await libraryPage.setupFolderAndTest('ColPicker');
        });

        await test.step('Hide all optional columns', async () => {
            await libraryPage.openColumnPicker();
            for (const label of optionalCols) {
                await libraryPage.toggleColumn(label);
            }
        });

        await test.step('Verify Test Name is still visible', async () => {
            await expect(libraryPage.columnHeader('Test Name')).toBeVisible();
        });

        await test.step('Verify all optional headers are gone', async () => {
            for (const label of optionalCols) {
                await expect(libraryPage.columnHeader(label)).not.toBeVisible();
            }
        });
    });

    test('Reset to default restores all columns and clears preference', async ({ page, libraryPage }) => {
        await test.step('Set up a folder with one test', async () => {
            await libraryPage.setupFolderAndTest('ColPicker');
        });

        await test.step('Hide the ID and Created columns', async () => {
            await libraryPage.openColumnPicker();
            await libraryPage.toggleColumn('ID');
            await libraryPage.toggleColumn('Created');
            await expect(libraryPage.columnHeader('ID')).not.toBeVisible();
        });

        await test.step('Click Reset to default and verify all columns are restored', async () => {
            await libraryPage.resetColumns();
            await expect(libraryPage.columnHeader('ID')).toBeVisible();
            await expect(libraryPage.columnHeader('Created')).toBeVisible();
        });

        await test.step('Reload and verify the cleared preference shows all columns', async () => {
            await page.reload();
            await expect(libraryPage.testTable).toBeVisible();
            await expect(libraryPage.columnHeader('ID')).toBeVisible();
            await expect(libraryPage.columnHeader('Created')).toBeVisible();
        });
    });

    test('picker closes when Escape is pressed', async ({ page, libraryPage }) => {
        await test.step('Set up a folder with one test', async () => {
            await libraryPage.setupFolderAndTest('ColPicker');
        });

        await test.step('Open the picker and verify it is visible', async () => {
            await libraryPage.openColumnPicker();
            await expect(libraryPage.columnPicker).toBeVisible();
        });

        await test.step('Press Escape and verify the picker closes', async () => {
            await page.keyboard.press('Escape');
            await expect(libraryPage.columnPicker).not.toBeVisible();
        });
    });

    test('picker closes when clicking outside', async ({ page, libraryPage }) => {
        await test.step('Set up a folder with one test', async () => {
            await libraryPage.setupFolderAndTest('ColPicker');
        });

        await test.step('Open the picker and verify it is visible', async () => {
            await libraryPage.openColumnPicker();
            await expect(libraryPage.columnPicker).toBeVisible();
        });

        await test.step('Click outside the popover and verify the picker closes', async () => {
            await page.locator('.grid-header h2').click();
            await expect(libraryPage.columnPicker).not.toBeVisible();
        });
    });
});

test.describe('Grid Column Selection — RequirementsPage (US3)', () => {

    test.beforeEach(async ({ page, libraryPage, requirementsPage }) => {
        await requirementsPage.open();
        await libraryPage.clearColumnPrefs();
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

    test('hiding Coverage column removes it from Requirements table', async ({ page, api }) => {
        await test.step('Seed a requirement so the table (with headers) renders', async () => {
            await api.createRequirement('REQ-COV-' + Date.now(), 'Coverage column requirement');
            await page.reload();
        });

        await test.step('Verify the Coverage header is visible by default', async () => {
            await expect(page.locator('thead th').filter({ hasText: 'Coverage' })).toBeVisible();
        });

        await test.step('Hide Coverage and verify it disappears while Requirement stays', async () => {
            await page.getByRole('button', { name: 'Columns' }).click();
            await page.getByRole('dialog', { name: 'Column visibility' })
                .locator('[role="checkbox"]').filter({ hasText: 'Coverage' }).click();

            await expect(page.locator('thead th').filter({ hasText: 'Coverage' })).not.toBeVisible();
            await expect(page.locator('thead th').filter({ hasText: 'Requirement' })).toBeVisible();
        });
    });

    test('Requirements column preference persists across navigation', async ({ page, libraryPage, requirementsPage }) => {
        await test.step('Hide the Identifier column and verify it disappears', async () => {
            await page.getByRole('button', { name: 'Columns' }).click();
            await page.getByRole('dialog', { name: 'Column visibility' })
                .locator('[role="checkbox"]').filter({ hasText: 'Identifier' }).click();

            await expect(page.locator('thead th').filter({ hasText: 'Identifier' })).not.toBeVisible();
        });

        await test.step('Navigate away and back, verifying the preference persists', async () => {
            await libraryPage.open();
            await requirementsPage.open();

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
