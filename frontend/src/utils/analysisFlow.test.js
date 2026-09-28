import test from 'node:test';
import assert from 'node:assert/strict';
import { keyState, llmName, buildAnalysisFlow, effectiveDraft, diagramModel, resolveRoute } from './analysisFlow.js';

const TS = {
    enabled: true, api_key: '', clear_api_key: false, api_key_status: 'ok', model: 'jev-1.13.0', timeout_seconds: 30,
    verdict_engine_enabled: true, narrative_enabled: true, escalate_below_pct: 0, llm_fallback_enabled: true,
    semantic_dedup_enabled: true, allow_auto_failure_analysis: false,
};
const FA = { enabled_on_completion: true, max_analyses_per_run: 20, parallel_groups: 4, dedup_enabled: true, redaction_enabled: true, prompt_template: 'p' };
const LLM = { label: 'OpenRouter', model_name: 'minimax', is_default: true, enabled: true, allow_auto_failure_analysis: false };
const APPROVED = { ...LLM, allow_auto_failure_analysis: true };

const flow = (over = {}) => buildAnalysisFlow({ aiEnabled: true, typesafe: TS, failureAnalysis: FA, provider: LLM, trigger: 'manual', ...over });
const step = (f, id) => f.steps.find((s) => s.id === id);
const part = (f, stepId, id) => step(f, stepId).parts.find((p) => p.id === id);
const chipOf = (f, key) => f.steps.flatMap((s) => s.chips).find((c) => c.key === key);
const allSends = (f) => f.steps.flatMap((s) => [...s.sends, ...s.parts.flatMap((p) => p.sends)]);

test('keyState reads unsaved key changes before the stored status', () => {
    assert.equal(keyState(TS), 'stored');
    assert.equal(keyState({ ...TS, clear_api_key: true, api_key: 'typed' }), 'removing');
    assert.equal(keyState({ ...TS, api_key_status: 'missing', api_key: ' ts-new ' }), 'new');
    assert.equal(keyState({ ...TS, api_key_status: 'missing' }), 'none');
    assert.equal(keyState({ ...TS, api_key_status: 'undecryptable' }), 'unreadable');
});

test('llmName names a provider once when its label is its model', () => {
    assert.equal(llmName(LLM), 'OpenRouter (minimax)');
    assert.equal(llmName({ label: 'm/m3', model_name: 'm/m3' }), 'm/m3');
    assert.equal(llmName(null), null);
});

test('AI off blocks at the start and nothing later runs or sends', () => {
    const f = flow({ aiEnabled: false });
    assert.equal(step(f, 'start').status, 'blocked');
    assert.equal(step(f, 'start').reason, 'AI features are off: nothing is analyzed.');
    assert.deepEqual(f.steps.slice(1).map((s) => s.status), ['idle', 'idle', 'idle', 'idle', 'idle']);
    assert.deepEqual(allSends(f), []);
    assert.equal(f.blocked, true);
});

test('by completion with "Auto-analyze on run completion" off is blocked', () => {
    const f = flow({ trigger: 'auto', failureAnalysis: { ...FA, enabled_on_completion: false } });
    assert.match(step(f, 'start').reason, /"Auto-analyze on run completion" is off/);
});

test('TypeSafe decides, the LLM explains and stands by', () => {
    const f = flow();
    assert.equal(step(f, 'decide').detail, 'TypeSafe.ai (jev-1.13.0) answers the verdict and defect-type questions.');
    assert.deepEqual(step(f, 'decide').sends, ['typesafe']);
    assert.equal(part(f, 'decide', 'answers').status, 'run');
    assert.match(part(f, 'decide', 'unsure').reason, /Never handed over/);
    assert.equal(part(f, 'decide', 'unavailable').detail, 'If TypeSafe errors or times out, OpenRouter (minimax) decides and explains, and the analysis says TypeSafe was unavailable.');
    assert.deepEqual(part(f, 'decide', 'unavailable').sends, ['llm']);
    assert.equal(part(f, 'explain', 'answers').detail, 'OpenRouter (minimax) writes the explanation.');
    assert.deepEqual(step(f, 'explain').sends, ['llm']);
    assert.ok(step(f, 'decide').notes.includes("The default LLM's own settings (key, model) are checked when the analysis runs."));
});

test('by completion without TypeSafe consent or LLM approval nothing can analyze', () => {
    const f = flow({ trigger: 'auto' });
    assert.equal(step(f, 'start').reason,
        'Nothing can analyze: the default LLM provider is not approved for automatic analysis, and TypeSafe.ai is not allowed on automatic analysis.');
});

