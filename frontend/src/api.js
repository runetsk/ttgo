import axios from 'axios';
import { toast } from './toast';

const api = axios.create({
    baseURL: '/api',
    withCredentials: true,
});

// Global response interceptor — shows a toast for every failed request
// unless the caller sets config._silent = true.
// A 401 on any route other than /auth/login and /auth/me triggers a redirect to /login.
api.interceptors.response.use(
    response => response,
    error => {
        const url = error.config?.url ?? '';
        const status = error?.response?.status;

        if (status === 401 && !url.includes('/auth/login') && !url.includes('/auth/me')) {
            window.dispatchEvent(new CustomEvent('auth:require-login'));
            return Promise.reject(error);
        }

        if (!error.config?._silent) {
            const message =
                error?.response?.data?.error ||
                error?.message ||
                'An unexpected error occurred';
            toast.error(message);
        }
        return Promise.reject(error);
    }
);

// In-flight dedup: if a second caller invokes this while the first request is
// still pending, both share the same promise. The cache clears on settle, so
// subsequent calls (after a mutation) always fetch fresh.
let _folderTreeInflight = null;
export const getFolderTree = () => {
    if (_folderTreeInflight) return _folderTreeInflight;
    _folderTreeInflight = api.get('/folders/tree')
        .then(res => res.data)
        .finally(() => { _folderTreeInflight = null; });
    return _folderTreeInflight;
};
// Pass { _silent: true } to suppress the global error toast (e.g. background sidebar sync).
export const getFolder = (id, options = {}) => api.get(`/folders/${id}`, options).then(res => res.data);
export const createFolder = (name, parentId) => api.post('/folders', { name, parent_id: parentId }).then(res => res.data);
export const deleteFolder = (id) => api.delete(`/folders/${id}`);
export const deleteFolders = (ids) => api.post('/folders/bulk-delete', { ids });
export const updateFolder = (id, name) => api.patch(`/folders/${id}`, { name }).then(res => res.data);
export const moveFolder = (id, parentId) => api.patch(`/folders/${id}/parent`, { parent_id: parentId });
export const bulkMoveFolders = (ids, parentId) => api.post('/folders/bulk-move', { ids, parent_id: parentId });

export const getCategories = (page = 1, pageSize = 10, search = '') => {
    const offset = (page - 1) * pageSize;
    const q = search ? `&q=${encodeURIComponent(search)}` : '';
    return api.get(`/categories?limit=${pageSize}&offset=${offset}${q}`).then(res => res.data);
};
export const createCategory = (name, description) => api.post('/categories', { name, description }).then(res => res.data);
export const deleteCategory = (id) => api.delete(`/categories/${id}`);
export const deleteCategories = (ids) => api.post('/categories/bulk-delete', { ids });

// opts.view: 'list' uses the slimmer server response (omits Steps/CustomValues,
// returns steps_count). Defaults to full-detail response for backwards compatibility.
export const getTests = (folderIds = [], categoryId, opts = {}) => {
    let params = new URLSearchParams();
    if (folderIds && folderIds.length > 0) {
        params.append('folder_ids', folderIds.join(','));
    }
    if (categoryId) {
        params.append('category_id', categoryId);
    }
    if (opts.view) {
        params.append('view', opts.view);
    }
    return api.get('/tests', { params }).then(res => res.data);
};

export const getTest = (id, config) => api.get(`/tests/${id}`, config).then(res => res.data);

export const createTest = (name, folderId, description) => api.post('/tests', { name, folder_id: folderId, description }).then(res => res.data);
export const updateTest = (test) => api.put(`/tests/${test.id}`, test).then(res => res.data);
export const moveTest = (testId, folderId) => api.put(`/tests/${testId}`, { folder_id: folderId }).then(res => res.data);
export const deleteTest = (id) => api.delete(`/tests/${id}`);
export const deleteTests = (ids) => api.post('/tests/bulk-delete', { ids });
export const exportTests = (ids, fields) =>
    api.post('/tests/export', { ids, fields }, { responseType: 'blob' }).then(res => {
        const disposition = res.headers['content-disposition'] || '';
        const match = disposition.match(/filename="?([^"]+)"?/);
        const filename = match ? match[1] : `ttgo-export-${new Date().toISOString().slice(0, 19).replace(/:/g, '')}.json`;
        const url = URL.createObjectURL(res.data);
        const a = document.createElement('a');
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        a.remove();
        URL.revokeObjectURL(url);
    });
