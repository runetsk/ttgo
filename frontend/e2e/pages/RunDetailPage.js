import { BasePage } from './BasePage.js';
import { runDetail, runCompare } from '../config.js';

// Single run detail (/runs/run/:runId): the results grid, per-result status
// selects, and the stats bar.
export class RunDetailPage extends BasePage {
    async open(runId) {
        await this.gotoReady(runDetail(runId));
    }

    // Compare view (same route with ?compareWith=<otherRunId>).
    async openCompare(runId, compareWithId) {
        await this.gotoReady(runCompare(runId, compareWithId));
    }

    resultRow(name) {
        return this.page.getByRole('row', { name });
    }

    // Expand a result row's detail panel. Clicking the chevron area of the
    // test-case cell (left of the link) expands it — the status/defect cells
    // stop propagation, so a plain row click won't.
    async expandResultRow(name) {
        await this.resultRow(name).locator('td').nth(1).click({ position: { x: 6, y: 8 } });
    }

    // Per-result status <select>. Two locators for the two ways specs address it:
    // by test-case id (stable testid) or by row (when only the name is known).
    statusSelect(testCaseId) {
        return this.page.getByTestId(`test-status-select-${testCaseId}`);
    }

    rowStatusSelect(rowName) {
        return this.resultRow(rowName).locator('select').first();
    }

    // Stats-bar pill, e.g. stat('passed') / stat('failed') / stat('pending').
    stat(kind) {
        return this.page.getByTestId(`stats-${kind}`);
    }

    get categories() {
        return this.page.getByTestId('run-categories');
    }

    async sortByColumn(name) {
        await this.page.getByRole('columnheader', { name }).click();
    }

    // ── Run header controls ──
    get runTitle() {
        return this.page.getByTestId('run-title');
    }

    // Run-level status <select> in the header (distinct from the per-result
    // statusSelect above).
    get runStatusSelect() {
        return this.page.getByTestId('run-status-select');
    }

    get assigneePicker() {
        return this.page.getByTestId('run-assignee-picker');
    }

    get completeRunButton() {
        return this.page.getByTestId('complete-run-button');
    }

    get reopenRunButton() {
        return this.page.getByTestId('reopen-run-button');
    }

    // Starts execution mode; scoped to the checked rows (see selectResult /
    // selectAllResults).
    get executeRunButton() {
        return this.page.getByTestId('execute-run-button');
    }

    async selectAllResults() {
        await this.page.getByTestId('select-all-results').click();
    }

    selectResult(testCaseId) {
        return this.page.getByTestId(`select-result-${testCaseId}`);
    }

    // ── Add-tests-to-run modal (embeds the shared tree picker) ──
    async openAddTests() {
        await this.page.getByTestId('add-test-to-run-button').click();
    }

    get treePicker() {
        return this.page.getByTestId('test-tree-picker');
    }

    treeTest(id) {
        return this.page.getByTestId(`test-tree-test-${id}`);
    }

    get treeSelectedCount() {
        return this.page.getByTestId('test-tree-selected-count');
    }

    async submitAddTests() {
        await this.page.getByTestId('add-tests-submit').click();
    }

    // ── Results toolbar & column filters ──
    get toolbar() {
        return this.page.getByTestId('run-results-toolbar');
    }

    async openColumnFilters() {
        await this.page.getByRole('button', { name: 'Column Filters' }).click();
    }

    async hideColumnFilters() {
        await this.page.getByRole('button', { name: 'Hide Filters' }).click();
    }

    get resultStatusFilter() {
        return this.page.getByTestId('filter-result-status');
    }

    // Result data rows carry data-result-id; the count reflects the active filter.
    get resultRows() {
        return this.page.locator('tbody tr[data-result-id]');
    }

    // ── Grouped view ──
    get groupedViewToggle() {
        return this.page.getByTestId('view-toggle-grouped');
    }

    get groupBySelect() {
        return this.page.getByTestId('group-by-select');
    }

    get groupHeader() {
        return this.page.getByTestId('group-header');
    }

    async collapseAll() {
        await this.page.getByTestId('collapse-all').click();
    }

    async expandAll() {
        await this.page.getByTestId('expand-all').click();
    }

    // ── Expanded result detail panel ──
    get resultDetail() {
        return this.page.getByTestId('run-result-detail');
    }

    get stepChecklist() {
        return this.page.getByTestId('result-step-checklist');
    }

    get attachScreenshots() {
        return this.page.getByTestId('attach-screenshots');
    }

    get attachScreenshotsInput() {
        return this.page.getByTestId('attach-screenshots-input');
    }

    get artifactScreenshot() {
        return this.page.getByTestId('artifact-screenshot');
    }

    // ── Compare tab ──
    get compareTab() {
        return this.page.getByTestId('run-compare-tab');
    }

    get compareSameRun() {
        return this.page.getByTestId('compare-same-run');
    }

    compareGroupCount(key) {
        return this.page.getByTestId(`compare-group-${key}-count`);
    }

    compareCount(key) {
        return this.page.getByTestId(`compare-count-${key}`);
    }

    compareRow(id) {
        return this.page.getByTestId(`compare-row-${id}`);
    }

    compareDetail(id) {
        return this.page.getByTestId(`compare-detail-${id}`);
    }
}
