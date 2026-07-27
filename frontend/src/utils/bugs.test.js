import test from 'node:test';
import assert from 'node:assert/strict';
import { activeBugs, bugHref } from './bugs.js';

test('activeBugs drops closed defects and keeps everything else', () => {
    const rows = [
        { id: 'a', status: 'open' },
        { id: 'b', status: 'closed' },
        { id: 'c', status: 'open' },
    ];
    assert.deepEqual(activeBugs(rows).map(d => d.id), ['a', 'c']);
    assert.deepEqual(activeBugs(null), []);
    assert.deepEqual(activeBugs([]), []);
    assert.deepEqual(activeBugs([null, undefined, { id: 'd', status: 'open' }]).map(d => d.id), ['d']);
});

test('activeBugs keeps fixed defects — they still await retest', () => {
    const rows = [
        { id: 'fixed', status: 'fixed' },
        { id: 'closed', status: 'closed' },
    ];
    assert.deepEqual(activeBugs(rows).map(d => d.id), ['fixed']);
});

test('activeBugs keeps an unrecognised status rather than dropping it', () => {
    // Mirrors deriveStatus() and the backend counters: closed vs. everything else.
    const rows = [{ id: 'weird', status: 'wontfix' }, { id: 'blank' }];
    assert.deepEqual(activeBugs(rows).map(d => d.id), ['weird', 'blank']);
});

test('bugHref points external bugs at their tracker, native bugs at the register', () => {
    assert.deepEqual(
        bugHref({ id: 'x', external_url: 'https://jira.example/PROJ-1' }),
        { href: 'https://jira.example/PROJ-1', external: true },
    );
    assert.deepEqual(
        bugHref({ id: 'x' }),
        { href: '/defects?focus=x', external: false },
    );
});