test('TypeSafe alone may analyze by completion (the server gate)', () => {
    const f = flow({ trigger: 'auto', typesafe: { ...TS, allow_auto_failure_analysis: true } });
    assert.equal(step(f, 'start').status, 'run');
    assert.equal(step(f, 'decide').status, 'run');
    assert.equal(part(f, 'explain', 'answers').status, 'warn');
    assert.equal(part(f, 'explain', 'answers').detail, 'No explanation: the default LLM provider is not approved for automatic analysis.');
    assert.equal(part(f, 'decide', 'unavailable').status, 'warn');
    assert.match(part(f, 'decide', 'unavailable').detail, /recorded as failed: the default LLM provider is not approved/);
    assert.equal(chipOf(f, 'provider.default').value, 'OpenRouter (minimax), not approved for automatic analysis');
});

test('semantic grouping alone cannot analyze, so nothing is sent', () => {
    const f = flow({ typesafe: { ...TS, verdict_engine_enabled: false }, provider: null });
    assert.equal(step(f, 'start').reason,
        'Nothing can analyze: no default LLM provider is configured, and TypeSafe.ai is not deciding verdicts.');
    assert.equal(step(f, 'group').status, 'idle');
    assert.deepEqual(allSends(f), []);
});

test('without TypeSafe the LLM decides and explains in one answer', () => {
    const f = flow({ typesafe: { ...TS, enabled: false } });
    assert.equal(step(f, 'decide').detail, 'OpenRouter (minimax) decides the verdict and explains it in one answer.');
    assert.equal(step(f, 'decide').notes[0], 'The LLM decides because TypeSafe.ai is off.');
    assert.deepEqual(step(f, 'decide').parts, []);
    assert.equal(step(f, 'explain').detail, 'Written in the same answer as the verdict (step 4).');
    assert.equal(part(f, 'group', 'semantic').reason, 'TypeSafe.ai is off.');
});

test('by completion without TypeSafe consent an approved LLM decides and grouping skips TypeSafe', () => {
    const f = flow({ trigger: 'auto', provider: APPROVED });
    assert.equal(step(f, 'start').status, 'run');
    assert.equal(part(f, 'group', 'semantic').reason, 'TypeSafe.ai is not allowed on automatic analysis.');
    assert.equal(step(f, 'decide').notes[0], 'The LLM decides because TypeSafe.ai is not allowed on automatic analysis.');
});

test('a removed, unreadable or missing key sends every decision down the unavailable path', () => {
    for (const ts of [{ ...TS, clear_api_key: true }, { ...TS, api_key_status: 'undecryptable' }, { ...TS, api_key_status: 'missing' }]) {
        const f = flow({ typesafe: ts });
        assert.equal(step(f, 'decide').status, 'warn');
        assert.match(step(f, 'decide').reason, /every decision takes the "TypeSafe unavailable" path/);
        assert.deepEqual(step(f, 'decide').sends, []);
        assert.equal(part(f, 'decide', 'answers').status, 'skip');
        assert.equal(part(f, 'decide', 'unsure').status, 'skip');
        assert.equal(part(f, 'decide', 'unavailable').status, 'warn');
        assert.equal(part(f, 'decide', 'unavailable').detail,
            'Without a usable key, OpenRouter (minimax) decides and explains, and the analysis says TypeSafe was unavailable.');
        assert.match(part(f, 'group', 'semantic').reason, /^No usable API key/);
    }
    assert.equal(chipOf(flow({ typesafe: { ...TS, clear_api_key: true } }), 'ts.api_key').value, 'Being removed');
});

test('a new key counts as usable but unchecked', () => {
    const f = flow({ typesafe: { ...TS, api_key_status: 'missing', api_key: 'ts-new' } });
    assert.equal(step(f, 'decide').status, 'run');
    assert.equal(chipOf(f, 'ts.api_key').value, 'New, not yet checked');
    assert.ok(step(f, 'decide').notes.includes('New API key, checked on first use.'));
});

