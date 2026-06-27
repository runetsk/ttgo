import { useEffect, useRef, useCallback } from 'react';

/**
 * Returns a function that creates a new AbortController, automatically
 * aborting the previous one. Also aborts on unmount.
 *
 * Usage:
 *   const getSignal = useAbortController();
 *   useEffect(() => {
 *       const signal = getSignal();
 *       fetchData({ signal });
 *   }, [deps]);
 */
export function useAbortController() {
    const controllerRef = useRef(null);

    const getSignal = useCallback(() => {
        if (controllerRef.current) {
            controllerRef.current.abort();
        }
        controllerRef.current = new AbortController();
        return controllerRef.current.signal;
    }, []);

    useEffect(() => {
        return () => {
            if (controllerRef.current) {
                controllerRef.current.abort();
            }
        };
    }, []);

    return getSignal;
}

export default useAbortController;
