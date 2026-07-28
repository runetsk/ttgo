// Derivation layer for the Defects triage queue: everything the page shows is
// computed from the raw defect list in memory, so filtering, sorting, search and
// the tile counts never cost a round-trip. Kept DOM-free and dependency-free
// (like utils/bugs.js) because this repo has no component-render harness — a
// pure helper is the only testable seam for these decisions.

// Severity, worst first. Doubles as the rank used by the "priority" sort and as
// the chip order in the filter bar.
export const SEVERITY_ORDER = ['critical', 'major', 'minor', 'trivial'];

// The four derived statuses plus "all", in tab order. `key` matches deriveStatus().
export const STATUS_TABS = [
    { key: 'all', label: 'All' },
    { key: 'triage', label: 'Needs triage' },
    { key: 'progress', label: 'In progress' },
    { key: 'fixed', label: 'Fixed' },
    { key: 'closed', label: 'Closed' },
];

export const SORT_OPTIONS = [
    { value: 'priority', label: 'Severity, then oldest' },
    { value: 'updated', label: 'Recently updated' },
    { value: 'tests', label: 'Tests affected' },
];

// The three STORED statuses, in workflow order, with the labels the edit modal
// offers. Deliberately not STATUS_TABS: those are the four DERIVED buckets the
// queue filters by, and "Needs triage" vs "In progress" is not something anyone
// picks here — it follows from whether the defect has an owner. Kept beside
// STATUS_TABS so the two can be cross-checked against deriveStatus in one place.
export const DEFECT_STATUS_OPTIONS = [
    { value: 'open', label: 'Open' },
    { value: 'fixed', label: 'Fixed (awaiting retest)' },
    { value: 'closed', label: 'Closed' },
];

// The four triage tiles, in strip order. Each one doubles as a filter preset —
// clicking a tile is how you enter that queue — so the presets live here beside
// the dimensions they set, for the same reason STATUS_TABS does: a tile can
// never name a status deriveStatus does not produce, or a severity that is not
// a real chip.
//
// `countKey` names a queueCounts() field. `tone` is set on the one tile that
// means "act now"; the rest are neutral surfaces.
//
// Every preset lands on EXACTLY the rows its count describes — that invariant is
// asserted per tile in defectQueue.test.js. It is why filterDefects carries the
// `openOnly` and `stale` dimensions: "critical open" counts criticals that are
// not closed and "stale" counts neglect, so without those two axes those tiles
// would open a wider view than the number on their face.
export const TRIAGE_TILES = [
    { key: 'triage', label: 'Needs triage', countKey: 'needsTriage', tone: 'alert', filters: { status: 'triage', severities: [] } },
    { key: 'critical', label: 'Critical open', countKey: 'criticalOpen', filters: { status: 'all', severities: ['critical'], openOnly: true } },
    { key: 'stale', label: 'Stale · 7d+', countKey: 'stale', filters: { status: 'all', severities: [], stale: true, sort: 'priority' } },
    { key: 'fixed', label: 'Fixed, awaiting retest', countKey: 'fixed', filters: { status: 'fixed', severities: [] } },
];

// Hint text under each tile's count. Only "Critical open" is dynamic: it reports
// how much work the criticals are blocking, which is the reason to look at them
// first. Lives here rather than in TriageStrip.jsx so it is testable — this repo
// has no component-render harness.
const TILE_HINTS = {
    triage: 'unassigned · no owner yet',
    stale: 'no update in 7 days',
    fixed: 'ready to verify',
};

export function tileHint(tile, counts) {
    if (!tile) return '';
    if (tile.key !== 'critical') return TILE_HINTS[tile.key] || '';
    const blocked = (counts && counts.blockingTests) || 0;
    return `blocking ${blocked} test${blocked === 1 ? '' : 's'}`;
}

// tilePressed answers the aria-pressed a tile renders: true when the filters it
// sets are exactly the ones in force. Every dimension a preset can carry is
// compared, so a tile can never read as pressed while the table shows something
// wider than the tile's own count.
export function tilePressed(tile, view) {
    if (!tile) return false;
    const want = tile.filters;
    const state = view || {};
    if ((want.status || 'all') !== (state.status || 'all')) return false;
    if (Boolean(want.stale) !== Boolean(state.stale)) return false;
    if (Boolean(want.openOnly) !== Boolean(state.openOnly)) return false;
    const have = state.severities || [];
    return want.severities.length === have.length && want.severities.every(sev => have.includes(sev));
}

