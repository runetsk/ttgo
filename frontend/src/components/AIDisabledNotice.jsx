import React from 'react';

/** Shown on /ai-generate when AI features are globally disabled. */
export default function AIDisabledNotice() {
    return (
        <div style={{
            display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
            textAlign: 'center', gap: 12, padding: '64px 24px', width: '100%',
        }}>
            <div style={{
                width: 56, height: 56, borderRadius: 14,
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                background: 'rgba(99,102,241,0.08)', border: '1px solid rgba(99,102,241,0.2)', color: '#818cf8',
            }}>
                <svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
                    <polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/>
                    <line x1="2" y1="2" x2="22" y2="22"/>
                </svg>
            </div>
            <h2 style={{ margin: 0, fontSize: '1.1rem', fontWeight: 700, color: 'var(--text-primary)' }}>
                AI features are turned off
            </h2>
            <p style={{ margin: 0, maxWidth: 400, fontSize: '0.9rem', color: 'var(--text-secondary)', lineHeight: 1.6 }}>
                AI test generation, import, and failure analysis have been disabled by an
                administrator. They can be re-enabled in Settings → AI Generation.
            </p>
        </div>
    );
}