export const assignCategory = (testId, categoryId) => api.post(`/tests/${testId}/categories`, { category_id: categoryId });

export const getTestRuns = (categoryId, status, sortBy, order, page = 1, pageSize = 20, folderID = null) => {
    let params = new URLSearchParams();
    if (categoryId) params.append('category_id', categoryId);
    if (status) params.append('status', status);
    if (sortBy) params.append('sort_by', sortBy);
    if (order) params.append('order', order);
    if (folderID !== null && folderID !== undefined) params.append('run_folder_id', folderID);

    const offset = (page - 1) * pageSize;
    params.append('limit', pageSize);
    params.append('offset', offset);

    return api.get('/runs', { params }).then(res => res.data);
};
export const getTestRun = (id) => api.get(`/runs/${id}`).then(res => res.data);
export const createTestRun = (categoryId, name, runFolderId = null) => api.post('/runs', { category_id: categoryId || null, name, run_folder_id: runFolderId }).then(res => res.data);
export const updateTestRun = (id, name, categoryId, status) => api.put(`/runs/${id}`, { name, ...(categoryId ? { category_id: categoryId } : {}), ...(status ? { status } : {}) }).then(res => res.data);
export const completeTestRun = (id) => api.post(`/runs/${id}/complete`).then(res => res.data);
export const reopenTestRun = (id) => api.post(`/runs/${id}/reopen`).then(res => res.data);
export const deleteTestRun = (id) => api.delete(`/runs/${id}`);
export const deleteTestRuns = (ids) => api.post('/runs/bulk-delete', { ids });

export const addRunResult = (runId, data) => {
    const payload = typeof data === 'string' ? { test_case_id: data } : data;
    return api.post(`/runs/${runId}/results`, payload).then(res => res.data);
};
export const deleteRunResult = (runId, resultId) => api.delete(`/runs/${runId}/results/${resultId}`);
export const updateRunResult = (runId, resultId, data) => {
    const payload = typeof data === 'string' ? { status: data } : data;
    return api.put(`/runs/${runId}/results/${resultId}`, payload);
};
export const retryRunResult = (runId, resultId) =>
    api.post(`/runs/${runId}/results/${resultId}/retry`).then(res => res.data);
export const bulkUpdateRunResults = (runId, resultIds, status, defectType) =>
    api.post(`/runs/${runId}/results/bulk-update`, { result_ids: resultIds, status, ...(defectType ? { defect_type: defectType } : {}) }).then(res => res.data);
export const uploadScreenshots = (runId, resultId, files) => {
    const fd = new FormData();
    files.forEach(f => fd.append('screenshots', f));
    return api.post(`/runs/${runId}/results/${resultId}/screenshots`, fd, {
        headers: { 'Content-Type': 'multipart/form-data' },
    }).then(res => res.data);
};

// ── Comments ──
export const listRunComments = (runId) =>
    api.get(`/runs/${runId}/comments`).then(res => res.data);
export const addRunComment = (runId, content) =>
    api.post(`/runs/${runId}/comments`, { content }).then(res => res.data);
export const listResultComments = (runId, resultId) =>
    api.get(`/runs/${runId}/results/${resultId}/comments`).then(res => res.data);
export const addResultComment = (runId, resultId, content) =>
    api.post(`/runs/${runId}/results/${resultId}/comments`, { content }).then(res => res.data);
export const updateComment = (commentId, content) =>
    api.put(`/comments/${commentId}`, { content }).then(res => res.data);
