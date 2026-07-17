import { BasePage } from './BasePage.js';
import { ROUTES } from '../config.js';

// Tabbed settings (/settings): custom fields, integrations (Jira/Confluence),
// tokens, webhooks, backups, users. Tabs are role/text buttons; some are
// URL-hash addressable. Owned by the settings rollout — extend as needed.
export class SettingsPage extends BasePage {
    async open() {
        await this.goto(ROUTES.SETTINGS);
    }

    async openTab(name) {
        await this.page.getByRole('button', { name }).click();
    }

    // ── Custom fields ───────────────────────────────────────────────────────
    get customFieldNameInput() {
        return this.page.getByTestId('custom-field-name-input');
    }

    // Only rendered when the type <select> is set to SELECT.
    get customFieldOptionsInput() {
        return this.page.getByTestId('custom-field-options-input');
    }

    // The options input only appears once type=SELECT, so set the type first.
    async addCustomField({ name, type = 'TEXT', options }) {
        await this.customFieldNameInput.fill(name);
        await this.page.locator('select').selectOption(type);
        if (options !== undefined) await this.customFieldOptionsInput.fill(options);
        await this.page.getByRole('button', { name: '+ Add Field' }).click();
    }

    // Existing-fields list renders one .glass-panel card per field.
    customFieldRow(name) {
        return this.page.locator('.glass-panel').filter({ hasText: name }).first();
    }

    // ── Integrations (Jira / Confluence config form) ────────────────────────
    get integrationBaseUrlInput() {
        return this.page.getByTestId('integration-base-url-input');
    }

    get integrationEmailInput() {
        return this.page.getByTestId('integration-email-input');
    }

    // Jira "Test Connection" ticket field (distinct from the import dialog input).
    get jiraTestTicketInput() {
        return this.page.getByTestId('jira-test-ticket-input');
    }

    // `label` is the checkbox label, e.g. /Enable Jira integration/i.
    async enableIntegration(label) {
        await this.page.getByLabel(label).check();
    }

    // The API-token field is the only password input in the panel.
    async fillIntegrationForm({ baseUrl, email, token }) {
        await this.integrationBaseUrlInput.fill(baseUrl);
        await this.integrationEmailInput.fill(email);
        await this.page.locator('input[type="password"]').fill(token);
    }

    async saveConfiguration() {
        await this.page.getByRole('button', { name: 'Save Configuration' }).click();
    }
}
