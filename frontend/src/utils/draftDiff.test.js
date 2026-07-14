import test from 'node:test';
import assert from 'node:assert/strict';
import { buildDraftDiff, diffText } from './draftDiff.js';

const original = {
    name: 'Sign in works',
    category: 'Functional',
    description: 'Basic login.',
    source_refs: ['AC-1'],
    steps: [
        { action: 'Open the page', expected_result: 'Page shown' },
        { action: 'Submit', expected_result: 'It works' },
    ],
};
const alternative = {
    name: 'Sign in with valid credentials works',
    category: 'Functional',
    description: 'Basic login.',
    source_refs: ['AC-1', 'AC-2'],
    steps: [
        { action: 'Open the page', expected_result: 'Page shown' },
        { action: 'Submit the form', expected_result: 'Dashboard is displayed' },
        { action: 'Check the header', expected_result: 'User menu visible' },
    ],
};

test('diffText marks additions and removals', () => {
    const parts = diffText('It works', 'Dashboard is displayed');
    assert.ok(parts.some(p => p.removed));
    assert.ok(parts.some(p => p.added));
});

test('buildDraftDiff aligns steps by index and classifies the tail', () => {
    const d = buildDraftDiff(original, alternative);
    assert.ok(d.name.some(p => p.added), 'name gained words');
    assert.equal(d.category.changed, false);
    assert.deepEqual(d.sourceRefs.added, ['AC-2']);
    assert.deepEqual(d.sourceRefs.removed, []);
    assert.equal(d.steps.length, 3);
    assert.equal(d.steps[0].type, 'unchanged');
    assert.equal(d.steps[1].type, 'changed');
    assert.equal(d.steps[2].type, 'added');
});

test('removed trailing steps are reported', () => {
    const d = buildDraftDiff(alternative, original);
    assert.equal(d.steps[2].type, 'removed');
});
