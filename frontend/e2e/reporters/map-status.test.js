import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mapStatus } from './map-status.js';

test('maps passed to PASS', () => {
    assert.equal(mapStatus('passed'), 'PASS');
});

test('maps failed and timedOut to FAIL', () => {
    assert.equal(mapStatus('failed'), 'FAIL');
    assert.equal(mapStatus('timedOut'), 'FAIL');
});

test('maps skipped to SKIP', () => {
    assert.equal(mapStatus('skipped'), 'SKIP');
});

test('maps interrupted to ERROR (an aborted run must not complete green)', () => {
    assert.equal(mapStatus('interrupted'), 'ERROR');
});

test('maps unknown status to FAIL (surfaces problems rather than hiding them)', () => {
    assert.equal(mapStatus('something-new'), 'FAIL');
});
