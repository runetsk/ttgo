import { test, expect } from '../../fixtures/test.js';

// Native defect flows — all linking is local, so no Jira/Confluence config or
// external mock is required.
test.describe('Native Defect Linking — run result panel', () => {

    test('create-and-link: new defect via API attaches to failed result and appears in list', async ({ api }) => {
        const { run, result } = await api.seedRunWithResult({ label: 'CreateAndLink' });

        // Mirrors the "New Defect" modal on a failed result.
        const { defect, link } = await api.createAndLinkResultDefect(run.id, result.id, {
            title: 'Null pointer on submit',
            severity: 'critical',
            status: 'open',
        });

        await test.step('Verify defect was created with correct fields', async () => {
            expect(defect.id).toBeTruthy();
            expect(defect.title).toBe('Null pointer on submit');
            expect(defect.severity).toBe('critical');
            expect(defect.status).toBe('open');
        });

        await test.step('Verify the link was created', async () => {
            expect(link.defect_id).toBe(defect.id);
            expect(link.run_result_id).toBe(result.id);
        });

        await test.step('Verify the defect appears in GET /runs/{id}/results/{result_id}/defect-links', async () => {
            const defects = await api.listResultDefects(run.id, result.id);
            const found = defects.find(d => d.id === defect.id);
            expect(found).toBeTruthy();
            expect(found.title).toBe('Null pointer on submit');
        });

        await test.step('Verify the defect appears in the global GET /defects list', async () => {
            const all = await api.listDefects();
            expect(all.find(d => d.id === defect.id)).toBeTruthy();
        });
    });

    test('link-existing: an already-created defect can be linked to a result via defect_id', async ({ api }) => {
        const { run, result } = await api.seedRunWithResult({ label: 'LinkExisting' });

        // Create the defect globally first (the "Search existing" dropdown selection).
        const defect = await api.createDefect({
            title: 'Existing global defect',
            severity: 'major',
        });

        const link = await api.linkResultDefect(run.id, result.id, defect.id);

        await test.step('Verify the link row points to the correct defect and result', async () => {
            expect(link.defect_id).toBe(defect.id);
            expect(link.run_result_id).toBe(result.id);
        });

        await test.step('Verify the defect now lists under result defects', async () => {
            const defects = await api.listResultDefects(run.id, result.id);
            expect(defects.some(d => d.id === defect.id)).toBeTruthy();
        });

        await test.step('Linking the same defect again returns 409 Conflict', async () => {
            const res = await api.post(`/runs/${run.id}/results/${result.id}/defect-links`, { defect_id: defect.id });
            expect(res.status()).toBe(409);
        });
    });

    test('unlink: a linked defect can be removed from a result', async ({ api }) => {
        const { run, result } = await api.seedRunWithResult({ label: 'Unlink' });

        const { defect } = await api.createAndLinkResultDefect(run.id, result.id, {
            title: 'Defect to unlink',
            severity: 'minor',
        });

        await test.step('Verify defect is present before unlinking', async () => {
            const defects = await api.listResultDefects(run.id, result.id);
            expect(defects.some(d => d.id === defect.id)).toBeTruthy();
        });

        await test.step('Unlink the defect', async () => {
            await api.unlinkResultDefect(run.id, result.id, defect.id);
        });

        await test.step('Verify defect is gone from result defects after unlink', async () => {
            const defects = await api.listResultDefects(run.id, result.id);
            expect(defects.some(d => d.id === defect.id)).toBeFalsy();
        });

        await test.step('Defect itself still exists globally after unlink', async () => {
            const all = await api.listDefects();
            expect(all.some(d => d.id === defect.id)).toBeTruthy();
        });
    });

    test('run-level defect listing: GET /runs/{id}/defect-links aggregates across results', async ({ api }) => {
        const ts = Date.now();
        const folder = await api.createFolder(`Run Defect Agg Folder ${ts}`);
        const tc1 = await api.createTest(`Agg Test A ${ts}`, folder.id);
        const tc2 = await api.createTest(`Agg Test B ${ts}`, folder.id);
        const run = await api.createRun(`Agg Run ${ts}`);
        const r1 = await api.addRunResult(run.id, tc1.id);
        const r2 = await api.addRunResult(run.id, tc2.id);
        await api.updateRunResult(run.id, r1.id, { status: 'FAIL' });
        await api.updateRunResult(run.id, r2.id, { status: 'FAIL' });

        const { defect: d1 } = await api.createAndLinkResultDefect(run.id, r1.id, {
            title: 'Agg defect alpha',
            severity: 'major',
        });
        const { defect: d2 } = await api.createAndLinkResultDefect(run.id, r2.id, {
            title: 'Agg defect beta',
            severity: 'minor',
        });

        await test.step('Both defects appear in the run-level defect aggregation', async () => {
            const rows = await api.listRunDefects(run.id);
            const ids = rows.map(row => row.id);
            expect(ids).toContain(d1.id);
            expect(ids).toContain(d2.id);
        });
    });

    test('create defect validation: title is required, invalid severity rejected', async ({ api }) => {
        const { run, result } = await api.seedRunWithResult({ label: 'Validation' });

        await test.step('Empty title returns 400', async () => {
            const res = await api.post(`/runs/${run.id}/results/${result.id}/defects`, { title: '', severity: 'minor' });
            expect(res.status()).toBe(400);
        });

        await test.step('Invalid severity returns 400', async () => {
            const res = await api.post(`/runs/${run.id}/results/${result.id}/defects`, { title: 'Bad severity defect', severity: 'super-high' });
            expect(res.status()).toBe(400);
        });

        await test.step('Missing defect_id in link request returns 400', async () => {
            const res = await api.post(`/runs/${run.id}/results/${result.id}/defect-links`, {});
            expect(res.status()).toBe(400);
        });

        await test.step('Linking non-existent defect_id returns 404', async () => {
            const res = await api.post(`/runs/${run.id}/results/${result.id}/defect-links`, { defect_id: 'does-not-exist-00000000' });
            expect(res.status()).toBe(404);
        });
    });

    test('global defect CRUD: create, update status/severity, delete', async ({ api }) => {
        let defect;

        await test.step('Create a global defect', async () => {
            defect = await api.createDefect({
                title: 'CRUD defect',
                severity: 'trivial',
                status: 'open',
            });
            expect(defect.id).toBeTruthy();
            expect(defect.severity).toBe('trivial');
            expect(defect.status).toBe('open');
        });

        await test.step('Update it to closed + critical', async () => {
            const res = await api.patch(`/defects/${defect.id}`, { status: 'closed', severity: 'critical' });
            expect(res.ok()).toBeTruthy();
            const updated = await res.json();
            expect(updated.status).toBe('closed');
            expect(updated.severity).toBe('critical');
        });

        await test.step('Delete the defect', async () => {
            const res = await api.delete(`/defects/${defect.id}`);
            expect(res.status()).toBe(204);
        });

        await test.step('Deleted defect no longer appears in global list', async () => {
            const all = await api.listDefects();
            expect(all.some(d => d.id === defect.id)).toBeFalsy();
        });
    });
});
