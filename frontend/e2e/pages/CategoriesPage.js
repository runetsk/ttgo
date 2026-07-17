import { BasePage } from './BasePage.js';
import { ROUTES } from '../config.js';

// Category (suite) manager, under the Quality section (/categories).
export class CategoriesPage extends BasePage {
    async open() {
        await this.goto(ROUTES.CATEGORIES);
    }

    // Assumes the page is already open (either via open() or top-nav).
    async create(name) {
        await this.page.getByTestId('open-create-category-modal').click();
        await this.page.getByTestId('category-name-input').fill(name);
        await this.page.getByTestId('create-category-button').click();
    }

    // open() + create() in one call, for specs that start elsewhere.
    async openAndCreate(name) {
        await this.open();
        await this.create(name);
    }

    // A category grid row, keyed off its `category-row-<id>` testid prefix.
    categoryRow(name) {
        return this.page.locator('[data-testid^="category-row-"]').filter({ hasText: name });
    }

    // Ticks a row's selection checkbox (drives the bulk-action bar).
    async selectCategory(name) {
        await this.categoryRow(name).locator('input[type="checkbox"]').check();
    }

    get bulkDeleteButton() {
        return this.page.getByTestId('bulk-delete-categories-button');
    }

    // Bulk delete confirms via a native `window.confirm` dialog — auto-accept it.
    async bulkDelete() {
        this.page.once('dialog', dialog => dialog.accept());
        await this.bulkDeleteButton.click();
    }
}