// A defect nobody has touched in this many days is stale. Inclusive: exactly 7
// days counts, which is what the "Stale · 7d+" tile promises.
export const STALE_DAYS = 7;

const DAY_MS = 24 * 60 * 60 * 1000;
const SEVERITY_RANK = SEVERITY_ORDER.reduce((acc, s, i) => { acc[s] = i; return acc; }, {});

// Accepts a Date, an epoch-ms number or an ISO string; falls back to the wall
// clock. Every export takes `now` so tests can pin it.
function toMillis(now) {
    if (now instanceof Date) return now.getTime();
    if (typeof now === 'number' && Number.isFinite(now)) return now;
    if (typeof now === 'string') {
        const parsed = Date.parse(now);
        if (!Number.isNaN(parsed)) return parsed;
    }
    return Date.now();
}

// Epoch ms for an ISO timestamp; 0 when missing or unparseable, so a row with no
// timestamp sorts as the oldest thing on the page rather than throwing.
function timestamp(iso) {
    if (!iso) return 0;
    const parsed = Date.parse(iso);
    return Number.isNaN(parsed) ? 0 : parsed;
}

// Whole days elapsed since `iso`, or null when there is no usable timestamp.
// Clamped at 0 so a clock-skewed future date reads as "today", not "-1 days".
function elapsedDays(iso, nowMs) {
    if (!iso) return null;
    const parsed = Date.parse(iso);
    if (Number.isNaN(parsed)) return null;
    return Math.max(0, Math.floor((nowMs - parsed) / DAY_MS));
}

// deriveStatus maps the three stored statuses onto the four the queue displays.
//
// The backend stores open | fixed | closed; "needs triage" and "in progress" are
// both stored `open` and split on ownership, because assignment IS the triage
// action here. Anything unrecognised is treated as open — the same "closed vs.
// everything else" bucketing the backend counters use — so an unknown value can
// never quietly disappear from the queue.
export function deriveStatus(defect) {
    const status = defect && defect.status;
    if (status === 'closed') return 'closed';
    if (status === 'fixed') return 'fixed';
    return defect && defect.assignee_id ? 'progress' : 'triage';
}

// statusLabel names a derived status for the row pill. It reads STATUS_TABS
// rather than a second table, so the pill and the tab that filters to it can
// never disagree about what a bucket is called. An unrecognised key prints
// itself — deriveStatus cannot produce one, so this only ever guards a caller.
const STATUS_LABELS = STATUS_TABS.reduce((acc, tab) => { acc[tab.key] = tab.label; return acc; }, {});

export function statusLabel(status) {
    return STATUS_LABELS[status] || String(status || '');
}

// ownerLabel is what the Owner cell prints. assignee_name is resolved server-side
// (display name, else email) and survives deactivation, so a defect that has an
// assignee_id but no name has lost its user row entirely — say so rather than
// printing an empty cell that reads as unassigned.
export function ownerLabel(defect) {
    if (!defect || !defect.assignee_id) return 'Unassigned';
    return defect.assignee_name || 'Unknown user';
}

// ownerInitials is the avatar's two characters: one per word for a real name,
// the first two for a single word. Emails split on their punctuation, so
// "mara.reyes@example.com" reads MR like the display name would.
export function ownerInitials(name) {
    const words = String(name || '').trim().split(/[\s@._-]+/).filter(Boolean);
    if (words.length === 0) return '';
    const joined = words.length > 1 ? words[0][0] + words[1][0] : words[0].slice(0, 2);
    return joined.toUpperCase();
}

// isStale is about neglect, not age: nothing has happened to this defect in
// STALE_DAYS, and it is not closed (a closed defect is supposed to sit still).
export function isStale(defect, now) {
    if (!defect || deriveStatus(defect) === 'closed') return false;
    const days = elapsedDays(defect.updated_at, toMillis(now));
    return days !== null && days >= STALE_DAYS;
}