export const deleteComment = (commentId) =>
    api.delete(`/comments/${commentId}`);

export const getCustomFields = () => api.get('/custom-fields').then(res => res.data);
export const createCustomField = (name, type, options, isMandatory) => api.post('/custom-fields', { name, type, options: options ? JSON.stringify(options) : null, is_mandatory: isMandatory }).then(res => res.data);
export const deleteCustomField = (id) => api.delete(`/custom-fields/${id}`);


// API Tokens
export const listTokens = () => api.get('/tokens').then(res => res.data);
export const createToken = (description, scope) => api.post('/tokens', { description, scope }).then(res => res.data);
export const deleteToken = (id) => api.delete(`/tokens/${id}`);

// Webhooks
export const listWebhooks = () => api.get('/webhooks').then(res => res.data);
export const createWebhook = (url, description, eventType = 'run.completed') =>
    api.post('/webhooks', { url, description, event_type: eventType }).then(res => res.data);
export const deleteWebhook = (id) => api.delete(`/webhooks/${id}`);

// Analytics (012-analytics-refactor: enhanced with global filter support)
export const getAnalyticsSummary = (filters = {}, signal) =>
    api.get('/analytics/summary', { params: filters, signal }).then(res => res.data);
export const getAnalyticsTrend = (filters = {}, signal) =>
    api.get('/analytics/trend', { params: filters, signal }).then(res => res.data);
export const getAnalyticsFlaky = (filters = {}, signal) =>
    api.get('/analytics/flaky', { params: filters, signal }).then(res => res.data);
export const getAnalyticsMostFailed = (filters = {}, signal) =>
    api.get('/analytics/most-failed', { params: filters, signal }).then(res => res.data);
export const getAnalyticsDuration = (filters = {}, signal) =>
    api.get('/analytics/duration', { params: filters, signal }).then(res => res.data);
export const getAnalyticsDurationTop = (filters = {}, signal) =>
    api.get('/analytics/duration/top', { params: filters, signal }).then(res => res.data);
export const getAnalyticsComponentHealth = (filters = {}, signal) =>
    api.get('/analytics/component-health', { params: filters, signal }).then(res => res.data);
export const getAnalyticsGrowth = (filters = {}, signal) =>
    api.get('/analytics/growth', { params: filters, signal }).then(res => res.data);
export const getAnalyticsPassingRate = (filters = {}, signal) =>
    api.get('/analytics/passing-rate', { params: filters, signal }).then(res => res.data);
export const getAnalyticsUniqueBugs = (filters = {}, signal) =>
    api.get('/analytics/unique-bugs', { params: filters, signal }).then(res => res.data);
export const getAnalyticsActivity = (filters = {}, signal) =>
    api.get('/analytics/activity', { params: filters, signal }).then(res => res.data);
export const getAnalyticsCompareRuns = (run1, run2, signal) =>
    api.get('/analytics/compare-runs', { params: { run1, run2 }, signal }).then(res => res.data);

// Run Folders
export const getRunFolders = () => api.get('/run-folders').then(res => res.data);
export const getRunFolderTree = () => api.get('/run-folders?view=tree').then(res => res.data);
export const createRunFolder = (name, parentId = null) => api.post('/run-folders', { name, parent_id: parentId }).then(res => res.data);
export const updateRunFolder = (id, name) => api.patch(`/run-folders/${id}`, { name }).then(res => res.data);
export const reorderRunFolder = (id, displayOrder) => api.patch(`/run-folders/${id}/order`, { display_order: displayOrder }).then(res => res.data);
export const moveRunFolder = (id, parentId) => api.patch(`/run-folders/${id}/parent`, { parent_id: parentId }).then(res => res.data);
export const deleteRunFolder = (id) => api.delete(`/run-folders/${id}`);
export const assignRunToFolder = (runId, folderId) => api.patch(`/runs/${runId}/folder`, { run_folder_id: folderId }).then(res => res.data);
export const copyTestRun = (runId, name = '', runFolderId = null) => api.post(`/runs/${runId}/copy`, { name, run_folder_id: runFolderId }).then(res => res.data);
export const copyRunFolder = (folderId, name = '', parentId = null) => api.post(`/run-folders/${folderId}/copy`, { name, parent_id: parentId }).then(res => res.data);

