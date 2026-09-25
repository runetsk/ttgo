import { createContext } from 'react';

// jumpToSetting scrolls the setting a chip names into view, flashes it and focuses its control,
// or the setting's tile when the control is disabled (non-admins, greyed-out dependents).
export function jumpToSetting(key) {
    const el = document.querySelector(`[data-setting="${key}"]`);
    if (!el) return;
    el.scrollIntoView({ behavior: 'smooth', block: 'center' });
    el.animate?.(
        [{ boxShadow: '0 0 0 3px rgba(99,102,241,0.75)' }, { boxShadow: '0 0 0 3px rgba(99,102,241,0)' }],
        { duration: 1600, easing: 'ease-out' },
    );
    const control = el.querySelector('input:not([disabled]), textarea:not([disabled]), select:not([disabled]), button[role="switch"]:not([disabled])');
    (control || el).focus({ preventScroll: true });
}

// RevealSettingContext lets a chip reveal a setting that may sit on another tab of Settings → AI;
// AISettingsPage provides a version that switches tab first. Without a provider it jumps in place.
export const RevealSettingContext = createContext(jumpToSetting);
