// Minimal TTGO REST client used by the e2e reporter. No Playwright dependency,
// so it can be unit-tested with an injected fetch.
//
// Every method throws on a non-2xx response. The reporter is responsible for
// catching, so that a reporting failure never fails the Playwright run.
export function createTtgoClient({ baseUrl, token, fetch = globalThis.fetch }) {
    const root = baseUrl.replace(/\/+$/, '');
    const headers = {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
    };

    async function request(method, path, body) {
        const res = await fetch(`${root}/api${path}`, {
            method,
            headers,
            body: body === undefined ? undefined : JSON.stringify(body),
        });
        if (!res.ok) {
            let detail = '';
            try {
                detail = await res.text();
            } catch {
                // ignore — the status code is enough to act on
            }
            throw new Error(`TTGO ${method} ${path} -> ${res.status} ${detail}`);
        }
        if (res.status === 204) return null;
        return res.json();
    }

    // Finds a root-level folder by exact name, creating it if absent. Returns its id.
    async function findOrCreateFolder(name) {
        const tree = await request('GET', '/folders/tree'); // array of root folders
        const found = (tree || []).find((f) => f.name === name);
        if (found) return found.id;
        const created = await request('POST', '/folders', { name, parent_id: null });
        return created.id;
    }

    // Finds a category by exact name, creating it if absent. Returns its id.
    async function findOrCreateCategory(name) {
        const res = await request(
            'GET',
            `/categories?limit=100&offset=0&q=${encodeURIComponent(name)}`
        );
        const found = (res?.categories || []).find((c) => c.name === name);
        if (found) return found.id;
        const created = await request('POST', '/categories', {
            name,
            description: 'Auto-managed by the Playwright e2e reporter',
        });
        return created.id;
    }

    // Ensures a test case exists for every name in `names` within `folderId`.
    // Returns a Map(name -> test_case_id). Existing cases are reused by name.
    async function ensureTestCases(folderId, names) {
        const existing = await request(
            'GET',
            `/tests?folder_ids=${encodeURIComponent(folderId)}`
        ); // array
        const map = new Map();
        for (const t of existing || []) map.set(t.name, t.id);
        for (const name of names) {
            if (!map.has(name)) {
                const created = await request('POST', '/tests', {
                    name,
                    folder_id: folderId,
                    description: '',
                });
                map.set(name, created.id);
            }
        }
        return map;
    }

    // NOTE: passing categoryId makes the backend SNAPSHOT every test case currently
    // assigned to that category into PENDING results. The reporter therefore creates
    // the run WITHOUT a category and labels it afterward via updateRun (see below).
    async function createRun({ name, categoryId = null, runFolderId = null }) {
        return request('POST', '/runs', {
            name,
            category_id: categoryId,
            run_folder_id: runFolderId,
        });
    }

    // Updates run fields (e.g. { category_id }) — a plain field update, no snapshot.
    async function updateRun(runId, fields) {
        return request('PUT', `/runs/${runId}`, fields);
    }

    async function addResult(runId, body) {
        return request('POST', `/runs/${runId}/results`, body);
    }

    async function completeRun(runId) {
        return request('POST', `/runs/${runId}/complete`);
    }

    return {
        request,
        findOrCreateFolder,
        findOrCreateCategory,
        ensureTestCases,
        createRun,
        updateRun,
        addResult,
        completeRun,
    };
}
