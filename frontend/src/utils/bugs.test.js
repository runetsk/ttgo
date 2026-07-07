import test from 'node:test';
import assert from 'node:assert/strict';
import { activeBugs, bugHref } from './bugs.js';

test('activeBugs keeps only open defects', () => {
    const rows = [
        { id: 'a', status: 'open' },
        { id: 'b', status: 'closed' },
        { id: 'c', status: 'open' },
    ];
    assert.deepEqual(activeBugs(rows).map(d => d.id), ['a', 'c']);
    assert.deepEqual(activeBugs(null), []);
    assert.deepEqual(activeBugs([]), []);
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
