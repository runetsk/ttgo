import React from 'react';

// SaveBar is the sticky Save / Discard bar of Settings → AI. It places what saveBarModel
// (utils/saveBar.js) derives; AISettingsPage decides when it shows.
export default function SaveBar({ model, result, saving, onSave, onDiscard, onReveal }) {
    const failed = !!result && model.problems.length === 0 && !saving;
    let body;
    if (saving) body = 'Saving…';
    else if (model.problems.length > 0) {
        body = (
            <>
                {model.message}{' '}
                {model.problems.map((p, i) => (
                    <React.Fragment key={p.settingKey}>
                        {i > 0 && ', '}
                        <button type="button" style={bs.link} data-testid={`ai-savebar-fix-${p.settingKey}`} onClick={() => onReveal(p.settingKey)}>
                            {p.label} ({p.tabLabel})
                        </button>
                    </React.Fragment>
                ))}
            </>
        );
    } else body = failed ? result : model.message;

    return (
        <div role="region" aria-label="Unsaved changes" data-testid="ai-savebar" style={bs.bar}>
            <span aria-hidden="true" style={{ ...bs.dot, background: failed ? 'var(--aig-tone-red-fg)' : 'var(--aig-tone-amber-fg)' }} />
            <div aria-live="polite" data-testid="ai-savebar-message" style={{ ...bs.message, ...(failed ? bs.messageBad : null) }}>{body}</div>
            <button type="button" className="action-btn" data-testid="ai-savebar-discard" disabled={saving} onClick={onDiscard} style={{ fontSize: '0.82rem' }}>
                Discard
            </button>
            <button type="button" className="primary-btn" data-testid="ai-savebar-save" disabled={saving || !model.canSave} onClick={onSave} style={{ fontSize: '0.82rem' }}>
                {saving ? 'Saving…' : 'Save changes'}
            </button>
        </div>
    );
}

const bs = {
    bar: {
        position: 'sticky', bottom: 0, zIndex: 5, display: 'flex', alignItems: 'center', gap: 10,
        padding: '10px 14px', marginTop: 8, borderRadius: 10, border: '1px solid var(--border-color)',
        background: 'var(--bg-secondary)', boxShadow: '0 -6px 20px rgba(0,0,0,0.18)',
    },
    dot: { width: 8, height: 8, borderRadius: '50%', flexShrink: 0 },
    message: { flex: 1, minWidth: 0, fontSize: '0.84rem', color: 'var(--text-primary)', lineHeight: 1.45 },
    messageBad: { color: 'var(--aig-tone-red-fg)', fontWeight: 600 },
    link: {
        padding: 0, border: 'none', background: 'none', color: 'var(--aig-tone-indigo-fg)', fontFamily: 'inherit',
        fontSize: 'inherit', fontWeight: 600, textDecoration: 'underline', cursor: 'pointer',
    },
};
