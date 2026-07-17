import { BasePage } from './BasePage.js';
import { ROUTES } from '../config.js';

// Requirements list (/requirements): create/edit modal, per-row actions, search,
// and the Jira/Confluence import dialogs. Mostly role/text locators, with a few
// data-testids (search, title, and the import inputs).
export class RequirementsPage extends BasePage {
    async open() {
        await this.goto(ROUTES.REQUIREMENTS);
    }

    row(text) {
        return this.page.locator('tr').filter({ hasText: text });
    }

    get searchInput() {
        return this.page.getByTestId('requirement-search-input');
    }

    // Title field is shared by the create and edit forms.
    get titleInput() {
        return this.page.getByTestId('requirement-title-input');
    }

    async openCreateModal() {
        await this.page.getByRole('button', { name: '+ New Requirement' }).click();
    }

    async create({ identifier, title, description }) {
        await this.openCreateModal();
        await this.page.getByTestId('requirement-identifier-input').fill(identifier);
        await this.titleInput.fill(title);
        if (description !== undefined) await this.page.getByTestId('requirement-description-input').fill(description);
        await this.page.getByRole('button', { name: 'Create', exact: true }).click();
    }

    // Open a row's kebab menu and click an action (e.g. 'Edit', 'Delete'). The
    // menu is portaled to body, so the item is page-scoped, not row-scoped.
    async rowAction(rowText, action) {
        await this.row(rowText).getByRole('button', { name: 'Actions' }).click();
        await this.page.locator('.context-menu-item').filter({ hasText: action }).click();
    }

    async saveChanges() {
        await this.page.getByRole('button', { name: 'Save Changes' }).click();
    }

    // ── Import dialogs (Jira / Confluence) ──────────────────────────────────
    get jiraTicketInput() {
        return this.page.getByTestId('jira-import-ticket-input');
    }

    get jiraBulkJqlInput() {
        return this.page.getByTestId('jira-bulk-jql-input');
    }

    // Entry points live under the consolidated "⬇ Import ▾" dropdown. Match by
    // title (stable) — a name regex also catches the transient "Importing…" button.
    async openImportMenu() {
        await this.page.getByTitle('Import requirements from an external source').click();
    }

    async importMenuItem(text) {
        await this.page.locator('.context-menu-item').filter({ hasText: text }).click();
    }

    async openImportJira() {
        await this.openImportMenu();
        await this.importMenuItem('Single ticket');
    }

    async openImportConfluence() {
        await this.openImportMenu();
        await this.importMenuItem('Single page');
    }

    async openBulkImportJira() {
        await this.openImportMenu();
        await this.importMenuItem('Bulk via JQL');
    }

    async openBulkImportConfluence() {
        await this.openImportMenu();
        await this.importMenuItem('Bulk from space');
    }

    async fetchJiraPreview(ticketKey) {
        await this.jiraTicketInput.fill(ticketKey);
        await this.page.getByRole('button', { name: 'Fetch Preview' }).click();
    }

    async importRequirement() {
        await this.page.getByRole('button', { name: 'Import Requirement' }).click();
    }

    // Confluence single-import step 1: spaces render as clickable cards (not a
    // <select>); click the one by name, scoped to the modal (the name also matches
    // the Quality nav item + page heading otherwise).
    async selectConfluenceSpace(spaceName) {
        await this.page.locator('.modal-content').getByText(spaceName, { exact: true }).click();
    }

    async browseConfluencePages() {
        await this.page.getByRole('button', { name: 'Browse Pages' }).click();
    }

    // Bulk Confluence: focusing the space <select> lazily triggers the spaces
    // fetch; selectOption then auto-waits for the option, so no settle sleep is
    // needed before loading pages.
    async loadBulkConfluenceSpace(label) {
        const select = this.page.locator('select');
        await select.focus();
        await select.selectOption({ label });
        await this.page.getByRole('button', { name: 'Load Pages' }).click();
    }

    async searchBulkJira(jql) {
        await this.jiraBulkJqlInput.fill(jql);
        await this.page.getByRole('button', { name: 'Search' }).click();
    }

    async selectAllBulk() {
        await this.page.getByRole('button', { name: 'Select All', exact: true }).click();
    }

    async deselectAllBulk() {
        await this.page.getByRole('button', { name: 'Deselect All' }).click();
    }

    async importSelected(count) {
        await this.page.getByRole('button', { name: new RegExp(`Import Selected \\(${count}\\)`) }).click();
    }

    // Import a single Jira ticket end-to-end — setup for the resync specs. The
    // Import button only renders once the preview resolves, so clicking it
    // implicitly waits for the fetch to complete.
    async importJiraTicket(ticketKey) {
        await this.openImportJira();
        await this.fetchJiraPreview(ticketKey);
        await this.importRequirement();
        await this.row(ticketKey).first().waitFor();
    }

    // ── Row source actions (edit / resync / unlink) via the kebab "Actions" menu ──
    async openRowActions(rowText) {
        await this.row(rowText).getByRole('button', { name: 'Actions' }).click();
    }

    // A menu item in the open kebab (portaled to body).
    menuItem(text) {
        return this.page.locator('.context-menu-item').filter({ hasText: text });
    }

    async resyncRow(rowText) {
        await this.rowAction(rowText, 'Re-sync');
    }

    // Edit a requirement's title via the row Edit action.
    async editTitle(currentTitle, newTitle) {
        await this.rowAction(currentTitle, 'Edit');
        await this.titleInput.clear();
        await this.titleInput.fill(newTitle);
        await this.saveChanges();
        await this.page.getByText(newTitle).first().waitFor();
    }
}
