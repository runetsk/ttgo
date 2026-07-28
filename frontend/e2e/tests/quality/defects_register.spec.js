import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

// The Defects register (/defects) as a triage queue.
//
// The page fetches the WHOLE defect list once and filters in memory, so every
// other spec's defects are on screen too. Each test therefore stamps its seeds
// with a unique token and types it into the search box first — after that,
// "the rows" means "the rows this test created", and the counts are exact.
test.describe('Defects register — triage queue', () => {

    test('the Needs triage tile filters the queue to unassigned open defects', async ({ api, defectsPage }) => {
        const stamp = `Triage-${Date.now()}`;
        let owner;

        await test.step('Seed one unassigned, one owned and one closed defect', async () => {
            const { users } = await (await api.get('/users/assignable')).json();
            owner = users[0];
            expect(owner?.id).toBeTruthy();

            // Titles are picked so none is a substring of another: row() filters
            // by case-insensitive hasText, so "Owned" would also match "Unowned".
            await api.createDefect({ title: `Fresh ${stamp}`, severity: 'major' });
            await api.createDefect({ title: `Taken ${stamp}`, severity: 'major', assignee_id: owner.id });
            const closed = await api.createDefect({ title: `Verified ${stamp}`, severity: 'minor' });
            await api.updateDefect(closed.id, { status: 'closed' });
        });

        await test.step('All three land in the register', async () => {
            await defectsPage.open();
            await defectsPage.search(stamp);
            await expect(defectsPage.rows).toHaveCount(3, { timeout: TIMEOUTS.ELEMENT });
        });

        await test.step('Assignment is the triage action: only the unowned one needs triage', async () => {
            await expect(defectsPage.statusPill(`Fresh ${stamp}`)).toHaveText('Needs triage');
            await expect(defectsPage.statusPill(`Taken ${stamp}`)).toHaveText('In progress');
            await expect(defectsPage.statusPill(`Verified ${stamp}`)).toHaveText('Closed');
        });

        await test.step('The tile drops the table to just that one', async () => {
            await defectsPage.tile('Needs triage').click();
            await expect(defectsPage.rows).toHaveCount(1);
            await expect(defectsPage.row(`Fresh ${stamp}`)).toBeVisible();
            await expect(defectsPage.row(`Taken ${stamp}`)).toHaveCount(0);
            await expect(defectsPage.row(`Verified ${stamp}`)).toHaveCount(0);
        });

        await test.step('The status tabs move between queues; All brings them back', async () => {
            await defectsPage.tab('Closed').click();
            await expect(defectsPage.rows).toHaveCount(1);
            await expect(defectsPage.row(`Verified ${stamp}`)).toBeVisible();

            // A tab only changes the status dimension — the typed search stays,
            // so this is still exactly the three seeded rows.
            await defectsPage.tab('All').click();
            await expect(defectsPage.rows).toHaveCount(3);
        });
    });

    test('selecting rows reveals the bulk bar and "Mark verified & close" closes them', async ({ page, api, defectsPage }) => {
        const stamp = `Bulk-${Date.now()}`;
        let first, second;

        await test.step('Seed two open defects', async () => {
            first = await api.createDefect({ title: `Bulk one ${stamp}`, severity: 'major' });
            second = await api.createDefect({ title: `Bulk two ${stamp}`, severity: 'minor' });
        });

        // The point of the bulk bar is one round-trip instead of one PATCH per
        // defect, so count the calls rather than only the resulting state.
        let bulkCalls = 0;
        await page.route('**/api/defects/bulk-update', (route) => {
            bulkCalls += 1;
            return route.continue();
        });

        await test.step('The bar appears only once rows are ticked', async () => {
            await defectsPage.open();
            await defectsPage.search(stamp);
            await expect(defectsPage.rows).toHaveCount(2, { timeout: TIMEOUTS.ELEMENT });

            await expect(defectsPage.bulkBar).toHaveCount(0);
            await defectsPage.selectAll.check();
            await expect(defectsPage.bulkBar).toBeVisible();
            await expect(defectsPage.bulkBar).toContainText('2 selected');
        });

        await test.step('Closing the selection patches both rows in one call', async () => {
            await defectsPage.bulkClose.click();

            await expect(defectsPage.statusPill(`Bulk one ${stamp}`)).toHaveText('Closed');
            await expect(defectsPage.statusPill(`Bulk two ${stamp}`)).toHaveText('Closed');
            expect(bulkCalls).toBe(1);
        });

        await test.step('The selection is dropped after a successful apply', async () => {
            await expect(defectsPage.bulkBar).toHaveCount(0);
        });

        await test.step('The server agrees — this was a real write, not a local patch', async () => {
            const all = await api.listDefects();
            expect(all.find(d => d.id === first.id).status).toBe('closed');
            expect(all.find(d => d.id === second.id).status).toBe('closed');
        });
    });

    test('expanding a row shows its affected tests and the run each last failed in', async ({ api, defectsPage }) => {
        const stamp = `Expand-${Date.now()}`;
        let seed;

        await test.step('Seed a failed result and hang a defect off it', async () => {
            seed = await api.seedRunWithResult({ label: `Expand ${stamp}` });
            await api.createAndLinkResultDefect(seed.run.id, seed.result.id, {
                title: `Regression ${stamp}`,
                description: 'Fails on submit',
                severity: 'critical',
            });
        });

        await test.step('The expand lists the affected test and where it last went red', async () => {
            await defectsPage.open();
            await defectsPage.search(stamp);
            await expect(defectsPage.rows).toHaveCount(1, { timeout: TIMEOUTS.ELEMENT });

            await defectsPage.expandRow(`Regression ${stamp}`);
            await expect(defectsPage.expandPanel).toBeVisible();
            await expect(defectsPage.affectedTest(seed.tc.name)).toHaveAttribute(
                'href',
                `/library/tests/${seed.tc.id}`,
            );

            const runLink = defectsPage.lastRunLinks;
            await expect(runLink).toHaveCount(1);
            await expect(runLink).toHaveText(seed.run.name);
            await expect(runLink).toHaveAttribute('href', `/runs/run/${seed.run.id}`);
        });

        await test.step('Retest is offered, because there is a test to retest', async () => {
            await expect(defectsPage.retestButton).toBeEnabled();
        });

        await test.step('A defect with no linked tests cannot be retested', async () => {
            await api.createDefect({ title: `Unlinked ${stamp}`, severity: 'minor' });
            await defectsPage.open();
            await defectsPage.search(`Unlinked ${stamp}`);
            await expect(defectsPage.rows).toHaveCount(1, { timeout: TIMEOUTS.ELEMENT });

            await defectsPage.expandRow(`Unlinked ${stamp}`);
            await expect(defectsPage.expandPanel).toContainText('No linked tests.');
            await expect(defectsPage.retestButton).toBeDisabled();
        });
    });

    test('changing a filter clears an existing selection', async ({ api, defectsPage }) => {
        const stamp = `Selection-${Date.now()}`;

        await test.step('Seed one critical and one minor defect', async () => {
            await api.createDefect({ title: `Sel critical ${stamp}`, severity: 'critical' });
            await api.createDefect({ title: `Sel minor ${stamp}`, severity: 'minor' });
        });

        await defectsPage.open();
        await defectsPage.search(stamp);
        await expect(defectsPage.rows).toHaveCount(2, { timeout: TIMEOUTS.ELEMENT });

        await test.step('A severity chip that hides the ticked row also drops the selection', async () => {
            await defectsPage.selectRow(`Sel critical ${stamp}`);
            await expect(defectsPage.bulkBar).toContainText('1 selected');

            // Without this, "Mark verified & close" would land on a row the user
            // can no longer see — the failure the clearing rule exists to prevent.
            await defectsPage.severityChip('minor').click();
            await expect(defectsPage.rows).toHaveCount(1);
            await expect(defectsPage.bulkBar).toHaveCount(0);
        });

        await test.step('A status tab clears it too', async () => {
            await defectsPage.severityChip('minor').click();
            await expect(defectsPage.rows).toHaveCount(2);
            await defectsPage.selectAll.check();
            await expect(defectsPage.bulkBar).toContainText('2 selected');

            await defectsPage.tab('Needs triage').click();
            await expect(defectsPage.bulkBar).toHaveCount(0);
        });

        await test.step('So does Sort — reordering hides nothing, but "N selected" would move', async () => {
            await defectsPage.selectAll.check();
            await expect(defectsPage.bulkBar).toContainText('2 selected');

            await defectsPage.sortSelect.selectOption('updated');
            await expect(defectsPage.bulkBar).toHaveCount(0);
        });
    });

    test('?focus= lands on the defect, expanded, and stays in the URL', async ({ page, api, defectsPage }) => {
        const stamp = `Focus-${Date.now()}`;
        let defect;

        await test.step('Seed a defect to deep-link into', async () => {
            defect = await api.createDefect({ title: `Focused ${stamp}`, severity: 'major' });
        });

        await test.step('The row is rendered and already expanded', async () => {
            await defectsPage.openFocused(defect.id);
            await expect(defectsPage.rowById(defect.id)).toBeVisible({ timeout: TIMEOUTS.ELEMENT });
            await expect(defectsPage.rowById(defect.id)).toHaveAttribute('aria-expanded', 'true');
            await expect(defectsPage.expandPanel).toBeVisible();
        });

        await test.step('The param survives — the page never strips it', async () => {
            // linked_bugs_requirements.spec.js asserts this URL after clicking a
            // linked bug; it used to pass only inside the window before the old
            // page stripped the param in its fetch callback.
            await expect(page).toHaveURL(new RegExp(`/defects\\?focus=${defect.id}`));
            await defectsPage.search(stamp);
            await expect(page).toHaveURL(new RegExp(`/defects\\?focus=${defect.id}`));
        });
    });
});
