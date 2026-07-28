import test from 'node:test';
import assert from 'node:assert/strict';
import {
    SEVERITY_ORDER,
    STALE_DAYS,
    STATUS_TABS,
    SORT_OPTIONS,
    TRIAGE_TILES,
    deriveStatus,
    isStale,
    ageLabel,
    ownerInitials,
    ownerLabel,
    statusLabel,
    filterDefects,
    sortDefects,
    queueCounts,
    tileHint,
    tilePressed,
} from './defectQueue.js';

const NOW = Date.parse('2026-07-28T12:00:00Z');
const DAY = 24 * 60 * 60 * 1000;
const ago = (days, hours = 0) => new Date(NOW - days * DAY - hours * 60 * 60 * 1000).toISOString();

// A defect shaped like the API's, overridable field by field.
const defect = (over = {}) => ({
    id: 'd1',
    title: 'Checkout hangs on Safari',
    status: 'open',
    severity: 'minor',
    assignee_id: null,
    external_key: '',
    linked_test_count: 0,
    created_at: ago(0),
    updated_at: ago(0),
    ...over,
});

test('deriveStatus: the full stored-status × ownership matrix', () => {
    const cases = [
        [{ status: 'open', assignee_id: null }, 'triage'],
        [{ status: 'open', assignee_id: undefined }, 'triage'],
        [{ status: 'open', assignee_id: '' }, 'triage'],
        [{ status: 'open', assignee_id: 'u1' }, 'progress'],
        [{ status: 'fixed', assignee_id: null }, 'fixed'],
        [{ status: 'fixed', assignee_id: 'u1' }, 'fixed'],
        [{ status: 'closed', assignee_id: null }, 'closed'],
        [{ status: 'closed', assignee_id: 'u1' }, 'closed'],
    ];
    for (const [over, expected] of cases) {
        assert.equal(deriveStatus(defect(over)), expected,
            `${over.status} / ${String(over.assignee_id)} should derive ${expected}`);
    }
});

test('deriveStatus: assignment is the triage action', () => {
    const untriaged = defect({ status: 'open', assignee_id: null });
    assert.equal(deriveStatus(untriaged), 'triage');
    assert.equal(deriveStatus({ ...untriaged, assignee_id: 'u9' }), 'progress');
});

test('deriveStatus: an unknown or missing status buckets as open, never vanishes', () => {
    assert.equal(deriveStatus({ status: 'wontfix' }), 'triage');
    assert.equal(deriveStatus({ status: 'wontfix', assignee_id: 'u1' }), 'progress');
    assert.equal(deriveStatus({}), 'triage');
    assert.equal(deriveStatus(null), 'triage');
});

test('statusLabel names every derived status the same way its tab does', () => {
    const byKey = Object.fromEntries(STATUS_TABS.map(t => [t.key, t.label]));
    for (const status of ['triage', 'progress', 'fixed', 'closed']) {
        assert.equal(statusLabel(status), byKey[status], `${status} pill must read like its tab`);
    }
    // Every status deriveStatus can produce has a label — the pill can never be blank.
    for (const row of [{ status: 'open' }, { status: 'open', assignee_id: 'u1' }, { status: 'fixed' }, { status: 'closed' }, { status: 'wontfix' }]) {
        assert.ok(statusLabel(deriveStatus(row)).length > 0);
    }
});

test('statusLabel prints an unrecognised key rather than nothing', () => {
    assert.equal(statusLabel('nonsense'), 'nonsense');
    assert.equal(statusLabel(''), '');
    assert.equal(statusLabel(undefined), '');
});

test('ownerLabel: unassigned, named, and an assignee whose user row is gone', () => {
    assert.equal(ownerLabel(defect({ assignee_id: null })), 'Unassigned');
    assert.equal(ownerLabel(defect({ assignee_id: '' })), 'Unassigned');
    assert.equal(ownerLabel(defect({ assignee_id: 'u1', assignee_name: 'Mara Reyes' })), 'Mara Reyes');
    // assignee_name is resolved server-side and survives deactivation, so blank
    // means the user is gone — that must not read as "nobody owns this".
    assert.equal(ownerLabel(defect({ assignee_id: 'u1', assignee_name: '' })), 'Unknown user');
    assert.equal(ownerLabel(null), 'Unassigned');
});

