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

// filterDefects applies the three filter dimensions the page owns. `status` is a
// derived status (or 'all'), `severities` is a multi-select where empty means all,
// and `query` matches the title or the external key — the two things a defect is
// searched by in practice.
export function filterDefects(rows, options) {
    const opts = options || {};
    const status = opts.status || 'all';
    const severities = opts.severities || [];
    const query = String(opts.query || '').trim().toLowerCase();

    return (rows || []).filter(defect => {
        if (!defect) return false;
        if (status !== 'all' && deriveStatus(defect) !== status) return false;
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