// ageLabel is about age, not neglect: time since the defect was created.
export function ageLabel(defect, now) {
    const days = elapsedDays(defect && defect.created_at, toMillis(now));
    if (days === null) return '—';
    if (days === 0) return 'today';
    if (days === 1) return '1 day';
    if (days < 30) return `${days} days`;
    return `${Math.round(days / 30)} mo`;
}

// filterDefects applies the filter dimensions the page owns. `status` is a derived
// status (or 'all'), `severities` is a multi-select where empty means all, and
// `query` matches the title or the external key — the two things a defect is
// searched by in practice.
//
// `openOnly` and `stale` are the two the filter bar has no control for: they exist
// because the triage tiles count "critical and not closed" and "nothing has
// happened here in a week", and a tile that opened a wider view than its own count
// would be lying. Only a tile sets them, and touching a tab or a chip clears them
// again (DefectsPage.afterFilterChange), so they can never linger invisibly.
// `now` is threaded through for `stale`, so tests can pin the clock the same way
// queueCounts lets them.
export function filterDefects(rows, options) {
    const opts = options || {};
    const status = opts.status || 'all';
    const severities = opts.severities || [];
    const query = String(opts.query || '').trim().toLowerCase();
    const openOnly = opts.openOnly === true;
    const staleOnly = opts.stale === true;
    const nowMs = toMillis(opts.now);

    return (rows || []).filter(defect => {
        if (!defect) return false;
        const derived = deriveStatus(defect);
        if (status !== 'all' && derived !== status) return false;
        if (openOnly && derived === 'closed') return false;
        if (staleOnly && !isStale(defect, nowMs)) return false;
        if (severities.length > 0 && !severities.includes(defect.severity)) return false;
        if (!query) return true;
        return String(defect.title || '').toLowerCase().includes(query)
            || String(defect.external_key || '').toLowerCase().includes(query);
    });
}

function severityRank(severity) {
    return severity in SEVERITY_RANK ? SEVERITY_RANK[severity] : SEVERITY_ORDER.length;
}

// sortDefects returns a new array; it never mutates the caller's rows, which are
// React state. Ties keep their incoming order (Array#sort is stable), so the
// server's created_at DESC ordering survives as the final tiebreak.
export function sortDefects(rows, sort) {
    const copy = (rows || []).filter(Boolean).slice();
    if (sort === 'updated') {
        copy.sort((a, b) => timestamp(b.updated_at) - timestamp(a.updated_at));
    } else if (sort === 'tests') {
        copy.sort((a, b) => (b.linked_test_count || 0) - (a.linked_test_count || 0));
    } else {
        // priority: worst severity first, then the oldest of that severity — the
        // one that has been ignored longest is the one to look at.
        copy.sort((a, b) => (severityRank(a.severity) - severityRank(b.severity))
            || (timestamp(a.created_at) - timestamp(b.created_at)));
    }
    return copy;
}

// queueCounts summarises the WHOLE list, not the filtered view: the tabs and
// tiles have to keep saying how much work exists while the table shows a slice.
//
// `blockingTests` sums linked_test_count across critical open defects. It is a
// sum, not a distinct count — the list carries no test ids — so one test blocked
// by two critical defects is counted twice. That over-counts in the direction of
// attention, which is the right way for a triage tile to be wrong.
export function queueCounts(rows, now) {
    const nowMs = toMillis(now);
    const list = (rows || []).filter(Boolean);
    const tabs = { all: list.length, triage: 0, progress: 0, fixed: 0, closed: 0 };
    let criticalOpen = 0;
    let stale = 0;
    let blockingTests = 0;

    for (const defect of list) {
        const status = deriveStatus(defect);
        tabs[status] += 1;
        if (status === 'closed') continue;
        if (defect.severity === 'critical') {
            criticalOpen += 1;
            blockingTests += defect.linked_test_count || 0;
        }
        if (isStale(defect, nowMs)) stale += 1;
    }

    return {
        tabs,
        open: list.length - tabs.closed,
        needsTriage: tabs.triage,
        criticalOpen,
        stale,
        fixed: tabs.fixed,
        blockingTests,
    };
}
