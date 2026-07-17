import { BasePage } from './BasePage.js';
import { requirement } from '../config.js';

// AI test-generation studio (/ai-generate). The shell has no data-testids, so
// entry and the header controls are role/text-based; the review surfaces
// (draft editor, compare modal, regen section, commit summary, reject popover,
// run history) are keyed on their own testids.
export class AiStudioPage extends BasePage {
    // Enter the studio from a requirement's detail page (direct entry button).
    async enterFromRequirement(reqId) {
        await this.page.goto(requirement(reqId));
        await this.page.getByRole('button', { name: /AI Generate Tests/i }).click();
    }

    // The folder picker lives in the Output section, collapsed by default.
    async selectOutputFolder(folderName) {
        await this.page.getByRole('button', { name: 'Output' }).click();
        await this.page.getByTestId('folder-tree-trigger').click();
        await this.page.getByTestId('folder-tree-search').fill(folderName);
        // Scope to the dropdown row, not the trigger (which also shows the name).
        await this.page.getByTestId('folder-tree-row').filter({ hasText: folderName }).click();
    }

    async generate() {
        await this.page.getByRole('button', { name: /^generate$/i }).click();
    }

    // Selects a pending draft by name. Both the list row and the auto-selected
    // detail pane render the name, hence .first().
    async selectDraft(name) {
        await this.page.getByText(name).first().click();
    }

    // ── Regenerate / compare ────────────────────────────────────────────────
    get regenSection() {
        return this.page.getByTestId('regen-section');
    }

    get regenInstructionInput() {
        return this.page.getByTestId('regen-instruction-input');
    }

    // Fills the optional instruction (when given) and fires the regen request.
    // Scoped to the regen-section button because the studio header's own button
    // also reads "Regenerate" once ai.hasGenerated flips.
    async regenerate(instruction) {
        if (instruction) await this.regenInstructionInput.fill(instruction);
        await this.regenSection.getByRole('button', { name: /^regenerate$/i }).click();
    }

    get draftCompare() {
        return this.page.getByTestId('draft-compare');
    }

    // Toggles the compare modal between 'split' and 'unified' (persisted to localStorage).
    async setCompareView(mode) {
        await this.page.getByRole('button', { name: new RegExp(`^${mode}$`, 'i') }).click();
    }

    async chooseNewVersion() {
        await this.page.getByRole('button', { name: /use new version/i }).click();
    }

    // ── Draft editor / autosave ─────────────────────────────────────────────
    get draftEditor() {
        return this.page.getByTestId('draft-editor');
    }

    get draftNameInput() {
        return this.draftEditor.locator('input').first();
    }

    get saveState() {
        return this.page.getByTestId('save-state');
    }

    // ── Reject / restore ────────────────────────────────────────────────────
    // Scope to the right-hand detail pane (the <aside> hosting the draft editor):
    // every pending row in the list also renders its own "Reject" button.
    async openRejectPopover() {
        const detailPane = this.page.locator('aside').filter({ has: this.draftEditor });
        await detailPane.getByRole('button', { name: /^reject$/i }).click();
    }

    get rejectPopover() {
        return this.page.getByTestId('reject-popover');
    }

    async selectRejectReason(reason) {
        await this.rejectPopover.getByRole('button', { name: reason }).click();
    }

    async confirmReject() {
        await this.rejectPopover.getByRole('button', { name: /^reject$/i }).click();
    }

    async restoreToPending() {
        await this.page.getByRole('button', { name: /restore to pending/i }).click();
    }

    // ── Accept / commit summary ─────────────────────────────────────────────
    // Opens the commit-summary confirm modal (does not fire the accept request).
    async acceptAllClean() {
        await this.page.getByRole('button', { name: /accept all/i }).click();
    }

    get commitSummary() {
        return this.page.getByTestId('commit-summary');
    }

    async confirmCommit() {
        await this.commitSummary.getByRole('button', { name: /accept \d+ draft/i }).click();
    }

    async doneCommit() {
        await this.commitSummary.getByRole('button', { name: 'Done' }).click();
    }

    // ── Run history / filter ────────────────────────────────────────────────
    get runHistory() {
        return this.page.getByTestId('run-history');
    }

    // Draft filter tabs (Pending / All / …); anchored so 'all' can't match
    // "Accept all clean".
    async filterDrafts(name) {
        await this.page.getByRole('button', { name: new RegExp(`^${name}\\b`, 'i') }).click();
    }
}
