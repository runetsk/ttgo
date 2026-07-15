import test from 'node:test';
import assert from 'node:assert/strict';
import { decodeEntities } from './decodeEntities.js';

test('decodes the bluemonday entity set', () => {
    assert.equal(decodeEntities('shows &#39;Welcome, Jane&#39; &amp; a link'), "shows 'Welcome, Jane' & a link");
    assert.equal(decodeEntities('count &lt; 5 &gt; 1'), 'count < 5 > 1');
    assert.equal(decodeEntities('say &quot;hi&quot; and &#34;bye&#34;'), 'say "hi" and "bye"');
});

test('decodes exactly one level (double-escaped stays single)', () => {
    assert.equal(decodeEntities('&amp;lt;'), '&lt;');
});

test('passthrough for empty / plain / nullish', () => {
    assert.equal(decodeEntities(''), '');
    assert.equal(decodeEntities('plain text'), 'plain text');
    assert.equal(decodeEntities(null), null);
    assert.equal(decodeEntities(undefined), undefined);
});
