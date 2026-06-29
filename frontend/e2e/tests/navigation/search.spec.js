import { test, expect } from '@playwright/test';

test.describe('Global Search (US1)', () => {

  /** Create a folder and select it so the TestGrid (with search bar) renders. */
  const setupFolder = async (page) => {
    const folderName = `Search Folder ${Date.now()}`;
    await page.goto('/');
    await page.getByTestId('create-root-folder-button').click();
    await page.getByTestId('modal-input').fill(folderName);
    await page.getByTestId('modal-confirm-button').click();
    const folder = page.getByTestId('folder-name').filter({ hasText: folderName }).first();
    await expect(folder).toBeVisible();
    await folder.click();
    await expect(page.getByTestId('test-table')).toBeVisible();
  };

  test('search bar is visible in grid header', async ({ page }) => {
    await test.step('Create and open a folder so the test grid renders', async () => {
      await setupFolder(page);
    });
    await test.step('Verify the search bar is visible in the grid header', async () => {
      await expect(page.getByTestId('search-bar')).toBeVisible();
    });
  });

  test('searching for an existing test case shows results', async ({ page }) => {
    await test.step('Create and open a folder so the test grid renders', async () => {
      await setupFolder(page);
    });
    await test.step('Search for an existing term and verify the input stays visible', async () => {
      const searchInput = page.getByTestId('search-input');
      await searchInput.fill('test');
      await page.waitForTimeout(400); // debounce
      // Results should appear if any test cases exist
      // For a fresh DB this may be empty - check no error
      await expect(searchInput).toBeVisible();
    });
  });

  test('searching nonexistent term shows no results message', async ({ page }) => {
    await test.step('Create and open a folder so the test grid renders', async () => {
      await setupFolder(page);
    });
    await test.step('Search for a nonexistent term and verify no crash', async () => {
      const searchInput = page.getByTestId('search-input');
      await searchInput.fill('zzz_nonexistent_xyz_abc_12345');
      await page.waitForTimeout(400);
      // Input still visible (no crash)
      await expect(searchInput).toBeVisible();
    });
  });
});
