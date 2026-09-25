import React, { useId, useState } from 'react';
import { aiGeneration } from '../api';
import { toast } from '../toast';
import { useAIGeneration } from '../contexts/AIGenerationContext';
import { useSaveBarSaving } from './aiSettings/saveBarContext';

/**
 * Global master switch for all AI features (generation, import, failure analysis), shown in the
 * Settings → AI header. Reads/writes the DB-backed flag exposed via AIGenerationContext.
 * Admin-only control; read-only for everyone else. AISettingsPage shows what "off" means.
 */
export default function AIFeaturesToggle({ isAdmin }) {
    const { aiFeaturesEnabled, setAiFeaturesEnabled } = useAIGeneration();
    const [saving, setSaving] = useState(false);
    const busy = useSaveBarSaving();
    const id = useId();

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
        <div style={s.wrap} data-setting="ai.enabled" tabIndex={-1}>
            <label htmlFor={id} style={s.label}>AI features</label>
            <span style={{ ...s.badge, ...(aiFeaturesEnabled ? s.badgeOn : s.badgeOff) }}>{aiFeaturesEnabled ? 'On' : 'Off'}</span>
            <button
                id={id}
                type="button"
                role="switch"
                data-testid="ai-features-switch"
                aria-checked={aiFeaturesEnabled ? 'true' : 'false'}
                disabled={!isAdmin || saving || busy}
                onClick={handleToggle}
                title={isAdmin ? 'Turn every AI feature on or off' : 'Only an admin can change this setting'}
                style={{ ...s.switch, background: aiFeaturesEnabled ? 'var(--accent-indigo)' : 'var(--border-color)', cursor: isAdmin ? 'pointer' : 'not-allowed', opacity: isAdmin ? 1 : 0.6 }}
            >
                <span style={{ ...s.knob, transform: aiFeaturesEnabled ? 'translateX(20px)' : 'translateX(2px)' }} />
            </button>
            {!isAdmin && <span style={s.lock}>Admin only</span>}
        </div>
    );
}

const s = {
    wrap: {
        display: 'flex', alignItems: 'center', gap: 10, flexShrink: 0, padding: '8px 12px', borderRadius: 10,
        border: '1px solid var(--border-color)', background: 'var(--bg-secondary)',
    },
    label: { fontSize: '0.85rem', fontWeight: 600, color: 'var(--text-primary)', cursor: 'pointer' },
    badge: { fontSize: '0.68rem', fontWeight: 700, padding: '1px 8px', borderRadius: 20, letterSpacing: '0.02em' },
    badgeOn: { color: 'var(--aig-tone-green-fg)', background: 'rgba(34,197,94,0.12)', border: '1px solid rgba(34,197,94,0.25)' },
    badgeOff: { color: 'var(--aig-tone-red-fg)', background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.2)' },
    lock: { fontSize: '0.72rem', color: 'var(--text-secondary)' },
    switch: { position: 'relative', flexShrink: 0, width: 42, height: 24, borderRadius: 999, border: 'none', padding: 0, transition: 'background 0.18s' },
    knob: {
        position: 'absolute', top: 2, left: 0, width: 20, height: 20, borderRadius: '50%', background: '#fff',
        transition: 'transform 0.18s', boxShadow: '0 1px 3px rgba(0,0,0,0.3)',
    },
};