test('takeover needs a threshold and an LLM', () => {
    const f = flow({ typesafe: { ...TS, escalate_below_pct: 90 } });
    assert.equal(part(f, 'decide', 'unsure').title, 'TypeSafe below 90% sure');
    assert.deepEqual(part(f, 'decide', 'unsure').sends, ['llm']);
    assert.equal(part(f, 'decide', 'answers').detail, "Confidence 90% or more: TypeSafe's decision is kept.");
    assert.ok(step(f, 'explain').parts.some((p) => p.id === 'unsure'));
    assert.equal(chipOf(f, 'ts.escalate_below_pct').value, '90%');

    const noLLM = flow({ typesafe: { ...TS, escalate_below_pct: 90 }, provider: null });
    assert.equal(part(noLLM, 'decide', 'unsure').reason, 'No LLM to hand over to: no default LLM provider is configured.');
    assert.ok(!step(noLLM, 'explain').parts.some((p) => p.id === 'unsure'));
});

test('explanations off: TypeSafe decisions are stored without one, the LLM paths still explain', () => {
    const f = flow({ typesafe: { ...TS, narrative_enabled: false } });
    assert.equal(part(f, 'explain', 'answers').status, 'skip');
    assert.equal(part(f, 'explain', 'answers').detail, 'No explanation; Explain on a result writes one on demand.');
    assert.equal(part(f, 'explain', 'unavailable').status, 'run');
    assert.equal(step(f, 'explain').status, 'run');
});

test('TypeSafe alone with no LLM role sends nothing to the LLM', () => {
    const f = flow({ typesafe: { ...TS, narrative_enabled: false, llm_fallback_enabled: false, escalate_below_pct: 0 } });
    assert.equal(step(f, 'explain').status, 'skip');
    assert.equal(step(f, 'explain').reason, 'No explanation is written; Explain on a result writes one on demand.');
    assert.equal(part(f, 'decide', 'unavailable').detail,
        'If TypeSafe errors or times out, the attempt is recorded as failed and can be retried; failure data never goes to the LLM in its place.');
    assert.ok(!allSends(f).includes('llm'));
    assert.ok(!step(f, 'decide').notes.some((n) => n.includes("default LLM's own settings")));
});

test('deduplicate off analyzes each result and skips the semantic merge', () => {
    const f = flow({ failureAnalysis: { ...FA, dedup_enabled: false } });
    assert.equal(step(f, 'group').detail, 'Each failing result is analyzed on its own.');
    assert.equal(part(f, 'group', 'semantic').reason, 'Needs "Deduplicate similar failures".');
    assert.equal(part(f, 'group', 'cap').title, 'Up to 20 results');
    assert.deepEqual(step(f, 'group').sends, []);
});

test('the semantic merge sends to TypeSafe when it runs', () => {
    const f = flow();
    assert.equal(part(f, 'group', 'semantic').status, 'run');
    assert.deepEqual(step(f, 'group').sends, ['typesafe']);
    assert.equal(part(f, 'group', 'cap').title, 'Up to 20 groups, largest first');
    assert.equal(part(f, 'group', 'parallel').title, '4 at a time');
});

test('redaction off warns and marks everything sent as unredacted', () => {
    const f = flow({ failureAnalysis: { ...FA, redaction_enabled: false } });
    assert.equal(step(f, 'evidence').status, 'warn');
    assert.equal(step(f, 'evidence').reason, 'Redact secrets is off: failure text is sent as recorded.');
    assert.equal(f.unredacted, true);
    assert.equal(flow().unredacted, false);
});

test('an invalid setting shows as invalid on its chip', () => {
    const f = flow({ invalid: ['fa.max_analyses_per_run'] });
    assert.deepEqual(chipOf(f, 'fa.max_analyses_per_run'), { key: 'fa.max_analyses_per_run', label: 'Max analyses per run', value: 'invalid', invalid: true });
});

test('effectiveDraft puts the saved value back in every invalid field', () => {
    const card = { draft: { a: 1, b: NaN }, saved: { a: 0, b: 20 }, errors: { b: 'bad' } };
    assert.deepEqual(effectiveDraft(card), { a: 1, b: 20 });
    assert.equal(effectiveDraft({ status: 'loading' }), null);
});

const ready = (draft, saved = draft, extra = {}) => ({ status: 'ready', draft, saved, dirty: false, errors: {}, ...extra });
const BASE = { aiEnabled: true, aiFeaturesStatus: 'ready', providers: [LLM], providersStatus: 'ready', typesafeCard: ready(TS), faCard: ready(FA), trigger: 'manual' };

