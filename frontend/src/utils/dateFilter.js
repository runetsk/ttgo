// Pure date helpers for column date-range filters. (No unit runner in this repo;
// these are exercised via Playwright E2E in Tasks 3-5.)

function toISODate(d) {
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return `${y}-${m}-${day}`;
}

/** Returns {from, to} ISO date strings for a named preset (local calendar). */
export function presetRange(preset) {
    const now = new Date();
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    switch (preset) {
        case 'today':
            return { from: toISODate(today), to: toISODate(today) };
        case 'last7': {
            const from = new Date(today);
            from.setDate(from.getDate() - 6);
            return { from: toISODate(from), to: toISODate(today) };
        }
        case 'last30': {
            const from = new Date(today);
            from.setDate(from.getDate() - 29);
            return { from: toISODate(from), to: toISODate(today) };
        }
        case 'thisMonth': {
            const from = new Date(now.getFullYear(), now.getMonth(), 1);
            return { from: toISODate(from), to: toISODate(today) };
        }
        default:
            return { from: null, to: null };
    }
}

/** Inclusive day-granular membership test. Empty range → true (no filter). */
export function inDateRange(iso, range) {
    const from = range?.from || null;
    const to = range?.to || null;
    if (!from && !to) return true;
    if (!iso || iso.startsWith('0001')) return false;
    const d = new Date(iso);
    if (isNaN(d.getTime())) return false;
    const day = toISODate(d); // local-calendar YYYY-MM-DD
    if (from && day < from) return false;
    if (to && day > to) return false;
    return true;
}

/** True when a range has at least one bound set. */
export function hasDateRange(range) {
    return !!(range && (range.from || range.to));
}
