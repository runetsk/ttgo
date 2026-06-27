
async function cleanup() {
    const API_URL = 'http://localhost:8080/api';
    try {
        // 1. Get runs
        const runsResp = await fetch(`${API_URL}/runs`);
        const { runs = [] } = await runsResp.json();
        for (const run of runs) {
            await fetch(`${API_URL}/runs/${run.id}`, { method: 'DELETE' });
            console.log(`Deleted run ${run.id}`);
        }

        // 2. Get suites
        const suitesResp = await fetch(`${API_URL}/suites`);
        const { suites = [] } = await suitesResp.json();
        for (const suite of suites) {
            await fetch(`${API_URL}/suites/${suite.id}`, { method: 'DELETE' });
            console.log(`Deleted suite ${suite.id}`);
        }

        // 3. Get folder tree
        const treeResp = await fetch(`${API_URL}/folders/tree`);
        const roots = await treeResp.json();
        if (roots && Array.isArray(roots) && roots.length > 0) {
            const rootIds = roots.filter(f => f && f.id).map(f => f.id);
            if (rootIds.length > 0) {
                await fetch(`${API_URL}/folders/bulk-delete`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ ids: rootIds })
                });
                console.log(`Deleted ${rootIds.length} root folders`);
            }
        }

        console.log('Cleanup complete');
    } catch (err) {
        console.error('Cleanup failed:', err.message);
    }
}

cleanup();