test('ownerInitials: names, single words and email fallbacks', () => {
    assert.equal(ownerInitials('Mara Reyes'), 'MR');
    assert.equal(ownerInitials('  mara   reyes  '), 'MR');
    assert.equal(ownerInitials('Mara Jane Reyes'), 'MJ');
    assert.equal(ownerInitials('admin'), 'AD');
    assert.equal(ownerInitials('mara.reyes@example.com'), 'MR');
    assert.equal(ownerInitials('a'), 'A');
});

test('ownerInitials splits an email on @ too, so a one-word local part takes the domain', () => {
    // Not a rule anyone would design, but a consequence of the split: the SAME user
    // reads "AD" with a display name and "AE" when the label falls back to their
    // email. Pinned so the quirk is visible rather than mistaken for intent.
    assert.equal(ownerInitials('admin'), 'AD');
    assert.equal(ownerInitials('admin@example.com'), 'AE');
});

test('ownerInitials returns nothing rather than throwing on empty input', () => {
    assert.equal(ownerInitials(''), '');
    assert.equal(ownerInitials(null), '');
    assert.equal(ownerInitials(undefined), '');
    assert.equal(ownerInitials('   '), '');
    assert.equal(ownerInitials('@.-_'), '');
});

test('isStale: the boundary is inclusive at exactly 7 days', () => {
    assert.equal(STALE_DAYS, 7);
    assert.equal(isStale(defect({ updated_at: ago(STALE_DAYS) }), NOW), true, 'exactly 7d is stale');
    assert.equal(isStale(defect({ updated_at: ago(STALE_DAYS, -1) }), NOW), false, '6d23h is not stale');
    assert.equal(isStale(defect({ updated_at: ago(STALE_DAYS + 1) }), NOW), true);
    assert.equal(isStale(defect({ updated_at: ago(0) }), NOW), false);
});

test('isStale: closed defects are never stale, fixed ones can be', () => {
    const old = { updated_at: ago(30) };
    assert.equal(isStale(defect({ ...old, status: 'closed' }), NOW), false);
    assert.equal(isStale(defect({ ...old, status: 'fixed' }), NOW), true);
    assert.equal(isStale(defect({ ...old, status: 'open', assignee_id: 'u1' }), NOW), true);
});

test('isStale reads updated_at, not created_at', () => {
    const touchedToday = defect({ created_at: ago(90), updated_at: ago(1) });
    assert.equal(isStale(touchedToday, NOW), false);
    const openedTodayButStamped = defect({ created_at: ago(0), updated_at: ago(20) });
    assert.equal(isStale(openedTodayButStamped, NOW), true);
});

test('isStale tolerates a missing or unparseable timestamp', () => {
    assert.equal(isStale(defect({ updated_at: null }), NOW), false);
    assert.equal(isStale(defect({ updated_at: 'not a date' }), NOW), false);
    assert.equal(isStale(null, NOW), false);
});

test('ageLabel walks today → 1 day → N days → N mo', () => {
    assert.equal(ageLabel(defect({ created_at: ago(0) }), NOW), 'today');
    assert.equal(ageLabel(defect({ created_at: ago(0, 5) }), NOW), 'today');
    assert.equal(ageLabel(defect({ created_at: ago(1) }), NOW), '1 day');
    assert.equal(ageLabel(defect({ created_at: ago(2) }), NOW), '2 days');
    assert.equal(ageLabel(defect({ created_at: ago(29) }), NOW), '29 days');
    assert.equal(ageLabel(defect({ created_at: ago(30) }), NOW), '1 mo');
    assert.equal(ageLabel(defect({ created_at: ago(75) }), NOW), '3 mo');
});

test('ageLabel reads created_at and survives bad input', () => {
    assert.equal(ageLabel(defect({ created_at: ago(10), updated_at: ago(0) }), NOW), '10 days');
    assert.equal(ageLabel(defect({ created_at: null }), NOW), '—');
    assert.equal(ageLabel(defect({ created_at: 'nope' }), NOW), '—');
    assert.equal(ageLabel(null, NOW), '—');
    // a clock-skewed future date reads as today rather than "-1 days"
    assert.equal(ageLabel(defect({ created_at: new Date(NOW + 3 * DAY).toISOString() }), NOW), 'today');
});

