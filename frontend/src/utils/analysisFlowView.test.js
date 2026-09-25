import test from 'node:test';
import assert from 'node:assert/strict';
import { selectedStepId } from './analysisFlowView.js';

const flow = (blocked, statuses) => ({
    blocked,
    steps: ['start', 'group', 'evidence', 'decide', 'explain', 'store'].map((id, i) => ({ id, status: statuses[i] })),
});
const RUNNING = flow(false, ['run', 'run', 'run', 'run', 'skip', 'run']);
const BLOCKED = flow(true, ['blocked', 'idle', 'idle', 'idle', 'idle', 'idle']);

test('defaults to Decide the verdict', () => {
    assert.equal(selectedStepId(RUNNING, null), 'decide');
});

test('keeps the picked step, skipped ones included', () => {
    assert.equal(selectedStepId(RUNNING, 'group'), 'group');
    assert.equal(selectedStepId(RUNNING, 'explain'), 'explain');
});

test('a blocked flow shows Start, which carries the reason', () => {
    assert.equal(selectedStepId(BLOCKED, null), 'start');
    assert.equal(selectedStepId(BLOCKED, 'group'), 'start', 'idle steps have no detail to show');
});

test('an unknown pick falls back, and no flow selects nothing', () => {
    assert.equal(selectedStepId(RUNNING, 'nope'), 'decide');
    assert.equal(selectedStepId(null, 'group'), null);
});
