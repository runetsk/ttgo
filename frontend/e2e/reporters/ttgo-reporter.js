import { createTtgoClient } from './ttgo-client.js';
import { testCaseName, buildResultBody } from './reporter-helpers.js';

// Opt-in Playwright reporter that mirrors each `playwright test` invocation as a
// TTGO test run. Activates only when TTGO_REPORT_TOKEN is set; otherwise it is a
// no-op so normal local runs are unaffected. All TTGO calls are wrapped so a
// reporting failure never fails the Playwright run.
export default class TtgoReporter {
    constructor() {
        this.token = process.env.TTGO_REPORT_TOKEN || '';
        this.enabled = Boolean(this.token);
        this.baseUrl = process.env.TTGO_REPORT_URL || 'http://localhost:8080';
        this.folderName = process.env.TTGO_REPORT_FOLDER || 'Playwright E2E';
        this.categoryName = process.env.TTGO_REPORT_CATEGORY || 'Playwright E2E';
        this.runPrefix = process.env.TTGO_REPORT_RUN_NAME || 'Playwright E2E';
        this.environment = process.env.TTGO_REPORT_ENV || 'e2e';
        this.results = new Map(); // test.id -> { test, result }; latest attempt wins
    }

    onBegin() {
        if (!this.enabled) {
            console.log('[ttgo] reporter disabled (set TTGO_REPORT_TOKEN to push results)');
        }
    }

    onTestEnd(test, result) {
        if (!this.enabled) return;
        // A retried test fires onTestEnd once per attempt; keep the last (final) one.
        this.results.set(test.id, { test, result });
    }

    async onEnd() {
        if (!this.enabled) return;
        const entries = [...this.results.values()];
        if (entries.length === 0) return;

        try {
            const client = createTtgoClient({ baseUrl: this.baseUrl, token: this.token });

            const folderId = await client.findOrCreateFolder(this.folderName);
            const categoryId = await client.findOrCreateCategory(this.categoryName);

            const named = entries.map(({ test, result }) => ({
                test,
                result,
                name: testCaseName(test),
            }));
            const caseMap = await client.ensureTestCases(folderId, named.map((e) => e.name));

            // Create the run WITHOUT a category, then attach the category as a label.
            // Creating with category_id would make the backend snapshot the category's
            // assigned test cases into PENDING results, which collide with the explicit
            // per-test results posted below (duplicate rows / attempt_number conflicts).
            // The category is a nice-to-have label, so a failure to attach is non-fatal.
            const runName = `${this.runPrefix} — ${new Date().toISOString()}`;
            const run = await client.createRun({ name: runName });
            try {
                await client.updateRun(run.id, { category_id: categoryId });
            } catch (e) {
                console.warn(`[ttgo] could not attach category (run still recorded): ${e.message}`);
            }

            let pushed = 0;
            for (const { test, result, name } of named) {
                const testCaseId = caseMap.get(name);
                if (!testCaseId) continue;
                const project = test.titlePath().filter(Boolean)[0] || 'chromium';
                const body = buildResultBody({
                    result,
                    testCaseId,
                    name,
                    environment: this.environment,
                    browser: project,
                });
                await client.addResult(run.id, body);
                pushed += 1;
            }

            await client.completeRun(run.id);
            console.log(`[ttgo] pushed ${pushed} result(s) to run ${run.id} (${this.baseUrl})`);
        } catch (err) {
            console.warn(`[ttgo] result reporting failed (run not affected): ${err.message}`);
        }
    }
}
