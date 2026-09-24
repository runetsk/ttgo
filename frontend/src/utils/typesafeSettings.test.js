import test from 'node:test';
import assert from 'node:assert/strict';
import { formFromSettings, buildTypeSafePatch, keyStatusLabel, canTestConnection, escalationValid, escalationNote, defaultProvider, describeRoute, typesafeStatus, verdictDependencyNote } from './typesafeSettings.js';

const settings = {
    enabled: false, api_key_masked: '', api_key_status: 'missing', model: 'jev-1.13.0', timeout_seconds: 30,
    verdict_engine_enabled: true, narrative_enabled: true, escalate_below_pct: 0, semantic_dedup_enabled: true, allow_auto_failure_analysis: false,
};

test('escalation threshold is a whole percentage from 0 to 100 and travels in the patch', () => {
    for (const ok of [0, 1, 90, 100]) assert.equal(escalationValid(ok), true, String(ok));
    for (const bad of [-1, 101, 12.5, NaN, '90', null]) assert.equal(escalationValid(bad), false, String(bad));
    const form = { ...formFromSettings(settings), escalate_below_pct: 90 };
    assert.deepEqual(buildTypeSafePatch(form, settings), { escalate_below_pct: 90 });
});

test('escalationNote says what the threshold does', () => {
    assert.match(escalationNote(0), /^Off/);
    assert.match(escalationNote(90), /less than 90% confidence are decided by your default LLM/);
    assert.match(escalationNote(101), /whole number/);
});

test('formFromSettings copies fields and starts with an empty key input', () => {
    const form = formFromSettings(settings);
    assert.equal(form.api_key, '');
    assert.equal(form.clear_api_key, false);
    assert.equal(form.model, 'jev-1.13.0');
    assert.equal(form.timeout_seconds, 30);
});

test('buildTypeSafePatch sends only changed fields and omits a blank key', () => {
    const form = { ...formFromSettings(settings), enabled: true, model: 'jev-1.13.0', timeout_seconds: 45 };
    assert.deepEqual(buildTypeSafePatch(form, settings), { enabled: true, timeout_seconds: 45 });
});

test('buildTypeSafePatch includes a typed key and never both key and clear', () => {
    const typed = { ...formFromSettings(settings), api_key: 'ts-new' };
    assert.deepEqual(buildTypeSafePatch(typed, settings), { api_key: 'ts-new' });
    const cleared = { ...formFromSettings(settings), clear_api_key: true, api_key: 'ignored' };
    assert.deepEqual(buildTypeSafePatch(cleared, settings), { clear_api_key: true });
});

test('buildTypeSafePatch returns null when nothing changed', () => {
    assert.equal(buildTypeSafePatch(formFromSettings(settings), settings), null);
});

test('buildTypeSafePatch carries the explanation switch like the other toggles', () => {
    const form = { ...formFromSettings(settings), narrative_enabled: false };
    assert.equal(form.narrative_enabled, false);
    assert.deepEqual(buildTypeSafePatch(form, settings), { narrative_enabled: false });
});

test('keyStatusLabel explains each status', () => {
    assert.equal(keyStatusLabel(settings), 'No API key stored');
    assert.equal(keyStatusLabel({ ...settings, api_key_status: 'ok', api_key_masked: '…4321' }), 'Key stored (…4321)');
    assert.equal(keyStatusLabel({ ...settings, api_key_status: 'undecryptable' }), 'Stored key cannot be decrypted — enter it again');
});

test('canTestConnection allows testing the stored key when the input is untouched', () => {
    const stored = { ...settings, api_key_status: 'ok', api_key_masked: '…4321' };
    assert.equal(canTestConnection(formFromSettings(stored), stored), true);
});

test('canTestConnection is false with no stored key', () => {
    assert.equal(canTestConnection(formFromSettings(settings), settings), false);
});

test('canTestConnection is false while a new unsaved key is typed', () => {
    const stored = { ...settings, api_key_status: 'ok', api_key_masked: '…4321' };
    const form = { ...formFromSettings(stored), api_key: 'ts-new' };
    assert.equal(canTestConnection(form, stored), false);
});

test('canTestConnection is false while clear_api_key is pending', () => {
    const stored = { ...settings, api_key_status: 'ok', api_key_masked: '…4321' };
    const form = { ...formFromSettings(stored), clear_api_key: true };
    assert.equal(canTestConnection(form, stored), false);
});

test('the LLM fallback switch travels in the patch', () => {
    const base = { ...settings, llm_fallback_enabled: true };
    const form = { ...formFromSettings(base), llm_fallback_enabled: false };
    assert.deepEqual(buildTypeSafePatch(form, base), { llm_fallback_enabled: false });
});

