import test from 'node:test';
import assert from 'node:assert/strict';
import { buildDraftDiff, diffText, leftParts, rightParts } from './draftDiff.js';

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

test('leftParts keeps common and removed, drops added', () => {
    const parts = diffText('It works', 'Dashboard is displayed');
    const left = leftParts(parts);
    assert.ok(left.every(p => !p.added), 'no additions on the original side');
    assert.ok(left.some(p => p.removed), 'removals retained on the original side');
});

test('rightParts keeps common and added, drops removed', () => {
    const parts = diffText('It works', 'Dashboard is displayed');
    const right = rightParts(parts);
    assert.ok(right.every(p => !p.removed), 'no removals on the new side');
    assert.ok(right.some(p => p.added), 'additions retained on the new side');
});

test('buildDraftDiff.summary counts changed fields and steps', () => {
    const d = buildDraftDiff(original, alternative);
    assert.equal(d.summary.nameChanged, true);
    assert.equal(d.summary.descriptionChanged, false);
    assert.equal(d.summary.categoryChanged, false);
    assert.equal(d.summary.stepsChanged, 1);
    assert.equal(d.summary.stepsAdded, 1);
    assert.equal(d.summary.stepsRemoved, 0);
    assert.equal(d.summary.stepsUnchanged, 1);
    assert.equal(d.summary.refsAdded, 1);
    assert.equal(d.summary.refsRemoved, 0);
});

test('decodes HTML entities before diffing step and description text', () => {
    const o = { name: 'n', category: 'c', description: "was &#39;old&#39;", source_refs: [],
        steps: [{ action: "click &#39;Save&#39;", expected_result: 'a &amp; b' }] };
    const a = { name: 'n', category: 'c', description: "now &#39;new&#39;", source_refs: [],
        steps: [{ action: "tap &#39;Save&#39;", expected_result: 'a &amp; b' }] };
    const d = buildDraftDiff(o, a);
    // Join each SIDE of the word-diff separately (as the compare modal does via
    // leftParts/rightParts) rather than all parts in array order — "old"/"new"
    // sit inside shared quote punctuation, so a naive full-array join
    // interleaves both sides into a string that contains neither intact.
    const oldDescText = leftParts(d.description).map(p => p.value).join('');
    const newDescText = rightParts(d.description).map(p => p.value).join('');
    assert.ok(oldDescText.includes("'old'") && newDescText.includes("'new'"), 'description entities decoded');
    assert.ok(!oldDescText.includes('&#39;') && !newDescText.includes('&#39;'), 'no raw entities in description diff');
    const actionText = d.steps[0].action.map(p => p.value).join('');
    assert.ok(actionText.includes("'Save'") && !actionText.includes('&#39;'), 'step action entities decoded');
});
