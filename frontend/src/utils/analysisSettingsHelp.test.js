import test from 'node:test';
import assert from 'node:assert/strict';
import { SETTING_HELP, STEP_HELP } from './analysisSettingsHelp.js';
import { buildAnalysisFlow } from './analysisFlow.js';

const TS = {
    enabled: true, api_key: '', clear_api_key: false, api_key_status: 'ok', model: 'jev-1.13.0', timeout_seconds: 30,
    verdict_engine_enabled: true, narrative_enabled: true, escalate_below_pct: 0, llm_fallback_enabled: true,
    semantic_dedup_enabled: true, allow_auto_failure_analysis: false,
};
const FA = { enabled_on_completion: true, max_analyses_per_run: 20, parallel_groups: 4, dedup_enabled: true, redaction_enabled: true, prompt_template: 'p' };
const LLM = { label: 'OpenRouter', model_name: 'minimax', is_default: true, enabled: true, allow_auto_failure_analysis: false };

function* inputs() {
    for (const trigger of ['manual', 'auto']) {
        for (const ts of [TS, { ...TS, enabled: false }, { ...TS, verdict_engine_enabled: false }, { ...TS, allow_auto_failure_analysis: true }, { ...TS, clear_api_key: true }]) {
            for (const provider of [LLM, { ...LLM, allow_auto_failure_analysis: true }, null]) {
                yield { aiEnabled: true, typesafe: ts, failureAnalysis: FA, provider, trigger };
            }
        }
    }
}

test('every setting the diagram names has a hint', () => {
    const keys = new Set();
    for (const input of inputs()) {
        for (const step of buildAnalysisFlow(input).steps) for (const c of step.chips) keys.add(c.key);
    }
    assert.equal(keys.size, 17, [...keys].join(', '));
    for (const key of keys) assert.ok(SETTING_HELP[key], `no hint for ${key}`);
});

test('every hint has a title and says what the setting does', () => {
    for (const [key, h] of Object.entries(SETTING_HELP)) {
        assert.equal(typeof h.title, 'string', key);
        assert.ok(h.what && h.what.length > 20, key);
    }
});

test('every step has a "How this step works" text', () => {
    const f = buildAnalysisFlow({ aiEnabled: true, typesafe: TS, failureAnalysis: FA, provider: LLM, trigger: 'manual' });
    for (const s of f.steps) assert.ok(STEP_HELP[s.id]?.what, s.id);
});

test('hints state the numbers the server uses', () => {
    assert.match(SETTING_HELP['ts.semantic_dedup_enabled'].what, /80%/);
    assert.match(SETTING_HELP['fa.max_analyses_per_run'].what, /1 to 500/);
    assert.match(SETTING_HELP['fa.parallel_groups'].what, /1 to 8/);
    assert.match(SETTING_HELP['ts.timeout_seconds'].what, /5 to 300/);
    assert.match(SETTING_HELP['fa.dedup_enabled'].details, /30000ms/);
});
