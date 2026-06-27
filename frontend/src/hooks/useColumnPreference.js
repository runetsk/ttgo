import { useState, useCallback } from 'react';

/**
 * Builds a Set of visible column keys by merging stored preferences with
 * column definitions. Handles two edge cases:
 *   - Key absent from storedPref → uses col.defaultVisible (new columns, FR-011)
 *   - Key in storedPref with no matching ColumnDef → silently ignored (FR-010)
 */
function buildVisibleSet(columnDefs, storedPref) {
    const result = new Set();
    for (const col of columnDefs) {
        const stored = storedPref[col.key];
        const visible = stored !== undefined ? stored : col.defaultVisible;
        if (visible) result.add(col.key);
    }
    return result;
}

function readPref(storageKey) {
    try {
        const raw = localStorage.getItem(storageKey);
        return raw ? JSON.parse(raw) : {};
    } catch {
        // Graceful degradation: storage unavailable or corrupted (FR-007)
        return {};
    }
}

function writePref(storageKey, pref) {
    try {
        localStorage.setItem(storageKey, JSON.stringify(pref));
    } catch {
        // Graceful degradation: storage unavailable (e.g. private browsing) — FR-007
    }
}

function removePref(storageKey) {
    try {
        localStorage.removeItem(storageKey);
    } catch {
        // ignore
    }
}

/**
 * Hook for managing per-grid column visibility with localStorage persistence.
 *
 * @param {string} gridId      - Stable grid identifier; used as localStorage key suffix.
 *                               Storage key format: `ttgo_columns_<gridId>`
 * @param {Array}  columnDefs  - Array of { key, label, mandatory, defaultVisible }
 * @returns {[Set<string>, Function, Function]}
 *   - visibleKeys:   Set of currently visible column keys
 *   - toggleColumn:  (key: string) => void  — noop for mandatory columns
 *   - resetColumns:  () => void             — clears storage and restores defaults
 */
export function useColumnPreference(gridId, columnDefs) {
    const storageKey = `ttgo_columns_${gridId}`;

    const [visibleKeys, setVisibleKeys] = useState(() =>
        buildVisibleSet(columnDefs, readPref(storageKey))
    );

    const toggleColumn = useCallback((key) => {
        const col = columnDefs.find(c => c.key === key);
        if (!col || col.mandatory) return;

        setVisibleKeys(prev => {
            const next = new Set(prev);
            if (next.has(key)) {
                next.delete(key);
            } else {
                next.add(key);
            }
            // Persist complete preference snapshot for all known columns
            const pref = {};
            for (const c of columnDefs) {
                pref[c.key] = next.has(c.key);
            }
            writePref(storageKey, pref);
            return next;
        });
    }, [columnDefs, storageKey]);

    const resetColumns = useCallback(() => {
        removePref(storageKey);
        setVisibleKeys(buildVisibleSet(columnDefs, {}));
    }, [columnDefs, storageKey]);

    return [visibleKeys, toggleColumn, resetColumns];
}
