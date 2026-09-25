import { createContext, useContext, useEffect, useLayoutEffect } from 'react';

// The Settings → AI save-bar registry. AISettingsPage provides it; each section with unsaved
// state registers through useSaveSection. `saving` is true while the bar saves, and every
// section locks its inputs meanwhile, so a save's response can replace the draft safely.
export const SaveBarContext = createContext({ registry: null, saving: false });

export function useSaveBarSaving() {
    return useContext(SaveBarContext).saving;
}

// useSaveSection must run on every render, before a section's early returns. save/discard are
// refreshed after every render (they close over the latest draft); only the summary fields
// re-render the page. Returns `saving`.
export function useSaveSection(key, { label, tab, dirty, errors, save, discard }) {
    const { registry, saving } = useContext(SaveBarContext);
    useLayoutEffect(() => { registry?.setHandle(key, { save, discard }); });
    const errorsKey = JSON.stringify(errors || []);
    useEffect(() => {
        registry?.setSummary(key, { key, label, tab, dirty: !!dirty, errors: JSON.parse(errorsKey) });
    }, [registry, key, label, tab, dirty, errorsKey]);
    useEffect(() => () => registry?.remove(key), [registry, key]);
    return saving;
}
