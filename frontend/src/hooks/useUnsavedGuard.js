import { useCallback, useEffect } from 'react';
import { useBlocker } from 'react-router-dom';
import { shouldBlockNavigation } from '../utils/saveBar';

// useUnsavedGuard warns before unsaved changes are lost: the browser's own prompt on close or
// reload, and a confirm on every router navigation (links, navigate(), Back/Forward, #hash
// links). A #hash typed into the address bar is outside the router; SettingsPage asks for that.
export function useUnsavedGuard(dirty, message) {
    useEffect(() => {
        if (!dirty) return undefined;
        const onBeforeUnload = (e) => { e.preventDefault(); e.returnValue = ''; };
        window.addEventListener('beforeunload', onBeforeUnload);
        return () => window.removeEventListener('beforeunload', onBeforeUnload);
    }, [dirty]);

    const shouldBlock = useCallback(
        ({ currentLocation, nextLocation }) => shouldBlockNavigation({ dirty, from: currentLocation, to: nextLocation }),
        [dirty],
    );
    const blocker = useBlocker(shouldBlock);
    useEffect(() => {
        if (blocker.state !== 'blocked') return;
        if (window.confirm(message)) blocker.proceed();
        else blocker.reset();
    }, [blocker, message]);
}
