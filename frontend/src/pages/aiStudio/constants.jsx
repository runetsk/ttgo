// Theme-aware color tokens. Values reference the CSS variables defined in
// index.css so the Studio follows the global light/dark theme toggle.
export const AIC = {
    bg1: 'var(--bg-primary)',
    bg2: 'var(--bg-secondary)',
    bg3: 'var(--bg-tertiary)',
    text: 'var(--text-primary)',
    dim: 'var(--text-secondary)',
    muted: 'var(--sidebar-muted)',
    border: 'var(--border-color)',
    border2: 'var(--glass-border)',
    surfaceTint: 'var(--aig-surface-tint)',
    surfaceSunken: 'var(--aig-surface-sunken)',
    indigo: '#6366f1',
    indigoSoft: 'var(--aig-indigo-soft)',
    teal: '#14b8a6',
    tealSoft: '#5eead4',
    green: '#22c55e',
    red: '#ef4444',
    amber: '#eab308',
};

export const MONO = "'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, monospace";

// ── Icons ────────────────────────────────────────────────────────────────────
export const Icon = {
    sparkles: (s = 14) => (
        <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M12 3v4"/><path d="M12 17v4"/><path d="M3 12h4"/><path d="M17 12h4"/><path d="M5.6 5.6l2.8 2.8"/><path d="M15.6 15.6l2.8 2.8"/><path d="M5.6 18.4l2.8-2.8"/><path d="M15.6 8.4l2.8-2.8"/></svg>
    ),
    bolt: (s = 14) => (
        <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>
    ),
    check: (s = 14) => (
        <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polyline points="20 6 9 17 4 12"/></svg>
    ),
    x: (s = 14) => (
        <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
    ),
    chevronR: (s = 14) => (
        <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polyline points="9 18 15 12 9 6"/></svg>
    ),
    edit: (s = 14) => (
        <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M12 20h9"/><path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4z"/></svg>
    ),
    copy: (s = 14) => (
        <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
    ),
    dots: (s = 14) => (
        <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/><circle cx="5" cy="12" r="1"/></svg>
    ),
    plus: (s = 14) => (
        <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
    ),
    alert: (s = 14) => (
        <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
    ),
    history: (s = 14) => (
        <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M3 3v5h5"/><path d="M3.05 13a9 9 0 1 0 .5-4"/><path d="M12 7v5l3 2"/></svg>
    ),
};

// ── Category ordering + colors ───────────────────────────────────────────────
export const CATEGORY_ORDER = [
    'Critical', 'Functional', 'Regression', 'Negative', 'Boundary', 'Edge Case',
    'Security', 'Performance', 'API', 'Mobile/Responsive', 'Accessibility', 'UI',
];

export function categoryTone(cat) {
    const key = (cat || '').toLowerCase();
    if (key.includes('critical') || key.includes('negative')) return { bg: 'rgba(239,68,68,0.12)', bd: 'rgba(239,68,68,0.35)', fg: 'var(--aig-tone-red-fg)' };
    if (key.includes('regression') || key.includes('functional')) return { bg: 'rgba(99,102,241,0.12)', bd: 'rgba(99,102,241,0.35)', fg: 'var(--aig-tone-indigo-fg)' };
    if (key.includes('security')) return { bg: 'rgba(234,179,8,0.12)', bd: 'rgba(234,179,8,0.35)', fg: 'var(--aig-tone-amber-fg)' };
    if (key.includes('access')) return { bg: 'rgba(20,184,166,0.12)', bd: 'rgba(20,184,166,0.35)', fg: 'var(--aig-tone-teal-fg)' };
    if (key.includes('perf')) return { bg: 'rgba(34,197,94,0.12)', bd: 'rgba(34,197,94,0.35)', fg: 'var(--aig-tone-green-fg)' };
    if (key.includes('ui') || key.includes('mobile')) return { bg: 'rgba(168,85,247,0.12)', bd: 'rgba(168,85,247,0.35)', fg: 'var(--aig-tone-purple-fg)' };
    if (key.includes('edge') || key.includes('boundary')) return { bg: 'rgba(234,179,8,0.12)', bd: 'rgba(234,179,8,0.35)', fg: 'var(--aig-tone-amber-fg)' };
    return { bg: 'var(--aig-surface-tint-strong)', bd: AIC.border, fg: AIC.dim };
}