test('defaultProvider picks the enabled default', () => {
    assert.equal(defaultProvider([{ id: 1, is_default: true, enabled: false }, { id: 2, is_default: false, enabled: true }]), null);
    assert.equal(defaultProvider([{ id: 1, is_default: true, enabled: true }]).id, 1);
    assert.equal(defaultProvider(undefined), null);
});

const provider = { label: 'OpenRouter', model_name: 'minimax', is_default: true, enabled: true, allow_auto_failure_analysis: false };
const route = { enabled: true, model: 'jev-1.13.0', verdict_engine_enabled: true, narrative_enabled: true, escalate_below_pct: 0, llm_fallback_enabled: true, allow_auto_failure_analysis: true };

test('describeRoute: TypeSafe decides, the LLM explains and stands by', () => {
    const r = describeRoute(route, true, provider);
    assert.deepEqual(r.manual, [
        'TypeSafe (jev-1.13.0) decides.',
        'OpenRouter (minimax) writes the explanation.',
        'If TypeSafe is unavailable, OpenRouter (minimax) decides.',
    ]);
    assert.match(r.auto[1], /No explanation: the default LLM provider is not approved for automatic analysis/);
});

test('describeRoute: TypeSafe only sends nothing to the LLM', () => {
    const r = describeRoute({ ...route, narrative_enabled: false, llm_fallback_enabled: false }, true, null);
    assert.equal(r.manual.at(-1), 'No failure data is sent to the LLM.');
    assert.match(r.manual[1], /Explain/);
    assert.match(r.manual[2], /recorded as failed/);
});

test('describeRoute: takeover threshold and missing key', () => {
    assert.match(describeRoute({ ...route, escalate_below_pct: 90 }, true, provider).manual[1], /Below 90% confidence OpenRouter \(minimax\) decides instead/);
    assert.deepEqual(describeRoute({ ...route, enabled: false }, false, provider).manual, ['OpenRouter (minimax) decides and explains in one call.']);
    assert.match(describeRoute({ ...route, enabled: false }, true, null).manual[0], /^Nothing can analyze: no default LLM provider is set and TypeSafe is off/);
    assert.match(describeRoute({ ...route, allow_auto_failure_analysis: false }, true, provider).auto[0], /^Nothing can analyze: the default LLM provider is not approved/);
});

test('describeRoute names a provider once when its label is its model', () => {
    const same = { ...provider, label: 'minimax/minimax-m3', model_name: 'minimax/minimax-m3' };
    assert.equal(describeRoute(route, true, same).manual[1], 'minimax/minimax-m3 writes the explanation.');
});

test('describeRoute: TypeSafe selected without a usable key is unavailable, so the fallback switch decides', () => {
    assert.deepEqual(describeRoute(route, false, provider).manual,
        ['TypeSafe has no usable API key, so OpenRouter (minimax) decides and explains in one call as its fallback.']);
    const off = describeRoute({ ...route, llm_fallback_enabled: false }, false, provider).manual;
    assert.equal(off.length, 1);
    assert.match(off[0], /recorded as failed until the key is saved again/);
    assert.match(off[0], /No failure data is sent to the LLM/);
    assert.match(describeRoute(route, false, null).manual[0], /^Every attempt fails: TypeSafe has no usable API key and no default LLM provider is set/);
    // Automatic analysis without TypeSafe's consent is a configured LLM route, key or not.
    assert.match(describeRoute({ ...route, allow_auto_failure_analysis: false }, false, { ...provider, allow_auto_failure_analysis: true }).auto[0],
        /decides and explains in one call\.$/);
});

test('typesafeStatus summarises the saved settings for the header', () => {
    assert.deepEqual(typesafeStatus({ enabled: false, api_key_status: 'ok' }), { label: 'Off', tone: 'muted' });
    assert.deepEqual(typesafeStatus({ enabled: true, api_key_status: 'ok' }), { label: 'On', tone: 'green' });
    assert.deepEqual(typesafeStatus({ enabled: true, api_key_status: 'missing' }), { label: 'No API key', tone: 'amber' });
    assert.deepEqual(typesafeStatus({ enabled: true, api_key_status: 'undecryptable' }), { label: 'Key unreadable', tone: 'red' });
    assert.deepEqual(typesafeStatus(null), { label: 'Off', tone: 'muted' });
});

test('verdictDependencyNote only speaks when verdicts are off', () => {
    assert.equal(verdictDependencyNote({ verdict_engine_enabled: true }), null);
    assert.match(verdictDependencyNote({ verdict_engine_enabled: false }), /Use for failure verdicts/);
});
