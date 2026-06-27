import React, { useState } from 'react';
import { aiGeneration } from '../api';
import { toast } from '../toast';
import { useAIGeneration } from '../contexts/AIGenerationContext';

/**
 * Global master switch for all AI features (generation, import, failure
 * analysis). Reads/writes the DB-backed flag exposed via AIGenerationContext.
 * Admin-only control; read-only for everyone else.
 */
export default function AIFeaturesToggle({ isAdmin }) {
    const { aiFeaturesEnabled, setAiFeaturesEnabled } = useAIGeneration();
    const [saving, setSaving] = useState(false);

    const handleToggle = async () => {
        if (!isAdmin || saving) return;
        const next = !aiFeaturesEnabled;
        setSaving(true);
        try {
            await aiGeneration.updateFeatureSettings(next);
            setAiFeaturesEnabled(next);
            toast.success(next ? 'AI features enabled' : 'AI features disabled');
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Failed to update AI features');
        } finally {
            setSaving(false);
        }
    };

    return (
        <div style={s.card}>
            <div style={{ flex: 1, minWidth: 0 }}>
                <div style={s.titleRow}>
                    <span style={s.title}>AI Features</span>
                    <span style={{ ...s.badge, ...(aiFeaturesEnabled ? s.badgeOn : s.badgeOff) }}>
                        {aiFeaturesEnabled ? 'On' : 'Off'}
                    </span>
                </div>
                <p style={s.desc}>
                    Enable AI test generation, import, and failure analysis across the app.
                    When off, all AI actions are hidden from the interface.
                </p>
                {!isAdmin && (
                    <p style={s.lockNote}>🔒 Only an admin can change this setting.</p>
                )}
                {!aiFeaturesEnabled && (
                    <p style={s.caveat}>
                        Note: this hides AI in the UI only. Automated failure analysis on run
                        completion (if enabled below) keeps running server-side.
                    </p>
                )}
            </div>
            <button
                type="button"
                role="switch"
                aria-checked={aiFeaturesEnabled ? 'true' : 'false'}
                aria-disabled={!isAdmin || saving}
                disabled={!isAdmin || saving}
                onClick={handleToggle}
                title={isAdmin ? 'Toggle AI features' : 'Admin only'}
                style={{
                    ...s.switch,
                    background: aiFeaturesEnabled ? '#6366f1' : 'var(--border-color)',
                    cursor: isAdmin ? 'pointer' : 'not-allowed',
                    opacity: isAdmin ? 1 : 0.6,
                }}
            >
                <span style={{ ...s.knob, transform: aiFeaturesEnabled ? 'translateX(20px)' : 'translateX(2px)' }} />
            </button>
        </div>
    );
}

const s = {
    card: {
        display: 'flex', alignItems: 'flex-start', gap: 16,
        padding: '16px 18px', borderRadius: 12,
        border: '1px solid var(--border-color)', background: 'var(--bg-tertiary)',
        marginBottom: 24,
    },
    titleRow: { display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 },
    title: { fontSize: '0.95rem', fontWeight: 700, color: 'var(--text-primary)' },
    badge: { fontSize: '0.68rem', fontWeight: 700, padding: '1px 8px', borderRadius: 20, letterSpacing: '0.02em' },
    badgeOn: { color: '#4ade80', background: 'rgba(34,197,94,0.12)', border: '1px solid rgba(34,197,94,0.25)' },
    badgeOff: { color: '#f87171', background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.2)' },
    desc: { margin: 0, fontSize: '0.845rem', color: 'var(--text-secondary)', lineHeight: 1.6, maxWidth: 560 },
    lockNote: { margin: '8px 0 0', fontSize: '0.78rem', color: 'var(--text-secondary)', opacity: 0.85 },
    caveat: { margin: '8px 0 0', fontSize: '0.78rem', color: '#fbbf24', lineHeight: 1.5 },
    switch: {
        position: 'relative', flexShrink: 0,
        width: 42, height: 24, borderRadius: 999, border: 'none', padding: 0,
        transition: 'background 0.18s', marginTop: 2,
    },
    knob: {
        position: 'absolute', top: 2, left: 0,
        width: 20, height: 20, borderRadius: '50%', background: '#fff',
        transition: 'transform 0.18s', boxShadow: '0 1px 3px rgba(0,0,0,0.3)',
    },
};
