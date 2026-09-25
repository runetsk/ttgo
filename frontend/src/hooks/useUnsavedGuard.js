import { useEffect } from 'react';
import { shouldGuardLink } from '../utils/saveBar';

// useUnsavedGuard warns before unsaved changes are lost: the browser's own prompt on close or
// reload, and a confirm on in-app links. The link listener runs in the capture phase, before
// React Router's Link sees the click. navigate() calls from code are not caught (the app uses
// <BrowserRouter>, which has no navigation blocker).
export function useUnsavedGuard(dirty, message) {
    useEffect(() => {
        if (!dirty) return undefined;
        const onBeforeUnload = (e) => { e.preventDefault(); e.returnValue = ''; };
        const onClick = (e) => {
            const a = e.target instanceof Element ? e.target.closest('a[href]') : null;
            if (!a) return;
            const link = { href: a.href, target: a.getAttribute('target'), download: a.hasAttribute('download') };
            if (!shouldGuardLink(link, e, window.location)) return;
            if (!window.confirm(message)) {
                e.preventDefault();
                e.stopPropagation();
            }
        };
        window.addEventListener('beforeunload', onBeforeUnload);
        document.addEventListener('click', onClick, true);
        return () => {
            window.removeEventListener('beforeunload', onBeforeUnload);
            document.removeEventListener('click', onClick, true);
        };
    }, [dirty, message]);
}
