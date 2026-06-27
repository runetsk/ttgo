import { useState, useCallback } from 'react';

export function usePagination(initialPage = 1, initialLimit = 20) {
    const [page, setPageState] = useState(initialPage);
    const [limit, setLimitState] = useState(initialLimit);
    const offset = (page - 1) * limit;
    const setPage = useCallback((p) => setPageState(p), []);
    const setLimit = useCallback((l) => { setLimitState(l); setPageState(1); }, []);
    const resetPagination = useCallback(() => { setPageState(initialPage); setLimitState(initialLimit); }, [initialPage, initialLimit]);
    return { page, limit, offset, setPage, setLimit, resetPagination };
}
export default usePagination;