// Shared corpus for the filter/sort/count tests: one defect per derived status,
// plus a second critical so the tiles have something to sum — and a CLOSED
// critical, which is the row that tells "critical open" apart from "critical".
// Without `f` the Critical tile's land-on-what-you-count assertion passes for the
// wrong reason.
const corpus = () => [
    defect({ id: 'a', title: 'Checkout payment step hangs', severity: 'critical', status: 'open', assignee_id: null, external_key: 'TT-1482', linked_test_count: 4, created_at: ago(1), updated_at: ago(1) }),
    defect({ id: 'b', title: 'Bulk import skips duplicate IDs', severity: 'critical', status: 'open', assignee_id: 'u1', external_key: 'TT-1477', linked_test_count: 3, created_at: ago(3), updated_at: ago(9) }),
    defect({ id: 'c', title: 'Login rate limit returns 500', severity: 'major', status: 'open', assignee_id: 'u2', external_key: '', linked_test_count: 2, created_at: ago(20), updated_at: ago(2) }),
    defect({ id: 'd', title: 'CSV export drops the last row', severity: 'major', status: 'fixed', assignee_id: 'u1', external_key: 'TT-1451', linked_test_count: 1, created_at: ago(12), updated_at: ago(8) }),
    defect({ id: 'e', title: 'Avatar initials wrong', severity: 'trivial', status: 'closed', assignee_id: 'u1', external_key: '', linked_test_count: 0, created_at: ago(40), updated_at: ago(30) }),
    defect({ id: 'f', title: 'Password reset token leaks', severity: 'critical', status: 'closed', assignee_id: 'u2', external_key: 'TT-1400', linked_test_count: 6, created_at: ago(60), updated_at: ago(45) }),
];

const ids = rows => rows.map(r => r.id);

test('filterDefects filters on the DERIVED status, not the stored one', () => {
    const rows = corpus();
    assert.deepEqual(ids(filterDefects(rows, { status: 'all' })), ['a', 'b', 'c', 'd', 'e', 'f']);
    assert.deepEqual(ids(filterDefects(rows, { status: 'triage' })), ['a']);
    assert.deepEqual(ids(filterDefects(rows, { status: 'progress' })), ['b', 'c']);
    assert.deepEqual(ids(filterDefects(rows, { status: 'fixed' })), ['d']);
    assert.deepEqual(ids(filterDefects(rows, { status: 'closed' })), ['e', 'f']);
});

test('filterDefects severity chips are multi-select, empty means all', () => {
    const rows = corpus();
    assert.deepEqual(ids(filterDefects(rows, { severities: [] })), ['a', 'b', 'c', 'd', 'e', 'f']);
    assert.deepEqual(ids(filterDefects(rows, { severities: ['critical'] })), ['a', 'b', 'f']);
    assert.deepEqual(ids(filterDefects(rows, { severities: ['critical', 'trivial'] })), ['a', 'b', 'e', 'f']);
    assert.deepEqual(ids(filterDefects(rows, { severities: ['blocker'] })), []);
});

test('filterDefects query matches title and external key, case- and space-insensitively', () => {
    const rows = corpus();
    assert.deepEqual(ids(filterDefects(rows, { query: 'checkout' })), ['a']);
    assert.deepEqual(ids(filterDefects(rows, { query: '  CHECKOUT  ' })), ['a']);
    assert.deepEqual(ids(filterDefects(rows, { query: 'tt-14' })), ['a', 'b', 'd', 'f']);
    assert.deepEqual(ids(filterDefects(rows, { query: 'TT-1451' })), ['d']);
    assert.deepEqual(ids(filterDefects(rows, { query: 'nothing here' })), []);
    assert.deepEqual(ids(filterDefects(rows, { query: '   ' })), ['a', 'b', 'c', 'd', 'e', 'f']);
});

