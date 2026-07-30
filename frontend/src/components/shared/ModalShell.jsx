import React, { useId } from 'react';
import useDialogFocus from '../../hooks/useDialogFocus';

export default function ModalShell({ title, subtitle, width = 540, maxHeight, onClose, footer, children }) {
    // Shared by the import, resync and gallery dialogs, none of which contained keyboard focus:
    // Tab walked straight out into the page behind them. Additive — the outside click and each
    // dialog's own keys are untouched, and the hook only claims focus if nothing inside has it.
    const dialogRef = useDialogFocus();
    const titleId = useId();

    return (
        <div className="modal-overlay" onClick={onClose}>
            <div
                ref={dialogRef}
                tabIndex={-1}
                role="dialog"
                aria-modal="true"
                aria-labelledby={titleId}
                className="modal-content"
                onClick={e => e.stopPropagation()}
                style={{
                    width, maxWidth: '90vw',
                    ...(maxHeight ? { maxHeight, display: 'flex', flexDirection: 'column' } : {}),
                }}
            >
                <header className="modal-header">
                    <h3 id={titleId} style={{ margin: 0 }}>
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
