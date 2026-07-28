import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

// The Defects register is themed entirely through custom properties: a hard-coded
// colour looks fine in whichever theme it was written against and unreadable in the
// other, and a var() naming a property nobody declares silently falls back to
// nothing (this codebase has already shipped --accent-purple and --surface-bg that
// way). Neither failure shows up in a build, a lint or an e2e run, so it is checked
// here — the stylesheet is just text, and node:test can read it.
//
// Scope is the .defects-* block, which is the whole redesigned page. Everything
// before it predates this and is not in scope.

const read = rel => readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8');

const appCss = read('../App.css');
const indexCss = read('../index.css');

// The block runs from the first `.defects-` rule to the end of the file.
const blockStart = appCss.indexOf('.defects-');
const defectsBlock = appCss.slice(blockStart);

test('the defects stylesheet block was actually found', () => {
    assert.ok(blockStart > 0, 'no .defects- rule in App.css — this test would pass vacuously');
    assert.ok(defectsBlock.includes('.defects-page'), 'the page shell rule must be inside the block');
    assert.ok(defectsBlock.length > 5000, 'the block looks truncated');
});

test('no hard-coded colour anywhere in the defects block', () => {
    // Strip comments first: they discuss colours in prose ("red / amber / purple").
    const code = defectsBlock.replace(/\/\*[\s\S]*?\*\//g, '');
    const offenders = [];
    for (const [i, line] of code.split('\n').entries()) {
        if (/#[0-9a-fA-F]{3,8}\b/.test(line)) offenders.push(`hex on line ${i + 1}: ${line.trim()}`);
        // rgb()/rgba()/hsl() are the other way to write a literal. Shadows and
        // scrims elsewhere in the app use them, but the defects block must not:
        // every tone it needs already exists as a token.
        if (/\b(rgba?|hsla?)\(/.test(line)) offenders.push(`literal colour on line ${i + 1}: ${line.trim()}`);
    }
    assert.deepEqual(offenders, [], `the defects block must be token-only:\n${offenders.join('\n')}`);
});

test('every custom property the defects block reads is declared in index.css', () => {
    // Declared globally…
    const declared = new Set([...indexCss.matchAll(/(--[\w-]+)\s*:/g)].map(m => m[1]));
    // …or by the block itself (--defects-tone is set per severity/status modifier).
    for (const m of defectsBlock.matchAll(/(--[\w-]+)\s*:/g)) declared.add(m[1]);

    // Reads WITH a fallback are checked too — `var(--x, 44px)` as well as `var(--x)`.
    // A fallback stops the property resolving to nothing, but it does not make the read
    // correct: `var(--surface-bg, white)` is exactly the light-theme failure this file's
    // header cites, and it would sail past the hex/rgb test above as well, since `white`
    // is neither. So the exemption is by NAME, not by syntax: --defects-tone is already
    // covered (the block declares it per severity/status modifier), which leaves the one
    // property below that really is set from JavaScript.
    const JS_SET = [
        // DefectsPage sets it inline from the bulk bar's measured height; the fallback
        // is what stands until the first measurement arrives. No stylesheet declares it.
        '--defects-bulkbar-h',
    ];
    const missing = new Set();
    for (const m of defectsBlock.matchAll(/var\(\s*(--[\w-]+)/g)) {
        if (!declared.has(m[1]) && !JS_SET.includes(m[1])) missing.add(m[1]);
    }
    assert.deepEqual([...missing], [],
        'undefined custom properties break one theme silently — with no fallback they resolve to '
        + `nothing, with one they pin a single theme's value: ${[...missing].join(', ')}`);
});

test('both themes declare every token the defects block reads', () => {
    // index.css declares the dark values on :root and overrides on [data-theme='light'].
    // A token declared only under one of the two is a token that is missing in the other.
    const lightStart = indexCss.indexOf("[data-theme='light']");
    assert.ok(lightStart > 0, 'no light-theme block in index.css');
    const rootScope = indexCss.slice(0, lightStart);
    const lightScope = indexCss.slice(lightStart);

    const inRoot = new Set([...rootScope.matchAll(/(--[\w-]+)\s*:/g)].map(m => m[1]));
    const inLight = new Set([...lightScope.matchAll(/(--[\w-]+)\s*:/g)].map(m => m[1]));
    const localToBlock = new Set([...defectsBlock.matchAll(/(--[\w-]+)\s*:/g)].map(m => m[1]));

    const used = new Set([...defectsBlock.matchAll(/var\(\s*(--[\w-]+)/g)].map(m => m[1]));
    const darkOnly = [...used].filter(t => !localToBlock.has(t) && inRoot.has(t) && !inLight.has(t));

    // Tokens that are genuinely theme-neutral — geometry, and the brand indigo that
    // is deliberately the same colour on both backgrounds — legitimately live only on
    // :root. Asserting against a reviewed list rather than emptiness keeps the check
    // meaningful: adding a token here should be a decision, not a reflex.
    const NEUTRAL = [
        '--radius-sm', '--radius-md', '--radius-lg', '--radius-xl',
        '--shadow-sm', '--transition', '--accent-indigo',
    ];
    const unexpected = darkOnly.filter(t => !NEUTRAL.includes(t));
    assert.deepEqual(unexpected, [],
        `these carry a dark-theme value with no light override: ${unexpected.join(', ')}`);
});