test('filterDefects openOnly drops closed rows without naming a status', () => {
    const rows = corpus();
    // The dimension the "Critical open" tile needs: severity plus not-closed, which
    // no single status tab can express.
    assert.deepEqual(ids(filterDefects(rows, { openOnly: true })), ['a', 'b', 'c', 'd']);
    assert.deepEqual(ids(filterDefects(rows, { openOnly: true, severities: ['critical'] })), ['a', 'b']);
    assert.deepEqual(ids(filterDefects(rows, { openOnly: false, severities: ['critical'] })), ['a', 'b', 'f']);
    // Only a literal true opts in — an absent option must never narrow the queue.
    assert.deepEqual(ids(filterDefects(rows, { openOnly: 'yes' })), ['a', 'b', 'c', 'd', 'e', 'f']);
});

test('filterDefects stale keeps exactly what isStale keeps, against the pinned clock', () => {
    const rows = corpus();
    assert.deepEqual(ids(filterDefects(rows, { stale: true, now: NOW })), ['b', 'd']);
    assert.deepEqual(ids(rows.filter(r => isStale(r, NOW))), ['b', 'd'], 'the filter and the row badge agree');
    // Closed rows are never stale, so `stale` implies not-closed.
    assert.equal(filterDefects(rows, { stale: true, status: 'closed', now: NOW }).length, 0);
    assert.deepEqual(ids(filterDefects(rows, { stale: false, now: NOW })), ['a', 'b', 'c', 'd', 'e', 'f']);
});

test('filterDefects combines every dimension', () => {
    const rows = corpus();
    assert.deepEqual(ids(filterDefects(rows, { status: 'progress', severities: ['critical'] })), ['b']);
    assert.deepEqual(ids(filterDefects(rows, { status: 'triage', severities: ['major'] })), []);
    assert.deepEqual(ids(filterDefects(rows, { status: 'progress', query: 'import' })), ['b']);
    assert.deepEqual(ids(filterDefects(rows, { stale: true, severities: ['critical'], now: NOW })), ['b']);
    assert.deepEqual(ids(filterDefects(rows, { openOnly: true, query: 'tt-14' })), ['a', 'b', 'd']);
});

test('filterDefects tolerates missing rows and options', () => {
    assert.deepEqual(filterDefects(null, {}), []);
    assert.deepEqual(filterDefects(undefined, undefined), []);
    assert.deepEqual(ids(filterDefects([null, defect({ id: 'a' })], {})), ['a']);
    assert.deepEqual(ids(filterDefects(corpus())), ['a', 'b', 'c', 'd', 'e', 'f']);
});

test('sortDefects priority: worst severity first, then oldest of that severity', () => {
    const rows = corpus();
    // criticals (a=1d, b=3d, f=60d) → oldest first is f; majors (c=20d, d=12d) → c then d
    assert.deepEqual(ids(sortDefects(rows, 'priority')), ['f', 'b', 'a', 'c', 'd', 'e']);
    assert.deepEqual(ids(sortDefects(rows)), ['f', 'b', 'a', 'c', 'd', 'e'], 'priority is the default');
    assert.deepEqual(SEVERITY_ORDER, ['critical', 'major', 'minor', 'trivial']);
});

test('sortDefects priority puts an unknown severity last', () => {
    const rows = [defect({ id: 'x', severity: 'cosmetic' }), defect({ id: 'y', severity: 'trivial' })];
    assert.deepEqual(ids(sortDefects(rows, 'priority')), ['y', 'x']);
});

test('sortDefects updated: most recently touched first', () => {
    assert.deepEqual(ids(sortDefects(corpus(), 'updated')), ['a', 'c', 'd', 'b', 'e', 'f']);
});

test('sortDefects tests: most affected tests first', () => {
    assert.deepEqual(ids(sortDefects(corpus(), 'tests')), ['f', 'a', 'b', 'c', 'd', 'e']);
    const rows = [defect({ id: 'x', linked_test_count: 0 }), defect({ id: 'y', linked_test_count: 9 })];
    assert.deepEqual(ids(sortDefects(rows, 'tests')), ['y', 'x']);
});

test('sortDefects returns a new array and never mutates the input', () => {
    const rows = corpus();
    const before = ids(rows);
    const sorted = sortDefects(rows, 'priority');
    assert.notEqual(sorted, rows);
    assert.deepEqual(ids(rows), before, 'React state must not be reordered in place');
});