export function groupDrafts(drafts, by, linkedReqId) {
    if (by === 'none') return [{ key: 'all', label: 'All drafts', items: drafts }];
    const map = new Map();
    const push = (k, label, d) => {
        if (!map.has(k)) map.set(k, { key: k, label, items: [] });
        map.get(k).items.push(d);
    };
    drafts.forEach(d => {
        if (by === 'category') {
            const c = (d.category || '').trim() || 'Uncategorized';
            push(c.toLowerCase(), c, d);
        } else if (by === 'requirement') {
            push(linkedReqId || '__none', linkedReqId || 'No requirement', d);
        }
    });
    const arr = Array.from(map.values());
    const orderLower = CATEGORY_ORDER.map(x => x.toLowerCase());
    if (by === 'category') {
        arr.sort((a, b) => {
            const ai = orderLower.indexOf(a.label.toLowerCase());
            const bi = orderLower.indexOf(b.label.toLowerCase());
            if (ai === -1 && bi === -1) return a.label.localeCompare(b.label);
            if (ai === -1) return 1;
            if (bi === -1) return -1;
            return ai - bi;
        });
    }
    return arr;
}

export function stripHtmlForDisplay(html) {
    if (!html) return '';
    return String(html)
        .replace(/<style[\s\S]*?<\/style>/gi, '')
        .replace(/<script[\s\S]*?<\/script>/gi, '')
        .replace(/<\/(p|div|br|li|tr|h[1-6])>/gi, '\n')
        .replace(/<br\s*\/?>/gi, '\n')
        .replace(/<[^>]+>/g, '')
        .replace(/&nbsp;/g, ' ')
        .replace(/&amp;/g, '&')
        .replace(/&lt;/g, '<')
        .replace(/&gt;/g, '>')
        .replace(/&quot;/g, '"')
        .replace(/&#39;/g, "'")
        .replace(/\n{3,}/g, '\n\n')
        .trim();
}

// ── Width constraints for resizable panes ───────────────────────────────────
export const LEFT_MIN = 220, LEFT_MAX = 520, LEFT_DEFAULT = 300;
export const RIGHT_MIN = 260, RIGHT_MAX = 560, RIGHT_DEFAULT = 380;

export function clamp(v, lo, hi) { return Math.max(lo, Math.min(hi, v)); }

export function readStoredWidth(key, def, lo, hi) {
    try {
        const v = parseInt(localStorage.getItem(key) || '', 10);
        return isNaN(v) ? def : clamp(v, lo, hi);
    } catch { return def; }
}

// ── Page shell styles (for header/tabs/empty state/import modal/etc.) ───────
export const pageStyles = {
    root: { padding: '24px 28px', minHeight: '100%' },
    container: { maxWidth: 860, margin: '0 auto' },
    studioWrap: { margin: '4px -28px 0', '--aig-studio-h': 'calc(100vh - 220px)' },
    studioOnlyWrap: { '--aig-studio-h': 'calc(100vh - 64px)' },

    pageHeader: {
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        gap: 16, marginBottom: 16,
    },
    aiIcon: {
        width: 38, height: 38, borderRadius: 10,
        background: 'linear-gradient(135deg, rgba(99,102,241,0.2), rgba(20,184,166,0.15))',
        border: '1px solid rgba(99,102,241,0.25)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        color: 'var(--aig-indigo-strong)', flexShrink: 0,
    },
    pageTitle: {
        margin: 0, fontSize: '1.15rem', fontWeight: 700,
        letterSpacing: '-0.01em', color: 'var(--text-primary)',
    },
    headerSub: {
        margin: '2px 0 0', fontSize: '0.82rem',
        color: 'var(--text-secondary)', fontWeight: 400,
    },
    reqBadge: {
        fontSize: '0.72rem', fontWeight: 700, color: 'var(--aig-indigo-strong)',
        background: 'var(--aig-accent-soft-bg)', border: '1px solid var(--aig-accent-soft-border)',
        padding: '1px 7px', borderRadius: 4, letterSpacing: '0.02em', flexShrink: 0,
    },
    reqTitle: {
        fontSize: '0.85rem', color: 'var(--text-secondary)',
        overflow: 'hidden', whiteSpace: 'nowrap', textOverflow: 'ellipsis',
    },
    backLink: {
        display: 'flex', alignItems: 'center', gap: 6,
        color: 'var(--text-secondary)', textDecoration: 'none',
        fontSize: '0.85rem', fontWeight: 500, flexShrink: 0,
        transition: 'color 0.15s',
    },
    accentBar: {
        height: 3, borderRadius: 2,
        background: 'linear-gradient(90deg, #6366f1 0%, #14b8a6 100%)',
        marginBottom: 14,
    },

    tabRow: {
        display: 'flex', gap: 4, marginBottom: 16, padding: '3px',
        borderRadius: 8, background: 'var(--bg-secondary, rgba(0,0,0,0.06))',
        width: 'fit-content',
    },
    tabBtn: {
        display: 'flex', alignItems: 'center', gap: 6,
        padding: '7px 16px', borderRadius: 6,
        fontSize: '0.82rem', fontWeight: 500,
        background: 'transparent', color: 'var(--text-secondary)',
        border: 'none', cursor: 'pointer', fontFamily: 'inherit',
        transition: 'all 0.15s',
    },
    tabBtnActive: {
        background: 'var(--bg-primary, #fff)',
        color: 'var(--text-primary)',
        boxShadow: '0 1px 3px rgba(0,0,0,0.08)',
    },
    tabBadge: {
        fontSize: '0.68rem', fontWeight: 600,
        padding: '1px 6px', borderRadius: 8,
        background: 'var(--accent, #6366f1)', color: '#fff', marginLeft: 2,
    },

    emptyGrid: {
        display: 'grid', gridTemplateColumns: '1.15fr 1fr',
        gap: 16, alignItems: 'start',
    },
    emptyPickerCard: {
        borderRadius: 12, background: 'var(--bg-secondary)',
        border: '1px solid var(--border-color)',
        padding: '18px 20px',
        transition: 'border-color 0.2s, box-shadow 0.2s',
    },
    emptyImportLink: {
        background: 'none', border: 'none', padding: 0, cursor: 'pointer',
        color: 'var(--text-secondary)', fontSize: '0.8rem',
        textDecoration: 'none',
    },
    emptyPathCard: {
        borderRadius: 12, background: 'var(--bg-secondary)',
        border: '1px solid var(--border-color)', overflow: 'hidden',
        transition: 'border-color 0.2s, box-shadow 0.2s',
    },
    emptyPathHead: {
        display: 'flex', alignItems: 'center', gap: 12,
        padding: '16px 18px 0',
    },
    emptyPathIconWrap: {
        width: 36, height: 36, borderRadius: 9,
        background: 'linear-gradient(135deg, rgba(99,102,241,0.2), rgba(99,102,241,0.1))',
        border: '1px solid rgba(99,102,241,0.25)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        color: 'var(--aig-indigo-strong)', flexShrink: 0,
    },
    emptyPathTitle: {
        margin: 0, fontSize: '1rem', fontWeight: 700,
        color: 'var(--text-primary)',
    },
    emptyPathDesc: {
        margin: '2px 0 0', fontSize: '0.78rem',
        color: 'var(--text-secondary)', lineHeight: 1.4,
    },
    emptyPathBody: { padding: '14px 18px 18px' },
    emptyPathLabel: {
        display: 'block', marginBottom: 8,
        fontSize: '0.75rem', fontWeight: 600,
        color: 'var(--text-secondary)',
        textTransform: 'uppercase', letterSpacing: '0.05em',
    },
    emptyLoading: {
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        gap: 10, padding: '20px 0',
    },

    emptySearchWrap: { position: 'relative', marginBottom: 8 },
    emptySearchIcon: {
        position: 'absolute', left: 10, top: '50%',
        transform: 'translateY(-50%)',
        color: 'var(--text-secondary)', pointerEvents: 'none',
    },
    emptySearchInput: {
        width: '100%', padding: '8px 32px',
        borderRadius: 7, border: '1px solid var(--border-color)',
        background: 'var(--bg-primary)', color: 'var(--text-primary)',
        fontSize: '0.875rem', fontFamily: 'inherit', outline: 'none',
        boxSizing: 'border-box', transition: 'border-color 0.15s',
    },
    emptySearchClear: {
        position: 'absolute', right: 8, top: '50%', transform: 'translateY(-50%)',
        background: 'none', border: 'none', cursor: 'pointer',
        color: 'var(--text-secondary)', fontSize: '0.75rem', padding: '2px 4px',
        borderRadius: 3,
    },
    emptyReqList: {
        maxHeight: 240, overflowY: 'auto',
        borderRadius: 8, border: '1px solid var(--border-color)',
        background: 'var(--bg-primary)', marginBottom: 10,
    },
    emptyReqRow: {
        width: '100%', display: 'flex', alignItems: 'center', gap: 8,
        padding: '8px 12px', border: 'none',
        borderBottom: '1px solid var(--border-color)',
        background: 'none', cursor: 'pointer', fontFamily: 'inherit',
        textAlign: 'left', transition: 'background 0.1s',
    },
    emptyReqBadge: {
        fontSize: '0.7rem', fontWeight: 700, color: 'var(--aig-indigo-strong)',
        background: 'var(--aig-accent-soft-bg)', border: '1px solid var(--aig-accent-soft-border)',
        padding: '1px 6px', borderRadius: 4, flexShrink: 0, letterSpacing: '0.02em',
    },
    emptyReqTitle: {
        fontSize: '0.875rem', color: 'var(--text-primary)',
        flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
    },
    emptyReqArrow: {
        color: 'var(--text-secondary)', flexShrink: 0, opacity: 0.4,
    },
    emptyReqNone: {
        padding: '14px 12px', fontSize: '0.83rem',
        color: 'var(--text-secondary)', textAlign: 'center',
    },
    emptyFooterRow: {
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        paddingTop: 8,
    },
    emptyCreateBtn: {
        display: 'inline-flex', alignItems: 'center', gap: 6,
        padding: '5px 12px', borderRadius: 6,
        border: '1px solid var(--aig-accent-soft-border)',
        background: 'var(--aig-accent-soft-bg)',
        color: 'var(--aig-indigo-strong)', fontSize: '0.82rem', fontWeight: 600,
        cursor: 'pointer', fontFamily: 'inherit',
        transition: 'all 0.15s',
    },
    emptyCreateBtnPrimary: {
        display: 'inline-flex', alignItems: 'center', gap: 6,
        padding: '8px 18px', borderRadius: 7, border: 'none',
        background: 'linear-gradient(135deg, #4f46e5 0%, #6366f1 100%)',
        color: '#fff', fontSize: '0.875rem', fontWeight: 600,
        cursor: 'pointer', fontFamily: 'inherit',
        boxShadow: '0 4px 14px rgba(99,102,241,0.35)',
        transition: 'all 0.15s',
    },

    importFeatureList: {
        display: 'flex', flexDirection: 'column', gap: 12, marginBottom: 18,
    },
    importFeature: {
        display: 'flex', alignItems: 'flex-start', gap: 10,
    },
    importFeatureIcon: {
        width: 28, height: 28, borderRadius: 7,
        background: 'rgba(20,184,166,0.08)',
        border: '1px solid rgba(20,184,166,0.15)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        color: '#14b8a6', flexShrink: 0, marginTop: 1,
    },
    importFeatureTitle: {
        display: 'block', fontSize: '0.84rem', fontWeight: 600,
        color: 'var(--text-primary)', lineHeight: 1.3,
    },
    importFeatureDesc: {
        display: 'block', fontSize: '0.76rem',
        color: 'var(--text-secondary)', lineHeight: 1.4, marginTop: 1,
    },
    importCtaBtn: {
        width: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center',
        gap: 8, padding: '10px 18px', borderRadius: 8,
        border: '1px solid rgba(20,184,166,0.35)',
        background: 'rgba(20,184,166,0.08)',
        color: '#14b8a6', fontSize: '0.86rem', fontWeight: 600,
        cursor: 'pointer', fontFamily: 'inherit',
        transition: 'all 0.18s',
    },

    fieldLabel: {
        display: 'block', marginBottom: 6,
        fontSize: '0.78rem', color: 'var(--text-secondary)', fontWeight: 500,
    },
    optionalTag: {
        fontSize: '0.7rem', fontWeight: 500,
        color: 'var(--text-secondary)',
        background: 'var(--aig-surface-tint-strong)',
        border: '1px solid var(--border-color)',
        padding: '0 6px', borderRadius: 4,
        textTransform: 'none', letterSpacing: '0.01em', marginLeft: 2,
    },

    importModalBackdrop: {
        position: 'fixed', inset: 0,
        background: 'var(--aig-modal-backdrop)', backdropFilter: 'blur(4px)',
        display: 'flex', alignItems: 'flex-start', justifyContent: 'center',
        padding: '6vh 16px 16px', zIndex: 100, overflowY: 'auto',
    },
    importModalCard: {
        width: '100%', maxWidth: 920,
        background: 'var(--bg-primary)',
        border: '1px solid var(--border-color)', borderRadius: 14,
        boxShadow: 'var(--shadow-md)',
        display: 'flex', flexDirection: 'column', minHeight: 0,
        maxHeight: '88vh',
    },
    importModalHead: {
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        gap: 16, padding: '14px 18px',
        borderBottom: '1px solid var(--border-color)',
    },
    importModalTitle: {
        margin: 0, fontSize: '1rem', fontWeight: 700,
        color: 'var(--text-primary)', letterSpacing: '-0.005em',
    },
    importModalSub: {
        margin: '2px 0 0', fontSize: '0.78rem',
        color: 'var(--text-secondary)', lineHeight: 1.4,
    },
    importModalClose: {
        flexShrink: 0, width: 30, height: 30, borderRadius: 8,
        background: 'transparent', border: '1px solid var(--border-color)',
        color: 'var(--text-secondary)', cursor: 'pointer',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
    },
    importModalBody: { padding: '14px 18px 18px', overflowY: 'auto' },
};
