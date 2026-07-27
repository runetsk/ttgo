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

// Where "open this bug" should point. External bugs (with an external_url)
// open their tracker in a new tab; native bugs deep-link into the Defects
// register, which focuses that row (DefectsPage reads ?focus=).
export function bugHref(defect) {
    if (defect && defect.external_url) {
        return { href: defect.external_url, external: true };
    }
    return { href: `/defects?focus=${defect ? defect.id : ''}`, external: false };
}