test('sortDefects tolerates empty input and unknown sort keys', () => {
    assert.deepEqual(sortDefects(null, 'priority'), []);
    assert.deepEqual(sortDefects([], 'tests'), []);
    assert.deepEqual(ids(sortDefects(corpus(), 'whatever')), ['f', 'b', 'a', 'c', 'd', 'e']);
    assert.deepEqual(ids(sortDefects([null, defect({ id: 'a' })], 'tests')), ['a']);
});

test('queueCounts fills the tabs from the derived statuses', () => {
    const counts = queueCounts(corpus(), NOW);
    assert.deepEqual(counts.tabs, { all: 6, triage: 1, progress: 2, fixed: 1, closed: 2 });
    assert.equal(counts.open, 4, 'everything that is not closed counts as open');
    assert.equal(counts.needsTriage, 1);
    assert.equal(counts.fixed, 1);
});

test('queueCounts tiles: critical open, blocking tests, stale', () => {
    const counts = queueCounts(corpus(), NOW);
    assert.equal(counts.criticalOpen, 2, 'a and b; the CLOSED critical f does not count');
    assert.equal(counts.blockingTests, 7, "4 + 3 linked tests; f's 6 are closed and excluded");
    assert.equal(counts.stale, 2, 'b (9d) and d (8d); e and f are closed so are never stale');
});

test('queueCounts ignores closed defects in every tile', () => {
    const rows = [
        defect({ id: 'a', severity: 'critical', status: 'closed', linked_test_count: 5, updated_at: ago(60) }),
    ];
    const counts = queueCounts(rows, NOW);
    assert.deepEqual(counts.tabs, { all: 1, triage: 0, progress: 0, fixed: 0, closed: 1 });
    assert.equal(counts.open, 0);
    assert.equal(counts.criticalOpen, 0);
    assert.equal(counts.blockingTests, 0);
    assert.equal(counts.stale, 0);
});

test('queueCounts counts the whole list, unaffected by any filter', () => {
    const rows = corpus();
    const filtered = filterDefects(rows, { status: 'triage' });
    assert.equal(filtered.length, 1);
    assert.equal(queueCounts(rows, NOW).tabs.all, 6, 'tiles keep counting the unfiltered set');
});

test('queueCounts handles an empty or missing list', () => {
    for (const empty of [[], null, undefined]) {
        const counts = queueCounts(empty, NOW);
        assert.deepEqual(counts.tabs, { all: 0, triage: 0, progress: 0, fixed: 0, closed: 0 });
        assert.deepEqual(
            { open: counts.open, needsTriage: counts.needsTriage, criticalOpen: counts.criticalOpen, stale: counts.stale, fixed: counts.fixed, blockingTests: counts.blockingTests },
            { open: 0, needsTriage: 0, criticalOpen: 0, stale: 0, fixed: 0, blockingTests: 0 },
        );
    }
});

test('queueCounts sums blocking tests, tolerating a missing linked_test_count', () => {
    const rows = [
        defect({ id: 'a', severity: 'critical', status: 'open', linked_test_count: undefined }),
        defect({ id: 'b', severity: 'critical', status: 'open', linked_test_count: 2 }),
    ];
    assert.equal(queueCounts(rows, NOW).blockingTests, 2);
});

test('the tab and sort option tables match the derived statuses', () => {
    assert.deepEqual(STATUS_TABS.map(t => t.key), ['all', 'triage', 'progress', 'fixed', 'closed']);
    const counts = queueCounts(corpus(), NOW);
    for (const tab of STATUS_TABS) {
        assert.equal(typeof counts.tabs[tab.key], 'number', `${tab.key} needs a count`);
        assert.equal(
            filterDefects(corpus(), { status: tab.key }).length,
            counts.tabs[tab.key],
            `${tab.key} tab count must match what the tab shows`,
        );
    }
    assert.deepEqual(SORT_OPTIONS.map(o => o.value), ['priority', 'updated', 'tests']);
});

