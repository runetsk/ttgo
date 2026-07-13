import test from 'node:test';
import assert from 'node:assert/strict';
import { presetFromConfig, presetMeta, PROVIDER_GROUPS } from './constants.js';

test('presetFromConfig maps direct provider types', () => {
    assert.equal(presetFromConfig({ provider_type: 'anthropic', endpoint_url: 'https://api.anthropic.com' }).key, 'anthropic');
    assert.equal(presetFromConfig({ provider_type: 'local', endpoint_url: 'http://localhost:11434' }).key, 'local');
    assert.equal(presetFromConfig({ provider_type: 'gemini', endpoint_url: 'https://generativelanguage.googleapis.com/v1beta/openai' }).key, 'gemini');
});

test('presetFromConfig distinguishes openai-compatible endpoints', () => {
    assert.equal(presetFromConfig({ provider_type: 'openai', endpoint_url: 'https://api.openai.com' }).key, 'openai');
    assert.equal(presetFromConfig({ provider_type: 'openai', endpoint_url: '' }).key, 'openai');
    assert.equal(presetFromConfig({ provider_type: 'openai', endpoint_url: 'https://openrouter.ai/api' }).key, 'openrouter');
    assert.equal(presetFromConfig({ provider_type: 'openai', endpoint_url: 'https://api.together.xyz' }).key, 'custom');
});

test('presetFromConfig tolerates missing fields', () => {
    assert.equal(presetFromConfig().key, 'openai');
    assert.equal(presetFromConfig({}).key, 'openai');
});

test('presetMeta returns the preset, or defaults to openai for unknown keys', () => {
    assert.equal(presetMeta('openrouter').endpoint, 'https://openrouter.ai/api');
    assert.equal(presetMeta('nonexistent').key, 'openai');
});

test('OpenRouter preset endpoint has no trailing /v1', () => {
    assert.equal(presetMeta('openrouter').endpoint, 'https://openrouter.ai/api');
    assert.ok(!presetMeta('openrouter').endpoint.endsWith('/v1'));
});

test('every preset carries the fields the UI needs', () => {
    for (const g of PROVIDER_GROUPS) {
        for (const p of g.presets) {
            assert.ok(p.key && p.label && p.providerType, `preset ${p.key} has key/label/providerType`);
            assert.equal(typeof p.endpoint, 'string', `preset ${p.key} endpoint is a string`);
            assert.ok(p.initial, `preset ${p.key} has an initial`);
        }
    }
});
