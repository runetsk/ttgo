import { BasePage } from './BasePage.js';
import { ROUTES } from '../config.js';

// Test-runs list (/runs): the run table, the New Test Run modal, column
// filters, and the run-folder sidebar.
export class RunsPage extends BasePage {
    async open() {
        await this.gotoReady(ROUTES.RUNS);
    }

    runRow(name) {
        return this.page.getByRole('row', { name });
    }

    // ── New Test Run modal ──
    async openNewRunModal() {
        await this.page.getByTestId('create-test-run-button').click();
    }

    get runNameInput() {
        return this.page.getByTestId('create-run-name-input');
    }

    get runFolderSelect() {
        return this.page.getByTestId('create-run-folder-select');
    }

    get runCategorySelect() {
        return this.page.getByTestId('create-run-category-select');
    }

    async submitNewRun() {
        await this.page.getByTestId('create-run-submit').click();
    }

    async cancelNewRun() {
        await this.page.getByTestId('create-run-cancel').click();
    }

    // Open the modal, fill the given fields, and submit. `folder`/`category` are
    // option values/labels; omit either to leave it at its default.
    async createRun({ name, folder, category } = {}) {
        await this.openNewRunModal();
        if (name !== undefined) await this.runNameInput.fill(name);
        if (folder !== undefined) await this.runFolderSelect.selectOption(folder);
        if (category !== undefined) await this.runCategorySelect.selectOption({ label: category });
        await this.submitNewRun();
    }

    // ── Column filters ──
    async openColumnFilters() {
        await this.page.getByRole('button', { name: 'Column Filters' }).click();
    }

    async filterByCategory(categoryId) {
        await this.page.getByTestId('filter-run-category').click();
        await this.page.getByTestId(`filter-run-category-option-${categoryId}`).click();
        await this.page.keyboard.press('Escape');
    }

    async clearCategoryFilter() {
        await this.page.getByTestId('filter-run-category').click();
        await this.page.getByTestId('filter-run-category-clear').click();
        await this.page.keyboard.press('Escape');
    }

    get statusFilter() {
        return this.page.getByTestId('filter-status-select');
    }

    get assigneeFilter() {
        return this.page.getByTestId('filter-run-assignee');
    }

    // Created-date range filter: open the popover, then fill the from/to inputs.
    async openCreatedFilter() {
        await this.page.getByTestId('filter-run-created_at').click();
    }

    get createdFrom() {
        return this.page.getByTestId('filter-run-created_at-from');
    }

    get createdTo() {
        return this.page.getByTestId('filter-run-created_at-to');
    }

    // ── Run list rows & per-run stat columns ──
    // Data rows carry data-testid="run-row-<id>"; the empty-state row does not,
    // so this collection counts only actual runs.
    get runRows() {
        return this.page.locator('[data-testid^="run-row-"]');
    }

    runPassed(runId) {
        return this.page.getByTestId(`run-passed-${runId}`);
    }

    runFailed(runId) {
        return this.page.getByTestId(`run-failed-${runId}`);
    }

    selectRunCheckbox(runId) {
        return this.page.getByTestId(`select-run-checkbox-${runId}`);
    }

    // ── Pagination ──
    get pageSizeSelector() {
        return this.page.getByTestId('page-size-selector');
    }

    get nextPageButton() {
        return this.page.getByTestId('next-page');
    }

    async nextPage() {
        await this.nextPageButton.click();
    }

    async prevPage() {
        await this.page.getByTestId('prev-page').click();
    }

    // ── Create-run modal tree picker ──
    get treePicker() {
        return this.page.getByTestId('test-tree-picker');
    }

    treeFolder(id) {
        return this.page.getByTestId(`test-tree-folder-${id}`);
    }

    get treeSelectedCount() {
        return this.page.getByTestId('test-tree-selected-count');
    }

    // Expand the Uncategorised sidebar group only when it is currently collapsed,
    // so a freshly-created run beneath it becomes visible.
    async expandUncategorised() {
        const toggle = this.page.locator('[data-testid="uncategorised-entry"] .expand-toggle');
        const expanded = await toggle.evaluate(el => el.classList.contains('expanded'));
        if (!expanded) await toggle.click();
    }

    // ── Run-folder sidebar ──
    get sidebar() {
        return this.page.getByTestId('run-folder-sidebar');
    }

    get sidebarCollapsed() {
        return this.page.getByTestId('run-folder-sidebar-collapsed');
    }

    get allRunsEntry() {
        return this.page.getByTestId('all-runs-entry');
    }

    folderItem(id) {
        return this.page.getByTestId(`run-folder-item-${id}`);
    }

    async collapseSidebar() {
        await this.page.getByTestId('sidebar-collapse-btn').click();
    }

    async expandSidebar() {
        await this.page.getByTestId('sidebar-expand-btn').click();
    }

    async openAddFolderModal() {
        await this.page.getByTestId('add-folder-btn').click();
    }

    // Hover the folder row to reveal its "⋮" menu, then open it.
    async openFolderMenu(id) {
        await this.folderItem(id).hover();
        await this.page.getByTestId(`folder-menu-${id}`).click();
    }

    async clickRenameFolder(id) {
        await this.page.getByTestId(`rename-folder-${id}`).click();
    }

    async clickDeleteFolder(id) {
        await this.page.getByTestId(`delete-folder-${id}`).click();
    }
}
