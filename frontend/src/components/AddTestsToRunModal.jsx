import React, { useState } from 'react';
import { addRunResultsBulk } from '../api';
import TestTreePicker from './TestTreePicker';

// Modal for adding more tests to an existing run, mirroring the New Test Run
// modal's tree picker. Tests already in the run are passed as lockedIds so they
// render checked + disabled and cannot be re-added.
export default function AddTestsToRunModal({ runId, existingTestCaseIds, onClose, onAdded }) {
    const [selectedIds, setSelectedIds] = useState(() => new Set());
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState(null);

    const count = selectedIds.size;

    const add = () => {
        if (count === 0) return;
        setLoading(true);
        setError(null);
        addRunResultsBulk(runId, Array.from(selectedIds))
            .then(() => { setLoading(false); onAdded(); })
            .catch(err => { setLoading(false); setError(err?.response?.data?.error || 'Failed to add tests'); });
    };

    const handleKeyDown = (e) => { if (e.key === 'Escape') onClose(); };

    return (
        <div className="modal-overlay" onClick={onClose} onKeyDown={handleKeyDown}>
            <div className="modal-content" onClick={e => e.stopPropagation()} style={{ maxWidth: 620, width: '92vw', padding: 0, overflow: 'hidden', display: 'flex', flexDirection: 'column', maxHeight: '86vh' }}>

                {/* Header */}
                <div style={{ padding: '20px 24px 16px', borderBottom: '1px solid var(--border-color)', display: 'flex', alignItems: 'center', gap: 12 }}>
                    <div style={{ width: 36, height: 36, borderRadius: 9, background: 'linear-gradient(135deg, #6366f1, #8b5cf6)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '1.2rem', flexShrink: 0, color: '#fff' }}>+</div>
                    <div style={{ flex: 1 }}>
                        <h3 style={{ margin: 0, fontSize: '1.05rem', fontWeight: 700, color: 'var(--text-primary)' }}>Add Tests to Run</h3>
                        <p style={{ margin: 0, fontSize: '0.78rem', color: 'var(--text-secondary)', marginTop: 1 }}>Pick tests from the library to add to this run</p>
                    </div>
                    <button onClick={onClose} style={{ background: 'none', border: 'none', color: 'var(--text-secondary)', fontSize: '1.2rem', cursor: 'pointer', padding: '4px 8px', borderRadius: 6, lineHeight: 1 }} title="Close">&times;</button>
                </div>

                {/* Body */}
                <div style={{ padding: '20px 24px', display: 'flex', flexDirection: 'column', gap: 16, overflowY: 'auto' }}>
                    {error && (
                        <div style={{ padding: '10px 14px', borderRadius: 8, background: 'rgba(239,68,68,0.08)', border: '1px solid rgba(239,68,68,0.25)', color: '#ef4444', fontSize: '0.83rem' }} data-testid="add-tests-error">{error}</div>
                    )}
                    <TestTreePicker selectedIds={selectedIds} onChange={setSelectedIds} lockedIds={existingTestCaseIds} />
                </div>

                {/* Footer */}
                <div style={{ padding: '14px 24px', borderTop: '1px solid var(--border-color)', background: 'var(--bg-secondary)', display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 8 }}>
                    <button type="button" className="action-btn" onClick={onClose} disabled={loading} style={{ padding: '8px 18px', fontSize: '0.85rem', marginRight: 'auto' }} data-testid="add-tests-cancel">Cancel</button>
                    <button type="button" className="primary-btn" onClick={add} disabled={loading || count === 0} style={{ padding: '8px 20px', fontSize: '0.85rem', opacity: (loading || count === 0) ? 0.5 : 1 }} data-testid="add-tests-submit">
                        {loading ? 'Adding…' : `Add ${count} test${count === 1 ? '' : 's'}`}
                    </button>
                </div>
            </div>
        </div>
    );
}