// ── Authentication ──
export const auth = {
    login: (email, password) =>
        api.post('/auth/login', { email, password }).then(res => res.data),
    logout: () =>
        api.post('/auth/logout').then(res => res.data),
    me: () =>
        api.get('/auth/me', { _silent: true }).then(res => res.data),
    changePassword: (currentPw, newPw) =>
        api.post('/auth/change-password', {
            current_password: currentPw,
            new_password: newPw,
        }).then(res => res.data),
};

// ── Demo data seeding (admin only) ──
export const seed = {
    status: () => api.get('/seed').then(res => res.data),
    load: () => api.post('/seed').then(res => res.data),
    remove: () => api.delete('/seed').then(res => res.data),
    resetAll: () => api.delete('/admin/reset').then(res => res.data),
};

// ── User management (admin only) ──
export const users = {
    list: (includeDeleted = false) =>
        api.get('/users', { params: includeDeleted ? { include_deleted: 'true' } : {} }).then(res => res.data),
    create: (data) =>
        api.post('/users', data).then(res => res.data),
    update: (id, data) =>
        api.patch(`/users/${id}`, data).then(res => res.data),
    delete: (id) =>
        api.delete(`/users/${id}`).then(res => res.data),
    restore: (id) =>
        api.post(`/users/${id}/restore`).then(res => res.data),
};

// ── Version history (006-test-case-versioning) ──
export const versions = {
    list: (testCaseId) =>
        api.get(`/tests/${testCaseId}/versions`).then(res => res.data),
    get: (testCaseId, versionId) =>
        api.get(`/tests/${testCaseId}/versions/${versionId}`).then(res => res.data),
    restore: (testCaseId, versionId) =>
        api.post(`/tests/${testCaseId}/versions/${versionId}/restore`).then(res => res.data),
};

// ── Requirements & traceability (007-req-traceability) ──
export const requirements = {
    list: () =>
        api.get('/requirements').then(res => res.data),
    create: (data) =>
        api.post('/requirements', data).then(res => res.data),
    get: (id, config) =>
        api.get(`/requirements/${id}`, config).then(res => res.data),
    update: (id, data) =>
        api.put(`/requirements/${id}`, data).then(res => res.data),
    delete: (id) =>
        api.delete(`/requirements/${id}`),
    bulkDelete: (ids) =>
        api.post('/requirements/bulk-delete', { ids }),
    listChildren: (id) =>
        api.get(`/requirements/${id}/children`).then(res => res.data),
    createLink: (reqId, testCaseId) =>
        api.post(`/requirements/${reqId}/links`, { test_case_id: testCaseId }).then(res => res.data),
    deleteLink: (reqId, testCaseId) =>
        api.delete(`/requirements/${reqId}/links/${testCaseId}`),
    listByTestCase: (testCaseId) =>
        api.get(`/tests/${testCaseId}/requirements`).then(res => res.data),
    // 011-jira-confluence-import
    importSingle: (sourceType, sourceKey, includeChildren = false) =>
        api.post('/requirements/import', { source_type: sourceType, source_key: sourceKey, include_children: includeChildren }).then(res => res.data),
    bulkImport: (sourceType, sourceKeys, includeChildren = false) =>
        api.post('/requirements/bulk-import', { source_type: sourceType, source_keys: sourceKeys, include_children: includeChildren }).then(res => res.data),
    resync: (id) =>
        api.post(`/requirements/${id}/resync`).then(res => res.data),
    resyncResolve: (id, resolution, remoteTitle, remoteDescription) =>
        api.post(`/requirements/${id}/resync/resolve`, { resolution, remote_title: remoteTitle, remote_description: remoteDescription }).then(res => res.data),
    unlink: (id) =>
        api.post(`/requirements/${id}/unlink`).then(res => res.data),
    postToJira: (id) =>
        api.post(`/requirements/${id}/post-to-jira`).then(res => res.data),
};

