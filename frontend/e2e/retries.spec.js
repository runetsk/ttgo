import { test, expect } from '@playwright/test';

const API_URL = 'http://localhost:8080/api';

test.describe('Test Run Retries', () => {

    // ── API helpers ───────────────────────────────────────────────────────────

    const createFolderAPI = async (request, name) => {
        const res = await request.post(`${API_URL}/folders`, { data: { name, parent_id: null } });
        expect(res.ok()).toBeTruthy();
        return res.json();
    };

    const createTestAPI = async (request, name, folderId) => {
        const res = await request.post(`${API_URL}/tests`, {
            data: { name, folder_id: folderId, description: '' }
        });
        expect(res.ok()).toBeTruthy();
        return res.json();
    };

    const createRunAPI = async (request, name) => {
        const res = await request.post(`${API_URL}/runs`, { data: { name } });
        expect(res.ok()).toBeTruthy();
        return res.json();
    };

    const addResultAPI = async (request, runId, testCaseId) => {
        const res = await request.post(`${API_URL}/runs/${runId}/results`, {
            data: { test_case_id: testCaseId }
        });
        expect(res.ok()).toBeTruthy();
        return res.json();
    };

    const updateResultAPI = async (request, runId, resultId, data) => {
        const res = await request.put(`${API_URL}/runs/${runId}/results/${resultId}`, { data });
        expect(res.ok()).toBeTruthy();
        return res.json();
    };

    const retryResultAPI = async (request, runId, resultId) => {
        const res = await request.post(`${API_URL}/runs/${runId}/results/${resultId}/retry`);
        expect(res.ok()).toBeTruthy();
        return res.json();
    };

    const getRunAPI = async (request, runId) => {
        const res = await request.get(`${API_URL}/runs/${runId}`);
        expect(res.ok()).toBeTruthy();
        return res.json();
    };

    // ── Tests ─────────────────────────────────────────────────────────────────

    test('retry button creates a new PENDING attempt and shows badge', async ({ page, request }) => {
        const ts = Date.now();
        const folder = await createFolderAPI(request, `Retry Folder ${ts}`);
        const tc = await createTestAPI(request, `Retry Test ${ts}`, folder.id);
        const run = await createRunAPI(request, `Retry Run ${ts}`);
        const r1 = await addResultAPI(request, run.id, tc.id);

        // Mark first attempt as FAIL
        await updateResultAPI(request, run.id, r1.id, { status: 'FAIL' });

        await page.goto(`/runs/run/${run.id}`);
        await page.waitForLoadState('domcontentloaded');

        const row = page.getByRole('row', { name: `Retry Test ${ts}` });
        await expect(row).toBeVisible();

        // Status shows FAIL
        await expect(row.locator('select').first()).toHaveValue('FAIL');

        // Click the retry (↻) button
        page.on('dialog', d => d.dismiss()); // guard against accidental confirms
        await row.getByTitle('Retry this test').click();

        // After retry, the row should now show PENDING (new attempt)
        await expect(row.locator('select').first()).toHaveValue('PENDING');

        // Attempt 2 badge should appear
        await expect(row.getByTitle(/Attempt 2/)).toBeVisible();
    });

    test('retry creates new attempt and aggregation shows only latest', async ({ page, request }) => {
        const ts = Date.now();
        const folder = await createFolderAPI(request, `Agg Folder ${ts}`);
        const tc = await createTestAPI(request, `Agg Test ${ts}`, folder.id);
        const run = await createRunAPI(request, `Agg Run ${ts}`);
        const r1 = await addResultAPI(request, run.id, tc.id);

        // Attempt 1: FAIL
        await updateResultAPI(request, run.id, r1.id, { status: 'FAIL' });

        // Retry via API → attempt 2 PASS
        const r2 = await retryResultAPI(request, run.id, r1.id);
        await updateResultAPI(request, run.id, r2.id, { status: 'PASS' });

        await page.goto(`/runs/run/${run.id}`);
        await page.waitForLoadState('domcontentloaded');

        // Stats bar should reflect latest attempt only: 1 passed, 0 failed
        await expect(page.getByTestId('stats-passed')).toContainText('1');
        await expect(page.getByTestId('stats-failed')).toContainText('0');
    });

    test('attempt badge click opens detail with timeline', async ({ page, request }) => {
        const ts = Date.now();
        const folder = await createFolderAPI(request, `Expand Folder ${ts}`);
        const tc = await createTestAPI(request, `Expand Test ${ts}`, folder.id);
        const run = await createRunAPI(request, `Expand Run ${ts}`);
        const r1 = await addResultAPI(request, run.id, tc.id);

        await updateResultAPI(request, run.id, r1.id, { status: 'FAIL' });
        const r2 = await retryResultAPI(request, run.id, r1.id);
        await updateResultAPI(request, run.id, r2.id, { status: 'PASS' });

        await page.goto(`/runs/run/${run.id}`);
        await page.waitForLoadState('domcontentloaded');

        const row = page.getByRole('row', { name: `Expand Test ${ts}` });
        const badge = row.getByTitle(/Attempt 2/);
        await expect(badge).toBeVisible();

        // Detail panel not visible before clicking badge
        await expect(page.getByTestId('run-result-detail')).not.toBeVisible();

        // Click badge to expand detail panel with timeline
        await badge.click();

        // Detail panel should appear with timeline dots for both attempts
        const detail = page.getByTestId('run-result-detail');
        await expect(detail).toBeVisible();
        await expect(detail.getByTitle(/Attempt 1/)).toBeVisible();
        await expect(detail.getByTitle(/Attempt 2/)).toBeVisible();

        // Click badge again to collapse
        await badge.click();
        await expect(page.getByTestId('run-result-detail')).not.toBeVisible();
    });

    test('retried_count indicator appears in summary bar', async ({ page, request }) => {
        const ts = Date.now();
        const folder = await createFolderAPI(request, `Summary Folder ${ts}`);
        const tc = await createTestAPI(request, `Summary Test ${ts}`, folder.id);
        const run = await createRunAPI(request, `Summary Run ${ts}`);
        const r1 = await addResultAPI(request, run.id, tc.id);

        // Before retry — no retried indicator
        await page.goto(`/runs/run/${run.id}`);
        await page.waitForLoadState('domcontentloaded');
        await expect(page.getByText(/retried/)).not.toBeVisible();

        // Retry via API
        await retryResultAPI(request, run.id, r1.id);

        await page.reload();
        await page.waitForLoadState('domcontentloaded');

        // Summary bar should now show "↻ 1 retried"
        await expect(page.getByText(/1 retried/)).toBeVisible();
    });

    test('run detail shows attempt number in expanded RunResultDetail', async ({ page, request }) => {
        const ts = Date.now();
        const folder = await createFolderAPI(request, `Detail Folder ${ts}`);
        const tc = await createTestAPI(request, `Detail Test ${ts}`, folder.id);
        const run = await createRunAPI(request, `Detail Run ${ts}`);
        const r1 = await addResultAPI(request, run.id, tc.id);

        const r2 = await retryResultAPI(request, run.id, r1.id);
        await updateResultAPI(request, run.id, r2.id, {
            status: 'FAIL',
            error_message: 'Still failing on attempt 2'
        });

        await page.goto(`/runs/run/${run.id}`);
        await page.waitForLoadState('domcontentloaded');

        // Expand the result row to show RunResultDetail
        const row = page.getByRole('row', { name: `Detail Test ${ts}` });
        await row.click();

        // RunResultDetail should show "Attempt 2" header
        await expect(page.getByTestId('run-result-detail').getByText(/Attempt 2/)).toBeVisible();
        await expect(page.getByText('Still failing on attempt 2')).toBeVisible();
    });

    test('retry API returns 404 for non-existent result', async ({ request }) => {
        const run = await createRunAPI(request, `404 Retry Run ${Date.now()}`);
        const res = await request.post(`${API_URL}/runs/${run.id}/results/non-existent-id/retry`);
        expect(res.status()).toBe(404);
    });

    test('retry API returns 400 for orphaned result', async ({ request }) => {
        const run = await createRunAPI(request, `Orphan Retry Run ${Date.now()}`);

        // Add a result without a test_case_id (orphaned)
        const orphanRes = await request.post(`${API_URL}/runs/${run.id}/results`, {
            data: { test_case_id: null, test_name_snapshot: 'Orphan Test' }
        });
        // Orphaned results may be rejected at add time; if created, retry should 400
        if (orphanRes.ok()) {
            const orphan = await orphanRes.json();
            const retryRes = await request.post(`${API_URL}/runs/${run.id}/results/${orphan.id}/retry`);
            expect(retryRes.status()).toBe(400);
        }
    });
});
