import { test as base, expect } from '@playwright/test';
import { LibraryPage } from '../pages/LibraryPage.js';
import { CategoriesPage } from '../pages/CategoriesPage.js';
import { RunsPage } from '../pages/RunsPage.js';
import { RunDetailPage } from '../pages/RunDetailPage.js';
import { RequirementsPage } from '../pages/RequirementsPage.js';
import { AiStudioPage } from '../pages/AiStudioPage.js';
import { RunExecutePage } from '../pages/RunExecutePage.js';
import { TestCaseDetailPage } from '../pages/TestCaseDetailPage.js';
import { SettingsPage } from '../pages/SettingsPage.js';
import { ApiClient } from '../helpers/api.js';
import { startFakeLLM } from '../helpers/fake-llm.js';
import { attachConsoleLogging } from '../helpers/diagnostics.js';

// Extended `test` that injects the page objects, the API client, and the
// support fixtures. Specs import { test, expect } from here instead of
// '@playwright/test'.
export const test = base.extend({
    // Page objects — instantiated per test, on demand.
    libraryPage: async ({ page }, use) => use(new LibraryPage(page)),
    categoriesPage: async ({ page }, use) => use(new CategoriesPage(page)),
    runsPage: async ({ page }, use) => use(new RunsPage(page)),
    runDetailPage: async ({ page }, use) => use(new RunDetailPage(page)),
    requirementsPage: async ({ page }, use) => use(new RequirementsPage(page)),
    aiStudioPage: async ({ page }, use) => use(new AiStudioPage(page)),
    runExecutePage: async ({ page }, use) => use(new RunExecutePage(page)),
    testCaseDetailPage: async ({ page }, use) => use(new TestCaseDetailPage(page)),
    settingsPage: async ({ page }, use) => use(new SettingsPage(page)),

    // API client bound to this test's request context: `await api.createRun(...)`.
    api: async ({ request }, use) => use(new ApiClient(request)),

    // Browser console/pageerror forwarding — auto, silent unless E2E_DEBUG is set.
    // Replaces the per-spec console-logging beforeEach.
    consoleLogging: [async ({ page }, use) => {
        attachConsoleLogging(page);
        await use();
    }, { auto: true }],

    // Factory: `const llm = await fakeLLM(envelope)` starts a fake OpenAI-compatible
    // provider and returns { url, providerId, dispose }; all are torn down at test end.
    fakeLLM: async ({ api }, use) => {
        const servers = [];
        await use(async (envelope) => {
            const server = await startFakeLLM(api, envelope);
            servers.push(server);
            return server;
        });
        for (const server of servers) await server.dispose();
    },
});

export { expect };
