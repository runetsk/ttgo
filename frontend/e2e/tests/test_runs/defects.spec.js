import { test, expect } from '@playwright/test';
import { API_URL } from '../../config.js';
import {
    createFolderAPI,
    createTestAPI,
    createRunAPI,
    addRunResultAPI,
    updateRunResultAPI,
    createDefectAPI,
    linkResultDefectAPI,
    unlinkResultDefectAPI,
    listResultDefectsAPI,
    createAndLinkResultDefectAPI,
} from '../../helpers/api.js';

// Native defect flows — no external mock required.
// All defect linking is local; no Jira/Confluence config needed.

test.describe('Native Defect Linking — run result panel', () => {

    async function seedFailedResult(request, label) {
        const ts = Date.now();
        const folder = await createFolderAPI(request, `Defect Folder ${label} ${ts}`);
        const tc = await createTestAPI(request, `Defect Test ${label} ${ts}`, folder.id);
        const run = await createRunAPI(request, `Defect Run ${label} ${ts}`);
        const result = await addRunResultAPI(request, run.id, tc.id);
        await updateRunResultAPI(request, run.id, result.id, { status: 'FAIL' });
        return { run, result, tc };
    }

    test('create-and-link: new defect via API attaches to failed result and appears in list', async ({ request }) => {
        const { run, result } = await seedFailedResult(request, 'CreateAndLink');

        // Create-and-link via the native endpoint (mirrors "New Defect" modal)
        const { defect, link } = await createAndLinkResultDefectAPI(request, run.id, result.id, {
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
            const defects = await listResultDefectsAPI(request, run.id, result.id);
            const found = defects.find(d => d.id === defect.id);
            expect(found).toBeTruthy();
            expect(found.title).toBe('Null pointer on submit');
        });

        await test.step('Verify the defect appears in the global GET /defects list', async () => {
            const res = await request.get(`${API_URL}/defects`);
            expect(res.ok()).toBeTruthy();
            const all = await res.json();
            const found = all.find(d => d.id === defect.id);
            expect(found).toBeTruthy();
        });
    });

    test('link-existing: an already-created defect can be linked to a result via defect_id', async ({ request }) => {
        const { run, result } = await seedFailedResult(request, 'LinkExisting');

        // First create the defect globally (represents the search-dropdown selection)
        const defect = await createDefectAPI(request, {
            title: 'Existing global defect',
            severity: 'major',
        });

        // Link it to this result (mirrors the "Search existing" dropdown → confirm)
        const link = await linkResultDefectAPI(request, run.id, result.id, defect.id);

        await test.step('Verify the link row points to the correct defect and result', async () => {
            expect(link.defect_id).toBe(defect.id);
            expect(link.run_result_id).toBe(result.id);
        });

        await test.step('Verify the defect now lists under result defects', async () => {
            const defects = await listResultDefectsAPI(request, run.id, result.id);
            expect(defects.some(d => d.id === defect.id)).toBeTruthy();
        });

        await test.step('Linking the same defect again returns 409 Conflict', async () => {
            const res = await request.post(
                `${API_URL}/runs/${run.id}/results/${result.id}/defect-links`,
                { data: { defect_id: defect.id } }
            );
            expect(res.status()).toBe(409);
        });
    });

    test('unlink: a linked defect can be removed from a result', async ({ request }) => {
        const { run, result } = await seedFailedResult(request, 'Unlink');

        const { defect } = await createAndLinkResultDefectAPI(request, run.id, result.id, {
            title: 'Defect to unlink',
            severity: 'minor',
        });

        await test.step('Verify defect is present before unlinking', async () => {
            const defects = await listResultDefectsAPI(request, run.id, result.id);
            expect(defects.some(d => d.id === defect.id)).toBeTruthy();
        });

        await test.step('Unlink the defect', async () => {
            await unlinkResultDefectAPI(request, run.id, result.id, defect.id);
        });

        await test.step('Verify defect is gone from result defects after unlink', async () => {
            const defects = await listResultDefectsAPI(request, run.id, result.id);
            expect(defects.some(d => d.id === defect.id)).toBeFalsy();
        });

        await test.step('Defect itself still exists globally after unlink', async () => {
            const res = await request.get(`${API_URL}/defects`);
            const all = await res.json();
            expect(all.some(d => d.id === defect.id)).toBeTruthy();
        });
    });

    test('run-level defect listing: GET /runs/{id}/defect-links aggregates across results', async ({ request }) => {
        const ts = Date.now();
        const folder = await createFolderAPI(request, `Run Defect Agg Folder ${ts}`);
        const tc1 = await createTestAPI(request, `Agg Test A ${ts}`, folder.id);
        const tc2 = await createTestAPI(request, `Agg Test B ${ts}`, folder.id);
        const run = await createRunAPI(request, `Agg Run ${ts}`);
        const r1 = await addRunResultAPI(request, run.id, tc1.id);
        const r2 = await addRunResultAPI(request, run.id, tc2.id);
        await updateRunResultAPI(request, run.id, r1.id, { status: 'FAIL' });
        await updateRunResultAPI(request, run.id, r2.id, { status: 'FAIL' });

        const { defect: d1 } = await createAndLinkResultDefectAPI(request, run.id, r1.id, {
            title: 'Agg defect alpha',
            severity: 'major',
        });
        const { defect: d2 } = await createAndLinkResultDefectAPI(request, run.id, r2.id, {
            title: 'Agg defect beta',
            severity: 'minor',
        });

        await test.step('Both defects appear in the run-level defect aggregation', async () => {
            const res = await request.get(`${API_URL}/runs/${run.id}/defect-links`);
            expect(res.ok()).toBeTruthy();
            const rows = await res.json();
            const ids = rows.map(row => row.id);
            expect(ids).toContain(d1.id);
            expect(ids).toContain(d2.id);
        });
    });

    test('create defect validation: title is required, invalid severity rejected', async ({ request }) => {
        const { run, result } = await seedFailedResult(request, 'Validation');

        await test.step('Empty title returns 400', async () => {
            const res = await request.post(
                `${API_URL}/runs/${run.id}/results/${result.id}/defects`,
                { data: { title: '', severity: 'minor' } }
            );
            expect(res.status()).toBe(400);
        });

        await test.step('Invalid severity returns 400', async () => {
            const res = await request.post(
                `${API_URL}/runs/${run.id}/results/${result.id}/defects`,
                { data: { title: 'Bad severity defect', severity: 'super-high' } }
            );
            expect(res.status()).toBe(400);
        });

        await test.step('Missing defect_id in link request returns 400', async () => {
            const res = await request.post(
                `${API_URL}/runs/${run.id}/results/${result.id}/defect-links`,
                { data: {} }
            );
            expect(res.status()).toBe(400);
        });

        await test.step('Linking non-existent defect_id returns 404', async () => {
            const res = await request.post(
                `${API_URL}/runs/${run.id}/results/${result.id}/defect-links`,
                { data: { defect_id: 'does-not-exist-00000000' } }
            );
            expect(res.status()).toBe(404);
        });
    });

    test('global defect CRUD: create, update status/severity, delete', async ({ request }) => {
        let defect;

        await test.step('Create a global defect', async () => {
            defect = await createDefectAPI(request, {
                title: 'CRUD defect',
                severity: 'trivial',
                status: 'open',
            });
            expect(defect.id).toBeTruthy();
            expect(defect.severity).toBe('trivial');
            expect(defect.status).toBe('open');
        });

        await test.step('Update it to closed + critical', async () => {
            const res = await request.patch(`${API_URL}/defects/${defect.id}`, {
                data: { status: 'closed', severity: 'critical' },
            });
            expect(res.ok()).toBeTruthy();
            const updated = await res.json();
            expect(updated.status).toBe('closed');
            expect(updated.severity).toBe('critical');
        });

        await test.step('Delete the defect', async () => {
            const res = await request.delete(`${API_URL}/defects/${defect.id}`);
            expect(res.status()).toBe(204);
        });

        await test.step('Deleted defect no longer appears in global list', async () => {
            const res = await request.get(`${API_URL}/defects`);
            const all = await res.json();
            expect(all.some(d => d.id === defect.id)).toBeFalsy();
        });
    });
});
