import { BasePage } from './BasePage.js';
import { testCase, TIMEOUTS } from '../config.js';

// Test-case detail (/library/tests/:testId): name/fields, steps editor, links.
export class TestCaseDetailPage extends BasePage {
    async open(testId) {
        await this.goto(testCase(testId));
    }

    get nameInput() {
        return this.page.getByTestId('test-case-name-input');
    }

    async save() {
        await this.page.getByRole('button', { name: 'Save Changes' }).click();
    }

    async cancel() {
        await this.page.getByRole('button', { name: 'Cancel' }).click();
    }

    // ── Custom fields (inline detail pane) ──────────────────────────────────
    // Each custom field renders a .detail-pane-custom-key label followed by its
    // <select> sibling.
    customFieldKey(fieldName) {
        return this.page.locator('.detail-pane-custom-key').filter({ hasText: fieldName });
    }

    customFieldSelect(fieldName) {
        return this.customFieldKey(fieldName).locator('xpath=following-sibling::select[1]');
    }

    // ── Linked bugs & requirements ──────────────────────────────────────────
    get linkedBugsPanel() {
        return this.page.getByTestId('linked-bugs-panel');
    }

    linkedBug(id) {
        return this.page.getByTestId(`linked-bug-${id}`);
    }

    linkedReqLink(id) {
        return this.page.getByTestId(`linked-req-link-${id}`);
    }

    // ── Requirement picker ──────────────────────────────────────────────────
    get reqSearchInput() {
        return this.page.getByTestId('req-search-input');
    }

    reqOption(id) {
        return this.page.getByTestId(`req-option-${id}`);
    }

    get activeReqOption() {
        return this.page.locator('[data-testid^="req-option-"][data-active="true"]');
    }

    // ── Steps editor ────────────────────────────────────────────────────────
    // Clicking the name input blurs whichever RichTextField is open so its blur
    // doesn't race the next activation.
    async deactivateEditors() {
        await this.nameInput.click();
    }

    async addStep(index) {
        await this.page.getByTestId('add-step-button').click();
        await this.page.getByTestId(`step-action-${index}`).waitFor({ state: 'visible', timeout: TIMEOUTS.ELEMENT });
    }

    // Activate a RichTextField (by container testid selector), wait for its
    // toolbar (confirms edit mode), then type into the ProseMirror surface.
    async fillRichField(containerSelector, text) {
        await this.deactivateEditors();
        await this.page.locator(`${containerSelector} .rich-text-field`).click();
        await this.page.locator(`${containerSelector} .rich-text-toolbar`)
            .waitFor({ state: 'visible', timeout: TIMEOUTS.UI_SETTLE });
        await this.page.locator(`${containerSelector} .ProseMirror`).click();
        await this.page.keyboard.type(text);
    }

    stepActionDisplay(index) {
        return this.page.locator(`[data-testid="step-action-${index}"] .rich-text-display`);
    }
}
