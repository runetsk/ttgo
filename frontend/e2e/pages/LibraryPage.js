import { BasePage } from './BasePage.js';
import { ROUTES } from '../config.js';

// localStorage keys backing the two grids' column-visibility preferences.
const COLUMN_PREF_KEYS = ['ttgo_columns_test-cases', 'ttgo_columns_requirements'];

// Test-case library: the folder sidebar + test grid (/library, served at '/').
export class LibraryPage extends BasePage {
    async open() {
        await this.goto(ROUTES.HOME);
    }

    // ── Folder sidebar ──────────────────────────────────────────────────────
    get sidebar() {
        return this.page.getByTestId('sidebar');
    }

    get resizeHandle() {
        return this.page.locator('.resize-handle');
    }

    get createRootFolderButton() {
        return this.page.getByTestId('create-root-folder-button');
    }

    get selectedFolder() {
        return this.page.locator('.folder-header.selected');
    }

    folderNode(name) {
        return this.page.getByTestId('folder-name').filter({ hasText: name }).first();
    }

    get firstFolder() {
        return this.page.getByTestId('folder-name').first();
    }

    // The folder's tree-row wrapper — carries the expand toggle and, once
    // expanded, the nested test-case nodes.
    folderContainer(name) {
        return this.page.getByTestId('folder-container').filter({ hasText: name }).first();
    }

    expandToggle(name) {
        return this.folderContainer(name).locator('.expand-toggle').first();
    }

    testCaseNode(folderName, testName) {
        return this.folderContainer(folderName).locator('.test-case-node').filter({ hasText: testName }).first();
    }

    get showTestsToggle() {
        return this.page.getByTestId('sidebar-show-tests-toggle');
    }

    // Test-case nodes only render in the tree when "Show tests in tree" is on.
    async ensureShowTestsInTree() {
        const toggle = this.showTestsToggle;
        if ((await toggle.getAttribute('class') || '').indexOf('active') === -1) {
            await toggle.click();
        }
    }

    async createRootFolder(name) {
        await this.createRootFolderButton.click();
        await this.fillModal(name);
        await this.confirmModal();
        // Wait for the created node to render before returning (replaces the old
        // inline "sidebar settle" sleep) so callers can act on a stable tree.
        const node = this.folderNode(name);
        await node.waitFor({ state: 'visible' });
        return node;
    }

    async createSubfolder(parentName, name) {
        await this.folderNode(parentName).click({ button: 'right' });
        await this.page.getByTestId('context-menu-create-subfolder').click();
        await this.fillModal(name);
        await this.confirmModal();
        const node = this.folderNode(name);
        await node.waitFor({ state: 'visible' });
        return node;
    }

    async selectFolder(name) {
        await this.folderNode(name).click();
    }

    // Right-click a folder and delete it through the context menu + confirm modal.
    async deleteFolderViaContextMenu(name) {
        await this.folderNode(name).click({ button: 'right' });
        await this.page.getByTestId('context-menu-delete-folder').click();
        await this.confirmModal();
    }

    get bulkDeleteFoldersButton() {
        return this.page.getByTestId('bulk-delete-folders-button');
    }

    async bulkDeleteFolders() {
        await this.bulkDeleteFoldersButton.click();
        await this.confirmModal();
    }

    // ── Test grid ───────────────────────────────────────────────────────────
    get testTable() {
        return this.page.getByTestId('test-table');
    }

    get testRows() {
        return this.page.getByTestId('test-row');
    }

    testRow(name) {
        return this.page.getByTestId('test-row').filter({ hasText: name });
    }

    get searchInput() {
        return this.page.getByTestId('search-input');
    }

    get selectAllCheckbox() {
        return this.page.locator('thead input[type="checkbox"]');
    }

    get bulkActionBar() {
        return this.page.locator('.bulk-action-bar');
    }

    async createTestCase(name) {
        await this.page.getByTestId('create-test-button').click();
        await this.fillModal(name);
        await this.confirmModal();
    }

    // Open a test case from the grid by clicking its name cell (avoids the
    // first-column checkbox).
    async openTestCase(name) {
        await this.testRow(name).getByText(name.trim()).first().click();
    }

    async bulkDeleteTests() {
        await this.page.getByTestId('bulk-delete-tests-button').click();
        await this.confirmModal();
    }

    // ── Column-visibility picker (grid header) ──────────────────────────────
    get columnsButton() {
        return this.page.getByRole('button', { name: 'Columns' });
    }

    get columnPicker() {
        return this.page.getByRole('dialog', { name: 'Column visibility' });
    }

    async openColumnPicker() {
        await this.columnsButton.click();
    }

    columnToggle(label) {
        return this.columnPicker.locator('[role="checkbox"]').filter({ hasText: label });
    }

    async toggleColumn(label) {
        await this.columnToggle(label).click();
    }

    columnHeader(label) {
        return this.page.locator('thead th').filter({ hasText: label });
    }

    async resetColumns() {
        await this.columnPicker.getByText('Reset to default').click();
    }

    // Reset the grids' column-visibility preferences to their defaults.
    async clearColumnPrefs() {
        await this.page.evaluate((keys) => {
            keys.forEach((k) => localStorage.removeItem(k));
        }, COLUMN_PREF_KEYS);
    }

    async openColumnFilters() {
        await this.page.getByRole('button', { name: 'Column Filters' }).click();
    }

    // Folder + one test case via the UI, landing back on the grid. `prefix` keeps
    // the generated folder name unique/recognizable.
    async setupFolderAndTest(prefix = 'Setup') {
        const ts = Date.now();
        const folderName = `${prefix} ${ts}`;
        const testName = `TC ${ts}`;
        await this.open();
        await this.clearColumnPrefs();
        await this.createRootFolder(folderName);
        await this.selectFolder(folderName);
        await this.createTestCase(testName);
        await this.testTable.waitFor({ state: 'visible' });
        return { folderName, testName };
    }
}
