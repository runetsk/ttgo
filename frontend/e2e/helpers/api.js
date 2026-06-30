import { API_URL } from '../config.js';

const MOCK_URL = 'http://localhost:9999';

// Throws on a non-2xx API response. Used instead of Playwright's `expect` so this
// shared module doesn't import @playwright/test (importing the test framework from
// a spec-shared helper trips Playwright's "two versions of @playwright/test" guard).
async function ensureOk(res) {
    if (!res.ok()) {
        const body = await res.text().catch(() => '');
        throw new Error(`API request failed: ${res.status()} ${res.statusText()} ${body}`.trim());
    }
}

// ── Requirements / integrations ──────────────────────────────────────────────
async function createRequirementAPI(request, identifier, title, description = '') {
    const res = await request.post(`${API_URL}/requirements`, {
        data: { identifier, title, description },
    });
    return await res.json();
}

async function configureJiraAPI(request, overrides = {}) {
    await request.put(`${API_URL}/settings/jira`, {
        data: {
            base_url: MOCK_URL,
            email: 'test@example.com',
            api_token: 'mock-token',
            enabled: true,
            default_project_key: 'PROJ',
            default_issue_type: 'Bug',
            ...overrides,
        },
    });
}

async function configureConfluenceAPI(request, overrides = {}) {
    await request.put(`${API_URL}/settings/confluence`, {
        data: {
            base_url: MOCK_URL,
            email: 'test@example.com',
            api_token: 'mock-token',
            enabled: true,
            ...overrides,
        },
    });
}

async function deleteAllRequirements(request) {
    const res = await request.get(`${API_URL}/requirements`);
    const reqs = await res.json();
    for (const r of (reqs || [])) {
        await request.delete(`${API_URL}/requirements/${r.id}`);
    }
}

// ── Folders / test cases / categories ────────────────────────────────────────
async function createFolderAPI(request, name, parentId = null) {
    const res = await request.post(`${API_URL}/folders`, { data: { name, parent_id: parentId } });
    await ensureOk(res);
    return res.json();
}

async function createTestAPI(request, name, folderId, description = 'API Test') {
    const res = await request.post(`${API_URL}/tests`, { data: { name, folder_id: folderId, description } });
    await ensureOk(res);
    return res.json();
}

async function createCategoryAPI(request, name, description = 'Created via API') {
    const res = await request.post(`${API_URL}/categories`, { data: { name, description } });
    await ensureOk(res);
    return res.json();
}

async function linkTestToCategoryAPI(request, testId, categoryId) {
    const res = await request.post(`${API_URL}/tests/${testId}/categories`, { data: { category_id: categoryId } });
    await ensureOk(res);
}

// Resolves a folder id by name from the folder tree (recurses into sub_folders).
async function getFolderIdByName(page, name) {
    const resp = await page.request.get(`${API_URL}/folders/tree`);
    const tree = await resp.json();
    const find = (nodes) => {
        for (const n of nodes || []) {
            if (n.name === name) return n.id;
            if (n.sub_folders) {
                const found = find(n.sub_folders);
                if (found) return found;
            }
        }
        return null;
    };
    return find(tree);
}

// ── Runs / results / run folders ─────────────────────────────────────────────
async function createRunAPI(request, name, { categoryId = null, runFolderId = null } = {}) {
    const res = await request.post(`${API_URL}/runs`, {
        data: { name, category_id: categoryId, run_folder_id: runFolderId },
    });
    await ensureOk(res);
    return res.json();
}

async function getRunAPI(request, runId) {
    const res = await request.get(`${API_URL}/runs/${runId}`);
    await ensureOk(res);
    return res.json();
}

// Adds a result to a run. `extra` may carry status/defect_type/etc.
async function addRunResultAPI(request, runId, testCaseId, extra = {}) {
    const res = await request.post(`${API_URL}/runs/${runId}/results`, {
        data: { test_case_id: testCaseId, ...extra },
    });
    await ensureOk(res);
    return res.json();
}

async function updateRunResultAPI(request, runId, resultId, data) {
    const res = await request.put(`${API_URL}/runs/${runId}/results/${resultId}`, { data });
    await ensureOk(res);
    return res.json().catch(() => ({}));
}

async function retryRunResultAPI(request, runId, resultId) {
    const res = await request.post(`${API_URL}/runs/${runId}/results/${resultId}/retry`);
    await ensureOk(res);
    return res.json();
}

// Returns the RunResult primary key (id) for a given test_case_id within a run.
async function getResultId(request, runId, testCaseId) {
    const run = await getRunAPI(request, runId);
    return run.run_results.find((r) => r.test_case_id === testCaseId)?.id;
}

async function createRunFolderAPI(request, name, parentId = null) {
    const res = await request.post(`${API_URL}/run-folders`, { data: { name, parent_id: parentId } });
    await ensureOk(res);
    return res.json();
}

async function deleteRunFolderAPI(request, id) {
    const res = await request.delete(`${API_URL}/run-folders/${id}`);
    await ensureOk(res);
}

// ── Defects ──────────────────────────────────────────────────────────────────

// Creates a global (standalone) defect. Returns the Defect object.
async function createDefectAPI(request, fields = {}) {
    const res = await request.post(`${API_URL}/defects`, { data: fields });
    await ensureOk(res);
    return res.json();
}

// Creates a defect and immediately links it to a run result in one call.
// Returns { defect, link }.
async function createAndLinkResultDefectAPI(request, runId, resultId, fields = {}) {
    const res = await request.post(`${API_URL}/runs/${runId}/results/${resultId}/defects`, { data: fields });
    await ensureOk(res);
    return res.json();
}

// Links an existing defect (by defect_id) to a run result. Returns the DefectLink.
async function linkResultDefectAPI(request, runId, resultId, defectId) {
    const res = await request.post(`${API_URL}/runs/${runId}/results/${resultId}/defect-links`, {
        data: { defect_id: defectId },
    });
    await ensureOk(res);
    return res.json();
}

// Unlinks a defect from a run result. Returns nothing (204).
async function unlinkResultDefectAPI(request, runId, resultId, defectId) {
    const res = await request.delete(
        `${API_URL}/runs/${runId}/results/${resultId}/defect-links/${defectId}`
    );
    await ensureOk(res);
}

// Lists defects linked to a specific run result. Returns Defect[].
async function listResultDefectsAPI(request, runId, resultId) {
    const res = await request.get(`${API_URL}/runs/${runId}/results/${resultId}/defect-links`);
    await ensureOk(res);
    return res.json();
}

export {
    API_URL,
    MOCK_URL,
    createRequirementAPI,
    configureJiraAPI,
    configureConfluenceAPI,
    deleteAllRequirements,
    createFolderAPI,
    createTestAPI,
    createCategoryAPI,
    linkTestToCategoryAPI,
    getFolderIdByName,
    createRunAPI,
    getRunAPI,
    addRunResultAPI,
    updateRunResultAPI,
    retryRunResultAPI,
    getResultId,
    createRunFolderAPI,
    deleteRunFolderAPI,
    createDefectAPI,
    createAndLinkResultDefectAPI,
    linkResultDefectAPI,
    unlinkResultDefectAPI,
    listResultDefectsAPI,
};
