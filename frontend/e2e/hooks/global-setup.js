import { spawn } from 'child_process';
import { dirname, join } from 'path';
import { fileURLToPath } from 'url';
import { request as playwrightRequest } from '@playwright/test';
import { API_URL } from '../config.js';

const __dirname = dirname(fileURLToPath(import.meta.url));

// Start the mock external API server (Jira/Confluence stand-in on :9999) unless
// it is already running.
async function startMockServer() {
    try {
        const res = await fetch('http://localhost:9999/rest/api/3/issue/PROJ-101');
        if (res.ok) {
            console.log('Mock server already running on :9999');
            return;
        }
    } catch {
        // Not running — fall through and spawn it below.
    }

    const child = spawn('node', [join(__dirname, '..', 'mocks', 'mock-external-server.cjs')], {
        stdio: 'pipe',
        detached: true,
    });

    await new Promise((resolve, reject) => {
        const timeout = setTimeout(() => reject(new Error('Mock server failed to start')), 10000);
        child.stdout.on('data', (data) => {
            if (data.toString().includes('running on')) {
                clearTimeout(timeout);
                resolve();
            }
        });
        child.stderr.on('data', (data) => {
            clearTimeout(timeout);
            reject(new Error(`Mock server error: ${data}`));
        });
    });

    child.unref();
    globalThis.__MOCK_SERVER_PID__ = child.pid;
}

// Log in via the API and persist the session cookie to .auth-state.json so every
// test starts authenticated (playwright.config.js storageState points here).
async function authenticateAndSaveState() {
    const apiContext = await playwrightRequest.newContext();

    const loginRes = await apiContext.post(`${API_URL}/auth/login`, {
        data: {
            email: process.env.TEST_ADMIN_EMAIL || 'admin@example.com',
            password: process.env.TEST_ADMIN_PASSWORD || 'changeme123',
        },
    });
    if (!loginRes.ok()) {
        const body = await loginRes.text();
        throw new Error(`Login failed: ${loginRes.status()} ${body}`);
    }

    const cookies = loginRes.headers()['set-cookie'];
    if (!cookies) {
        throw new Error('No session cookie returned from login');
    }
    const sessionMatch = cookies.match(/session_token=([^;]+)/);
    if (!sessionMatch) {
        throw new Error('session_token cookie not found in response');
    }

    const storageState = {
        cookies: [
            {
                name: 'session_token',
                value: sessionMatch[1],
                domain: 'localhost',
                path: '/',
                httpOnly: true,
                secure: false,
                sameSite: 'Lax',
                expires: -1,
            },
        ],
        origins: [],
    };

    const fs = await import('fs');
    fs.writeFileSync(join(__dirname, '..', '.auth-state.json'), JSON.stringify(storageState, null, 2));

    await apiContext.dispose();
}

export default async function globalSetup() {
    await startMockServer();
    await authenticateAndSaveState();
}