test('every triage tile names a real count and a real filter dimension', () => {
    assert.deepEqual(TRIAGE_TILES.map(t => t.key), ['triage', 'critical', 'stale', 'fixed']);
    const rows = corpus();
    const counts = queueCounts(rows, NOW);
    const tabKeys = STATUS_TABS.map(t => t.key);
    const sortValues = SORT_OPTIONS.map(o => o.value);

    for (const tile of TRIAGE_TILES) {
        assert.equal(typeof counts[tile.countKey], 'number', `${tile.key} must read a real queueCounts field`);
        assert.ok(tabKeys.includes(tile.filters.status), `${tile.key} must select a real status tab`);
        for (const sev of tile.filters.severities) {
            assert.ok(SEVERITY_ORDER.includes(sev), `${tile.key} must select a real severity chip`);
        }
        if (tile.filters.sort) {
            assert.ok(sortValues.includes(tile.filters.sort), `${tile.key} must select a real sort`);
        }
    }
});

// THE tile invariant, asserted for all four: a tile is a filter preset, so
// clicking one must leave exactly the rows it just counted on screen. Anything
// looser makes the number on the tile a lie the moment it is acted on.
test('every triage tile lands on exactly the rows it counts', () => {
    const rows = corpus();
    const counts = queueCounts(rows, NOW);

    for (const tile of TRIAGE_TILES) {
        const landed = filterDefects(rows, { ...tile.filters, now: NOW });
        assert.equal(landed.length, counts[tile.countKey],
            `${tile.key} says ${counts[tile.countKey]} and lands on ${landed.length}`);
        assert.ok(counts[tile.countKey] > 0, `${tile.key} needs a non-zero count to be a real assertion`);
    }
});

test('the tile presets pick out the right rows, not merely the right number', () => {
    const rows = corpus();
    const byKey = Object.fromEntries(TRIAGE_TILES.map(t => [t.key, t]));
    const landed = key => ids(filterDefects(rows, { ...byKey[key].filters, now: NOW }));

    assert.deepEqual(landed('triage'), ['a']);
    assert.deepEqual(landed('critical'), ['a', 'b'], 'the closed critical f stays out');
    assert.deepEqual(landed('stale'), ['b', 'd']);
    assert.deepEqual(landed('fixed'), ['d']);
});

test('only Needs triage is toned', () => {
    assert.deepEqual(TRIAGE_TILES.filter(t => t.tone).map(t => t.key), ['triage']);
});

test('a tile reads as pressed only when its whole preset is the view in force', () => {
    const byKey = Object.fromEntries(TRIAGE_TILES.map(t => [t.key, t]));
    const view = tile => ({ status: 'all', severities: [], ...tile.filters });

    for (const tile of TRIAGE_TILES) {
        assert.equal(tilePressed(tile, view(tile)), true, `${tile.key} must be pressed in its own view`);
    }
    // The neutral view presses nothing — this is what the Stale tile used to fail:
    // its preset was byte-identical to a fresh page load, so it was a button that
    // changed nothing and could not honestly claim a pressed state.
    const neutral = { status: 'all', severities: [], openOnly: false, stale: false };
    for (const tile of TRIAGE_TILES) {
        assert.equal(tilePressed(tile, neutral), false, `${tile.key} must not be pressed in the neutral view`);
    }
    // Every dimension counts, including the two the filter bar cannot reach.
    assert.equal(tilePressed(byKey.critical, { status: 'all', severities: ['critical'] }), false,
        'same severities but openOnly off is a WIDER view than the tile');
    assert.equal(tilePressed(byKey.stale, { status: 'all', severities: [], stale: true }), true);
    assert.equal(tilePressed(byKey.triage, { status: 'triage', severities: ['major'] }), false);
    assert.equal(tilePressed(null, neutral), false);
    assert.equal(tilePressed(byKey.triage, undefined), false);
});

test('tileHint names the tile, and Critical open reports what it blocks', () => {
    const counts = queueCounts(corpus(), NOW);
    const byKey = Object.fromEntries(TRIAGE_TILES.map(t => [t.key, t]));

    assert.equal(tileHint(byKey.critical, counts), 'blocking 7 tests');
    assert.equal(tileHint(byKey.critical, { blockingTests: 1 }), 'blocking 1 test');
    assert.equal(tileHint(byKey.critical, {}), 'blocking 0 tests');
    assert.equal(tileHint(byKey.stale, counts), 'no update in 7 days');
    // Every tile says something — a blank line under a count reads as a missing value.
    for (const tile of TRIAGE_TILES) {
        assert.ok(tileHint(tile, counts).length > 0, `${tile.key} needs a hint`);
    }
    assert.equal(tileHint(null, counts), '');
});