test('diagramModel waits for every input and names the ones that failed', () => {
    assert.deepEqual(diagramModel({ ...BASE, faCard: { status: 'loading' } }), { state: 'loading' });
    assert.deepEqual(diagramModel({ ...BASE, aiFeaturesStatus: 'loading' }), { state: 'loading' });
    assert.deepEqual(diagramModel({ ...BASE, providersStatus: 'error', typesafeCard: { status: 'error' } }),
        { state: 'error', failed: ['the LLM providers', 'the TypeSafe.ai settings'] });
});

test('diagramModel draws the valid draft over the saved settings', () => {
    const m = diagramModel({
        ...BASE, providersStatus: 'stale',
        faCard: ready({ ...FA, max_analyses_per_run: NaN, dedup_enabled: false }, FA, { dirty: true, errors: { max_analyses_per_run: 'x' } }),
    });
    assert.equal(m.state, 'ready');
    assert.equal(m.stale, true);
    assert.equal(m.dirty, true);
    assert.equal(chipOf(m.flow, 'fa.max_analyses_per_run').value, 'invalid');
    assert.equal(part(m.flow, 'group', 'cap').title, 'Up to 20 results');
});

test('diagramModel reads the key status from the saved TypeSafe settings', () => {
    const form = { ...TS };
    delete form.api_key_status;
    const m = diagramModel({ ...BASE, typesafeCard: ready(form, { ...TS, api_key_status: 'undecryptable' }) });
    assert.equal(step(m.flow, 'decide').status, 'warn');
});

test('an undecryptable default LLM key leaves the LLM out with the server reason', () => {
    const bad = { ...APPROVED, api_key_status: 'undecryptable' };
    const withTs = resolveRoute({ aiEnabled: true, typesafe: TS, provider: bad, trigger: 'manual' });
    assert.equal(withTs.llm, null);
    assert.equal(withTs.llmReason, "the default LLM provider's stored key can't be decrypted — re-enter it");
    assert.equal(withTs.canAnalyze, true, 'TypeSafe still decides');
    const llmOnly = resolveRoute({ aiEnabled: true, typesafe: { ...TS, enabled: false }, provider: bad, trigger: 'manual' });
    assert.equal(llmOnly.canAnalyze, false);
    const fine = resolveRoute({ aiEnabled: true, typesafe: TS, provider: { ...APPROVED, api_key_status: 'ok' }, trigger: 'manual' });
    assert.equal(fine.llm, 'OpenRouter (minimax)');
});

test('the LLM chips show the call timeout and hedging, with the server defaults when unset', () => {
    const f = flow({ failureAnalysis: { ...FA, llm_call_timeout_seconds: 60, hedge_after_seconds: 10 } });
    assert.equal(chipOf(f, 'fa.llm_call_timeout_seconds').value, '60 s');
    assert.equal(chipOf(f, 'fa.hedge_after_seconds').value, 'After 10 s');
    const defaults = flow();
    assert.equal(chipOf(defaults, 'fa.llm_call_timeout_seconds').value, '45 s');
    assert.equal(chipOf(defaults, 'fa.hedge_after_seconds').value, 'Off');
    assert.equal(chipOf(flow({ provider: null }), 'fa.llm_call_timeout_seconds'), undefined, 'no LLM, no LLM chips');
    const llmOnly = flow({ typesafe: { ...TS, enabled: false } });
    assert.equal(chipOf(llmOnly, 'fa.llm_call_timeout_seconds').value, '45 s', 'shown on the LLM-decides route too');
});

test('the explanation step says the decision is shown first', () => {
    const f = flow();
    assert.ok(step(f, 'explain').notes.some((n) => n.includes('"Explanation being written…"')), JSON.stringify(step(f, 'explain').notes));
    const off = flow({ typesafe: { ...TS, narrative_enabled: false } });
    assert.ok(!step(off, 'explain').notes.some((n) => n.includes('Explanation being written')), 'nothing is written after the decision');
});

test('past triage examples are shown on the evidence step', () => {
    assert.equal(chipOf(flow(), 'fa.few_shot_examples').value, 'Up to 4');
    assert.ok(step(flow(), 'evidence').notes.some((n) => /^Up to 4 past failures/.test(n)));
    const off = flow({ failureAnalysis: { ...FA, few_shot_examples: 0 } });
    assert.equal(chipOf(off, 'fa.few_shot_examples').value, 'Off');
    assert.ok(!step(off, 'evidence').notes.some((n) => /past failures/.test(n)));
    assert.equal(chipOf(flow({ invalid: ['fa.few_shot_examples'] }), 'fa.few_shot_examples').invalid, true);
});
