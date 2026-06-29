import { defineConfig, devices } from '@playwright/test';
import { BASE_URL } from './e2e/config.js';

export default defineConfig({
    testDir: './e2e/tests',
    fullyParallel: false,
    forbidOnly: !!process.env.CI,
    retries: process.env.CI ? 2 : 0,
    workers: 1,
    reporter: [
        ['html'],
        ['./e2e/reporters/ttgo-reporter.js'],
    ],
    timeout: 30000,
    expect: {
        timeout: 15000,
    },
    use: {
        baseURL: BASE_URL,
        // Capture a screenshot + trace on failure; the reporter uploads the screenshot
        // to the TTGO result, and the trace is retained locally for `show-trace`.
        trace: 'retain-on-failure',
        screenshot: 'only-on-failure',
        storageState: './e2e/.auth-state.json',
        // Run with a visible browser locally; stay headless in CI (no display server).
        headless: !!process.env.CI,
    },
    projects: [
        {
            name: 'chromium',
            use: { ...devices['Desktop Chrome'] },
        },
    ],
    // Only auto-start the vite dev server when the suite targets it; against an
    // already-running stack (e.g. Docker on :80) there is nothing to launch.
    webServer: BASE_URL.includes(':5173') ? {
        command: 'npm run dev',
        url: BASE_URL,
        reuseExistingServer: !process.env.CI,
        cwd: './',
    } : undefined,
    globalSetup: './e2e/hooks/global-setup.js',
    globalTeardown: './e2e/hooks/global-teardown.js',
});
