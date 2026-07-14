import { useEffect } from 'react';

// Studio keyboard shortcuts. Guards mirror RunExecutePage's executor keys:
// never fire while typing, never fire when disabled.
export default function useStudioShortcuts({ enabled, onNext, onPrev, onAccept, onReject }) {
    useEffect(() => {
        if (!enabled) return undefined;
        const onKey = (e) => {
            const tag = e.target.tagName;
            if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || e.target.isContentEditable) return;
            if (e.metaKey || e.ctrlKey || e.altKey) return;
            if (e.key === 'j' || e.key === 'ArrowDown') { e.preventDefault(); onNext(); }
            else if (e.key === 'k' || e.key === 'ArrowUp') { e.preventDefault(); onPrev(); }
            else if (e.key === 'a' || e.key === 'A') { e.preventDefault(); onAccept(); }
            else if (e.key === 'r' || e.key === 'R') { e.preventDefault(); onReject(); }
        };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
    }, [enabled, onNext, onPrev, onAccept, onReject]);
}
