import React, { useState, useEffect } from 'react';

export default function Modal({
    title,
    message,
    defaultValue = "",
    placeholder = "Type here...",
    confirmText = "Confirm",
    cancelText = "Cancel",
    type = "prompt", // "prompt" or "confirm"
    onConfirm,
    onCancel,
    confirmStyle
}) {
    const [value, setValue] = useState(defaultValue);

    useEffect(() => {
        const handleKeyDown = (e) => {
            if (e.key === 'Escape') onCancel();
            if (e.key === 'Enter' && type === 'prompt' && value.trim()) onConfirm(value);
            if (e.key === 'Enter' && type === 'confirm') onConfirm();
        };
        window.addEventListener('keydown', handleKeyDown);
        return () => window.removeEventListener('keydown', handleKeyDown);
    }, [onConfirm, onCancel, type, value]);

    return (
        <div className="modal-overlay" onClick={onCancel}>
            <div className="modal-content" onClick={e => e.stopPropagation()}>
                <header className="modal-header">
                    <h3 className="modal-title">{title}</h3>
                </header>
                <div className="modal-body">
                    {message && <div style={{ marginBottom: 16 }}>{message}</div>}
                    {type === "prompt" && (
                        <input
                            autoFocus
                            className="modern-input"
                            style={{ width: '100%' }}
                            value={value}
                            onChange={e => setValue(e.target.value)}
                            placeholder={placeholder}
                            data-testid="modal-input"
                        />
                    )}
                </div>
                <footer className="modal-footer">
                    <button className="action-btn" onClick={onCancel}>
                        {cancelText}
                    </button>
                    <button
                        className="primary-btn"
                        onClick={() => type === "prompt" ? onConfirm(value) : onConfirm()}
                        style={{ background: (confirmStyle === 'danger' || (type === 'confirm' && confirmText === 'Delete')) ? 'var(--accent-red)' : undefined }}
                        data-testid="modal-confirm-button"
                    >
                        {confirmText}
                    </button>
                </footer>
            </div>
        </div>
    );
}