export const traceability = {
    getMatrix: (config) =>
        api.get('/traceability', config).then(res => res.data),
};

// ── Jira integration (007-req-traceability) ──
export const jira = {
    getConfig: () =>
        api.get('/settings/jira', { _silent: true }).then(res => res.data),
    upsertConfig: (data) =>
        api.put('/settings/jira', data).then(res => res.data),
    fetchTicket: (ticketId) =>
        api.get(`/jira/ticket/${ticketId}`).then(res => res.data),
    // 011: JQL search for bulk import
    search: (jql, startAt = 0, maxResults = 25) =>
        api.post('/jira/search', { jql, start_at: startAt, max_results: maxResults }).then(res => res.data),
};

// ── Confluence integration (011-jira-confluence-import) ──
export const confluence = {
    getConfig: () =>
        api.get('/settings/confluence', { _silent: true }).then(res => res.data),
    upsertConfig: (data) =>
        api.put('/settings/confluence', data).then(res => res.data),
    listSpaces: (cursor, limit) => {
        const params = {};
        if (cursor) params.cursor = cursor;
        if (limit) params.limit = limit;
        return api.get('/confluence/spaces', { params }).then(res => res.data);
    },
    listPages: (spaceId, title, label, cursor, limit) => {
        const params = { space_id: spaceId };
        if (title) params.title = title;
        if (label) params.label = label;
        if (cursor) params.cursor = cursor;
        if (limit) params.limit = limit;
        return api.get('/confluence/pages', { params }).then(res => res.data);
    },
    getPage: (pageId) =>
        api.get(`/confluence/pages/${pageId}`).then(res => res.data),
    listChildPages: (pageId, cursor, limit) => {
        const params = {};
        if (cursor) params.cursor = cursor;
        if (limit) params.limit = limit;
        return api.get(`/confluence/pages/${pageId}/children`, { params }).then(res => res.data);
    },
};

// ── Test case execution history ──
export const listTestExecutions = (testCaseId) =>
    api.get(`/tests/${testCaseId}/executions`).then(res => res.data);

// ── Defect links (008-jira-integration) — test-case-level helpers kept for create-issue ──
export const defectLinks = {
    createIssue: (testCaseId, data) =>
        api.post(`/tests/${testCaseId}/defect-links/create-issue`, data).then(res => res.data),
};

// All defect links in a run (summary view)
export const listRunDefectLinks = (runId) =>
    api.get(`/runs/${runId}/defect-links`).then(res => res.data);

// Run-result-scoped defect links
export const resultDefectLinks = {
    list: (runId, resultId) =>
        api.get(`/runs/${runId}/results/${resultId}/defect-links`).then(res => res.data),
    link: (runId, resultId, jiraKey) =>
        api.post(`/runs/${runId}/results/${resultId}/defect-links`, { jira_issue_key: jiraKey }).then(res => res.data),
    unlink: (runId, resultId, jiraKey) =>
        api.delete(`/runs/${runId}/results/${resultId}/defect-links/${encodeURIComponent(jiraKey)}`),
};

