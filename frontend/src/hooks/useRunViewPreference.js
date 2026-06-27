import { useCallback, useState } from 'react';

const STORAGE_KEY = 'ttgo_run_detail_view';
const DEFAULT = { view: 'list', groupBy: 'status' };
const VALID_VIEWS = new Set(['list', 'grouped']);
const VALID_GROUPS = new Set(['status', 'ai_verdict', 'defect_type', 'error_signature', 'failure_type', 'environment']);

function read() {
    try {
        const raw = localStorage.getItem(STORAGE_KEY);
        if (!raw) return { ...DEFAULT };
        const parsed = JSON.parse(raw);
        return {
            view:    VALID_VIEWS.has(parsed.view)     ? parsed.view    : DEFAULT.view,
            groupBy: VALID_GROUPS.has(parsed.groupBy) ? parsed.groupBy : DEFAULT.groupBy,
        };
    } catch {
        return { ...DEFAULT };
    }
}

function write(value) {
    try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(value));
    } catch {
        // private browsing or quota — silently ignore
    }
}

export function useRunViewPreference() {
    const [state, setState] = useState(read);

    const setView = useCallback((view) => {
        setState(prev => {
            const next = { ...prev, view };
            write(next);
            return next;
        });
    }, []);

    const setGroupBy = useCallback((groupBy) => {
        setState(prev => {
            const next = { ...prev, groupBy };
            write(next);
            return next;
        });
    }, []);

    return { view: state.view, groupBy: state.groupBy, setView, setGroupBy };
}
