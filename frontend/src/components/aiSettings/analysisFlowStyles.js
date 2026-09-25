// Styles for the failure-analysis process diagram (AnalysisFlowDiagram.jsx). Theme tokens only,
// so it reads in dark and light.
const tone = {
    run: { border: 'rgba(99,102,241,0.35)', fg: 'var(--aig-tone-indigo-fg)', bg: 'rgba(99,102,241,0.08)' },
    skip: { border: 'var(--border-color)', fg: 'var(--text-secondary)', bg: 'transparent' },
    warn: { border: 'rgba(234,179,8,0.45)', fg: 'var(--aig-tone-amber-fg)', bg: 'rgba(234,179,8,0.1)' },
    blocked: { border: 'rgba(239,68,68,0.45)', fg: 'var(--aig-tone-red-fg)', bg: 'rgba(239,68,68,0.1)' },
    idle: { border: 'var(--border-color)', fg: 'var(--text-secondary)', bg: 'transparent' },
};
const byStatus = (fn) => Object.fromEntries(Object.entries(tone).map(([k, t]) => [k, fn(t, k)]));
const dashedIfSkip = (k) => (k === 'skip' ? 'dashed' : 'solid');

export const fs = {
    panel: {
        display: 'flex', flexDirection: 'column', gap: 12, padding: '16px 18px', borderRadius: 12,
        border: '1px solid var(--border-color)', background: 'var(--bg-secondary)',
    },
    head: { display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap' },
    title: { fontSize: '0.95rem', fontWeight: 700, color: 'var(--text-primary)' },
    unsaved: { marginTop: 4, fontSize: '0.72rem', fontWeight: 600, color: 'var(--aig-tone-amber-fg)' },
    segmented: {
        display: 'inline-flex', flexWrap: 'wrap', maxWidth: '100%', border: '1px solid var(--border-color)',
        borderRadius: 8, overflow: 'hidden',
    },
    segment: {
        padding: '6px 12px', fontSize: '0.78rem', fontWeight: 600, border: 'none', background: 'transparent',
        color: 'var(--text-secondary)', cursor: 'pointer', fontFamily: 'inherit',
    },
    segmentOn: { background: 'rgba(99,102,241,0.15)', color: 'var(--aig-tone-indigo-fg)' },
    legend: { display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: '6px 12px', fontSize: '0.72rem', color: 'var(--text-secondary)' },
    steps: { listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column' },
    step: { display: 'flex', gap: 12, minWidth: 0 },
    stepIdle: { opacity: 0.45 },
    rail: { display: 'flex', flexDirection: 'column', alignItems: 'center', flexShrink: 0, width: 26 },
    dot: {
        width: 26, height: 26, borderRadius: '50%', display: 'flex', alignItems: 'center', justifyContent: 'center',
        fontSize: '0.75rem', fontWeight: 700, borderWidth: 1.5, flexShrink: 0,
    },
    dotTone: byStatus((t, k) => ({ borderColor: t.border, color: t.fg, background: t.bg, borderStyle: dashedIfSkip(k) })),
    line: { flex: 1, width: 2, background: 'var(--border-color)', margin: '4px 0' },
    body: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 6, padding: '2px 0 18px' },
    stepHead: { display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' },
    stepTitle: { margin: 0, fontSize: '0.88rem', fontWeight: 700, color: 'var(--text-primary)' },
    status: {
        display: 'inline-flex', alignItems: 'center', gap: 4, fontSize: '0.68rem', fontWeight: 700,
        padding: '1px 8px', borderRadius: 20, borderWidth: 1,
    },
    statusTone: byStatus((t, k) => ({ color: t.fg, background: t.bg, borderColor: t.border, borderStyle: dashedIfSkip(k) })),
    tag: {
        fontSize: '0.68rem', fontWeight: 600, padding: '1px 8px', borderRadius: 20,
        color: 'var(--aig-tone-indigo-fg)', border: '1px solid rgba(99,102,241,0.3)',
    },
    tagUnredacted: { color: 'var(--aig-tone-amber-fg)', borderColor: 'rgba(234,179,8,0.45)', background: 'rgba(234,179,8,0.1)' },
    reason: { margin: 0, fontSize: '0.78rem', fontWeight: 600, color: 'var(--text-primary)', lineHeight: 1.5 },
    detail: { margin: 0, fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: 1.55 },
    note: { margin: 0, fontSize: '0.74rem', color: 'var(--text-secondary)', lineHeight: 1.5, fontStyle: 'italic' },
    partsLabel: {
        fontSize: '0.7rem', fontWeight: 700, color: 'var(--text-secondary)', textTransform: 'uppercase',
        letterSpacing: '0.05em', marginBottom: 6,
    },
    parts: { display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: 8 },
    part: {
        display: 'flex', flexDirection: 'column', gap: 4, padding: '8px 10px', borderRadius: 8, borderWidth: 1,
        background: 'var(--bg-tertiary)', minWidth: 0,
    },
    partTone: byStatus((t, k) => ({ borderColor: t.border, borderStyle: dashedIfSkip(k), opacity: k === 'skip' ? 0.75 : 1 })),
    partHead: { display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' },
    partTitle: { fontSize: '0.8rem', fontWeight: 600, color: 'var(--text-primary)' },
    chips: { display: 'flex', flexWrap: 'wrap', gap: 6 },
    // inline-block, not flex: a long "Label: value" wraps as one sentence on narrow screens.
    chip: {
        display: 'inline-block', maxWidth: '100%', lineHeight: 1.45, padding: '3px 9px', borderRadius: 12,
        border: '1px solid var(--border-color)', background: 'var(--bg-tertiary)', color: 'var(--text-primary)',
        fontSize: '0.72rem', cursor: 'pointer', fontFamily: 'inherit', textAlign: 'left',
    },
    chipInvalid: { borderColor: 'rgba(239,68,68,0.45)', color: 'var(--aig-tone-red-fg)' },
    chipLabel: { color: 'var(--text-secondary)' },
    howBtn: {
        alignSelf: 'flex-start', padding: 0, border: 'none', background: 'transparent', color: 'var(--aig-tone-indigo-fg)',
        fontSize: '0.74rem', fontWeight: 600, cursor: 'pointer', fontFamily: 'inherit',
    },
    how: {
        flexDirection: 'column', gap: 6, padding: '8px 10px', borderRadius: 8,
        border: '1px solid var(--border-color)', background: 'var(--bg-tertiary)',
    },
    howPara: { margin: 0, fontSize: '0.76rem', color: 'var(--text-secondary)', lineHeight: 1.55 },
    error: {
        padding: '10px 12px', borderRadius: 8, border: '1px solid rgba(239,68,68,0.45)', background: 'rgba(239,68,68,0.08)',
        color: 'var(--aig-tone-red-fg)', fontSize: '0.82rem', fontWeight: 600,
    },
    stale: { fontSize: '0.76rem', fontWeight: 600, color: 'var(--aig-tone-amber-fg)' },
    skeleton: { height: 44, borderRadius: 8, background: 'var(--bg-tertiary)', opacity: 0.7 },
};
