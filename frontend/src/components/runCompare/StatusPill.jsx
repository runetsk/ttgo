import React from 'react';

const STYLES = {
    PASS:    { bg: 'rgba(34,197,94,0.12)',   fg: 'var(--accent-green)' },
    FAIL:    { bg: 'rgba(239,68,68,0.12)',   fg: 'var(--accent-red)' },
    ERROR:   { bg: 'rgba(239,68,68,0.12)',   fg: 'var(--accent-red)' },
    SKIP:    { bg: 'rgba(156,163,175,0.12)', fg: '#9ca3af' },
    PENDING: { bg: 'rgba(234,179,8,0.12)',   fg: 'var(--warning-color)' },
};
const LABELS = { PASS: 'Pass', FAIL: 'Fail', ERROR: 'Error', SKIP: 'Skip', PENDING: 'Pending' };

export default function StatusPill({ status }) {
    if (!status) return <span style={{ color: 'var(--text-secondary)' }}>—</span>;
    const s = STYLES[status] || STYLES.SKIP;
    return (
        <span style={{ background: s.bg, color: s.fg, padding: '2px 8px', borderRadius: 6, fontSize: '0.72rem', fontWeight: 600, whiteSpace: 'nowrap' }}>
            {LABELS[status] || status}
        </span>
    );
}
