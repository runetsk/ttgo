import React from 'react';

function formatDate(iso) {
    if (!iso) return '';
    const d = new Date(iso);
    return d.toLocaleString(undefined, {
        month: 'short', day: 'numeric', year: 'numeric',
        hour: '2-digit', minute: '2-digit',
    });
}

export default function RestoreConfirmDialog({ version, loading, onConfirm, onCancel }) {
    return (
        <div style={styles.backdrop} onClick={e => e.stopPropagation()}>
            <div style={styles.dialog}>
                <h3 style={styles.title}>Restore this version?</h3>
                <p style={styles.body}>
                    The test case will be restored to the version from{' '}
                    <strong>{formatDate(version?.created_at)}</strong>.
                    Current content will be replaced.
                </p>
                <div style={styles.actions}>
                    <button
                        style={styles.cancelBtn}
                        onClick={onCancel}
                        disabled={loading}
                    >
                        Cancel
                    </button>
                    <button
                        style={{ ...styles.confirmBtn, opacity: loading ? 0.6 : 1 }}
                        onClick={onConfirm}
                        disabled={loading}
                    >
                        {loading ? 'Restoring…' : 'Restore'}
                    </button>
                </div>
            </div>
        </div>
    );
}

const styles = {
    backdrop: {
        position: 'fixed', inset: 0, zIndex: 2000,
        background: 'rgba(0,0,0,0.6)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
    },
    dialog: {
        background: '#1a1a2e', borderRadius: 8, padding: 24, maxWidth: 380, width: '90%',
        border: '1px solid #2a2a4a', color: '#e0e0e0', boxShadow: '0 8px 32px rgba(0,0,0,0.5)',
    },
    title: { margin: '0 0 12px', fontSize: 16, fontWeight: 600 },
    body: { fontSize: 13, color: '#ccc', lineHeight: 1.6, margin: '0 0 20px' },
    actions: { display: 'flex', justifyContent: 'flex-end', gap: 10 },
    cancelBtn: {
        background: 'none', border: '1px solid #444', color: '#aaa',
        padding: '6px 16px', borderRadius: 4, cursor: 'pointer', fontSize: 13,
    },
    confirmBtn: {
        background: '#f59e0b', border: 'none', color: '#000',
        padding: '6px 16px', borderRadius: 4, cursor: 'pointer',
        fontSize: 13, fontWeight: 600,
    },
};