// ── AI Test Case Generation (010-ai-test-generation) ──
export const aiGeneration = {
    // Provider config (admin)
    listProviders: () =>
        api.get('/settings/llm-providers').then(res => res.data),
    createProvider: (data) =>
        api.post('/settings/llm-providers', data).then(res => res.data),
    updateProvider: (id, data) =>
        api.put(`/settings/llm-providers/${id}`, data).then(res => res.data),
    deleteProvider: (id) =>
        api.delete(`/settings/llm-providers/${id}`),
    testConnection: (id) =>
        api.post(`/settings/llm-providers/${id}/test`).then(res => res.data),
    setDefault: (id) =>
        api.post(`/settings/llm-providers/${id}/set-default`).then(res => res.data),
    // Coverage config
    getCoverageConfig: () =>
        api.get('/settings/ai-gen-coverage').then(res => res.data),
    updateCoverageConfig: (data) =>
        api.put('/settings/ai-gen-coverage', data).then(res => res.data),
    // Global AI master switch
    getFeatureSettings: () =>
        api.get('/settings/ai-features').then(res => res.data),
    updateFeatureSettings: (enabled) =>
        api.put('/settings/ai-features', { enabled }).then(res => res.data),
    // Prompt template
    getTemplate: () =>
        api.get('/settings/ai-gen-template').then(res => res.data),
    updateTemplate: (content) =>
        api.put('/settings/ai-gen-template', { content }).then(res => res.data),
    resetTemplate: () =>
        api.post('/settings/ai-gen-template/reset').then(res => res.data),
    // Parent template (for requirements with children)
    updateParentTemplate: (content) =>
        api.put('/settings/ai-gen-parent-template', { content }).then(res => res.data),
    resetParentTemplate: () =>
        api.post('/settings/ai-gen-parent-template/reset').then(res => res.data),
    // Generation
    generateTests: (requirementId, data) =>
        api.post(`/requirements/${requirementId}/generate-tests`, data).then(res => res.data),
    acceptGeneratedTests: (requirementId, data) =>
        api.post(`/requirements/${requirementId}/accept-generated-tests`, data).then(res => res.data),
};

// ── QTest integration (013-qtest-sync) ──
// Shared qtest config cache: both a persistent result and an in-flight promise,
// so concurrent mount effects across FolderNode/TestGrid/QTestSyncPanel reuse
// a single request. Cache is cleared on upsertConfig so Settings edits take
// effect immediately.
let _qtestConfigCache = null;       // null means "not yet fetched" or invalidated
let _qtestConfigInflight = null;

export const qtest = {
    getConfig: () => {
        if (_qtestConfigCache !== null) return Promise.resolve(_qtestConfigCache);
        if (_qtestConfigInflight) return _qtestConfigInflight;
        _qtestConfigInflight = api.get('/settings/qtest', { _silent: true })
            .then(res => {
                _qtestConfigCache = res.data;
                return res.data;
            })
            .finally(() => { _qtestConfigInflight = null; });
        return _qtestConfigInflight;
    },
    upsertConfig: (data) =>
        api.put('/settings/qtest', data).then(res => {
            _qtestConfigCache = res.data;  // keep cache in sync with the write
            return res.data;
        }),
    testConnection: () =>
        api.post('/settings/qtest/test-connection').then(res => res.data),
    listProjects: () =>
        api.get('/qtest/projects').then(res => res.data),
    listModules: (projectId) =>
        api.get('/qtest/modules', { params: projectId ? { project_id: projectId } : {} }).then(res => res.data),
    listEnabledProjects: () =>
        api.get('/qtest/enabled-projects').then(res => res.data),
    addEnabledProject: (projectId, projectName) =>
        api.post('/qtest/enabled-projects', { project_id: projectId, project_name: projectName }).then(res => res.data),
    removeEnabledProject: (projectId) =>
        api.post('/qtest/enabled-projects/remove', { project_id: projectId }).then(res => res.data),
    setDefaultProject: (projectId) =>
        api.post('/qtest/enabled-projects/set-default', { project_id: projectId }).then(res => res.data),
    listTestCases: (projectId, moduleId, recursive = false) =>
        api.get('/qtest/test-cases', { params: { project_id: projectId, module_id: moduleId, recursive } }).then(res => res.data),
    importTestCases: (data) =>
        api.post('/qtest/import', data).then(res => res.data),
    upload: (testCaseIds, moduleId, onConflict = 'skip', projectId) =>
        api.post('/qtest/upload', { test_case_ids: testCaseIds, module_id: moduleId, on_conflict: onConflict, project_id: projectId }).then(res => res.data),
    uploadFolder: (folderId, projectId, parentModuleId = 0, onConflict = 'skip', recursive = false) =>
        api.post('/qtest/upload-folder', { folder_id: folderId, project_id: projectId, parent_module_id: parentModuleId, on_conflict: onConflict, recursive }).then(res => res.data),
    unlinkFolder: (folderId, recursive = true) =>
        api.post('/qtest/unlink-folder', { folder_id: folderId, recursive }).then(res => res.data),
    bulkUnlinkMappings: (testCaseIds) =>
        api.post('/qtest/bulk-unlink', { test_case_ids: testCaseIds }).then(res => res.data),
    sync: (testCaseIds = []) =>
        api.post('/qtest/sync', { test_case_ids: testCaseIds }).then(res => res.data),
    getMapping: (testCaseId) =>
        api.get(`/tests/${testCaseId}/qtest-mapping`, { _silent: true }).then(res => res.data),
    batchGetMappings: (testCaseIds) =>
        api.post('/qtest/batch-mappings', { test_case_ids: testCaseIds }, { _silent: true }).then(res => res.data),
    unlinkMapping: (testCaseId) =>
        api.delete(`/tests/${testCaseId}/qtest-mapping`),
};

