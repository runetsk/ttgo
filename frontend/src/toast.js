// Module-level toast handler — set by ToastProvider on mount.
// This lets api.js (non-React) fire toasts without needing React context.
let _handler = null;

export const _setHandler = (fn) => { _handler = fn; };

export const toast = {
    error:   (message) => _handler?.({ message, type: 'error' }),
    success: (message) => _handler?.({ message, type: 'success' }),
    warning: (message) => _handler?.({ message, type: 'warning' }),
    info:    (message) => _handler?.({ message, type: 'info' }),
};
