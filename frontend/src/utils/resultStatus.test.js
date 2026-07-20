import test from 'node:test';
import assert from 'node:assert/strict';
import { isFailureStatus } from './resultStatus.js';

test('FAIL and ERROR are failures', () => {
    assert.equal(isFailureStatus('FAIL'), true);
    assert.equal(isFailureStatus('ERROR'), true, 'ERROR is triageable — this is the Gap A guard');
});

test('every other result status is not a failure', () => {
    for (const status of ['PASS', 'SKIP', 'PENDING', 'RUNNING']) {
        assert.equal(isFailureStatus(status), false, `${status} must not be treated as a failure`);
    }
});

test('missing or malformed statuses are not failures', () => {
    for (const status of [null, undefined, '', 'fail', 'Error', 'FAILED', 'garbage', 0, 1, {}, []]) {
        assert.equal(isFailureStatus(status), false, `${JSON.stringify(status)} must not be a failure`);
    }
});
