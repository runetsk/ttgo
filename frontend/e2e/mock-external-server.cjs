const http = require('http');
const fs = require('fs');
const path = require('path');

const jiraFixtures = JSON.parse(fs.readFileSync(path.join(__dirname, 'fixtures/jira-tickets.json'), 'utf8'));
const confFixtures = JSON.parse(fs.readFileSync(path.join(__dirname, 'fixtures/confluence-pages.json'), 'utf8'));

const PORT = 9999;

const server = http.createServer((req, res) => {
    const url = new URL(req.url, `http://localhost:${PORT}`);
    const pathname = url.pathname;
    const method = req.method;

    res.setHeader('Content-Type', 'application/json');
    res.setHeader('Access-Control-Allow-Origin', '*');
    res.setHeader('Access-Control-Allow-Headers', '*');

    if (method === 'OPTIONS') {
        res.writeHead(204);
        res.end();
        return;
    }

    // ── Jira: GET /rest/api/3/issue/:key ──
    const issueMatch = pathname.match(/^\/rest\/api\/3\/issue\/(.+)$/);
    if (issueMatch && method === 'GET') {
        const key = decodeURIComponent(issueMatch[1]);
        const ticket = jiraFixtures[key];
        if (ticket) {
            res.writeHead(200);
            res.end(JSON.stringify(ticket));
        } else {
            res.writeHead(404);
            res.end(JSON.stringify({ errorMessages: ['Issue does not exist'] }));
        }
        return;
    }

    // ── Jira: POST /rest/api/3/search ──
    if (pathname === '/rest/api/3/search' && method === 'POST') {
        let body = '';
        req.on('data', chunk => { body += chunk; });
        req.on('end', () => {
            const all = Object.values(jiraFixtures);
            const parsed = JSON.parse(body || '{}');
            const startAt = parsed.startAt || 0;
            const maxResults = parsed.maxResults || 25;
            const slice = all.slice(startAt, startAt + maxResults);
            res.writeHead(200);
            res.end(JSON.stringify({
                startAt,
                maxResults,
                total: all.length,
                issues: slice,
            }));
        });
        return;
    }

    // ── Confluence: GET /wiki/api/v2/spaces ──
    if (pathname === '/wiki/api/v2/spaces' && method === 'GET') {
        const limit = parseInt(url.searchParams.get('limit') || '50');
        const spaces = confFixtures.spaces.slice(0, limit);
        res.writeHead(200);
        res.end(JSON.stringify({ results: spaces, _links: {} }));
        return;
    }

    // ── Confluence: GET /wiki/api/v2/spaces/:id/pages ──
    const spacePagesMatch = pathname.match(/^\/wiki\/api\/v2\/spaces\/(.+)\/pages$/);
    if (spacePagesMatch && method === 'GET') {
        const spaceId = spacePagesMatch[1];
        const pages = (confFixtures.pages[spaceId] || []).map(p => ({
            ...p,
            _links: p._links || {},
        }));
        res.writeHead(200);
        res.end(JSON.stringify({ results: pages, _links: {} }));
        return;
    }

    // ── Confluence: GET /wiki/api/v2/pages/:id ──
    const pageMatch = pathname.match(/^\/wiki\/api\/v2\/pages\/(.+)$/);
    if (pageMatch && method === 'GET') {
        const pageId = pageMatch[1].split('?')[0];
        const page = confFixtures.pageDetails[pageId];
        if (page) {
            res.writeHead(200);
            res.end(JSON.stringify(page));
        } else {
            res.writeHead(404);
            res.end(JSON.stringify({ error: 'Page not found' }));
        }
        return;
    }

    // ── Fallback ──
    res.writeHead(404);
    res.end(JSON.stringify({ error: 'Mock endpoint not found', path: pathname }));
});

server.on('error', (err) => {
    if (err.code === 'EADDRINUSE') {
        console.log(`Port ${PORT} already in use — assuming mock server is already running.`);
        process.exit(0);
    }
    throw err;
});

server.listen(PORT, () => {
    console.log(`Mock external API server running on http://localhost:${PORT}`);
});
