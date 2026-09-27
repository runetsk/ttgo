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

    // ── Settings → AI (header + tabs Providers / Prompts / Limits & budget / Failure analysis) ──
    async openAISettings(tabLabel) {
        await this.page.getByRole('button', { name: 'AI', exact: true }).click();
        if (tabLabel) await this.page.getByRole('tab', { name: tabLabel }).click();
    }

    // ── TypeSafe.ai card (AI → Failure analysis) ────────────────────────────
    async openTypeSafeCard() {
        await this.openAISettings('Failure analysis');
        await this.page.getByTestId('typesafe-settings').scrollIntoViewIfNeeded();
    }

    get typesafeKeyInput() { return this.page.getByTestId('typesafe-api-key'); }
    get typesafeKeyStatus() { return this.page.getByTestId('typesafe-key-status'); }
    // TypeSafe.ai saves through the page save bar.
    get typesafeSaveButton() { return this.page.getByTestId('ai-savebar-save'); }
    get typesafeClearKeyCheckbox() { return this.page.getByTestId('typesafe-clear-key'); }
    get typesafeModelInput() { return this.page.getByTestId('typesafe-model'); }

    // ── AI Failure Analysis section (AI → Failure analysis) ─────────────────
    async openFailureAnalysis() {
        await this.openAISettings('Failure analysis');
        await this.page.getByTestId('analysis-flow').scrollIntoViewIfNeeded();
    }

    flowStep(id) { return this.page.getByTestId(`analysis-flow-step-${id}`); }
    flowPart(stepId, id) { return this.page.getByTestId(`analysis-flow-part-${stepId}-${id}`); }
    flowChip(key) { return this.page.getByTestId(`analysis-flow-chip-${key}`); }
    flowTrigger(mode) { return this.page.getByTestId(`analysis-flow-trigger-${mode}`); }

    // The diagram shows one step's detail at a time; click a step to show its parts and chips.
    async selectFlowStep(id) {
        await this.flowStep(id).getByRole('button').click();
        await this.page.getByTestId('analysis-flow-detail').and(this.page.locator(`[data-step="${id}"]`)).waitFor();
    }
    get flowDetail() { return this.page.getByTestId('analysis-flow-detail'); }

    // ── Suggestion accuracy panel (AI → Failure analysis) ───────────────────
    async openAccuracyPanel() {
        await this.openAISettings('Failure analysis');
        await this.accuracyPanel.scrollIntoViewIfNeeded();
    }

    get accuracyPanel() { return this.page.getByTestId('accuracy-panel'); }
    get accuracyHeadlineLabel() { return this.page.getByTestId('accuracy-headline-label'); }
    get accuracyHeadlineRate() { return this.page.getByTestId('accuracy-headline-rate'); }
    get accuracyPolicyFilter() { return this.page.getByTestId('accuracy-policy-filter'); }
    accuracyEngine(key) { return this.page.getByTestId(`accuracy-engine-${key}`); }

    // ── Settings → AI save bar ──────────────────────────────────────────────
    get saveBar() { return this.page.getByTestId('ai-savebar'); }
    get saveBarSave() { return this.page.getByTestId('ai-savebar-save'); }
    get saveBarDiscard() { return this.page.getByTestId('ai-savebar-discard'); }
    get saveBarMessage() { return this.page.getByTestId('ai-savebar-message'); }
}
