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

async function createTestAPI(request, name, folderId, description = 'API Test', extra = {}) {
    const res = await request.post(`${API_URL}/tests`, {
        data: { name, folder_id: folderId, description, ...extra },
    });
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

// Links an existing defect directly to a test case. Returns the DefectLink.
async function linkTestCaseDefectAPI(request, testId, defectId) {
    const res = await request.post(`${API_URL}/tests/${testId}/defect-links`, {
        data: { defect_id: defectId },
    });
    await ensureOk(res);
    return res.json();
}

// Patches a defect (e.g. { status: 'closed' }). Returns the updated Defect.
async function updateDefectAPI(request, id, data) {
    const res = await request.patch(`${API_URL}/defects/${id}`, { data });
    await ensureOk(res);
    return res.json();
}

// Links an existing requirement to a test case. Returns the link.
async function linkRequirementToTestCaseAPI(request, reqId, testCaseId) {
    const res = await request.post(`${API_URL}/requirements/${reqId}/links`, {
        data: { test_case_id: testCaseId },
    });
    await ensureOk(res);
    return res.json();
}

// ── AI failure analysis ──────────────────────────────────────────────────────

// Runs synchronous AI failure analysis on ONE run result and returns the created
// RunResultAnalysis (incl. the computed `suggested_defect_type`).
//
// ⚠️ This path is LLM rate-limited server-side (isLLMPath matches the /analyze
// suffix, server.go), so keep specs to a small number of calls — a burst gets
// 429s, not analyses.
async function analyzeRunResultAPI(request, resultId) {
    const res = await request.post(`${API_URL}/run-results/${resultId}/analyze`);
    await ensureOk(res);
    return res.json();
}

// ── Seed helpers ──────────────────────────────────────────────────────────────

// Folder + test + run + one result, optionally forced to a status. Unifies the
// seedFailedResult / inline "folder→test→run→addResult→FAIL" pattern repeated
// across the run specs. Returns every created entity so callers assert on what
// they need.
async function seedRunWithResult(request, { status = 'FAIL', label = 'Seed' } = {}) {
    const ts = Date.now();
    const folder = await createFolderAPI(request, `${label} Folder ${ts}`);
    const tc = await createTestAPI(request, `${label} Test ${ts}`, folder.id);
    const run = await createRunAPI(request, `${label} Run ${ts}`);
    const result = await addRunResultAPI(request, run.id, tc.id);
    if (status) await updateRunResultAPI(request, run.id, result.id, { status });
    return { folder, tc, run, result };
}

// The multi-result sibling of seedRunWithResult: one folder, one run, and one
// test case + result per requested status, in order. seedRunWithResult creates a
// run PER result, so it cannot express the thing selection specs need — several
// rows with DIFFERENT statuses on the SAME run (a mixed bulk selection).
//
// Returns { folder, run, rows } with rows[i] = { tc, result, status } matching the
// `statuses` order, so callers can destructure positionally:
//   const [failRow, errorRow, passRow] = seed.rows;
async function seedRunWithResults(request, { statuses = ['FAIL'], label = 'Seed' } = {}) {
    const ts = Date.now();
    const folder = await createFolderAPI(request, `${label} Folder ${ts}`);
    const run = await createRunAPI(request, `${label} Run ${ts}`);
    const rows = [];
    for (const [i, status] of statuses.entries()) {
        // The index keeps names unique when the same status appears twice.
        const tc = await createTestAPI(request, `${label} ${status} ${i + 1} ${ts}`, folder.id);
        const result = await addRunResultAPI(request, run.id, tc.id);
        if (status) await updateRunResultAPI(request, run.id, result.id, { status });
        rows.push({ tc, result, status });
    }
    return { folder, run, rows };
}

// Category with one linked test case, each in its own new folder. `tag` keeps
// the generated names unique.
async function setupCategoryWithTest(request, tag) {
    const folder = await createFolderAPI(request, `Folder ${tag}`);
    const tc = await createTestAPI(request, `Test ${tag}`, folder.id);
    const category = await createCategoryAPI(request, `Category ${tag}`);
    await linkTestToCategoryAPI(request, tc.id, category.id);
    return { folder, tc, category };
}

function findInFolderTree(nodes, name) {
    for (const n of nodes || []) {
        if (n.name === name) return n.id;
        if (n.sub_folders) {
            const found = findInFolderTree(n.sub_folders, name);
            if (found) return found;
        }
    }
    return null;
}

// ── Client ────────────────────────────────────────────────────────────────────

// Sends API requests bound to a single Playwright `request` context, so specs
// pass params/structs instead of building URLs and bodies by hand. Injected into
// tests via the `api` fixture (`new ApiClient(request)`).
//
// Two layers:
//   - low-level get/post/put/patch/delete: prefix API_URL, wrap { data }, and
//     return the raw APIResponse (no status check) — for asserting status codes
//     and negative cases.
//   - semantic methods: the common operations. Seeding methods return parsed
//     JSON (throwing on non-2xx); status-sensitive ones (AI generate/accept)
//     return the raw response so the caller can assert the code.
class ApiClient {
    constructor(request) {
        this.request = request;
    }

    // Low-level. `path` is relative to API_URL, e.g. '/runs'.
    get(path) {
        return this.request.get(`${API_URL}${path}`);
    }

    post(path, body) {
        return this.request.post(`${API_URL}${path}`, body === undefined ? undefined : { data: body });
    }

    put(path, body) {
        return this.request.put(`${API_URL}${path}`, body === undefined ? undefined : { data: body });
    }

    patch(path, body) {
        return this.request.patch(`${API_URL}${path}`, body === undefined ? undefined : { data: body });
    }

    delete(path) {
        return this.request.delete(`${API_URL}${path}`);
    }

    // requirements / integrations
    createRequirement(...a) { return createRequirementAPI(this.request, ...a); }
    configureJira(...a) { return configureJiraAPI(this.request, ...a); }
    configureConfluence(...a) { return configureConfluenceAPI(this.request, ...a); }
    deleteAllRequirements() { return deleteAllRequirements(this.request); }
    linkRequirementToTestCase(...a) { return linkRequirementToTestCaseAPI(this.request, ...a); }

    // folders / tests / categories
    createFolder(...a) { return createFolderAPI(this.request, ...a); }
    createTest(...a) { return createTestAPI(this.request, ...a); }
    createCategory(...a) { return createCategoryAPI(this.request, ...a); }
    linkTestToCategory(...a) { return linkTestToCategoryAPI(this.request, ...a); }
    async getFolderIdByName(name) {
        return findInFolderTree(await (await this.get('/folders/tree')).json(), name);
    }

    // runs / results / run folders
    createRun(...a) { return createRunAPI(this.request, ...a); }
    getRun(...a) { return getRunAPI(this.request, ...a); }
    addRunResult(...a) { return addRunResultAPI(this.request, ...a); }
    updateRunResult(...a) { return updateRunResultAPI(this.request, ...a); }
    retryRunResult(...a) { return retryRunResultAPI(this.request, ...a); }
    getResultId(...a) { return getResultId(this.request, ...a); }
    createRunFolder(...a) { return createRunFolderAPI(this.request, ...a); }
    deleteRunFolder(...a) { return deleteRunFolderAPI(this.request, ...a); }
    async completeRun(runId) {
        const res = await this.post(`/runs/${runId}/complete`);
        await ensureOk(res);
        return res;
    }

    // defects
    createDefect(...a) { return createDefectAPI(this.request, ...a); }
    createAndLinkResultDefect(...a) { return createAndLinkResultDefectAPI(this.request, ...a); }
    linkResultDefect(...a) { return linkResultDefectAPI(this.request, ...a); }
    unlinkResultDefect(...a) { return unlinkResultDefectAPI(this.request, ...a); }
    listResultDefects(...a) { return listResultDefectsAPI(this.request, ...a); }
    linkTestCaseDefect(...a) { return linkTestCaseDefectAPI(this.request, ...a); }
    updateDefect(...a) { return updateDefectAPI(this.request, ...a); }
    async listDefects() {
        const res = await this.get('/defects');
        await ensureOk(res);
        return res.json();
    }
    async listRunDefects(runId) {
        const res = await this.get(`/runs/${runId}/defect-links`);
        await ensureOk(res);
        return res.json();
    }

    // AI generation — status-sensitive (201 create vs 200 idempotent replay), so
    // these return the raw response for the caller to assert on.
    createAiGeneration({ requirementId, providerId = null, idempotencyKey }) {
        return this.post('/ai-generations', {
            requirement_id: requirementId,
            provider_id: providerId,
            idempotency_key: idempotencyKey,
        });
    }
    acceptAiGeneration(runId, { folderId, draftIds, groupByCategory = true }) {
        return this.post(`/ai-generations/${runId}/accept`, {
            folder_id: folderId,
            draft_ids: draftIds,
            group_by_category: groupByCategory,
        });
    }
    async getTraceability() {
        const res = await this.get('/traceability');
        await ensureOk(res);
        return res.json();
    }

    // AI failure analysis — LLM rate-limited, see analyzeRunResultAPI.
    analyzeRunResult(...a) { return analyzeRunResultAPI(this.request, ...a); }

    // LLM providers (used by the fake-LLM helper)
    async createLlmProvider(fields) {
        const res = await this.post('/settings/llm-providers', fields);
        await ensureOk(res);
        return res.json();
    }
    deleteLlmProvider(id) {
        return this.delete(`/settings/llm-providers/${id}`);
    }

    // seed helpers
    seedRunWithResult(opts) { return seedRunWithResult(this.request, opts); }
    seedRunWithResults(opts) { return seedRunWithResults(this.request, opts); }
    setupCategoryWithTest(tag) { return setupCategoryWithTest(this.request, tag); }
}

export {
    API_URL,
    MOCK_URL,
    ApiClient,
    seedRunWithResult,
    seedRunWithResults,
    setupCategoryWithTest,
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
    linkTestCaseDefectAPI,
    updateDefectAPI,
    linkRequirementToTestCaseAPI,
    analyzeRunResultAPI,
};
