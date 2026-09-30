// Styles shared by the AI settings cards and the controls in SettingsControls.jsx.
export const cs = {
    section: { display: 'flex', flexDirection: 'column', gap: 14 },
    sectionHead: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap' },
    sectionHeadLeft: { display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' },
    sectionDot: { width: 6, height: 6, borderRadius: '50%', background: 'linear-gradient(135deg, #6366f1, #14b8a6)', flexShrink: 0 },
    sectionTitle: { margin: 0, fontSize: '0.9rem', fontWeight: 700, color: 'var(--text-primary)' },
    modifiedBadge: {
        fontSize: '0.7rem', fontWeight: 600, color: '#fbbf24', background: 'rgba(234,179,8,0.1)',
        border: '1px solid rgba(234,179,8,0.2)', padding: '1px 8px', borderRadius: 20,
    },
    // Capped so a description stays readable when the page is wide.
    desc: { margin: 0, fontSize: '0.845rem', color: 'var(--text-secondary)', lineHeight: 1.6, maxWidth: 760 },
    groupTitle: {
        fontSize: '0.72rem', fontWeight: 700, color: 'var(--text-secondary)', textTransform: 'uppercase',
        letterSpacing: '0.06em', marginTop: 4,
    },
    // Tile rows: at most three columns (togglesGrid) or two (togglesGrid2, for four tiles as 2×2),
    // one column on a narrow screen.
    togglesGrid: { display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(max(260px, calc((100% - 20px) / 3)), 1fr))', gap: 10 },
    togglesGrid2: { display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(max(260px, calc((100% - 10px) / 2)), 1fr))', gap: 10 },
    toggleCard: {
        display: 'flex', flexDirection: 'column', gap: 8, padding: '12px 14px', borderRadius: 10,
        borderWidth: 1, borderStyle: 'solid', borderColor: 'var(--border-color)', background: 'var(--bg-tertiary)',
        transition: 'background 0.15s, border-color 0.15s, box-shadow 0.15s',
    },
    // The main area of a ToggleCard: the overlay checkbox covers exactly this, not the help below.
    toggleMain: { position: 'relative', display: 'flex', alignItems: 'flex-start', gap: 10 },
    labelRow: { display: 'flex', alignItems: 'center', gap: 10, minWidth: 0, flexWrap: 'wrap' },
    toggleIcon: {
        width: 30, height: 30, borderRadius: 8, borderWidth: 1, borderStyle: 'solid', borderColor: 'transparent',
        display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0,
    },
    toggleLabel: { fontSize: '0.86rem', fontWeight: 600, color: 'var(--text-primary)', marginBottom: 2 },
    toggleDesc: { fontSize: '0.76rem', color: 'var(--text-secondary)', lineHeight: 1.5 },
    toggleNote: { fontSize: '0.72rem', color: 'var(--aig-tone-amber-fg)', marginTop: 4, fontWeight: 600 },
    fieldError: { color: 'var(--aig-tone-red-fg)', fontWeight: 600 },
    // The ⓘ after a label's last word. Raised above a ToggleCard's overlay checkbox so it gets its own clicks.
    helpBtn: {
        position: 'relative', zIndex: 1, flexShrink: 0, width: 16, height: 16, padding: 0, marginLeft: 6, borderRadius: '50%',
        display: 'inline-flex', alignItems: 'center', justifyContent: 'center', verticalAlign: 'middle', top: -1,
        border: '1.5px solid currentColor', background: 'transparent', color: 'var(--text-secondary)',
        fontFamily: 'Georgia, serif', fontSize: '0.62rem', fontWeight: 700, fontStyle: 'italic', lineHeight: 1,
        cursor: 'pointer', opacity: 0.75,
    },
    helpBtnOn: { color: 'var(--aig-tone-indigo-fg)', opacity: 1 },
    helpText: {
        flexDirection: 'column', gap: 6, padding: '8px 10px', borderRadius: 8,
        border: '1px solid var(--border-color)', background: 'var(--bg-secondary)',
    },
    // A FieldRow's explanation spans the row, under both the label and the control.
    helpTextFull: { flexBasis: '100%' },
    helpPara: { margin: 0, fontSize: '0.76rem', color: 'var(--text-secondary)', lineHeight: 1.55 },
    switch: { position: 'relative', borderRadius: 10, transition: 'background 0.15s', flexShrink: 0 },
    switchKnob: {
        position: 'absolute', top: 2, left: 2, borderRadius: '50%', background: '#fff',
        transition: 'transform 0.15s', boxShadow: '0 1px 2px rgba(0,0,0,0.25)',
    },
    overlayInput: { position: 'absolute', inset: 0, width: '100%', height: '100%', margin: 0, opacity: 0, cursor: 'inherit' },
    inlineSwitch: {
        position: 'relative', display: 'inline-flex', alignItems: 'center', gap: 8,
        fontSize: '0.78rem', color: 'var(--text-secondary)', userSelect: 'none',
    },
    fieldRow: {
        display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 16, flexWrap: 'wrap',
        padding: '12px 14px', borderRadius: 10, border: '1px solid var(--border-color)', background: 'var(--bg-tertiary)',
    },
    fieldLabelCol: { display: 'flex', flexDirection: 'column', gap: 2, minWidth: 0, flex: '1 1 260px' },
    fieldLabel: { fontSize: '0.86rem', fontWeight: 600, color: 'var(--text-primary)' },
    fieldHint: { margin: 0, fontSize: '0.76rem', color: 'var(--text-secondary)', lineHeight: 1.5 },
    fieldControl: { display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap', justifyContent: 'flex-end' },
    // Up to three columns: the min track is a third of the row, or 220px on a narrow screen.
    fieldGrid: {
        display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(max(220px, calc((100% - 48px) / 3)), 1fr))',
        gap: '16px 24px', padding: '12px 14px', borderRadius: 10,
        border: '1px solid var(--border-color)', background: 'var(--bg-tertiary)',
    },
    compactField: { display: 'flex', flexDirection: 'column', gap: 6, minWidth: 0, borderRadius: 8 },
    compactControl: { display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' },
    suffixWrap: { display: 'inline-flex', alignItems: 'center', gap: 6 },
    suffix: { fontSize: '0.8rem', color: 'var(--text-secondary)' },
    // The outlined secondary action used beside a primary button.
    secondaryBtn: {
        display: 'inline-flex', alignItems: 'center', gap: 6, padding: '8px 12px', fontSize: '0.8125rem', fontWeight: 500,
        borderRadius: 8, border: '1px solid var(--border-color)', background: 'transparent', color: 'var(--text-secondary)',
        cursor: 'pointer', fontFamily: 'inherit', whiteSpace: 'nowrap',
    },
};
