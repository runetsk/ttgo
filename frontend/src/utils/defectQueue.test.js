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
    assert.equal(ownerInitials('admin@example.com'), 'AE');
    assert.equal(ownerInitials('a'), 'A');
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
// plus a second critical so the tiles have something to sum.
const corpus = () => [
    defect({ id: 'a', title: 'Checkout payment step hangs', severity: 'critical', status: 'open', assignee_id: null, external_key: 'TT-1482', linked_test_count: 4, created_at: ago(1), updated_at: ago(1) }),
    defect({ id: 'b', title: 'Bulk import skips duplicate IDs', severity: 'critical', status: 'open', assignee_id: 'u1', external_key: 'TT-1477', linked_test_count: 3, created_at: ago(3), updated_at: ago(9) }),
    defect({ id: 'c', title: 'Login rate limit returns 500', severity: 'major', status: 'open', assignee_id: 'u2', external_key: '', linked_test_count: 2, created_at: ago(20), updated_at: ago(2) }),
    defect({ id: 'd', title: 'CSV export drops the last row', severity: 'major', status: 'fixed', assignee_id: 'u1', external_key: 'TT-1451', linked_test_count: 1, created_at: ago(12), updated_at: ago(8) }),
    defect({ id: 'e', title: 'Avatar initials wrong', severity: 'trivial', status: 'closed', assignee_id: 'u1', external_key: '', linked_test_count: 0, created_at: ago(40), updated_at: ago(30) }),
];

const ids = rows => rows.map(r => r.id);

test('filterDefects filters on the DERIVED status, not the stored one', () => {
    const rows = corpus();
    assert.deepEqual(ids(filterDefects(rows, { status: 'all' })), ['a', 'b', 'c', 'd', 'e']);
    assert.deepEqual(ids(filterDefects(rows, { status: 'triage' })), ['a']);
    assert.deepEqual(ids(filterDefects(rows, { status: 'progress' })), ['b', 'c']);
    assert.deepEqual(ids(filterDefects(rows, { status: 'fixed' })), ['d']);
    assert.deepEqual(ids(filterDefects(rows, { status: 'closed' })), ['e']);
});

test('filterDefects severity chips are multi-select, empty means all', () => {
    const rows = corpus();
    assert.deepEqual(ids(filterDefects(rows, { severities: [] })), ['a', 'b', 'c', 'd', 'e']);
    assert.deepEqual(ids(filterDefects(rows, { severities: ['critical'] })), ['a', 'b']);
    assert.deepEqual(ids(filterDefects(rows, { severities: ['critical', 'trivial'] })), ['a', 'b', 'e']);
    assert.deepEqual(ids(filterDefects(rows, { severities: ['blocker'] })), []);
});

test('filterDefects query matches title and external key, case- and space-insensitively', () => {
    const rows = corpus();
    assert.deepEqual(ids(filterDefects(rows, { query: 'checkout' })), ['a']);
    assert.deepEqual(ids(filterDefects(rows, { query: '  CHECKOUT  ' })), ['a']);
    assert.deepEqual(ids(filterDefects(rows, { query: 'tt-14' })), ['a', 'b', 'd']);
    assert.deepEqual(ids(filterDefects(rows, { query: 'TT-1451' })), ['d']);
    assert.deepEqual(ids(filterDefects(rows, { query: 'nothing here' })), []);
    assert.deepEqual(ids(filterDefects(rows, { query: '   ' })), ['a', 'b', 'c', 'd', 'e']);
});

test('filterDefects combines the three dimensions', () => {
    const rows = corpus();
    assert.deepEqual(ids(filterDefects(rows, { status: 'progress', severities: ['critical'] })), ['b']);
    assert.deepEqual(ids(filterDefects(rows, { status: 'triage', severities: ['major'] })), []);
    assert.deepEqual(ids(filterDefects(rows, { status: 'progress', query: 'import' })), ['b']);
});

test('filterDefects tolerates missing rows and options', () => {
    assert.deepEqual(filterDefects(null, {}), []);
    assert.deepEqual(filterDefects(undefined, undefined), []);
    assert.deepEqual(ids(filterDefects([null, defect({ id: 'a' })], {})), ['a']);
    assert.deepEqual(ids(filterDefects(corpus())), ['a', 'b', 'c', 'd', 'e']);
});

test('sortDefects priority: worst severity first, then oldest of that severity', () => {
    const rows = corpus();
    // criticals (a=1d, b=3d) → oldest first is b; majors (c=20d, d=12d) → c then d
    assert.deepEqual(ids(sortDefects(rows, 'priority')), ['b', 'a', 'c', 'd', 'e']);
    assert.deepEqual(ids(sortDefects(rows)), ['b', 'a', 'c', 'd', 'e'], 'priority is the default');
    assert.deepEqual(SEVERITY_ORDER, ['critical', 'major', 'minor', 'trivial']);
});

test('sortDefects priority puts an unknown severity last', () => {
    const rows = [defect({ id: 'x', severity: 'cosmetic' }), defect({ id: 'y', severity: 'trivial' })];
    assert.deepEqual(ids(sortDefects(rows, 'priority')), ['y', 'x']);
});

test('sortDefects updated: most recently touched first', () => {
    assert.deepEqual(ids(sortDefects(corpus(), 'updated')), ['a', 'c', 'd', 'b', 'e']);
});

test('sortDefects tests: most affected tests first', () => {
    assert.deepEqual(ids(sortDefects(corpus(), 'tests')), ['a', 'b', 'c', 'd', 'e']);
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
    assert.deepEqual(ids(sortDefects(corpus(), 'whatever')), ['b', 'a', 'c', 'd', 'e']);
    assert.deepEqual(ids(sortDefects([null, defect({ id: 'a' })], 'tests')), ['a']);
});

test('queueCounts fills the tabs from the derived statuses', () => {
    const counts = queueCounts(corpus(), NOW);
    assert.deepEqual(counts.tabs, { all: 5, triage: 1, progress: 2, fixed: 1, closed: 1 });
    assert.equal(counts.open, 4, 'everything that is not closed counts as open');
    assert.equal(counts.needsTriage, 1);
    assert.equal(counts.fixed, 1);
});

test('queueCounts tiles: critical open, blocking tests, stale', () => {
    const counts = queueCounts(corpus(), NOW);
    assert.equal(counts.criticalOpen, 2, 'a and b; the closed trivial does not count');
    assert.equal(counts.blockingTests, 7, '4 + 3 linked tests across the critical open defects');
    assert.equal(counts.stale, 2, 'b (9d) and d (8d); e is closed so it is never stale');
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
    assert.equal(queueCounts(rows, NOW).tabs.all, 5, 'tiles keep counting the unfiltered set');
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

test('the Needs triage and Fixed tiles land on exactly the rows they count', () => {
    const rows = corpus();
    const counts = queueCounts(rows, NOW);
    const byKey = Object.fromEntries(TRIAGE_TILES.map(t => [t.key, t]));

    // Critical open and Stale are deliberately looser than their number: neither
    // "not closed" nor "not updated in 7 days" is a filter dimension, so those
    // two tiles open a wider view than the count they advertise.
    assert.equal(filterDefects(rows, byKey.triage.filters).length, counts.needsTriage);
    assert.equal(filterDefects(rows, byKey.fixed.filters).length, counts.fixed);
});

test('only Needs triage is toned, and only Stale has no pressed state', () => {
    assert.deepEqual(TRIAGE_TILES.filter(t => t.tone).map(t => t.key), ['triage']);
    assert.deepEqual(TRIAGE_TILES.filter(t => t.pressable === false).map(t => t.key), ['stale']);
});
