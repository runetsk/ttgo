/* eslint-disable react-refresh/only-export-components -- STATUS_COLORS is a plain constant map consumed by later execution-mode tasks; splitting it into its own file would ripple imports with no runtime benefit */
import React, { useEffect, useState, useCallback } from 'react';
import { useParams, Link } from 'react-router-dom';
import { getTestRun, getTest } from '../api';
import { latestAttempts } from '../utils/runResults';
import SafeHTML from '../components/shared/SafeHTML';

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
    const [caseCache, setCaseCache] = useState({}); // test_case_id -> test case (or {steps: []} on error)

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

    const currentTestCaseId = queue[currentIdx]?.test_case_id || null;
    useEffect(() => {
        if (!currentTestCaseId || caseCache[currentTestCaseId]) return;
        let cancelled = false;
        getTest(currentTestCaseId)
            .then(tc => { if (!cancelled) setCaseCache(prev => ({ ...prev, [currentTestCaseId]: tc })); })
            .catch(() => { if (!cancelled) setCaseCache(prev => ({ ...prev, [currentTestCaseId]: { steps: [] } })); });
        return () => { cancelled = true; };
    }, [currentTestCaseId, caseCache]);

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

                        {(() => {
                            if (!current.test_case_id) {
                                return (
                                    <p style={{ fontSize: '0.83rem', color: 'var(--text-secondary)' }}>
                                        The original test case was deleted — only the recorded name remains.
                                    </p>
                                );
                            }
                            const tc = caseCache[current.test_case_id];
                            if (!tc) return <p style={{ fontSize: '0.83rem', color: 'var(--text-secondary)' }}>Loading steps…</p>;
                            const steps = [...(tc.steps || [])].sort((a, b) => (a.order_index || 0) - (b.order_index || 0));
                            return (
                                <div data-testid="execute-steps">
                                    {tc.description && (
                                        <SafeHTML html={tc.description}
                                            style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', marginBottom: 12 }} />
                                    )}
                                    {steps.length === 0 ? (
                                        <p style={{ fontSize: '0.83rem', color: 'var(--text-secondary)' }}>
                                            No steps authored for this test case.
                                        </p>
                                    ) : steps.map((s, i) => (
                                        <div key={s.id || i} data-testid={`execute-step-${s.order_index}`}
                                            style={{
                                                display: 'flex', gap: 12, padding: '10px 12px', marginBottom: 8,
                                                background: 'var(--bg-secondary)', borderRadius: 8,
                                                border: '1px solid var(--border-color)',
                                            }}>
                                            <span style={{
                                                minWidth: 22, height: 22, borderRadius: '50%', flexShrink: 0,
                                                background: 'var(--bg-primary)', border: '1px solid var(--border-color)',
                                                color: 'var(--text-secondary)', fontSize: '0.7rem', fontWeight: 600,
                                                display: 'flex', alignItems: 'center', justifyContent: 'center',
                                            }}>{i + 1}</span>
                                            <div style={{ flex: 1, minWidth: 0 }}>
                                                <SafeHTML html={s.action} style={{ fontSize: '0.87rem' }} />
                                                {s.expected_result && (
                                                    <div style={{ marginTop: 4, display: 'flex', gap: 6, alignItems: 'baseline' }}>
                                                        <span style={{ fontSize: '0.7rem', fontWeight: 700, color: 'var(--accent-green)', textTransform: 'uppercase', flexShrink: 0 }}>Expected</span>
                                                        <SafeHTML html={s.expected_result} style={{ fontSize: '0.82rem', color: 'var(--text-secondary)' }} />
                                                    </div>
                                                )}
                                            </div>
                                        </div>
                                    ))}
                                </div>
                            );
                        })()}

                        {/* Verdict bar mounts here (Task 7) */}

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
