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

test('buildResultBody attaches captured stdout and stderr as log_text on failure', () => {
    const body = buildResultBody({
        result: {
            status: 'failed', duration: 10, retry: 0,
            error: { message: 'boom', stack: 'at x' },
            stdout: ['step 1 ok\n', Buffer.from('step 2 ok\n')],
            stderr: ['warn: slow response\n'],
        },
        testCaseId: 'tc3',
        name: 'a.spec.js › logs',
    });
    assert.equal(body.log_text, 'step 1 ok\nstep 2 ok\n[stderr] warn: slow response\n');
});

test('buildResultBody keeps the tail of an oversized log and omits log_text on a pass', () => {
    const big = 'x'.repeat(250000) + 'THE END';
    const failed = buildResultBody({
        result: { status: 'failed', duration: 10, retry: 0, error: { message: 'boom' }, stdout: [big], stderr: [] },
        testCaseId: 'tc4', name: 'a.spec.js › big',
    });
    assert.ok(failed.log_text.length <= 200000 + 40, `capped: ${failed.log_text.length}`);
    assert.ok(failed.log_text.endsWith('THE END'), 'the tail is what the analysis needs');
    assert.ok(failed.log_text.startsWith('…[truncated]'), 'the cut is marked at the head');
    const passed = buildResultBody({
        result: { status: 'passed', duration: 10, retry: 0, stdout: ['noise\n'], stderr: [] },
        testCaseId: 'tc5', name: 'a.spec.js › ok',
    });
    assert.equal(passed.log_text, undefined);
});

test('buildResultBody strips ANSI colour codes from the error, the stack and the log', () => {
    const esc = '\u001b';
    const body = buildResultBody({
        result: {
            status: 'failed', duration: 10, retry: 0,
            error: {
                message: `Error: ${esc}[2mexpect(${esc}[22m${esc}[31mreceived${esc}[39m${esc}[2m).${esc}[22mtoBe`,
                stack: `Error: boom\n    at ${esc}[90mfile.js:1:1${esc}[39m`,
            },
            stdout: [`${esc}[32m✓${esc}[39m step ok\n`],
            stderr: [],
        },
        testCaseId: 'tc6', name: 'a.spec.js › ansi',
    });
    assert.equal(body.error_message, 'Error: expect(received).toBe');
    assert.equal(body.stack_trace, 'Error: boom\n    at file.js:1:1');
    assert.equal(body.log_text, '✓ step ok\n');
});
