import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
    testDir: './e2e',
    testIgnore: '**/reporters/**',
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
        baseURL: 'http://localhost:5173',
        trace: 'on-first-retry',
        storageState: './e2e/.auth-state.json',
    },
    projects: [
        {
            name: 'chromium',
            use: { ...devices['Desktop Chrome'] },
        },
    ],
    webServer: {
        command: 'npm run dev',
        url: 'http://localhost:5173',
        reuseExistingServer: !process.env.CI,
        cwd: './',
    },
    globalSetup: './e2e/global-setup.js',
    globalTeardown: './e2e/global-teardown.js',
});
