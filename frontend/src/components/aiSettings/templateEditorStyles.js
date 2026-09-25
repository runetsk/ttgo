// Styles for TemplateEditor (Settings → AI → Prompts). Theme tokens only.
export const ts = {
    section: { display: 'flex', flexDirection: 'column', gap: 14 },
    head: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap' },
    segmented: { display: 'inline-flex', gap: 2, padding: 3, borderRadius: 9, background: 'var(--bg-tertiary)', border: '1px solid var(--border-color)' },
    segment: {
        display: 'inline-flex', alignItems: 'center', gap: 6, padding: '6px 14px', borderRadius: 7, border: 'none',
        background: 'transparent', color: 'var(--text-secondary)', fontFamily: 'inherit', fontSize: '0.82rem', fontWeight: 500, cursor: 'pointer',
    },
    segmentOn: { background: 'var(--bg-secondary)', color: 'var(--text-primary)', fontWeight: 600, boxShadow: '0 1px 2px rgba(0,0,0,0.2)' },
    dirtyDot: { width: 7, height: 7, borderRadius: '50%', background: 'var(--aig-tone-amber-fg)' },
    actions: { display: 'flex', gap: 8, alignItems: 'center' },
    unsaved: {
        fontSize: '0.7rem', fontWeight: 600, color: 'var(--aig-tone-amber-fg)', background: 'rgba(234,179,8,0.1)',
        border: '1px solid rgba(234,179,8,0.25)', padding: '1px 8px', borderRadius: 20,
    },
    desc: { margin: 0, fontSize: '0.845rem', color: 'var(--text-secondary)', lineHeight: 1.6 },
    missing: {
        padding: '10px 14px', borderRadius: 8, background: 'rgba(239,68,68,0.08)', border: '1px solid rgba(239,68,68,0.25)',
        fontSize: '0.8rem', color: 'var(--aig-tone-red-fg)', lineHeight: 1.5,
    },
    grid: { display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) 220px', gap: 12, alignItems: 'stretch' },
    editorWrap: { borderRadius: 10, border: '1px solid var(--border-color)', overflow: 'hidden', background: 'var(--bg-primary)', display: 'flex', flexDirection: 'column' },
    readOnly: { padding: '7px 14px', borderBottom: '1px solid var(--border-color)', fontSize: '0.78rem', color: 'var(--text-secondary)' },
    editor: {
        width: '100%', minHeight: 320, flex: 1, fontFamily: '"SF Mono", "Fira Code", "Cascadia Code", monospace', fontSize: '0.8rem',
        resize: 'vertical', lineHeight: 1.65, border: 'none', borderRadius: 0, background: 'transparent', padding: 14, boxSizing: 'border-box',
    },
    footer: {
        display: 'flex', justifyContent: 'space-between', padding: '6px 12px', borderTop: '1px solid var(--border-color)',
        fontSize: '0.72rem', color: 'var(--text-secondary)', fontFamily: 'monospace',
    },
    side: {
        display: 'flex', flexDirection: 'column', gap: 2, padding: '10px 8px', borderRadius: 10,
        border: '1px solid var(--border-color)', background: 'var(--bg-secondary)',
    },
    sideTitle: { fontSize: '0.7rem', fontWeight: 700, letterSpacing: '0.06em', textTransform: 'uppercase', color: 'var(--text-secondary)', padding: '0 8px 6px' },
    ph: {
        display: 'flex', alignItems: 'center', gap: 8, width: '100%', padding: '7px 8px', borderRadius: 7, border: 'none',
        background: 'transparent', color: 'var(--text-primary)', fontFamily: 'inherit', textAlign: 'left', cursor: 'pointer',
    },
    phName: { fontFamily: 'monospace', fontSize: '0.74rem', flex: 1, minWidth: 0, overflowWrap: 'anywhere' },
    phReq: { fontSize: '0.66rem', fontWeight: 600, color: 'var(--text-secondary)' },
    phMissing: { color: 'var(--aig-tone-red-fg)' },
    sideNote: { margin: '6px 8px 0', fontSize: '0.72rem', color: 'var(--text-secondary)', lineHeight: 1.5 },
};
