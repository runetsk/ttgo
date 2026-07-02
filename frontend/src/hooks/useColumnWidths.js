import { useState, useCallback, useEffect, useRef } from 'react';

/**
 * Hook for managing per-grid column widths with drag-to-resize and localStorage persistence.
 *
 * @param {string} gridId      - Stable grid identifier; used as localStorage key suffix.
 *                               Storage key format: `ttgo_colwidths_<gridId>`
 * @param {Array}  columnDefs  - Array of { key, label, ..., defaultWidth? }
 * @returns {{ columnWidths, startResize, resetWidths, resetColumnWidth, isResizing }}
 */

const DEFAULT_WIDTH = 150;
const MIN_COL_WIDTH = 50;

function readWidths(storageKey) {
    try {
        const raw = localStorage.getItem(storageKey);
        return raw ? JSON.parse(raw) : {};
    } catch {
        return {};
    }
}

function writeWidths(storageKey, widths) {
    try {
        localStorage.setItem(storageKey, JSON.stringify(widths));
    } catch {
        // Graceful degradation: storage unavailable (e.g. private browsing)
    }
}

function buildWidthMap(columnDefs, storedWidths) {
    const map = {};
    for (const col of columnDefs) {
        map[col.key] = storedWidths[col.key] ?? col.defaultWidth ?? DEFAULT_WIDTH;
    }
    return map;
}

export function useColumnWidths(gridId, columnDefs) {
    const storageKey = `ttgo_colwidths_${gridId}`;

    const [columnWidths, setColumnWidths] = useState(() =>
        buildWidthMap(columnDefs, readWidths(storageKey))
    );
    const [isResizing, setIsResizing] = useState(false);

    // Refs for the drag operation — avoids stale closures in window listeners
    const resizingCol = useRef(null);
    const startX = useRef(0);
    const startWidth = useRef(0);
    const latestWidths = useRef(columnWidths);

    // Keep ref in sync with state
    useEffect(() => {
        latestWidths.current = columnWidths;
    }, [columnWidths]);

    const startResize = useCallback((columnKey, e) => {
        e.preventDefault();
        e.stopPropagation(); // Prevent sort click on the <th>
        resizingCol.current = columnKey;
        startX.current = e.clientX;
        startWidth.current = latestWidths.current[columnKey] || DEFAULT_WIDTH;
        setIsResizing(true);
        document.body.style.cursor = 'col-resize';
        document.body.style.userSelect = 'none';
    }, []);

    useEffect(() => {
        const handleMouseMove = (e) => {
            if (!resizingCol.current) return;
            const delta = e.clientX - startX.current;
            const newWidth = Math.max(MIN_COL_WIDTH, startWidth.current + delta);
            const col = resizingCol.current;
            setColumnWidths(prev => ({ ...prev, [col]: newWidth }));
        };

        const handleMouseUp = () => {
            if (!resizingCol.current) return;
            resizingCol.current = null;
            setIsResizing(false);
            document.body.style.cursor = '';
            document.body.style.userSelect = '';
            // Persist all widths on mouse up
            setColumnWidths(prev => {
                writeWidths(storageKey, prev);
                return prev;
            });
        };

        window.addEventListener('mousemove', handleMouseMove);
        window.addEventListener('mouseup', handleMouseUp);
        return () => {
            window.removeEventListener('mousemove', handleMouseMove);
            window.removeEventListener('mouseup', handleMouseUp);
        };
    }, [storageKey]);

    // Reset a single column to its default width
    const resetColumnWidth = useCallback((columnKey) => {
        const col = columnDefs.find(c => c.key === columnKey);
        const defaultW = col?.defaultWidth ?? DEFAULT_WIDTH;
        setColumnWidths(prev => {
            const next = { ...prev, [columnKey]: defaultW };
            writeWidths(storageKey, next);
            return next;
        });
    }, [columnDefs, storageKey]);

    // Reset all columns to their default widths
    const resetWidths = useCallback(() => {
        try { localStorage.removeItem(storageKey); } catch { /* localStorage unavailable — state below still resets to defaults */ }
        setColumnWidths(buildWidthMap(columnDefs, {}));
    }, [columnDefs, storageKey]);

    return { columnWidths, startResize, resetWidths, resetColumnWidth, isResizing };
}
