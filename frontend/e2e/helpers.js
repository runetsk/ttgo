import { API_URL } from './config.js';

const MOCK_URL = 'http://localhost:9999';

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

export {
    API_URL,
    MOCK_URL,
    createRequirementAPI,
    configureJiraAPI,
    configureConfluenceAPI,
    deleteAllRequirements,
};
