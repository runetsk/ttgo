import { test } from 'node:test';
import assert from 'node:assert/strict';
import { testCaseName, buildResultBody, extractSteps } from './reporter-helpers.js';

// Minimal Playwright TestCase stub: only titlePath() is used by the helpers.
const fakeTest = (titlePath) => ({ titlePath: () => titlePath });

test('testCaseName drops the root ("") and project segments', () => {
    const t = fakeTest(['', 'chromium', 'e2e/analytics.spec.js', 'Analytics', 'loads charts']);
    assert.equal(testCaseName(t), 'e2e/analytics.spec.js › Analytics › loads charts');
});

test('buildResultBody maps a passing result with timing and no error fields', () => {
    const body = buildResultBody({
        result: { status: 'passed', duration: 1234.6, retry: 0, startTime: '2026-06-29T10:00:00.000Z' },
        testCaseId: 'tc1',
        name: 'a.spec.js › ok',
        environment: 'e2e',
        browser: 'chromium',
    });
    assert.equal(body.status, 'PASS');
    assert.equal(body.test_case_id, 'tc1');
    assert.equal(body.test_name_snapshot, 'a.spec.js › ok');
    assert.equal(body.attempt_number, 1);
    assert.equal(body.duration_ms, 1235);
    assert.equal(body.environment, 'e2e');
    assert.equal(body.browser, 'chromium');
    assert.equal(body.start_time, '2026-06-29T10:00:00.000Z');
    assert.equal(body.end_time, '2026-06-29T10:00:01.235Z');
    assert.equal(body.error_message, undefined);
    assert.equal(body.stack_trace, undefined);
    assert.equal(body.defect_type, undefined);
});

test('buildResultBody includes error details + attempt number + defect_type on failure', () => {
    const body = buildResultBody({
        result: { status: 'timedOut', duration: 500, retry: 1, error: { message: 'Timeout', stack: 'at foo' } },
        testCaseId: 'tc2',
        name: 'a.spec.js › bad',
    });
    assert.equal(body.status, 'FAIL');
    assert.equal(body.attempt_number, 2);
    assert.equal(body.failure_type, 'timedOut');
    assert.equal(body.error_message, 'Timeout');
    assert.equal(body.stack_trace, 'at foo');
    assert.equal(body.defect_type, 'to_investigate');
});

test('buildResultBody maps interrupted to ERROR with error detail but no defect_type', () => {
    const body = buildResultBody({
        result: { status: 'interrupted', duration: 10, error: { message: 'aborted', stack: 'at bar' } },
        testCaseId: 'tc4',
        name: 'a.spec.js › aborted',
    });
    assert.equal(body.status, 'ERROR');
    assert.equal(body.failure_type, 'interrupted');
    assert.equal(body.error_message, 'aborted');
    assert.equal(body.defect_type, undefined);
});

test('buildResultBody truncates very long error text', () => {
    const long = 'x'.repeat(5000);
    const body = buildResultBody({
        result: { status: 'failed', duration: 1, errors: [{ message: long, stack: long }] },
        testCaseId: 'tc3',
        name: 'a.spec.js › bad',
    });
    assert.ok(body.error_message.length < long.length);
    assert.ok(body.error_message.endsWith('…[truncated]'));
});

test('extractSteps keeps only test.step entries, ordered', () => {
    const result = {
        steps: [
            { category: 'hook', title: 'Before Hooks' },
            { category: 'test.step', title: 'Seed data' },
            { category: 'pw:api', title: "page.goto('/runs')" },
            { category: 'test.step', title: 'Filter by category' },
            { category: 'expect', title: 'expect toBeVisible' },
        ],
    };
    assert.deepEqual(extractSteps(result), [
        { action: 'Seed data', expected_result: '', order_index: 0 },
        { action: 'Filter by category', expected_result: '', order_index: 1 },
    ]);
});

test('extractSteps returns [] when there are no test.step entries', () => {
    assert.deepEqual(extractSteps({ steps: [{ category: 'pw:api', title: 'x' }] }), []);
    assert.deepEqual(extractSteps({}), []);
});
