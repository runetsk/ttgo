import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

// Bulk "Set defect type" is a TRIAGE-only action: the request carries no status, so
// the backend applies the defect type to the selected FAIL/ERROR rows and leaves
// everything else — status included — untouched.
//
// The mixed selection is the whole point. Triaging a run means selecting rows and
// deciding, and a real selection contains passes. If the picker were implemented as a
// status update carrying a defect type, or if the WS patch broadcast an empty status
// key, the PASS row's status would be silently rewritten in the grid the user is
// looking at. That is the data-corruption case this spec exists to catch.
test.describe('Run detail — bulk defect-type triage', () => {
    test('applies the defect type to the failures only and never rewrites a status', async ({
        page, api, runDetailPage,
    }) => {
        // Seeding three rows plus the WS-driven grid refresh after the bulk write
        // exceeds the suite's 10s default.
        test.setTimeout(TIMEOUTS.AI_LIFECYCLE);

        let seed, failRow, errorRow, passRow;

        await test.step('Seed one run holding a FAIL, an ERROR and a PASS result', async () => {
            seed = await api.seedRunWithResults({
                statuses: ['FAIL', 'ERROR', 'PASS'],
                label: 'Bulk Triage',
            });
            [failRow, errorRow, passRow] = seed.rows;
        });

        await test.step('Open the run with the optional defect_type column pinned visible', async () => {
            // Every defect-type control is gated on isVisible('defect_type'), so leaving
            // this to stored prefs is flaky.
            await runDetailPage.pinColumns({ defect_type: true });
            await runDetailPage.open(seed.run.id);
            await expect(runDetailPage.toolbar).toBeVisible({ timeout: TIMEOUTS.HEAVY_GRID });
        });

        await test.step('Select all three rows, mixing failures and a pass', async () => {
            for (const row of seed.rows) {
                await runDetailPage.selectResult(row.tc.id).click();
            }
            await expect(runDetailPage.bulkActionBar).toBeVisible({ timeout: TIMEOUTS.UI_SETTLE });
        });

        await test.step('Apply "Product Bug" from the bulk picker', async () => {
            const bulk = page.waitForResponse(r =>
                r.url().includes('/results/bulk-update') && r.request().method() === 'POST');
            await runDetailPage.bulkDefectTypeSelect.selectOption('product_bug');
            expect((await bulk).status()).toBe(200);
        });

        await test.step('The toast reports both what was updated and what was skipped', async () => {
            // Skipped rows are reported, not dropped silently: a partial result that looks
            // like a partial failure is the fastest way to lose trust in a bulk action.
            await expect(page.getByText('2 updated, 1 skipped (not failures)'))
                .toBeVisible({ timeout: TIMEOUTS.UI_SETTLE });
        });

        await test.step('The live grid triages both failures', async () => {
            // Asserted BEFORE the statuses on purpose: these two assertions can only pass
            // once the WS delta has been applied to the grid, which is what makes the
            // status assertions below a real test of the broadcast patch rather than a
            // vacuous check against a grid that never updated.
            await expect(runDetailPage.defectTypeSelect(failRow.tc.id))
                .toHaveValue('product_bug', { timeout: TIMEOUTS.ELEMENT });
            await expect(runDetailPage.defectTypeSelect(errorRow.tc.id))
                .toHaveValue('product_bug', { timeout: TIMEOUTS.ELEMENT });
            // A non-failure row has no defect-type control at all — the cell renders a dash —
            // so the PASS row could not have been given one.
            await expect(runDetailPage.defectTypeSelect(passRow.tc.id)).toHaveCount(0);
        });

        await test.step('Every status in the live grid survives the bulk write untouched', async () => {
            // A patch carrying an empty `status` key would blank all three of these; a patch
            // broadcast over the skipped ids would hand the PASS row someone else's decision.
            await expect(runDetailPage.statusSelect(failRow.tc.id)).toHaveValue('FAIL');
            await expect(runDetailPage.statusSelect(errorRow.tc.id)).toHaveValue('ERROR');
            await expect(runDetailPage.statusSelect(passRow.tc.id)).toHaveValue('PASS');
        });

        await test.step('The server agrees: failures triaged, PASS untouched, statuses unchanged', async () => {
            // The grid can only show what the WS patch carried; this reads the rows the
            // reports are actually computed from.
            const run = await api.getRun(seed.run.id);
            const stored = new Map((run.run_results || []).map(r => [r.id, r]));

            for (const [row, defectType] of [[failRow, 'product_bug'], [errorRow, 'product_bug'], [passRow, '']]) {
                const saved = stored.get(row.result.id);
                expect(saved, `the seeded ${row.status} result must still be on the run`).toBeTruthy();
                expect(saved.status, `${row.status} row status must be unchanged`).toBe(row.status);
                expect(saved.defect_type, `${row.status} row defect_type`).toBe(defectType);
            }
        });
    });
});
