import test from 'node:test';
import assert from 'node:assert/strict';
import { runToDebug } from './aiRunDebug.js';

test('runToDebug maps a completed run to the LlmFeedbackPanel debug shape', () => {
    const debug = runToDebug({
        id: 'r1',
        duration_ms: 1234,
        model_name: 'gpt-test',
        finish_reason: 'stop',
        max_tokens_budget: 8192,
        retry_count: 1,
        provider_label: 'My OpenAI',
        provider_type: 'openai',
        request_context: 'PROMPT TEXT',
        template_type: 'standard',
        prompt_tokens: 100,
        completion_tokens: 200,
        total_tokens: 300,
    });
    assert.deepEqual(debug, {
        duration_ms: 1234,
        model: 'gpt-test',
        finish_reason: 'stop',
        max_tokens_budget: 8192,
        retried: true,
        provider_label: 'My OpenAI',
        provider_type: 'openai',
        request_context: 'PROMPT TEXT',
        template_type: 'standard',
        usage: { prompt_tokens: 100, completion_tokens: 200, total_tokens: 300 },
    });
});

test('runToDebug omits usage when the provider reported no tokens', () => {
    const debug = runToDebug({ duration_ms: 5, retry_count: 0, prompt_tokens: 0, completion_tokens: 0, total_tokens: 0 });
    assert.equal(debug.usage, undefined);
    assert.equal(debug.retried, false);
});

test('runToDebug returns null for missing run', () => {
    assert.equal(runToDebug(null), null);
});
