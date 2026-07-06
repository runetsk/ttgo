/* eslint-disable react-refresh/only-export-components -- STATUS_COLORS is a plain constant map consumed by later execution-mode tasks; splitting it into its own file would ripple imports with no runtime benefit */
import React, { useEffect, useState, useCallback } from 'react';
import { useParams, Link } from 'react-router-dom';
import { getTestRun } from '../api';
import { latestAttempts } from '../utils/runResults';

export const STATUS_COLORS = {
    PASS: 'var(--accent-green)', FAIL: 'var(--accent-red)', ERROR: 'var(--accent-red)',
    PENDING: 'var(--warning-color)', SKIP: '#94a3b8', RUNNING: '#3b82f6',
};

export default function RunExecutePage() {
    const { runId } = useParams();
    const [run, setRun] = useState(null);
    const [queue, setQueue] = useState([]);
    const [currentIdx, setCurrentIdx] = useState(0);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        getTestRun(runId)
            .then(data => {
                if (!data || !data.id) {
                    setRun(null);
                    setLoading(false);
                    return;
                }
                const q = latestAttempts(data.run_results || [])
                    .sort((a, b) => (a.test_name_snapshot || '').localeCompare(b.test_name_snapshot || ''));
                const firstPending = q.findIndex(r => r.status === 'PENDING');
                setRun(data);
                setQueue(q);
                setCurrentIdx(firstPending === -1 ? 0 : firstPending);
                setLoading(false);
            })
            .catch(() => setLoading(false));
    }, [runId]);

    const goPrev = useCallback(() => setCurrentIdx(i => Math.max(0, i - 1)), []);
    const goNext = useCallback(() => setCurrentIdx(i => Math.min(queue.length - 1, i + 1)), [queue.length]);

    if (loading) return <div style={{ padding: 24 }}>Loading run…</div>;
    if (!run) return <div style={{ padding: 24 }}>Run not found</div>;

    const current = queue[currentIdx];
    const executed = queue.filter(r => r.status !== 'PENDING').length;

    return (
        <div className="test-grid-container" data-testid="run-execute-page">
            {/* Header */}
            <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 16 }}>
                <Link to={`/runs/run/${runId}`} data-testid="execute-exit"
                    style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', textDecoration: 'none' }}>
                    ← Exit
                </Link>
                <h2 style={{ fontSize: '1.1rem', fontWeight: 700, margin: 0 }}>{run.name}</h2>
                <span data-testid="execute-progress"
                    style={{ marginLeft: 'auto', fontSize: '0.85rem', fontWeight: 600, color: 'var(--text-secondary)' }}>
                    {executed} / {queue.length} executed
                </span>
            </div>

            {queue.length === 0 ? (
                <div style={{ padding: '48px 0', textAlign: 'center', color: 'var(--text-secondary)' }}>
                    <p>This run has no tests yet. Add tests from the run page first.</p>
                </div>
            ) : (
                <div style={{ display: 'flex', gap: 16, alignItems: 'flex-start' }}>
                    {/* Sidebar queue */}
                    <div style={{
                        width: 260, flexShrink: 0, maxHeight: 'calc(100vh - 180px)', overflowY: 'auto',
                        border: '1px solid var(--border-color)', borderRadius: 10, padding: 6,
                    }}>
                        {queue.map((r, i) => (
                            <button
                                key={r.id}
                                data-testid={`execute-jump-${r.id}`}
                                onClick={() => setCurrentIdx(i)}
                                style={{
                                    display: 'flex', alignItems: 'center', gap: 8, width: '100%',
                                    padding: '7px 10px', borderRadius: 7, border: 'none', textAlign: 'left',
                                    cursor: 'pointer', fontSize: '0.8rem',
                                    background: i === currentIdx ? 'rgba(99,102,241,0.12)' : 'transparent',
                                    color: i === currentIdx ? 'var(--text-primary)' : 'var(--text-secondary)',
                                    fontWeight: i === currentIdx ? 600 : 400,
                                }}
                            >
                                <span style={{
                                    width: 9, height: 9, borderRadius: '50%', flexShrink: 0,
                                    background: STATUS_COLORS[r.status] || '#94a3b8',
                                }} />
                                <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                                    {r.test_name_snapshot}
                                </span>
                            </button>
                        ))}
                    </div>

                    {/* Current test panel */}
                    <div style={{
                        flex: 1, border: '1px solid var(--border-color)', borderRadius: 10,
                        padding: '18px 22px', minHeight: 300,
                    }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 4 }}>
                            <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                                Test {currentIdx + 1} of {queue.length}
                            </span>
                            <span className={`status-badge ${current.status.toLowerCase()}`}>{current.status}</span>
                        </div>
                        <h3 data-testid="execute-current-name" style={{ margin: '4px 0 14px', fontSize: '1.15rem', fontWeight: 700 }}>
                            {current.test_name_snapshot}
                        </h3>

                        {/* Steps panel mounts here (Task 6); verdict bar mounts here (Task 7) */}

                        <div style={{ display: 'flex', gap: 8, marginTop: 18 }}>
                            <button className="action-btn" onClick={goPrev} disabled={currentIdx === 0}
                                data-testid="execute-prev" style={{ padding: '6px 14px' }}>
                                ‹ Prev
                            </button>
                            <button className="action-btn" onClick={goNext} disabled={currentIdx >= queue.length - 1}
                                data-testid="execute-next" style={{ padding: '6px 14px' }}>
                                Next ›
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
