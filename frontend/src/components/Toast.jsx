import { useState, useCallback, useEffect, useRef } from 'react';
import { _setHandler } from '../toast';

const STYLES = {
    error:   { accent: 'var(--accent-red)',    bg: 'rgba(239,68,68,0.12)',   border: 'rgba(239,68,68,0.35)'   },
    success: { accent: 'var(--accent-green)',  bg: 'rgba(16,185,129,0.12)',  border: 'rgba(16,185,129,0.35)'  },
    warning: { accent: 'var(--warning-color)', bg: 'rgba(245,158,11,0.12)', border: 'rgba(245,158,11,0.35)' },
    info:    { accent: 'var(--accent-indigo)', bg: 'rgba(99,102,241,0.12)', border: 'rgba(99,102,241,0.35)'  },
};

const ICONS = { error: '✕', success: '✓', warning: '⚠', info: 'ℹ' };

export function ToastProvider({ children }) {
    const [toasts, setToasts] = useState([]);
    const counterRef = useRef(0);
    // Deduplication: track message keys shown in the last second.
    // Prevents StrictMode's double-invoke from firing the same toast twice.
    const recentKeys = useRef(new Set());

    const addToast = useCallback(({ message, type = 'error' }) => {
        const key = `${type}:${message}`;
        if (recentKeys.current.has(key)) return;
        recentKeys.current.add(key);
        setTimeout(() => recentKeys.current.delete(key), 1000);

        const id = ++counterRef.current;
        setToasts(prev => [...prev, { id, message, type }]);
        setTimeout(() => setToasts(prev => prev.filter(t => t.id !== id)), 5000);
    }, []);

    // Register handler so toast.js module can trigger toasts from api.js
    useEffect(() => {
        _setHandler(addToast);
        return () => _setHandler(null);
    }, [addToast]);

    const dismiss = useCallback((id) => {
        setToasts(prev => prev.filter(t => t.id !== id));
    }, []);

    return (
        <>
            {children}
            <ToastContainer toasts={toasts} onDismiss={dismiss} />
        </>
    );
}

function ToastContainer({ toasts, onDismiss }) {
    if (!toasts.length) return null;
    return (
        <div style={{
            position: 'fixed',
            bottom: 24,
            right: 24,
            display: 'flex',
            flexDirection: 'column',
            gap: 8,
            zIndex: 9999,
            pointerEvents: 'none',
        }}>
            {toasts.map(t => (
                <ToastItem key={t.id} toast={t} onDismiss={onDismiss} />
            ))}
        </div>
    );
}

function ToastItem({ toast, onDismiss }) {
    const s = STYLES[toast.type] || STYLES.error;
    return (
        <div style={{
            pointerEvents: 'all',
            display: 'flex',
            alignItems: 'flex-start',
            gap: 10,
            padding: '12px 14px',
            background: s.bg,
            border: `1px solid ${s.border}`,
            borderRadius: 'var(--radius-md)',
            boxShadow: '0 4px 24px rgba(0,0,0,0.35)',
            backdropFilter: 'blur(8px)',
            maxWidth: 360,
            minWidth: 220,
            animation: 'toastSlideIn 0.2s ease',
        }}>
            <span style={{ color: s.accent, fontSize: '0.8rem', fontWeight: 700, flexShrink: 0, marginTop: 2 }}>
                {ICONS[toast.type]}
            </span>
            <span style={{ color: 'var(--text-primary)', fontSize: '0.85rem', flex: 1, lineHeight: 1.45 }}>
                {toast.message}
            </span>
            <button
                onClick={() => onDismiss(toast.id)}
                style={{
                    background: 'none', border: 'none', cursor: 'pointer',
                    color: 'var(--text-secondary)', padding: 0,
                    fontSize: '1.1rem', lineHeight: 1, flexShrink: 0,
                    opacity: 0.6, marginTop: -1,
                }}
            >
                ×
            </button>
        </div>
    );
}
