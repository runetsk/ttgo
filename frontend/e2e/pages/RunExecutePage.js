import { BasePage } from './BasePage.js';
import { runExecute } from '../config.js';

// Manual run-execution runner (/runs/run/:runId/execute): the step-by-step queue
// driver. All controls are namespaced `execute-*`.
export class RunExecutePage extends BasePage {
    async open(runId) {
        await this.gotoReady(runExecute(runId));
    }

    get currentName() {
        return this.page.getByTestId('execute-current-name');
    }

    get progress() {
        return this.page.getByTestId('execute-progress');
    }

    get steps() {
        return this.page.getByTestId('execute-steps');
    }

    get doneBanner() {
        return this.page.getByTestId('execute-done-banner');
    }

    async next() {
        await this.page.getByTestId('execute-next').click();
    }

    async prev() {
        await this.page.getByTestId('execute-prev').click();
    }

    async pass() {
        await this.page.getByTestId('execute-pass').click();
    }

    async skip() {
        await this.page.getByTestId('execute-skip').click();
    }

    // Fail requires a confirm (with optional defect type + note).
    async fail({ defectType, note } = {}) {
        await this.page.getByTestId('execute-fail').click();
        if (defectType) await this.defectTypeSelect.selectOption(defectType);
        if (note !== undefined) await this.failNote.fill(note);
        await this.confirmFail();
    }

    // The run-level fail panel controls, exposed for the granular per-step flow
    // (mark a step Fail → panel opens → fill the step note → confirm).
    get defectTypeSelect() {
        return this.page.getByTestId('execute-defect-type');
    }

    get failNote() {
        return this.page.getByTestId('execute-fail-note');
    }

    async confirmFail() {
        await this.page.getByTestId('execute-fail-confirm').click();
    }

    // ── Per-step verdicts ──
    async stepPass(index) {
        await this.page.getByTestId(`execute-step-pass-${index}`).click();
    }

    async stepFail(index) {
        await this.page.getByTestId(`execute-step-fail-${index}`).click();
    }

    stepNote(index) {
        return this.page.getByTestId(`execute-step-note-${index}`);
    }

    async completeRun() {
        await this.page.getByTestId('execute-complete-run').click();
    }
}
