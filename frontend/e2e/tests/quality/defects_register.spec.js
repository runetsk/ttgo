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

    test('the Critical open tile shows the criticals that are still open, not every critical', async ({ api, defectsPage }) => {
        const stamp = `Critical-${Date.now()}`;

        await test.step('Seed an open critical, a closed critical and an open major', async () => {
            await api.createDefect({ title: `Live crit ${stamp}`, severity: 'critical' });
            const done = await api.createDefect({ title: `Done crit ${stamp}`, severity: 'critical' });
            await api.updateDefect(done.id, { status: 'closed' });
            await api.createDefect({ title: `Lesser ${stamp}`, severity: 'major' });
        });

        // The tile counts criticals that are NOT closed. Before "openOnly" existed as a
        // filter dimension it navigated to every critical, so a tile reading 1 landed
        // the user on 2 rows — the number on its face was wrong the moment it was used.
        await test.step('The tile drops the table to the open critical alone', async () => {
            await defectsPage.open();
            await defectsPage.search(stamp);
            await expect(defectsPage.rows).toHaveCount(3, { timeout: TIMEOUTS.ELEMENT });

            await defectsPage.tile('Critical open').click();
            await expect(defectsPage.rows).toHaveCount(1);
            await expect(defectsPage.row(`Live crit ${stamp}`)).toBeVisible();
            await expect(defectsPage.row(`Done crit ${stamp}`)).toHaveCount(0);
        });

        await test.step('And it says so: a tile whose filters are in force reads as pressed', async () => {
            await expect(defectsPage.tile('Critical open')).toHaveAttribute('aria-pressed', 'true');
            await expect(defectsPage.tile('Needs triage')).toHaveAttribute('aria-pressed', 'false');
        });

        // aria-pressed promises a toggle, so the tile has to honour it — and for the
        // tiles whose preset is invisible in the filter bar it is the only on-screen
        // way back out: "Reset filters" exists only inside the empty state, so a tile
        // that returns rows used to leave a narrowed queue with nothing to clear it.
        await test.step('Clicking the pressed tile again releases it', async () => {
            await defectsPage.tile('Critical open').click();
            await expect(defectsPage.tile('Critical open')).toHaveAttribute('aria-pressed', 'false');
            await expect(defectsPage.severityChip('critical')).toHaveAttribute('aria-pressed', 'false');
            await expect(defectsPage.rows).toHaveCount(3);

            // Back into the tile's queue for the step below.
            await defectsPage.tile('Critical open').click();
            await expect(defectsPage.rows).toHaveCount(1);
        });

        // "not closed" has no control in the filter bar, so it must not survive a manual
        // filter — otherwise the table stays narrower than every count on the page while
        // nothing on screen says why. The severity chip DOES have a control, so it stays
        // (and says so with its own aria-pressed): both criticals come back, not all three.
        await test.step('Touching a status tab releases it — the invisible dimension goes with it', async () => {
            await defectsPage.tab('All').click();
            await expect(defectsPage.tile('Critical open')).toHaveAttribute('aria-pressed', 'false');
            await expect(defectsPage.severityChip('critical')).toHaveAttribute('aria-pressed', 'true');
            await expect(defectsPage.rows).toHaveCount(2);
            await expect(defectsPage.row(`Done crit ${stamp}`)).toHaveCount(1);
        });
    });

    test('the Stale tile is a real filter, not a button that resets the page', async ({ api, defectsPage }) => {
        const stamp = `Stale-${Date.now()}`;

        // updated_at is set by the server, so a freshly seeded defect can never be
        // stale. What is assertable — and what actually broke — is that the tile
        // NARROWS: its preset used to be byte-identical to a fresh page load, so
        // clicking a tile reading "9" left every defect on screen.
        await test.step('Seed two defects that were touched a moment ago', async () => {
            await api.createDefect({ title: `Recent one ${stamp}`, severity: 'major' });
            await api.createDefect({ title: `Recent two ${stamp}`, severity: 'minor' });
        });

        await defectsPage.open();
        await defectsPage.search(stamp);
        await expect(defectsPage.rows).toHaveCount(2, { timeout: TIMEOUTS.ELEMENT });

        await test.step('Nothing seeded this second is 7 days old, so the queue is empty', async () => {
            await defectsPage.tile('Stale').click();
            await expect(defectsPage.rows).toHaveCount(0);
            await expect(defectsPage.emptyState).toBeVisible();
            await expect(defectsPage.tile('Stale')).toHaveAttribute('aria-pressed', 'true');
        });

        // Stale is the tile with no counterpart in the filter bar at all — its preset
        // sets only "not closed" and "untouched for 7 days", so nothing else on the
        // page reflects it. The tile itself has to be the way out.
        await test.step('The tile is a toggle: clicking it again releases the filter', async () => {
            await defectsPage.tile('Stale').click();
            await expect(defectsPage.tile('Stale')).toHaveAttribute('aria-pressed', 'false');
            await expect(defectsPage.rows).toHaveCount(2);

            // Re-applied, so the Reset step below still starts from the empty state.
            await defectsPage.tile('Stale').click();
            await expect(defectsPage.rows).toHaveCount(0);
        });

        await test.step('Reset filters brings them back and releases the tile', async () => {
            await defectsPage.resetFiltersButton.click();
            await expect(defectsPage.tile('Stale')).toHaveAttribute('aria-pressed', 'false');
            // Reset clears the search too, so this is the whole register again.
            await expect(defectsPage.rows.first()).toBeVisible();
            await defectsPage.search(stamp);
            await expect(defectsPage.rows).toHaveCount(2);
        });
    });

    test('the whole list is fetched once, and no filter refetches it', async ({ page, api, defectsPage }) => {
        const stamp = `Once-${Date.now()}`;
        await api.createDefect({ title: `Counted ${stamp}`, severity: 'major' });

        // The redesign's load-bearing claim: one request on mount, everything else
        // derived in memory. A filter-driven refetch would regress the thing the page
        // was rebuilt for and nothing else in the suite would notice.
        let listCalls = 0;
        await page.route('**/api/defects', (route) => {
            if (route.request().method() === 'GET') listCalls += 1;
            return route.continue();
        });

        await defectsPage.open();
        await expect(defectsPage.rows.first()).toBeVisible({ timeout: TIMEOUTS.ELEMENT });
        // Baselined rather than asserted as exactly 1: against the Vite dev server
        // StrictMode double-invokes the mount effect, which is a dev-only artefact. What
        // matters — and what a regression would break — is that the number stops moving.
        const afterMount = listCalls;
        expect(afterMount).toBeGreaterThan(0);

        await test.step('Search, tab, tile, chip and sort all derive from that one response', async () => {
            await defectsPage.search(stamp);
            await expect(defectsPage.rows).toHaveCount(1);
            await defectsPage.tab('Needs triage').click();
            await defectsPage.tile('Critical open').click();
            await defectsPage.severityChip('major').click();
            await defectsPage.sortSelect.selectOption('updated');
            await expect(defectsPage.rows).toHaveCount(1);
            expect(listCalls).toBe(afterMount);
        });
    });

    test('deleting a defect asks first, and a failed delete leaves the row alone', async ({ page, api, defectsPage }) => {
        const stamp = `Delete-${Date.now()}`;
        let target;

        await test.step('Seed one defect to delete and one bystander', async () => {
            target = await api.createDefect({ title: `Doomed ${stamp}`, severity: 'minor' });
            await api.createDefect({ title: `Bystander ${stamp}`, severity: 'minor' });
        });

        await defectsPage.open();
        await defectsPage.search(stamp);
        await expect(defectsPage.rows).toHaveCount(2, { timeout: TIMEOUTS.ELEMENT });

        await test.step('Cancel keeps the row — the dialog names the defect first', async () => {
            await defectsPage.expandRow(`Doomed ${stamp}`);
            await defectsPage.deleteButton.click();
            await expect(defectsPage.deleteDialog).toContainText(`Doomed ${stamp}`);
            await defectsPage.deleteCancel.click();
            await expect(defectsPage.deleteDialog).toHaveCount(0);
            await expect(defectsPage.rows).toHaveCount(2);
        });

        await test.step('A rejected DELETE keeps the row too — nothing is removed optimistically', async () => {
            await page.route(`**/api/defects/${target.id}`, (route) => (
                route.request().method() === 'DELETE'
                    ? route.fulfill({ status: 500, contentType: 'application/json', body: '{"error":"boom"}' })
                    : route.continue()
            ));
            await defectsPage.deleteButton.click();
            await defectsPage.confirmModal();
            await expect(defectsPage.deleteDialog).toHaveCount(0);
            await expect(defectsPage.row(`Doomed ${stamp}`)).toHaveCount(1);
            await page.unroute(`**/api/defects/${target.id}`);
        });

        await test.step('Confirming removes exactly that row, and the server agrees', async () => {
            await defectsPage.deleteButton.click();
            await defectsPage.confirmModal();
            await expect(defectsPage.row(`Doomed ${stamp}`)).toHaveCount(0);
            await expect(defectsPage.row(`Bystander ${stamp}`)).toHaveCount(1);

            const all = await api.listDefects();
            expect(all.find(d => d.id === target.id)).toBeUndefined();
        });
    });

    test('?focus= on a defect that no longer exists says so instead of looking normal', async ({ api, defectsPage }) => {
        const stamp = `Gone-${Date.now()}`;
        const doomed = await api.createDefect({ title: `Vanishing ${stamp}`, severity: 'minor' });
        await api.delete(`/defects/${doomed.id}`);

        // utils/bugs.js bugHref makes this link load-bearing from every test case's
        // linked-bugs panel, so a dead one has to be legible as a dead one.
        await defectsPage.openFocused(doomed.id);
        await expect(defectsPage.focusMissingNotice).toBeVisible({ timeout: TIMEOUTS.ELEMENT });
        await expect(defectsPage.expandPanel).toHaveCount(0);
    });

    test('Retest builds a run from the defect\'s tests and opens it', async ({ page, api, defectsPage }) => {
        const stamp = `Retest-${Date.now()}`;
        let seed;

        await test.step('Seed a failing result with a defect hung off it', async () => {
            seed = await api.seedRunWithResult({ label: `Retest ${stamp}` });
            await api.createAndLinkResultDefect(seed.run.id, seed.result.id, {
                title: `Retestable ${stamp}`,
                severity: 'major',
            });
        });

        // buildRetestRun is unit-tested to death, but nothing checked that its output
        // reaches createTestRun with the right argument order, or that the navigation
        // happens — a run created with no test cases would have looked identical here.
        let posted = null;
        await page.route('**/api/runs', (route) => {
            const req = route.request();
            if (req.method() === 'POST') posted = req.postDataJSON();
            return route.continue();
        });

        await defectsPage.open();
        await defectsPage.search(stamp);
        await expect(defectsPage.rows).toHaveCount(1, { timeout: TIMEOUTS.ELEMENT });
        await defectsPage.expandRow(`Retestable ${stamp}`);
        await expect(defectsPage.retestButton).toBeEnabled();
        await defectsPage.retestButton.click();

        await test.step('It lands on the new run, carrying exactly the affected test', async () => {
            await expect(page).toHaveURL(/\/runs\/run\/[^/]+$/, { timeout: TIMEOUTS.ELEMENT });
            expect(posted).toBeTruthy();
            expect(posted.name).toBe(`Retest: Retestable ${stamp}`);
            expect(posted.test_case_ids).toEqual([seed.tc.id]);
            // POST /runs rejects a body carrying both, so the retest never sends one.
            expect(posted.category_id ?? null).toBeNull();
        });
    });

    test('creating a defect from a filtered queue shows the row it just made', async ({ api, defectsPage }) => {
        const stamp = `Created-${Date.now()}`;
        await api.createDefect({ title: `Existing ${stamp}`, severity: 'minor' });

        await defectsPage.open();
        await defectsPage.search(stamp);
        await expect(defectsPage.rows).toHaveCount(1, { timeout: TIMEOUTS.ELEMENT });

        // A new defect is unowned and open, so it lands in Needs triage — never in
        // the Closed queue someone might be standing in. The modal used to close
        // onto an unchanged table, which reads as a failed save and invites a
        // duplicate; the page drops back to the unfiltered view instead.
        await test.step('Create while filtered to Closed', async () => {
            await defectsPage.tab('Closed').click();
            await expect(defectsPage.rows).toHaveCount(0);

            await defectsPage.newDefectButton.click();
            await defectsPage.modalTitle.fill(`Brand new ${stamp}`);
            await defectsPage.modalSeverity.selectOption('major');
            await defectsPage.modalSave.click();
        });

        await test.step('The filters step aside and the new row is on screen', async () => {
            await expect(defectsPage.modalSave).toHaveCount(0, { timeout: TIMEOUTS.ELEMENT });
            await expect(defectsPage.row(`Brand new ${stamp}`)).toBeVisible();
            await expect(defectsPage.tab('Closed')).toHaveAttribute('aria-pressed', 'false');
        });
    });

    // The selection rule is "a bulk action can only ever land on rows in this queue",
    // which every filter control upholds by dropping the selection. An edit is the one
    // way a row leaves the queue without a filter being touched — so it has to uphold it
    // too, or "Mark verified & close" writes to a defect that is not on screen.
    test('editing a defect out of the queue takes it out of the selection', async ({ api, defectsPage }) => {
        const stamp = `EditOut-${Date.now()}`;
        const defect = await api.createDefect({ title: `Movable ${stamp}`, severity: 'major' });

        await defectsPage.open();
        await defectsPage.search(stamp);
        await expect(defectsPage.rows).toHaveCount(1, { timeout: TIMEOUTS.ELEMENT });
        await defectsPage.selectAll.check();
        await expect(defectsPage.bulkBar).toContainText('1 selected');

        await test.step('Rename it so it no longer matches the search it was ticked in', async () => {
            await defectsPage.expandRow(`Movable ${stamp}`);
            await defectsPage.openDetailButton.click();
            await defectsPage.modalTitle.fill(`Renamed out of ${Date.now()}`);
            await defectsPage.modalSave.click();
            await expect(defectsPage.modalSave).toHaveCount(0, { timeout: TIMEOUTS.ELEMENT });
        });

        await test.step('The row is gone from the queue, and so is the bulk bar', async () => {
            await expect(defectsPage.rows).toHaveCount(0);
            await expect(defectsPage.emptyState).toBeVisible();
            await expect(defectsPage.bulkBar).toHaveCount(0);
        });

        await test.step('The defect itself is untouched apart from the rename', async () => {
            const stored = (await api.listDefects()).find(d => d.id === defect.id);
            expect(stored.status).toBe('open');
        });
    });

    test('a defect owned by a deactivated user is still editable', async ({ api, defectsPage }) => {
        const stamp = `Deactivated-${Date.now()}`;
        let defect;

        await test.step('Seed a defect, then deactivate its owner', async () => {
            const created = await api.post('/users', {
                email: `leaver-${Date.now()}@example.com`,
                display_name: `Leaver ${stamp}`,
                password: 'changeme123',
                role: 'member',
            });
            expect(created.status()).toBe(201);
            const owner = await created.json();

            defect = await api.createDefect({ title: `Orphaned ${stamp}`, severity: 'minor', assignee_id: owner.id });
            const patched = await api.patch(`/users/${owner.id}`, { active: false });
            expect(patched.status()).toBe(200);
        });

        // The owner is gone from /users/assignable but is kept in the picker so an
        // unrelated save cannot silently unassign them. That guard used to make the
        // form unusable: the payload echoed the id back, the server rejected any
        // assignee that is not an active user, and the modal swallowed the 400 —
        // the dialog just sat there. An untouched assignee is now simply not sent.
        await test.step('Editing another field saves instead of failing with a swallowed 400', async () => {
            await defectsPage.open();
            await defectsPage.search(stamp);
            await expect(defectsPage.rows).toHaveCount(1, { timeout: TIMEOUTS.ELEMENT });

            await defectsPage.expandRow(`Orphaned ${stamp}`);
            await defectsPage.openDetailButton.click();
            // The owner is offered, and marked so nobody picks them on purpose.
            await expect(defectsPage.modalAssignee).toHaveValue(defect.assignee_id);
            await expect(defectsPage.modalAssignee).toContainText('(inactive)');

            await defectsPage.modalSeverity.selectOption('critical');
            await defectsPage.modalSave.click();
            await expect(defectsPage.modalSave).toHaveCount(0, { timeout: TIMEOUTS.ELEMENT });
        });

        await test.step('The change landed and the owner is still there', async () => {
            const stored = (await api.listDefects()).find(d => d.id === defect.id);
            expect(stored.severity).toBe('critical');
            expect(stored.assignee_id).toBe(defect.assignee_id);
        });
    });

    test('a failed bulk update explains itself and keeps the selection', async ({ page, api, defectsPage }) => {
        const stamp = `BulkFail-${Date.now()}`;
        await api.createDefect({ title: `Stubborn ${stamp}`, severity: 'major' });

        await defectsPage.open();
        await defectsPage.search(stamp);
        await expect(defectsPage.rows).toHaveCount(1, { timeout: TIMEOUTS.ELEMENT });
        await defectsPage.selectAll.check();
        await expect(defectsPage.bulkBar).toContainText('1 selected');

        await test.step('The server\'s own reason is shown, not a generic failure', async () => {
            await page.route('**/api/defects/bulk-update', route => route.fulfill({
                status: 400,
                contentType: 'application/json',
                body: '{"error":"too many ids (max 500 per request)"}',
            }));
            await defectsPage.bulkClose.click();
            await expect(defectsPage.bulkError).toContainText('too many ids');
        });

        await test.step('The selection survives, so the action can just be retried', async () => {
            await expect(defectsPage.bulkBar).toContainText('1 selected');
            await expect(defectsPage.statusPill(`Stubborn ${stamp}`)).toHaveText('Needs triage');
        });

        await test.step('The failure does not follow the next selection around', async () => {
            // The bar never unmounts — the page renders it always and it hides itself on
            // an empty selection — so without retiring the message it would still be
            // there over the next set of rows ticked, which nothing has been tried on.
            await defectsPage.bulkClear.click();
            await expect(defectsPage.bulkBar).toHaveCount(0);
            await defectsPage.selectAll.check();
            await expect(defectsPage.bulkBar).toContainText('1 selected');
            await expect(defectsPage.bulkError).toHaveCount(0);
        });

        await test.step('Retrying against a working server applies it', async () => {
            await page.unroute('**/api/defects/bulk-update');
            await defectsPage.bulkClose.click();
            await expect(defectsPage.statusPill(`Stubborn ${stamp}`)).toHaveText('Closed');
        });
    });

    // The bulk call captures its ids at click time, so a selection that moves while the
    // request is in flight is one the reply no longer describes: on success the page clears
    // whatever is ticked, silently discarding rows added since, and on failure the message
    // lands over a set nothing was ever tried on. The bar therefore locks the selection for
    // the round-trip, and stamps each failure with the selection it was fired on so a late
    // one cannot be shown against a different one.
    test('a bulk apply in flight locks the selection, and a late failure cannot land on another', async ({ page, api, defectsPage }) => {
        const stamp = `Race-${Date.now()}`;

        await test.step('Seed three defects and tick them all', async () => {
            await api.createDefect({ title: `Race one ${stamp}`, severity: 'major' });
            await api.createDefect({ title: `Race two ${stamp}`, severity: 'major' });
            await api.createDefect({ title: `Race three ${stamp}`, severity: 'minor' });

            await defectsPage.open();
            await defectsPage.search(stamp);
            await expect(defectsPage.rows).toHaveCount(3, { timeout: TIMEOUTS.ELEMENT });
            await defectsPage.selectAll.check();
            await expect(defectsPage.bulkBar).toContainText('3 selected');
        });

        // The reply is held open inside the route handler, so "in flight" is a state the
        // test opens and closes rather than a window it has to race.
        let release;
        const held = new Promise(resolve => { release = resolve; });
        await page.route('**/api/defects/bulk-update', async (route) => {
            await held;
            await route.fulfill({
                status: 500,
                contentType: 'application/json',
                body: '{"error":"held while the selection moved"}',
            });
        });

        await test.step('While it is in flight, nothing that ticks a row is live', async () => {
            await defectsPage.bulkClose.click();
            await expect(defectsPage.selectAll).toBeDisabled();
            await expect(defectsPage.row(`Race three ${stamp}`).getByTestId('defects-row-select')).toBeDisabled();
            // Clear is the one bar control that was never given the busy flag, and it is
            // the one that empties the selection outright.
            await expect(defectsPage.bulkClear).toBeDisabled();
            await expect(defectsPage.bulkClose).toBeDisabled();

            // Forced past the actionability check, because refusing the click IS the fix:
            // a disabled checkbox takes no input, so the count cannot move.
            await defectsPage.row(`Race three ${stamp}`).getByTestId('defects-row-select')
                .click({ force: true });
            await expect(defectsPage.bulkBar).toContainText('3 selected');
        });

        await test.step('Released, the failure lands on exactly the selection it was fired on', async () => {
            release();
            await expect(defectsPage.bulkError).toContainText('held while the selection moved', {
                timeout: TIMEOUTS.ELEMENT,
            });
            await expect(defectsPage.bulkBar).toContainText('3 selected');
            await expect(defectsPage.selectAll).toBeEnabled();
            await expect(defectsPage.bulkClear).toBeEnabled();
        });

        await page.unroute('**/api/defects/bulk-update');

        // No filter control is locked — every one of them drops the selection instead, so
        // this is the one way left for a reply to come back to a selection that is gone.
        // It has to be retired rather than shown against whatever is ticked by then.
        await test.step('A filter change mid-flight retires that apply\'s failure for good', async () => {
            let releaseSecond;
            const heldSecond = new Promise(resolve => { releaseSecond = resolve; });
            await page.route('**/api/defects/bulk-update', async (route) => {
                await heldSecond;
                await route.fulfill({
                    status: 500,
                    contentType: 'application/json',
                    body: '{"error":"reply to a selection that is gone"}',
                });
            });

            await defectsPage.bulkClose.click();
            await defectsPage.sortSelect.selectOption('updated');
            await expect(defectsPage.bulkBar).toHaveCount(0);

            const landed = page.waitForResponse(r => r.url().includes('/api/defects/bulk-update'));
            releaseSecond();
            await landed;

            // And re-ticking byte-for-byte the same three rows starts clean: the failure
            // belonged to that one visit to the selection, not to the id set.
            await defectsPage.selectAll.check();
            await expect(defectsPage.bulkBar).toContainText('3 selected');
            await expect(defectsPage.bulkError).toHaveCount(0);
            await page.unroute('**/api/defects/bulk-update');
        });

        await test.step('Nothing was written — both applies failed', async () => {
            const mine = (await api.listDefects()).filter(d => d.title.includes(stamp));
            expect(mine).toHaveLength(3);
            expect(mine.every(d => d.status === 'open')).toBe(true);
        });
    });

    // The other half of the lock above, and the one that actually loses data. Freezing the
    // checkboxes keeps the id SET still; it does nothing about the row's own write paths.
    // DefectModal seeds its form from the row as it was BEFORE the bulk write and sends the
    // FULL record on save (it is a full-record editor by design), so a detail opened on a
    // selected row and saved while the bulk is in flight PATCHes the pre-bulk status and
    // severity straight back over it — "Mark verified & close" silently reverts for that row,
    // with nothing on screen to say so. The freeze is scoped to SELECTED rows: those are the
    // only ones the call is writing to.
    test('a bulk apply in flight freezes the write paths of the rows it is writing to', async ({ page, api, defectsPage }) => {
        const stamp = `Overwrite-${Date.now()}`;
        let target;

        await test.step('Seed a selected defect and a bystander that stays unticked', async () => {
            target = await api.createDefect({ title: `Ticked ${stamp}`, severity: 'major' });
            await api.createDefect({ title: `Untouched ${stamp}`, severity: 'minor' });

            await defectsPage.open();
            await defectsPage.search(stamp);
            await expect(defectsPage.rows).toHaveCount(2, { timeout: TIMEOUTS.ELEMENT });
            await defectsPage.selectRow(`Ticked ${stamp}`);
            await expect(defectsPage.bulkBar).toContainText('1 selected');
        });

        // Counted rather than inferred: the whole point is that no stale full-record PATCH
        // can be issued while the bulk is in flight, and a PATCH is the only way it could be.
        let patches = 0;
        await page.route('**/api/defects/*', (route) => {
            if (route.request().method() === 'PATCH') patches += 1;
            return route.continue();
        });

        // Held open inside the route handler, so "in flight" is a state the test opens and
        // closes rather than a window it has to race.
        let release;
        const held = new Promise(resolve => { release = resolve; });
        await page.route('**/api/defects/bulk-update', async (route) => {
            await held;
            await route.continue();
        });

        // Expanded BEFORE the apply: expanding is deliberately not locked (it is read-only,
        // and it is how the user watches what is happening), so the panel is already open and
        // its buttons have to be the thing that refuses.
        await defectsPage.expandRow(`Ticked ${stamp}`);
        await expect(defectsPage.openDetailButton).toBeEnabled();

        await test.step('While it is in flight, the selected row cannot be opened, deleted or retested', async () => {
            await defectsPage.bulkClose.click();
            await expect(defectsPage.openDetailButton).toBeDisabled();
            await expect(defectsPage.deleteButton).toBeDisabled();
            await expect(defectsPage.retestButton).toBeDisabled();
        });

        await test.step('Forcing the click past the actionability check opens nothing', async () => {
            // Refusing the click IS the fix, so the click has to be forced to prove it:
            // a disabled button takes no input, so the modal never opens and no PATCH exists
            // to overwrite the bulk write with.
            await defectsPage.openDetailButton.click({ force: true });
            await expect(defectsPage.modalSave).toHaveCount(0);
            await defectsPage.deleteButton.click({ force: true });
            await expect(defectsPage.deleteDialog).toHaveCount(0);
        });

        // The lock is on the rows the call captured, not on the table: a row outside the
        // selection is not being written to, so freezing it would cost a working modal for
        // no hazard at all.
        await test.step('A row the apply is not writing to stays fully usable', async () => {
            await defectsPage.expandRow(`Untouched ${stamp}`);
            await expect(defectsPage.openDetailButton).toBeEnabled();
            await expect(defectsPage.deleteButton).toBeEnabled();
        });

        await test.step('Released, the close lands and the server kept it', async () => {
            release();
            await expect(defectsPage.statusPill(`Ticked ${stamp}`)).toHaveText('Closed', {
                timeout: TIMEOUTS.ELEMENT,
            });
            expect(patches).toBe(0);

            const stored = (await api.listDefects()).find(d => d.id === target.id);
            expect(stored.status).toBe('closed');
            expect(stored.severity).toBe('major');
        });

        await page.unroute('**/api/defects/bulk-update');
        await page.unroute('**/api/defects/*');

        // And the freeze really is only for the round-trip.
        await test.step('Afterwards the row is editable again', async () => {
            await defectsPage.expandRow(`Ticked ${stamp}`);
            await expect(defectsPage.openDetailButton).toBeEnabled();
        });
    });

    // …and that freeze has to follow the ids the apply CAPTURED, not what is ticked on
    // screen. Nothing locks the filter controls, and by design they DROP the selection
    // (afterFilterChange) so a bulk action can never land on rows filtered out of sight —
    // so one sort change mid-flight empties it. A freeze keyed on the live selection
    // therefore let go of the very rows the request was still writing to, and the stale
    // full-record PATCH the row's modal issues then landed on top of the bulk write.
    test('the freeze follows the ids the apply captured, not the selection on screen', async ({ page, api, defectsPage }) => {
        const stamp = `Captured-${Date.now()}`;
        let target;

        await test.step('Seed one defect, tick it and open its panel', async () => {
            target = await api.createDefect({ title: `Captured ${stamp}`, severity: 'major' });

            await defectsPage.open();
            await defectsPage.search(stamp);
            await expect(defectsPage.rows).toHaveCount(1, { timeout: TIMEOUTS.ELEMENT });
            await defectsPage.selectRow(`Captured ${stamp}`);
            await expect(defectsPage.bulkBar).toContainText('1 selected');
            // Expanded before the apply: expanding is read-only and deliberately never
            // locked, so the panel's own buttons are what have to refuse.
            await defectsPage.expandRow(`Captured ${stamp}`);
            await expect(defectsPage.openDetailButton).toBeEnabled();
        });

        let patches = 0;
        await page.route('**/api/defects/*', (route) => {
            if (route.request().method() === 'PATCH') patches += 1;
            return route.continue();
        });

        // The RESPONSE is held here, not the request — unlike the test above. The server
        // applies the close immediately and only the page is kept waiting, which is the
        // shape that actually loses data: a stale save issued while the page still thinks
        // it is waiting lands AFTER the bulk write and wins.
        let release, served;
        const held = new Promise(resolve => { release = resolve; });
        const reached = new Promise(resolve => { served = resolve; });
        await page.route('**/api/defects/bulk-update', async (route) => {
            const response = await route.fetch();
            served();
            await held;
            await route.fulfill({ response });
        });

        await test.step('The apply goes out and the server has already applied it', async () => {
            await defectsPage.bulkClose.click();
            await reached;
        });

        await test.step('A sort change empties the selection — the freeze stays put', async () => {
            await defectsPage.sortSelect.selectOption('updated');
            // The selection really is gone: the bar only renders while something is ticked.
            await expect(defectsPage.bulkBar).toHaveCount(0);

            await expect(defectsPage.openDetailButton).toBeDisabled();
            await expect(defectsPage.deleteButton).toBeDisabled();
            await expect(defectsPage.retestButton).toBeDisabled();
            // Table-wide and driven by the same in-flight ids, so it holds too.
            await expect(defectsPage.selectAll).toBeDisabled();
        });

        await test.step('So no stale full-record save can be issued over the bulk write', async () => {
            // Forced past the actionability check, because refusing the click IS the fix.
            await defectsPage.openDetailButton.click({ force: true });
            await expect(defectsPage.modalSave).toHaveCount(0);
            await defectsPage.deleteButton.click({ force: true });
            await expect(defectsPage.deleteDialog).toHaveCount(0);
        });

        // The other thing keying on the captured ids buys: the freeze is a property of the
        // defect, not of a row component, so filtering it off the queue and back does not
        // reset it.
        await test.step('It survives the row leaving the queue and coming back', async () => {
            await defectsPage.tab('Closed').click();
            await expect(defectsPage.row(`Captured ${stamp}`)).toHaveCount(0);
            await defectsPage.tab('All').click();
            await expect(defectsPage.row(`Captured ${stamp}`)).toHaveCount(1);
            await expect(defectsPage.openDetailButton).toBeDisabled();
        });

        await test.step('Released, the close is what stands', async () => {
            release();
            await expect(defectsPage.statusPill(`Captured ${stamp}`)).toHaveText('Closed', {
                timeout: TIMEOUTS.ELEMENT,
            });
            await expect(defectsPage.openDetailButton).toBeEnabled();
            await expect(defectsPage.selectAll).toBeEnabled();
            expect(patches).toBe(0);

            const stored = (await api.listDefects()).find(d => d.id === target.id);
            expect(stored.status).toBe('closed');
            expect(stored.severity).toBe('major');
        });

        await page.unroute('**/api/defects/bulk-update');
        await page.unroute('**/api/defects/*');
    });

    // …and the hole that neither of the two freezes above can close, because it does not go
    // through a button at all.
    //
    // A dialog OPENED BEFORE the apply started was opened while nothing was locked, so every
    // disabled control was live at the time and none of them is on the path its Save takes. The
    // overlay does not help: no dialog in this app used to contain focus, nothing behind one is
    // `inert`, and the page renders the bulk bar above the modal in the DOM — so Tab walked out
    // of the open editor straight into "Mark verified & close". Fire that, then Save, and the
    // modal's full-record PATCH (seeded from the PRE-bulk snapshot) lands after the bulk and
    // wins. Five rounds of guarding paths, five different paths; the guard therefore sits on the
    // WRITE — DefectModal.submit and DefectsPage.confirmDelete both ask isBulkLocked immediately
    // before issuing their request, so every path has to pass through it.
    test('a dialog opened before a bulk apply cannot write over it', async ({ page, api, defectsPage }) => {
        const stamp = `Preopened-${Date.now()}`;
        let target;

        await test.step('Tick a row and open its editor while nothing at all is in flight', async () => {
            target = await api.createDefect({ title: `Preopened ${stamp}`, severity: 'major' });

            await defectsPage.open();
            await defectsPage.search(stamp);
            await expect(defectsPage.rows).toHaveCount(1, { timeout: TIMEOUTS.ELEMENT });
            await defectsPage.selectRow(`Preopened ${stamp}`);
            await expect(defectsPage.bulkBar).toContainText('1 selected');

            await defectsPage.expandRow(`Preopened ${stamp}`);
            // Live, because nothing is locked yet — which is the whole premise.
            await expect(defectsPage.openDetailButton).toBeEnabled();
            await defectsPage.openDetailButton.click();
            await expect(defectsPage.modalSave).toBeVisible();
        });

        // Half one: close the keyboard path. role="dialog" + aria-modal claim the page behind is
        // unreachable, and useDialogFocus is what makes that true — without it Tab ran off the
        // modal's last control, wrapped to the top of the page and reached the bulk bar.
        await test.step('The open dialog contains focus — Tab never reaches the bar behind it', async () => {
            await expect(defectsPage.modalDialog).toHaveAttribute('aria-modal', 'true');
            expect(await defectsPage.tabOutOf(defectsPage.modalDialog)).toBeNull();
        });

        // Counted rather than inferred: a PATCH is the only way the pre-bulk record could be
        // written back, so zero of them is the claim.
        let patches = 0;
        await page.route('**/api/defects/*', (route) => {
            if (route.request().method() === 'PATCH') patches += 1;
            return route.continue();
        });

        // The RESPONSE is held, not the request: the server has already closed the defect and
        // only the page is kept waiting. That is the shape that loses data — a save issued now
        // lands AFTER the bulk write.
        let release, served;
        const held = new Promise(resolve => { release = resolve; });
        const reached = new Promise(resolve => { served = resolve; });
        await page.route('**/api/defects/bulk-update', async (route) => {
            const response = await route.fetch();
            served();
            await held;
            await route.fulfill({ response });
        });

        await test.step('Half two: the bar is driven directly, as if focus had got there', async () => {
            // dispatchEvent rather than a click — the overlay covers the bar, and the point is
            // that the WRITE refuses regardless of how its trigger was reached. Any future path
            // to this button is covered by the same assertion.
            await defectsPage.bulkClose.dispatchEvent('click');
            await reached;
        });

        await test.step('Save is refused, says why, and issues no request at all', async () => {
            await defectsPage.modalSave.click();
            await expect(defectsPage.modalRefusal).toContainText('bulk update', {
                timeout: TIMEOUTS.UI_SETTLE,
            });
            // Refused, not closed: nothing the user typed is thrown away.
            await expect(defectsPage.modalSave).toBeVisible();
            expect(patches).toBe(0);
        });

        await test.step('Released, the bulk close is what stands — nothing reverted it', async () => {
            release();
            await expect(defectsPage.statusPill(`Preopened ${stamp}`)).toHaveText('Closed', {
                timeout: TIMEOUTS.ELEMENT,
            });
            expect(patches).toBe(0);

            const stored = (await api.listDefects()).find(d => d.id === target.id);
            expect(stored.status).toBe('closed');
            expect(stored.severity).toBe('major');
        });

        await page.unroute('**/api/defects/bulk-update');

        // Guarding the ROUND-TRIP alone would only have moved this overwrite to the far side of
        // it. Nothing is in flight now and no button is disabled, but this dialog is still
        // holding the pre-bulk snapshot — so the refusal has to outlive the request that caused
        // it, or waiting two seconds and pressing Save is the whole bug again.
        await test.step('Still refused once the apply has LANDED — the snapshot is what is stale', async () => {
            await defectsPage.modalSave.click();
            await expect(defectsPage.modalRefusal).toContainText('bulk update', {
                timeout: TIMEOUTS.UI_SETTLE,
            });
            expect(patches).toBe(0);

            const stored = (await api.listDefects()).find(d => d.id === target.id);
            expect(stored.status).toBe('closed');
        });

        // Reopening is what clears it: a fresh editor is seeded from the post-bulk row, so it
        // has nothing stale to write and saves normally.
        await test.step('Afterwards the editor works again, on the row as it now is', async () => {
            await defectsPage.modalCancel.click();
            await expect(defectsPage.modalSave).toHaveCount(0);

            await expect(defectsPage.openDetailButton).toBeEnabled();
            await defectsPage.openDetailButton.click();
            await defectsPage.modalSeverity.selectOption('critical');
            await defectsPage.modalSave.click();
            await expect(defectsPage.modalSave).toHaveCount(0, { timeout: TIMEOUTS.ELEMENT });

            const stored = (await api.listDefects()).find(d => d.id === target.id);
            expect(stored.severity).toBe('critical');
            expect(stored.status).toBe('closed');
        });

        await page.unroute('**/api/defects/*');
    });

    // The same hole through the other dialog. components/Modal is the shared confirm, so the
    // delete confirmation can be standing open when an apply starts too — and confirming it
    // would DELETE the row the bulk call is mid-write on.
    test('a delete confirmation opened before a bulk apply cannot destroy the row', async ({ page, api, defectsPage }) => {
        const stamp = `PreDelete-${Date.now()}`;
        let target;

        await test.step('Tick a row and open its delete confirmation before anything is in flight', async () => {
            target = await api.createDefect({ title: `Doomed early ${stamp}`, severity: 'major' });

            await defectsPage.open();
            await defectsPage.search(stamp);
            await expect(defectsPage.rows).toHaveCount(1, { timeout: TIMEOUTS.ELEMENT });
            await defectsPage.selectRow(`Doomed early ${stamp}`);
            await expect(defectsPage.bulkBar).toContainText('1 selected');

            await defectsPage.expandRow(`Doomed early ${stamp}`);
            await expect(defectsPage.deleteButton).toBeEnabled();
            await defectsPage.deleteButton.click();
            await expect(defectsPage.deleteDialog).toContainText(`Doomed early ${stamp}`);
        });

        await test.step('components/Modal contains focus too — this was an app-wide gap', async () => {
            await expect(defectsPage.deleteDialog).toHaveAttribute('aria-modal', 'true');
            expect(await defectsPage.tabOutOf(defectsPage.deleteDialog)).toBeNull();
        });

        let deletes = 0;
        await page.route('**/api/defects/*', (route) => {
            if (route.request().method() === 'DELETE') deletes += 1;
            return route.continue();
        });

        let release, served;
        const held = new Promise(resolve => { release = resolve; });
        const reached = new Promise(resolve => { served = resolve; });
        await page.route('**/api/defects/bulk-update', async (route) => {
            const response = await route.fetch();
            served();
            await held;
            await route.fulfill({ response });
        });

        await test.step('Confirming while the apply is in flight deletes nothing and says why', async () => {
            await defectsPage.bulkClose.dispatchEvent('click');
            await reached;

            await defectsPage.confirmModal();
            await expect(defectsPage.deleteDialog).toHaveCount(0);
            // The refusal lands in the row's own expand panel — the only place Delete is
            // reachable from, so it is on screen.
            await expect(defectsPage.rowNotice).toContainText('bulk update', {
                timeout: TIMEOUTS.UI_SETTLE,
            });
            expect(deletes).toBe(0);
        });

        await test.step('Released, the defect is closed and still exists', async () => {
            release();
            await expect(defectsPage.statusPill(`Doomed early ${stamp}`)).toHaveText('Closed', {
                timeout: TIMEOUTS.ELEMENT,
            });
            expect(deletes).toBe(0);

            const stored = (await api.listDefects()).find(d => d.id === target.id);
            expect(stored).toBeTruthy();
            expect(stored.status).toBe('closed');
        });

        await page.unroute('**/api/defects/bulk-update');
        await page.unroute('**/api/defects/*');
    });

    // The other half of what useDialogFocus promises. Containment stops Tab leaving an open
    // dialog; this is what happens when the dialog goes — focus has to land back on the control
    // that opened it, or a keyboard user is dropped on <body> and has to Tab the whole register
    // again to get back to the row they were working on.
    //
    // It read the restore target in a passive effect, which is too late: BOTH dialogs autoFocus
    // a field, and React does that during commit — before any effect, layout or passive, runs.
    // So the "opener" it recorded was the dialog's own input, and closing the dialog focused a
    // node that had just been unmounted, i.e. nothing. Captured during render instead, which is
    // the only point at which focus is still on the button that was pressed.
    test('closing a dialog hands focus back to the control that opened it', async ({ api, defectsPage }) => {
        const stamp = `Restore-${Date.now()}`;

        await test.step('Seed one defect and expand it', async () => {
            await api.createDefect({ title: `Restored ${stamp}`, severity: 'major' });
            await defectsPage.open();
            await defectsPage.search(stamp);
            await expect(defectsPage.rows).toHaveCount(1, { timeout: TIMEOUTS.ELEMENT });
            await defectsPage.expandRow(`Restored ${stamp}`);
            await expect(defectsPage.openDetailButton).toBeEnabled();
        });

        await test.step('The editor takes focus on open and gives it back to Open detail', async () => {
            await defectsPage.openDetailButton.click();
            await expect(defectsPage.modalSave).toBeVisible();
            // The premise: autoFocus has already moved focus INTO the dialog, so whatever the
            // restore target is, it was not read after this point.
            await expect(defectsPage.modalTitle).toBeFocused();

            await defectsPage.modalCancel.click();
            await expect(defectsPage.modalDialog).toHaveCount(0);
            expect(await defectsPage.focusedTestId()).toBe('defects-open-detail');
        });

        // components/Modal is the shared confirm/prompt, so this half lands app-wide.
        await test.step('The delete confirmation gives it back to Delete', async () => {
            await defectsPage.deleteButton.click();
            await expect(defectsPage.deleteDialog).toContainText(`Restored ${stamp}`);

            await defectsPage.deleteCancel.click();
            await expect(defectsPage.deleteDialog).toHaveCount(0);
            expect(await defectsPage.focusedTestId()).toBe('defects-delete');
        });

        // And from a control that is nowhere near the row — the create dialog is opened from
        // the title bar, so a restore that merely happened to land in the expand panel would
        // pass the two steps above and still be wrong here.
        await test.step('The create dialog gives it back to + New defect', async () => {
            await defectsPage.newDefectButton.click();
            await expect(defectsPage.modalSave).toBeVisible();
            await expect(defectsPage.modalTitle).toBeFocused();

            await defectsPage.modalCancel.click();
            await expect(defectsPage.modalDialog).toHaveCount(0);
            expect(await defectsPage.focusedTestId()).toBe('defects-new');
        });
    });

    // The bulk endpoint tolerates unknown ids rather than rejecting the whole call, so a
    // defect somebody else deleted since this page loaded simply comes back missing from the
    // response. The page used to patch only what came back, which left that row on screen,
    // unchanged, after an apply that reported success — the user is told they closed
    // something that does not exist any more.
    test('a bulk apply drops rows the server no longer knows about', async ({ api, defectsPage }) => {
        const stamp = `Vanished-${Date.now()}`;
        let survivor, doomed;

        await test.step('Seed two defects and tick them both', async () => {
            survivor = await api.createDefect({ title: `Still here ${stamp}`, severity: 'major' });
            doomed = await api.createDefect({ title: `Deleted elsewhere ${stamp}`, severity: 'minor' });

            await defectsPage.open();
            await defectsPage.search(stamp);
            await expect(defectsPage.rows).toHaveCount(2, { timeout: TIMEOUTS.ELEMENT });
            await defectsPage.selectAll.check();
            await expect(defectsPage.bulkBar).toContainText('2 selected');
        });

        // Deleted out of band — another user, another tab — after this page loaded its list.
        await test.step('One of them is deleted from under the page', async () => {
            const gone = await api.delete(`/defects/${doomed.id}`);
            expect(gone.ok()).toBe(true);
        });

        await test.step('The apply closes the survivor and takes the ghost off the queue', async () => {
            await defectsPage.bulkClose.click();
            await expect(defectsPage.statusPill(`Still here ${stamp}`)).toHaveText('Closed', {
                timeout: TIMEOUTS.ELEMENT,
            });
            await expect(defectsPage.row(`Deleted elsewhere ${stamp}`)).toHaveCount(0);
            await expect(defectsPage.rows).toHaveCount(1);
        });

        await test.step('The server agrees on both counts', async () => {
            const all = await api.listDefects();
            expect(all.find(d => d.id === survivor.id).status).toBe('closed');
            expect(all.find(d => d.id === doomed.id)).toBeUndefined();
        });
    });

    test('bulk assign gives a selection an owner, and unassign sends it back to triage', async ({ page, api, defectsPage }) => {
        const stamp = `Assign-${Date.now()}`;
        let owner;

        await test.step('Seed two unowned defects', async () => {
            const { users } = await (await api.get('/users/assignable')).json();
            owner = users[0];
            expect(owner?.id).toBeTruthy();
            await api.createDefect({ title: `Orphan one ${stamp}`, severity: 'major' });
            await api.createDefect({ title: `Orphan two ${stamp}`, severity: 'minor' });
        });

        await defectsPage.open();
        await defectsPage.search(stamp);
        await expect(defectsPage.rows).toHaveCount(2, { timeout: TIMEOUTS.ELEMENT });
        await defectsPage.selectAll.check();

        // A failed load used to be terminal: the catch pinned the list to [], and an
        // empty list is indistinguishable from a loaded one to the "have they been
        // fetched yet" guard — so one flaky response left the menu reading "No
        // assignable users." for the life of the page, with bulk assign dead until a
        // reload. It stays unloaded and retryable instead, like a row's affected tests.
        await test.step('A failed user load offers a Retry, not a permanent "no users"', async () => {
            await page.route('**/api/users/assignable', route => route.fulfill({
                status: 500,
                contentType: 'application/json',
                body: '{"error":"users are unavailable"}',
            }));
            await defectsPage.bulkAssign.click();
            await expect(defectsPage.bulkUsersRetry).toBeVisible({ timeout: TIMEOUTS.ELEMENT });

            await page.unroute('**/api/users/assignable');
            await defectsPage.bulkUsersRetry.click();
            await expect(defectsPage.bulkAssignee(owner.display_name || owner.email)).toBeVisible({
                timeout: TIMEOUTS.ELEMENT,
            });
            await page.keyboard.press('Escape');
        });

        await test.step('The user list loads when the menu is first opened', async () => {
            await defectsPage.bulkAssign.click();
            await expect(defectsPage.bulkAssignee(owner.display_name || owner.email)).toBeVisible({
                timeout: TIMEOUTS.ELEMENT,
            });
        });

        // Assignment IS the triage action here, so both pills must move.
        await test.step('Assigning moves both rows out of Needs triage', async () => {
            await defectsPage.bulkAssignee(owner.display_name || owner.email).click();
            await expect(defectsPage.statusPill(`Orphan one ${stamp}`)).toHaveText('In progress');
            await expect(defectsPage.statusPill(`Orphan two ${stamp}`)).toHaveText('In progress');
        });

        await test.step('Set severity applies across the whole selection too', async () => {
            await defectsPage.selectAll.check();
            await defectsPage.bulkSeverity.click();
            await defectsPage.page.locator('.defects-menu').getByRole('button', { name: 'Trivial' }).click();
            await expect(defectsPage.row(`Orphan one ${stamp}`).locator('.defects-sev-pill')).toHaveText('trivial');
            await expect(defectsPage.row(`Orphan two ${stamp}`).locator('.defects-sev-pill')).toHaveText('trivial');
        });

        await test.step('Unassigning sends them straight back', async () => {
            await defectsPage.selectAll.check();
            await defectsPage.bulkAssign.click();
            await defectsPage.bulkUnassign.click();
            await expect(defectsPage.statusPill(`Orphan one ${stamp}`)).toHaveText('Needs triage');
            await expect(defectsPage.statusPill(`Orphan two ${stamp}`)).toHaveText('Needs triage');
        });

        await test.step('The server agrees — the owner really was cleared', async () => {
            const all = await api.listDefects();
            const mine = all.filter(d => d.title.includes(stamp));
            expect(mine).toHaveLength(2);
            expect(mine.every(d => !d.assignee_id)).toBe(true);
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

    // Geometry, deliberately — not visibility. toBeVisible() ignores where in the
    // viewport an element sits and .click() auto-scrolls to it, so an element parked
    // under the floating bar or below the fold passes every ordinary assertion. Both
    // regressions this pins did exactly that: the bar covered the last row of the
    // queue with nowhere left to scroll, and the bulk menus opened downward off a bar
    // pinned to the bottom of the page.
    test('the floating bulk bar never covers the queue, and its menus open on screen', async ({ page, api, defectsPage }) => {
        const stamp = `Geometry-${Date.now()}`;
        const SEEDS = 14;

        // A short viewport plus more rows than fit: the last row can only land under
        // the bar when the queue has something to scroll in the first place.
        await page.setViewportSize({ width: 1280, height: 620 });

        await test.step(`Seed ${SEEDS} defects, enough to overflow the queue`, async () => {
            for (let i = 0; i < SEEDS; i++) {
                await api.createDefect({ title: `Row ${String(i).padStart(2, '0')} ${stamp}`, severity: 'major' });
            }
        });

        await defectsPage.open();
        await defectsPage.search(stamp);
        await expect(defectsPage.rows).toHaveCount(SEEDS, { timeout: TIMEOUTS.ELEMENT });
        await defectsPage.selectAll.check();
        await expect(defectsPage.bulkBar).toContainText(`${SEEDS} selected`);

        await test.step('Scrolled all the way down, the last row is still clear of the bar', async () => {
            const geo = await page.evaluate(() => {
                const wrap = document.querySelector('.defects-table-wrap');
                wrap.scrollTop = wrap.scrollHeight;
                const rows = [...document.querySelectorAll('[data-testid="defects-row"]')];
                const last = rows[rows.length - 1];
                const bar = document.querySelector('.defects-bulkbar').getBoundingClientRect();
                const box = last.querySelector('[data-testid="defects-row-select"]').getBoundingClientRect();
                const onTop = document.elementFromPoint(box.left + box.width / 2, box.top + box.height / 2);
                return {
                    scrollable: Math.round(wrap.scrollHeight - wrap.clientHeight),
                    clearance: Math.round(bar.top - last.getBoundingClientRect().bottom),
                    checkboxReachable: onTop ? onTop.getAttribute('data-testid') === 'defects-row-select' : false,
                };
            });
            // Without this the rest is vacuous: with nothing to scroll, the last row
            // sits nowhere near the bar and would clear it by accident.
            expect(geo.scrollable).toBeGreaterThan(0);
            expect(geo.clearance).toBeGreaterThanOrEqual(0);
            expect(geo.checkboxReachable).toBe(true);
        });

        await test.step('The Assign menu opens upward, entirely inside the viewport', async () => {
            await defectsPage.bulkAssign.click();
            await expect(defectsPage.bulkUnassign).toBeVisible({ timeout: TIMEOUTS.ELEMENT });

            const viewport = page.viewportSize();
            const menu = await page.locator('.defects-menu').boundingBox();
            expect(menu.y).toBeGreaterThanOrEqual(0);
            expect(menu.y + menu.height).toBeLessThanOrEqual(viewport.height);

            // Unassign and its warning are the LAST things in the menu, so a menu
            // that opens downward loses exactly the two the design calls for.
            const unassign = await defectsPage.bulkUnassign.boundingBox();
            expect(unassign.y + unassign.height).toBeLessThanOrEqual(viewport.height);
            const note = await page.locator('.defects-menu-note').last().boundingBox();
            expect(note.y + note.height).toBeLessThanOrEqual(viewport.height);
        });
    });

    // The companion to the test above, at the width where the bar STOPS being one row.
    // The wide case is the easy one: a 44px bar clears the queue under any spacer big
    // enough to notice. The bar wraps, though — narrow the viewport and put its inline
    // failure message on it and it becomes three rows and ~130px, taller than any
    // constant that was ever measured against the unwrapped bar. That is why the
    // scroller's clearance is driven by the bar's MEASURED height (BulkBar's
    // ResizeObserver → --defects-bulkbar-h): pinned at 84px, this exact case leaves the
    // last row 32px BEHIND the bar with no scroll range left to reach it.
    test('the bulk bar still clears the queue once it wraps at a narrow width', async ({ page, api, defectsPage }) => {
        const stamp = `Wrapped-${Date.now()}`;
        const SEEDS = 10;

        // 680px is inside the page's declared support range (the stylesheet carries a
        // max-width:640px block), and it is where the controls stop fitting on one line.
        await page.setViewportSize({ width: 680, height: 900 });

        await test.step(`Seed ${SEEDS} defects and tick them all`, async () => {
            for (let i = 0; i < SEEDS; i++) {
                await api.createDefect({ title: `Row ${String(i).padStart(2, '0')} ${stamp}`, severity: 'major' });
            }
            await defectsPage.open();
            await defectsPage.search(stamp);
            await expect(defectsPage.rows).toHaveCount(SEEDS, { timeout: TIMEOUTS.ELEMENT });
            await defectsPage.selectAll.check();
        });

        await test.step('A failed apply puts the inline error on the bar, which wraps it', async () => {
            await page.route('**/api/defects/bulk-update', route => route.fulfill({
                status: 500,
                contentType: 'application/json',
                body: JSON.stringify({ error: 'bulk update is temporarily unavailable' }),
            }));
            await defectsPage.bulkClose.click();
            await expect(defectsPage.bulkError).toBeVisible({ timeout: TIMEOUTS.ELEMENT });

            // The clearance is measured, so wait for the measurement rather than for a
            // frame count: the spacer must end up TALLER than the bar it is clearing.
            await expect.poll(async () => page.evaluate(() => {
                const bar = document.querySelector('.defects-bulkbar').getBoundingClientRect().height;
                const spacer = document.querySelector('.defects-bulk-spacer').getBoundingClientRect().height;
                return Math.round(spacer - bar);
            }), { timeout: TIMEOUTS.ELEMENT }).toBeGreaterThan(0);
        });

        await test.step('Scrolled all the way down, the last row is still clear of the wrapped bar', async () => {
            const geo = await page.evaluate(() => {
                const wrap = document.querySelector('.defects-table-wrap');
                wrap.scrollTop = wrap.scrollHeight;
                const rows = [...document.querySelectorAll('[data-testid="defects-row"]')];
                const last = rows[rows.length - 1];
                const bar = document.querySelector('.defects-bulkbar').getBoundingClientRect();
                const box = last.querySelector('[data-testid="defects-row-select"]').getBoundingClientRect();
                const onTop = document.elementFromPoint(box.left + box.width / 2, box.top + box.height / 2);
                return {
                    barHeight: Math.round(bar.height),
                    scrollable: Math.round(wrap.scrollHeight - wrap.clientHeight),
                    clearance: Math.round(bar.top - last.getBoundingClientRect().bottom),
                    checkboxReachable: onTop ? onTop.getAttribute('data-testid') === 'defects-row-select' : false,
                };
            });
            // Without this the case is not the one being pinned: a bar that fits on one
            // line (44px at 1280px) is already covered by the test above.
            expect(geo.barHeight).toBeGreaterThan(60);
            expect(geo.scrollable).toBeGreaterThan(0);
            expect(geo.clearance).toBeGreaterThanOrEqual(0);
            expect(geo.checkboxReachable).toBe(true);
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

        // Measured, not eyeballed: the panel's padding rule is a bare class, and
        // `.defects-table td { padding: 0 10px }` is the more specific selector — it won
        // outright, and the panel shipped with the headings flush against the border of
        // the row above and the last line of the description on its bottom edge. Nothing
        // in a build, a lint or the rest of this suite noticed.
        await test.step('The panel is padded off the rows it sits between', async () => {
            const pad = await defectsPage.expandPanel.locator('.defects-expand-cell').evaluate((cell) => {
                const cs = getComputedStyle(cell);
                return { top: parseFloat(cs.paddingTop), bottom: parseFloat(cs.paddingBottom) };
            });
            expect(pad.top).toBeGreaterThan(0);
            expect(pad.bottom).toBeGreaterThan(0);
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

    // The expand's affected-tests load is left retryable on failure (never pinned to an
    // empty list), so Retry has to look like it did something — otherwise the only
    // feedback a user gets from pressing it is a second failure that looks identical to
    // the first, or, if it succeeds, a panel that changes for no visible reason.
    test('Retry on the affected-tests panel visibly restarts the load', async ({ page, api, defectsPage }) => {
        const stamp = `Affected-${Date.now()}`;
        let seed;

        await test.step('Seed a failed result with a defect on it, and break the tests call', async () => {
            seed = await api.seedRunWithResult({ label: `Affected ${stamp}` });
            await api.createAndLinkResultDefect(seed.run.id, seed.result.id, {
                title: `Flaky load ${stamp}`,
                severity: 'major',
            });
            await page.route('**/api/defects/*/tests', route => route.fulfill({
                status: 500,
                contentType: 'application/json',
                body: '{"error":"tests are unavailable"}',
            }));
        });

        await defectsPage.open();
        await defectsPage.search(stamp);
        await expect(defectsPage.rows).toHaveCount(1, { timeout: TIMEOUTS.ELEMENT });

        await test.step('The failure offers a Retry rather than an empty panel', async () => {
            await defectsPage.expandRow(`Flaky load ${stamp}`);
            await expect(defectsPage.affectedRetry).toBeVisible({ timeout: TIMEOUTS.ELEMENT });
            await expect(defectsPage.expandPanel).not.toContainText('No linked tests.');
        });

        // Held open, so "while the retry is in flight" is a state the test can assert in
        // rather than a frame it has to catch.
        let release;
        const held = new Promise(resolve => { release = resolve; });
        await page.unroute('**/api/defects/*/tests');
        await page.route('**/api/defects/*/tests', async (route) => {
            await held;
            await route.continue();
        });

        await test.step('Pressing it puts the panel back to Loading…, then to the test', async () => {
            await defectsPage.affectedRetry.click();
            await expect(defectsPage.expandPanel).toContainText('Loading…');
            await expect(defectsPage.affectedRetry).toHaveCount(0);

            release();
            await expect(defectsPage.affectedTest(seed.tc.name)).toBeVisible({ timeout: TIMEOUTS.ELEMENT });
            await expect(defectsPage.retestButton).toBeEnabled();
        });

        await page.unroute('**/api/defects/*/tests');
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
