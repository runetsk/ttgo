import test from 'node:test';
import assert from 'node:assert/strict';
import { formFromSettings, buildTypeSafePatch, keyStatusLabel, canTestConnection } from './typesafeSettings.js';

const settings = {
    enabled: false, api_key_masked: '', api_key_status: 'missing', model: 'jev-1.13.0', timeout_seconds: 30,
    verdict_engine_enabled: true, semantic_dedup_enabled: true, allow_auto_failure_analysis: false,
};

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
