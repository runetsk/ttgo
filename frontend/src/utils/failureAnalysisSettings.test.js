import test from 'node:test';
import assert from 'node:assert/strict';
import { validateFailureAnalysisDraft, isFailureAnalysisDirty, parseWholeNumber, withFailureAnalysisDefaults, hedgeMax } from './failureAnalysisSettings.js';

const FA = {
    enabled_on_completion: false, max_analyses_per_run: 20, parallel_groups: 4, dedup_enabled: true, redaction_enabled: true,
    prompt_template: 'Classify.', llm_call_timeout_seconds: 45, hedge_after_seconds: 0, few_shot_examples: 4,
};

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

test('the LLM call timeout must be a whole number of seconds from 10 to 120', () => {
    for (const bad of [9, 121, NaN, 12.5, undefined]) {
        assert.match(validateFailureAnalysisDraft({ ...FA, llm_call_timeout_seconds: bad }).llm_call_timeout_seconds, /from 10 to 120/, String(bad));
    }
    for (const ok of [10, 120]) assert.equal(validateFailureAnalysisDraft({ ...FA, llm_call_timeout_seconds: ok }).llm_call_timeout_seconds, undefined);
});

test('hedging is 0 (off) or 3 s up to one second below the call timeout', () => {
    for (const ok of [0, 3, 44]) assert.equal(validateFailureAnalysisDraft({ ...FA, hedge_after_seconds: ok }).hedge_after_seconds, undefined, String(ok));
    for (const bad of [1, 2, 45, 60, NaN, 2.5, -1]) {
        assert.match(validateFailureAnalysisDraft({ ...FA, hedge_after_seconds: bad }).hedge_after_seconds, /0 \(off\) or .* from 3 to 44/, String(bad));
    }
    assert.match(validateFailureAnalysisDraft({ ...FA, llm_call_timeout_seconds: 10, hedge_after_seconds: 10 }).hedge_after_seconds, /from 3 to 9/);
    assert.equal(hedgeMax(45), 44);
    assert.equal(hedgeMax(NaN), 119, 'with an invalid timeout the widest range applies; the timeout has its own error');
});

test('past triage examples must be a whole number from 0 to 8', () => {
    for (const bad of [-1, 9, NaN, 1.5]) {
        assert.match(validateFailureAnalysisDraft({ ...FA, few_shot_examples: bad }).few_shot_examples, /from 0 to 8/, String(bad));
    }
    for (const ok of [0, 8]) assert.equal(validateFailureAnalysisDraft({ ...FA, few_shot_examples: ok }).few_shot_examples, undefined);
});

test('withFailureAnalysisDefaults fills fields an older server does not send, and nothing else', () => {
    const old = { enabled_on_completion: false, max_analyses_per_run: 20, parallel_groups: 4, dedup_enabled: true, redaction_enabled: true, prompt_template: 'p' };
    const filled = withFailureAnalysisDefaults(old);
    assert.deepEqual([filled.llm_call_timeout_seconds, filled.hedge_after_seconds, filled.few_shot_examples], [45, 0, 4]);
    assert.deepEqual(validateFailureAnalysisDraft(filled), {});
    assert.equal(withFailureAnalysisDefaults({ ...FA, few_shot_examples: 0 }).few_shot_examples, 0, 'a stored 0 stays 0');
    assert.equal(withFailureAnalysisDefaults(null), null);
});

test('isFailureAnalysisDirty covers the new fields', () => {
    assert.equal(isFailureAnalysisDirty({ ...FA, hedge_after_seconds: 10 }, FA), true);
    assert.equal(isFailureAnalysisDirty({ ...FA, few_shot_examples: 0 }, FA), true);
    assert.equal(isFailureAnalysisDirty({ ...FA, llm_call_timeout_seconds: 60 }, FA), true);
});
