// Bug (defect) helpers shared by the Defects register and the test-case
// Active Bugs panel. Kept DOM-free so they're unit-testable.

export const SEVERITY_COLORS = {
    critical: '#ef4444',
    major: '#f97316',
    minor: '#eab308',
    trivial: '#6b7280',
};

// Inline-style object for a severity "chip" (pill); neutral grey fallback.
export function severityChipStyle(severity) {
    const c = SEVERITY_COLORS[severity] || '#94a3b8';
    return {
        background: c + '22',
        color: c,
        border: `1px solid ${c}55`,
        padding: '1px 9px',
        borderRadius: 99,
        fontSize: '0.74rem',
        fontWeight: 600,
        textTransform: 'capitalize',
    };
}

// Keep only active bugs. "Active" is everything that is not closed — the same
// "closed vs. everything else" bucketing the backend counters use — so `fixed`
// (awaiting retest) stays visible, and an unrecognised status can never quietly
// drop a live defect off the panel.
export function activeBugs(defects) {
    return (defects || []).filter(d => d && d.status !== 'closed');
}

// countOpen is activeBugs' number: every stored status except `closed`. Kept beside
// it so the analytics summary strip and the panels can never disagree about what
// "open" means.
export function countOpen(defects) {
    return activeBugs(defects).length;
}

// How a stored defect status is badged. `fixed` needs its own entry: without it the
// fallback labels a fixed defect "Open", which is a lie the reader can act on — and
// an unrecognised status falls back to the same badge, which is the honest direction
// (it is not closed, so it is outstanding).
const BUG_STATUS_CONFIG = {
    closed: { label: 'Closed', color: '#22c55e', icon: '●' },
    fixed: { label: 'Fixed', color: '#6366f1', icon: '◐' },
    open: { label: 'Open', color: '#f97316', icon: '○' },
};

export function bugStatusConfig(status) {
    return BUG_STATUS_CONFIG[status] || BUG_STATUS_CONFIG.open;
}

// Where "open this bug" should point. External bugs (with an external_url)
// open their tracker in a new tab; native bugs deep-link into the Defects
// register, which focuses that row (DefectsPage reads ?focus=).
export function bugHref(defect) {
    if (defect && defect.external_url) {
        return { href: defect.external_url, external: true };
    }
    return { href: `/defects?focus=${defect ? defect.id : ''}`, external: false };
}
