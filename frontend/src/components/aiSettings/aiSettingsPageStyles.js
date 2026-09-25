// Styles for AISettingsPage (Settings → AI). Theme tokens only, so it reads in dark and light.
const TONE_FG = { neutral: 'var(--text-secondary)', ok: 'var(--aig-tone-green-fg)', warn: 'var(--aig-tone-amber-fg)', bad: 'var(--aig-tone-red-fg)' };

export const ps = {
    page: { display: 'flex', flexDirection: 'column', gap: 20, animation: 'aigenSettingsFadeIn 0.2s ease both' },
    header: { display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 16, flexWrap: 'wrap' },
    title: { margin: '0 0 4px', fontSize: '1.15rem', fontWeight: 700, color: 'var(--text-primary)' },
    desc: { margin: 0, fontSize: '0.845rem', color: 'var(--text-secondary)', lineHeight: 1.6, maxWidth: 520 },
    banner: {
        padding: '10px 14px', borderRadius: 10, fontSize: '0.82rem', lineHeight: 1.5,
        color: 'var(--aig-tone-amber-fg)', background: 'rgba(234,179,8,0.1)', border: '1px solid rgba(234,179,8,0.25)',
    },
    tiles: { display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: 10 },
    tile: {
        display: 'flex', flexDirection: 'column', gap: 4, padding: '12px 14px', borderRadius: 10, textAlign: 'left',
        border: '1px solid var(--border-color)', background: 'var(--bg-secondary)', color: 'var(--text-primary)',
        fontFamily: 'inherit', cursor: 'pointer', minWidth: 0,
    },
    tileLabel: { fontSize: '0.68rem', fontWeight: 700, letterSpacing: '0.06em', textTransform: 'uppercase', color: 'var(--text-secondary)' },
    tileValue: { fontSize: '0.95rem', fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
    tileSub: (tone) => ({ fontSize: '0.76rem', color: TONE_FG[tone] || TONE_FG.neutral, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }),
    meterTrack: { display: 'block', height: 5, borderRadius: 3, background: 'var(--bg-tertiary)', overflow: 'hidden', marginTop: 4 },
    meterFill: (pct, tone) => ({ display: 'block', width: `${pct}%`, height: '100%', borderRadius: 3, background: tone === 'neutral' ? 'var(--accent-indigo)' : TONE_FG[tone] }),
    tablist: { display: 'flex', gap: 24, borderBottom: '1px solid var(--border-color)', overflowX: 'auto' },
    tab: {
        display: 'inline-flex', alignItems: 'center', gap: 7, padding: '10px 2px 9px', minHeight: 40,
        border: 'none', borderBottom: '2px solid transparent', background: 'none', color: 'var(--text-secondary)',
        fontFamily: 'inherit', fontSize: '0.88rem', fontWeight: 500, cursor: 'pointer', whiteSpace: 'nowrap',
    },
    // Same shorthand as `tab`: React drops a removed longhand without restoring the shorthand's color.
    tabOn: { color: 'var(--text-primary)', borderBottom: '2px solid var(--accent-indigo)', fontWeight: 600 },
    dirtyDot: { width: 7, height: 7, borderRadius: '50%', background: 'var(--aig-tone-amber-fg)' },
    panel: { display: 'flex', flexDirection: 'column', gap: 32 },
};