// ── AI Test Import (014-ai-test-import) ──
export const aiImport = {
    parse: (data) =>
        api.post('/import/parse', data).then(res => res.data),
    accept: (data) =>
        api.post('/import/accept', data).then(res => res.data),
};

// ── AI Failure Analysis ──
export const analyzeRunResult = (runResultId) =>
    api.post(`/run-results/${runResultId}/analyze`).then(r => r.data);

export const listRunResultAnalyses = (runResultId) =>
    api.get(`/run-results/${runResultId}/analyses`).then(r => r.data);

export const getCurrentRunAnalyses = (runId) =>
    api.get(`/runs/${runId}/analyses/current`).then(r => r.data);

export const analyzeRunFailures = (runId) =>
    api.post(`/runs/${runId}/analyze-failures`).then(r => r.data);

export const getRunAnalysisJob = (runId) =>
    api.get(`/runs/${runId}/analysis-job`, { _silent: true }).then(r => r.data);

export const cancelRunAnalysisJob = (runId) =>
    api.post(`/runs/${runId}/analysis-job/cancel`).then(r => r.data);

export const getFailureAnalysisSettings = () =>
    api.get('/settings/ai-failure-analysis').then(r => r.data);

export const updateFailureAnalysisSettings = (body) =>
    api.put('/settings/ai-failure-analysis', body).then(r => r.data);

export const resetFailureAnalysisPrompt = () =>
    api.post('/settings/ai-failure-analysis/prompt/reset').then(r => r.data);

// ── Database Backups (015-database-backups) ──
export const backups = {
    list: () => api.get('/backups').then(res => res.data),
    create: () => api.post('/backups').then(res => res.data),
    get: (id) => api.get(`/backups/${id}`).then(res => res.data),
    delete: (id) => api.delete(`/backups/${id}`).then(res => res.data),
    download: (id) => api.get(`/backups/${id}/download`, { responseType: 'blob' }),
    restore: (id, confirmation) =>
        api.post(`/backups/${id}/restore`, { confirmation }).then(res => res.data),
    uploadRestore: (file, confirmation) => {
        const fd = new FormData();
        fd.append('file', file);
        fd.append('confirmation', confirmation);
        return api.post('/backups/upload-restore', fd, {
            headers: { 'Content-Type': 'multipart/form-data' },
        }).then(res => res.data);
    },
    schedule: {
        get: () => api.get('/backup-schedule').then(res => res.data),
        update: (data) => api.put('/backup-schedule', data).then(res => res.data),
    },
    maintenanceStatus: () => api.get('/maintenance-status', { _silent: true }).then(res => res.data),
};

export default api;
