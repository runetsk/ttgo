export default async function globalTeardown() {
    if (globalThis.__MOCK_SERVER_PID__) {
        try {
            process.kill(globalThis.__MOCK_SERVER_PID__);
        } catch {
            // Already dead
        }
    }
}
