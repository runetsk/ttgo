import test from 'node:test';
import assert from 'node:assert/strict';
import { secretNote, secretPlaceholder, canClearSecret, secretPayload } from './secretStatus.js';

test('secretNote says what is stored and flags a secret that cannot be decrypted', () => {
    assert.equal(secretNote('missing', ''), null);
    assert.equal(secretNote(undefined, ''), null);
    assert.deepEqual(secretNote('ok', '****1234'), { tone: 'muted', text: 'Current: ****1234 — leave blank to keep' });
    assert.deepEqual(secretNote('ok', '', 'token'), { tone: 'muted', text: 'Token stored — leave blank to keep' });
    assert.deepEqual(secretNote('undecryptable', '', 'token'), { tone: 'red', text: 'Stored token cannot be decrypted — enter it again' });
    assert.deepEqual(secretNote('undecryptable', ''), { tone: 'red', text: 'Stored key cannot be decrypted — enter it again' });
});

test('secretPlaceholder only offers "leave blank" for a usable secret', () => {
    assert.equal(secretPlaceholder('ok', 'sk-…'), 'Leave blank to keep the current key');
    assert.equal(secretPlaceholder('ok', 'x', 'token'), 'Leave blank to keep the current token');
    assert.equal(secretPlaceholder('undecryptable', 'sk-…'), 'sk-…');
    assert.equal(secretPlaceholder('missing', 'sk-…'), 'sk-…');
});

test('canClearSecret: any stored secret, readable or not, can be removed', () => {
    assert.equal(canClearSecret('ok'), true);
    assert.equal(canClearSecret('undecryptable'), true);
    assert.equal(canClearSecret('missing'), false);
    assert.equal(canClearSecret(undefined), false);
});

test('secretPayload sends a typed value, omits a blank one and lets clear win', () => {
    assert.deepEqual(secretPayload('api_key', 'clear_api_key', '  sk-1 ', false), { api_key: 'sk-1' });
    assert.deepEqual(secretPayload('api_key', 'clear_api_key', '', false), {});
    assert.deepEqual(secretPayload('api_key', 'clear_api_key', '   ', false), {});
    assert.deepEqual(secretPayload('api_token', 'clear_api_token', 'typed', true), { clear_api_token: true });
});
