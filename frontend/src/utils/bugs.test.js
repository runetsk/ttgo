import test from 'node:test';
import assert from 'node:assert/strict';
import { activeBugs, bugHref, bugStatusConfig, countOpen } from './bugs.js';

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

test('countOpen is activeBugs by the numbers, so the summary strip cannot disagree', () => {
    const rows = [
        { id: 'a', status: 'open' },
        { id: 'b', status: 'fixed' },
        { id: 'c', status: 'closed' },
        { id: 'd', status: 'wontfix' },
    ];
    assert.equal(countOpen(rows), 3, 'fixed and an unknown status both count as outstanding');
    assert.equal(countOpen(rows), activeBugs(rows).length);
    assert.equal(rows.length - countOpen(rows), 1, 'and the remainder is exactly the closed ones');
    assert.equal(countOpen([]), 0);
    assert.equal(countOpen(null), 0);
});

test('bugStatusConfig labels fixed as Fixed, never as Open', () => {
    // The bug this table shipped with: no `fixed` entry, so the fallback badged a
    // fixed defect "Open".
    assert.equal(bugStatusConfig('fixed').label, 'Fixed');
    assert.equal(bugStatusConfig('open').label, 'Open');
    assert.equal(bugStatusConfig('closed').label, 'Closed');
    // Anything unrecognised badges as Open, matching activeBugs' "not closed" rule.
    for (const unknown of ['wontfix', '', undefined, null]) {
        assert.equal(bugStatusConfig(unknown).label, 'Open');
    }
    // Every entry is renderable — the badge reads all three fields.
    for (const status of ['open', 'fixed', 'closed']) {
        const cfg = bugStatusConfig(status);
        assert.ok(cfg.color && cfg.icon && cfg.label, `${status} needs a full config`);
    }
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
