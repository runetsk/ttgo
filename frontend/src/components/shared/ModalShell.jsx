import React from 'react';

export default function ModalShell({ title, subtitle, width = 540, maxHeight, onClose, footer, children }) {
    return (
        <div className="modal-overlay" onClick={onClose}>
            <div
                className="modal-content"
                onClick={e => e.stopPropagation()}
                style={{
                    width, maxWidth: '90vw',
                    ...(maxHeight ? { maxHeight, display: 'flex', flexDirection: 'column' } : {}),
                }}
            >
                <header className="modal-header">
                    <h3 style={{ margin: 0 }}>
                        {title}
                        {subtitle && (
                            <span style={{ fontSize: '0.75rem', fontWeight: 400, color: 'var(--text-secondary)', marginLeft: 8 }}>
                                {subtitle}
                            </span>
                        )}
                    </h3>
                    <button className="modal-close-btn" onClick={onClose}>×</button>
                </header>
                <div className="modal-body" style={maxHeight ? { flex: 1, overflow: 'auto', minHeight: 0 } : undefined}>
                    {children}
                </div>
                {footer && (
                    <footer className="modal-footer">
                        {footer}
                    </footer>
                )}
            </div>
        </div>
    );
}
