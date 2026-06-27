import { useState, useEffect, useCallback, useRef } from 'react';

export function useApi(apiFn, deps = []) {
    const [data, setData] = useState(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState(null);
    const mountedRef = useRef(true);

    const fetch = useCallback(() => {
        setLoading(true);
        setError(null);
        apiFn()
            .then(result => { if (mountedRef.current) { setData(result); setLoading(false); } })
            .catch(err => { if (mountedRef.current) { setError(err); setLoading(false); } });
    // eslint-disable-next-line react-hooks/exhaustive-deps
    }, deps);

    useEffect(() => {
        mountedRef.current = true;
        fetch();
        return () => { mountedRef.current = false; };
    }, [fetch]);

    return { data, loading, error, refetch: fetch };
}
export default useApi;
