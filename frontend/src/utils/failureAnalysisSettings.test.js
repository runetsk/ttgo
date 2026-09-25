import test from 'node:test';
import assert from 'node:assert/strict';
import { validateFailureAnalysisDraft, isFailureAnalysisDirty, parseWholeNumber } from './failureAnalysisSettings.js';

const FA = { enabled_on_completion: false, max_analyses_per_run: 20, parallel_groups: 4, dedup_enabled: true, redaction_enabled: true, prompt_template: 'Classify.' };

test('a valid draft has no errors', () => {
    assert.deepEqual(validateFailureAnalysisDraft(FA), {});
});

test('the cap must be a whole number from 1 to 500, as the server requires', () => {
    for (const bad of [0, 501, NaN, 2.5, undefined]) {
        assert.match(validateFailureAnalysisDraft({ ...FA, max_analyses_per_run: bad }).max_analyses_per_run, /from 1 to 500/, String(bad));
    }
    for (const ok of [1, 500]) assert.equal(validateFailureAnalysisDraft({ ...FA, max_analyses_per_run: ok }).max_analyses_per_run, undefined);
});

test('groups at once must be a whole number from 1 to 8', () => {
    for (const bad of [0, 9, NaN, 1.5]) {
        assert.match(validateFailureAnalysisDraft({ ...FA, parallel_groups: bad }).parallel_groups, /from 1 to 8/, String(bad));
    }
    assert.equal(validateFailureAnalysisDraft({ ...FA, parallel_groups: 8 }).parallel_groups, undefined);
});

test('the prompt cannot be empty', () => {
    assert.match(validateFailureAnalysisDraft({ ...FA, prompt_template: '  ' }).prompt_template, /cannot be empty/);
});

test('parseWholeNumber keeps an emptied input as NaN instead of 0', () => {
    assert.ok(Number.isNaN(parseWholeNumber('')));
    assert.equal(parseWholeNumber('12'), 12);
    assert.equal(parseWholeNumber('2.5'), 2.5);
    assert.ok(Number.isNaN(parseWholeNumber('abc')));
});

test('isFailureAnalysisDirty compares the fields that are saved', () => {
    assert.equal(isFailureAnalysisDirty(FA, { ...FA }), false);
    assert.equal(isFailureAnalysisDirty({ ...FA, dedup_enabled: false }, FA), true);
    assert.equal(isFailureAnalysisDirty({ ...FA, max_analyses_per_run: NaN }, FA), true);
    assert.equal(isFailureAnalysisDirty({ ...FA, id: 'other' }, FA), false);
});
