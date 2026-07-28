import { BasePage } from './BasePage.js';
import { ROUTES, defectsFocus } from '../config.js';

// Defects register (/defects) — the triage queue: four filter tiles, a filter bar
// (status tabs + severity chips + sort), a bulk bar that appears with a selection,
// and rows that expand into their affected tests.
//
// Tiles and tabs share labels ("Needs triage" and "Fixed" appear in both), so each
// group is addressed through its own container rather than page-wide, or the two
// would collide under strict mode.
export class DefectsPage extends BasePage {
    async open() {
        await this.gotoReady(ROUTES.DEFECTS);
    }

    // Deep link from a test case's linked-bugs panel: lands unfiltered with the
    // target row expanded and scrolled to. The param stays in the URL.
    async openFocused(defectId) {
        await this.gotoReady(defectsFocus(defectId));
    }

    // ── Title bar ──
    // No testid on the search box; the aria-label is the stable handle.
    get searchInput() {
        return this.page.getByLabel('Search defects by title or external key');
    }

    // The register lists every defect in the workspace, so specs narrow it to
    // their own seeds by searching for a per-test stamp.
    async search(text) {
        await this.searchInput.fill(text);
    }

    get newDefectButton() {
        return this.page.getByTestId('defects-new');
    }

    // ── Triage strip ──
    // Each tile is a real button whose accessible name starts with its label
    // (the count and hint follow), so a substring match is enough.
    tile(label) {
        return this.page.locator('.defects-triage-strip').getByRole('button', { name: label });
    }

    // ── Filter bar ──
    tab(label) {
        return this.page.locator('.defects-tabs').getByRole('button', { name: label });
    }

    severityChip(severity) {
        return this.page.locator(`.defects-sev-chips .defects-sev--${severity}`);
    }

    get sortSelect() {
        return this.page.locator('#defects-sort');
    }

    // ── Rows ──
    // Data rows carry data-testid="defects-row"; the expand panel and the empty
    // state do not, so this collection counts only defects.
    get rows() {
        return this.page.getByTestId('defects-row');
    }

    // hasText is a case-insensitive SUBSTRING match, so seed titles that must be
    // told apart cannot be substrings of one another ("Owned" also finds "Unowned").
    row(title) {
        return this.rows.filter({ hasText: title });
    }

    // Attribute form, not #id: a defect id is a UUID and may start with a digit,
    // which is not a valid CSS id selector.
    rowById(defectId) {
        return this.page.locator(`[id="defect-row-${defectId}"]`);
    }

    // The derived-status pill — Needs triage / In progress / Fixed / Closed.
    // Read-only text by design: it used to be a toggle button that wrote on click.
    statusPill(title) {
        return this.row(title).locator('.defects-status-pill');
    }

    get selectAll() {
        return this.page.getByTestId('defects-select-all');
    }

    async selectRow(title) {
        await this.row(title).getByTestId('defects-row-select').check();
    }

    // Expanding is a click on the row itself; the checkbox and the external-key
    // chip stop propagation, so click the title cell.
    async expandRow(title) {
        await this.row(title).locator('.defects-defect-title').click();
    }

    // ── Expand panel ──
    // Only one row expands at a time, so this is page-scoped.
    get expandPanel() {
        return this.page.getByTestId('defects-expand');
    }

    affectedTest(name) {
        return this.expandPanel.getByRole('link', { name });
    }

    // The "last failed in" run link beside each affected test.
    get lastRunLinks() {
        return this.expandPanel.locator('.defects-affected-run');
    }

    get openDetailButton() {
        return this.page.getByTestId('defects-open-detail');
    }

    get retestButton() {
        return this.page.getByTestId('defects-retest');
    }

    get deleteButton() {
        return this.page.getByTestId('defects-delete');
    }

    // Delete goes through components/Modal (BasePage's modalConfirm), not
    // window.confirm — the point of the change, and the reason it is reachable here.
    get deleteDialog() {
        return this.page.locator('.modal-content').filter({ hasText: 'Delete Defect' });
    }

    get deleteCancel() {
        return this.deleteDialog.getByRole('button', { name: 'Cancel' });
    }

    // "Couldn't load the affected tests." → Retry, inside the expand panel.
    get affectedRetry() {
        return this.page.getByTestId('defects-affected-retry');
    }

    // ── Bulk bar ──
    // Renders only while at least one row is ticked, so assert on it after
    // selecting rather than before.
    get bulkBar() {
        return this.page.getByTestId('defects-bulk-bar');
    }

    get bulkAssign() {
        return this.page.getByTestId('defects-bulk-assign');
    }

    get bulkUnassign() {
        return this.page.getByTestId('defects-bulk-unassign');
    }

    get bulkSeverity() {
        return this.page.getByTestId('defects-bulk-severity');
    }

    get bulkClose() {
        return this.page.getByTestId('defects-bulk-close');
    }

    get bulkClear() {
        return this.page.getByTestId('defects-bulk-clear');
    }

    // The inline failure notice; role=alert so it is announced, not just coloured.
    get bulkError() {
        return this.bulkBar.getByRole('alert');
    }

    // Shown in the Assign… menu when the user list could not be loaded. The load is
    // retryable on purpose: a failure that pinned an empty list would read as "nobody
    // can be assigned" and leave bulk assign dead until the page was reloaded.
    get bulkUsersRetry() {
        return this.page.getByTestId('defects-bulk-users-retry');
    }

    // A user row inside the open Assign… menu, by display name or email.
    bulkAssignee(name) {
        return this.page.locator('.defects-menu').getByRole('button', { name });
    }

    // ── Create/edit modal (components/DefectModal), reached from "+ New defect"
    //    and from a row expand's "Open detail" ──
    // The title input has no testid; it is the modal's only autofocused text box,
    // so its required-field label is the stable handle.
    get modalTitle() {
        return this.page.locator('.glass-panel input.modern-input').first();
    }

    get modalSeverity() {
        return this.page.getByTestId('defect-severity');
    }

    get modalAssignee() {
        return this.page.getByTestId('defect-assignee');
    }

    get modalSave() {
        return this.page.getByTestId('defect-save');
    }

    // ── Notices ──
    // Shown when ?focus= names a defect that no longer exists.
    get focusMissingNotice() {
        return this.page.getByTestId('defects-focus-missing');
    }

    // ── Empty state ──
    get emptyState() {
        return this.page.getByTestId('defects-empty');
    }

    get resetFiltersButton() {
        return this.page.getByTestId('defects-reset-filters');
    }
}
