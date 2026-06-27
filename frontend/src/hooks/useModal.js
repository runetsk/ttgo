import { useState, useCallback } from 'react';

export function useModal() {
    const [state, setState] = useState(null);
    const open = useCallback((message, onConfirm) => setState({ isOpen: true, message, onConfirm }), []);
    const close = useCallback(() => setState(null), []);
    return {
        isOpen: !!state,
        message: state?.message || '',
        onConfirm: state?.onConfirm || null,
        open,
        close,
    };
}
export default useModal;
