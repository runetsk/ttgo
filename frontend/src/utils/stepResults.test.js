import test from 'node:test';
import assert from 'node:assert/strict';
import { isManualStepResults, parseStepVerdicts, buildStepResults } from './stepResults.js';

test('isManualStepResults detects the manual shape', () => {
    assert.equal(isManualStepResults([{ order_index: 0, status: 'PASS' }]), true);
    assert.equal(isManualStepResults([]), false);
    assert.equal(isManualStepResults(null), false);
    assert.equal(isManualStepResults([{ name: 'automation step', duration: 5 }]), false);
    assert.equal(isManualStepResults('not-an-array'), false);
});

test('parseStepVerdicts keys by order_index', () => {
    const map = parseStepVerdicts([
        { order_index: 0, status: 'PASS', note: '' },
        { order_index: 2, status: 'FAIL', note: 'broke' },
    ]);
    assert.deepEqual(map[0], { status: 'PASS', note: '' });
    assert.deepEqual(map[2], { status: 'FAIL', note: 'broke' });
    assert.deepEqual(parseStepVerdicts(null), {});
    assert.deepEqual(parseStepVerdicts([{ name: 'x' }]), {});
});

test('buildStepResults snapshots authored steps with verdicts', () => {
    const authored = [
        { order_index: 1, action: '<p>b</p>', expected_result: '<p>B</p>' },
        { order_index: 0, action: '<p>a</p>', expected_result: '<p>A</p>' },
    ];
    const verdicts = { 0: { status: 'PASS', note: '' }, 1: { status: 'FAIL', note: 'x' } };
    const out = buildStepResults(authored, verdicts);
    assert.equal(out.length, 2);
    assert.deepEqual(out[0], { order_index: 0, action: '<p>a</p>', expected_result: '<p>A</p>', status: 'PASS', note: '' });
    assert.deepEqual(out[1], { order_index: 1, action: '<p>b</p>', expected_result: '<p>B</p>', status: 'FAIL', note: 'x' });
});

test('buildStepResults defaults unmarked steps to empty status', () => {
    const out = buildStepResults([{ order_index: 0, action: '<p>a</p>', expected_result: '' }], {});
    assert.deepEqual(out[0], { order_index: 0, action: '<p>a</p>', expected_result: '', status: '', note: '' });
});
