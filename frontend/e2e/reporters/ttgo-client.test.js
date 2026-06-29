import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createTtgoClient } from './ttgo-client.js';

// Builds a fake fetch that records calls and returns whatever `handler(call)` gives.
function makeFetch(handler) {
    const calls = [];
    const fetch = async (url, opts) => {
        const call = {
            url,
            method: opts.method,
            headers: opts.headers,
            body: opts.body ? JSON.parse(opts.body) : undefined,
        };
        calls.push(call);
        const res = handler(call) || {};
        return {
            ok: res.ok ?? true,
            status: res.status ?? 200,
            json: async () => res.json ?? {},
            text: async () => res.text ?? '',
        };
    };
    return { fetch, calls };
}

test('request sends Bearer auth + JSON and throws on non-2xx', async () => {
    const { fetch, calls } = makeFetch(() => ({ ok: false, status: 500, text: 'boom' }));
    const c = createTtgoClient({ baseUrl: 'http://x:8080', token: 'tok', fetch });
    await assert.rejects(() => c.request('GET', '/runs'), /500 boom/);
    assert.equal(calls[0].url, 'http://x:8080/api/runs');
    assert.equal(calls[0].headers.Authorization, 'Bearer tok');
});

test('findOrCreateFolder reuses an existing root folder', async () => {
    const { fetch, calls } = makeFetch((call) =>
        call.method === 'GET' ? { json: [{ id: 'f1', name: 'Playwright E2E' }] } : {}
    );
    const c = createTtgoClient({ baseUrl: 'http://x:8080', token: 't', fetch });
    assert.equal(await c.findOrCreateFolder('Playwright E2E'), 'f1');
    assert.equal(calls.filter((x) => x.method === 'POST').length, 0);
});

test('findOrCreateFolder creates the folder when missing', async () => {
    const { fetch, calls } = makeFetch((call) => {
        if (call.method === 'GET') return { json: [] };
        if (call.method === 'POST') return { json: { id: 'fNew', name: call.body.name } };
        return {};
    });
    const c = createTtgoClient({ baseUrl: 'http://x:8080', token: 't', fetch });
    assert.equal(await c.findOrCreateFolder('Playwright E2E'), 'fNew');
    assert.deepEqual(
        calls.find((x) => x.method === 'POST').body,
        { name: 'Playwright E2E', parent_id: null }
    );
});

test('findOrCreateCategory finds an exact name match in the categories envelope', async () => {
    const { fetch } = makeFetch((call) =>
        call.method === 'GET'
            ? { json: { categories: [{ id: 'c1', name: 'Playwright E2E' }], total: 1 } }
            : {}
    );
    const c = createTtgoClient({ baseUrl: 'http://x:8080', token: 't', fetch });
    assert.equal(await c.findOrCreateCategory('Playwright E2E'), 'c1');
});

test('ensureTestCases reuses existing by name and creates only the missing ones', async () => {
    const { fetch, calls } = makeFetch((call) => {
        if (call.method === 'GET') return { json: [{ id: 'tA', name: 'A' }] };
        if (call.method === 'POST') return { json: { id: 'tB', name: call.body.name } };
        return {};
    });
    const c = createTtgoClient({ baseUrl: 'http://x:8080', token: 't', fetch });
    const map = await c.ensureTestCases('f1', ['A', 'B']);
    assert.equal(map.get('A'), 'tA');
    assert.equal(map.get('B'), 'tB');
    const posts = calls.filter((x) => x.method === 'POST');
    assert.equal(posts.length, 1);
    assert.deepEqual(posts[0].body, { name: 'B', folder_id: 'f1', description: '' });
});

test('createRun / addResult / completeRun hit the right endpoints and bodies', async () => {
    const { fetch, calls } = makeFetch((call) => {
        if (call.url.endsWith('/api/runs')) return { json: { id: 'run9' } };
        if (call.url.endsWith('/results')) return { json: { id: 'res1' } };
        return { json: { id: 'run9', status: 'PASS' } };
    });
    const c = createTtgoClient({ baseUrl: 'http://x:8080', token: 't', fetch });

    const run = await c.createRun({ name: 'R', categoryId: 'c1' });
    assert.equal(run.id, 'run9');
    assert.deepEqual(calls[0].body, { name: 'R', category_id: 'c1', run_folder_id: null });

    const res = await c.addResult('run9', { test_case_id: 'tA', status: 'PASS' });
    assert.equal(res.id, 'res1');
    assert.equal(calls[1].url, 'http://x:8080/api/runs/run9/results');
    assert.deepEqual(calls[1].body, { test_case_id: 'tA', status: 'PASS' });

    await c.completeRun('run9');
    assert.equal(calls[2].url, 'http://x:8080/api/runs/run9/complete');
});

test('updateRun PUTs run fields (e.g. category_id) without a name or snapshot', async () => {
    const { fetch, calls } = makeFetch(() => ({ json: { status: 'updated' } }));
    const c = createTtgoClient({ baseUrl: 'http://x:8080', token: 't', fetch });
    await c.updateRun('run9', { category_id: 'c1' });
    assert.equal(calls[0].method, 'PUT');
    assert.equal(calls[0].url, 'http://x:8080/api/runs/run9');
    assert.deepEqual(calls[0].body, { category_id: 'c1' });
});
