// Shared base for every page object. Holds the Playwright `page` and the
// cross-cutting primitives every page reuses: navigation, the generic
// create/rename/delete modal (modal-input → modal-confirm-button), and top-nav.
//
// Page objects expose locators + actions only; specs keep their own `expect`
// assertions against the locators these return.
export class BasePage {
    constructor(page) {
        this.page = page;
    }

    async goto(route) {
        await this.page.goto(route);
    }

    // goto + settle. `page.goto` already waits for 'load' (which is past
    // 'domcontentloaded'), so the extra wait is a no-op safety net matching the
    // goto→domcontentloaded idiom the run specs use.
    async gotoReady(route) {
        await this.page.goto(route);
        await this.page.waitForLoadState('domcontentloaded');
    }

    get modalInput() {
        return this.page.getByTestId('modal-input');
    }

    get modalConfirm() {
        return this.page.getByTestId('modal-confirm-button');
    }

    // fill() replaces existing content, so this covers both create and rename.
    async fillModal(value) {
        await this.modalInput.fill(value);
    }

    async confirmModal() {
        await this.modalConfirm.click();
    }

    // Top-nav buttons are role=button by label (Tests / Categories / Quality / …).
    // Exact by default: a non-exact 'Tests' also matches the sidebar
    // "Hide tests in tree" toggle, tripping strict mode.
    async nav(name, { exact = true } = {}) {
        await this.page.getByRole('button', { name, exact }).click();
    }
}
