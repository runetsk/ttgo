import React, { useEffect, useState, useCallback, useRef } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import { getTestRun, getTest, updateRunResult, completeTestRun, uploadScreenshots } from '../api';
import { latestAttempts } from '../utils/runResults';
import SafeHTML from '../components/shared/SafeHTML';
import { toast } from '../toast';
import { STATUS_COLORS } from '../utils/statusColors';
import { buildStepResults, parseStepVerdicts } from '../utils/stepResults';

export default function RunExecutePage() {
    const { runId } = useParams();
    const navigate = useNavigate();
    const [run, setRun] = useState(null);
    const [queue, setQueue] = useState([]);
    const [currentIdx, setCurrentIdx] = useState(0);
    const [loading, setLoading] = useState(true);
    const [caseCache, setCaseCache] = useState({}); // test_case_id -> test case (or {steps: []} on error)
    const [failPanelOpen, setFailPanelOpen] = useState(false);
    const [defectType, setDefectType] = useState('to_investigate');
    const [failNote, setFailNote] = useState('');
    const [failShotCount, setFailShotCount] = useState(0);
    const [uploadingFailShots, setUploadingFailShots] = useState(false);
    const [stepVerdicts, setStepVerdicts] = useState({}); // order_index -> { status, note }
    const stepSaveTimer = useRef(null);
    const startedAtRef = useRef(null);

    // Restart the informal timer whenever a different test becomes current.
    useEffect(() => {
        startedAtRef.current = Date.now();
        setFailPanelOpen(false);
        setDefectType('to_investigate');
        setFailNote('');
        setFailShotCount(0);
        setUploadingFailShots(false);
        setStepVerdicts(parseStepVerdicts(queue[currentIdx]?.steps));
    }, [currentIdx, queue]);

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

    // Advance to the next PENDING test after the current one (wrapping around);
    // stay put when none remain — the done banner takes over.
    const advance = useCallback((fromIdx, updatedQueue) => {
        const n = updatedQueue.length;
        for (let step = 1; step <= n; step++) {
            const i = (fromIdx + step) % n;
            if (updatedQueue[i].status === 'PENDING') {
                setCurrentIdx(i);
                return;
            }
        }
    }, []);

    const submitVerdict = useCallback(async (status) => {
        const r = queue[currentIdx];
        if (!r || r.status === status) return;
        if (stepSaveTimer.current) clearTimeout(stepSaveTimer.current);
        const payload = { status, duration_ms: Date.now() - startedAtRef.current };
        if (status === 'FAIL') {
            payload.defect_type = defectType;
            if (failNote.trim()) payload.error_message = failNote.trim();
        }
        const markedTc = r.test_case_id ? caseCache[r.test_case_id] : null;
        if (markedTc && Object.keys(stepVerdicts).length > 0) {
            payload.steps = buildStepResults(markedTc.steps || [], stepVerdicts);
        }
        try {
            await updateRunResult(runId, r.id, payload);
        } catch {
            toast.error('Failed to save verdict');
            return; // leave the queue untouched; the user can retry
        }
        const updated = queue.map((item, i) =>
            i === currentIdx
                ? { ...item, status, defect_type: payload.defect_type ?? item.defect_type, error_message: payload.error_message ?? item.error_message, steps: payload.steps ?? item.steps }
                : item
        );
        setQueue(updated);
        setFailPanelOpen(false);
        advance(currentIdx, updated);
    }, [queue, currentIdx, runId, defectType, failNote, advance, caseCache, stepVerdicts]);

    const persistSteps = useCallback((verdicts) => {
        const r = queue[currentIdx];
        const tc = r?.test_case_id ? caseCache[r.test_case_id] : null;
        if (!r || !tc) return;
        const steps = buildStepResults(tc.steps || [], verdicts);
        if (stepSaveTimer.current) clearTimeout(stepSaveTimer.current);
        stepSaveTimer.current = setTimeout(() => {
            updateRunResult(runId, r.id, { steps }).catch(() => toast.error('Failed to save step result'));
        }, 500);
    }, [queue, currentIdx, caseCache, runId]);

    const markStep = useCallback((orderIndex, status) => {
        const next = { ...stepVerdicts, [orderIndex]: { status, note: stepVerdicts[orderIndex]?.note || '' } };
        setStepVerdicts(next);
        persistSteps(next);
        if (status === 'FAIL') setFailPanelOpen(true);
    }, [stepVerdicts, persistSteps]);

    const setStepNote = useCallback((orderIndex, note) => {
        const next = { ...stepVerdicts, [orderIndex]: { status: stepVerdicts[orderIndex]?.status || '', note } };
        setStepVerdicts(next);
        persistSteps(next);
    }, [stepVerdicts, persistSteps]);

    useEffect(() => {
        const onKey = (e) => {
            const tag = e.target.tagName;
            if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || e.target.isContentEditable) return;
            if (failPanelOpen) return; // don't fire verdicts under the fail form
            if (e.key === 'ArrowRight') { e.preventDefault(); goNext(); }
            else if (e.key === 'ArrowLeft') { e.preventDefault(); goPrev(); }
            else {
                // Verdict keys disarm once the run is fully executed so a stray keypress
                // can't mutate a resolved result; arrows keep working for review.
                const allDone = queue.length > 0 && queue.every(r => r.status !== 'PENDING');
                if (allDone) return;
                if (e.key === 'p' || e.key === 'P') submitVerdict('PASS');
                else if (e.key === 's' || e.key === 'S') submitVerdict('SKIP');
                else if (e.key === 'f' || e.key === 'F') setFailPanelOpen(true);
            }
        };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
    }, [failPanelOpen, goNext, goPrev, submitVerdict, queue]);

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
                <>
                    {queue.length > 0 && queue.every(r => r.status !== 'PENDING') && (
                        <div data-testid="execute-done-banner" style={{
                            display: 'flex', alignItems: 'center', gap: 12, padding: '12px 16px',
                            marginBottom: 14, borderRadius: 10,
                            background: 'rgba(34,197,94,0.08)', border: '1px solid rgba(34,197,94,0.3)',
                        }}>
                            <span style={{ fontSize: '0.9rem', fontWeight: 600, color: 'var(--accent-green)' }}>
                                All tests executed.
                            </span>
                            <button
                                className="primary-btn"
                                data-testid="execute-complete-run"
                                onClick={async () => {
                                    try {
                                        await completeTestRun(runId);
                                        navigate(`/runs/run/${runId}`);
                                    } catch {
                                        toast.error('Failed to complete run'); // stay on the page; user can exit manually
                                    }
                                }}
                                style={{ marginLeft: 'auto', padding: '7px 18px', fontSize: '0.85rem' }}
                            >
                                ✓ Complete Run
                            </button>
                        </div>
                    )}
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
                                    ) : steps.map((s, i) => {
                                        const v = stepVerdicts[s.order_index] || { status: '', note: '' };
                                        const btn = (active, color, bg) => ({
                                            width: 30, height: 30, borderRadius: 7, cursor: 'pointer', fontSize: '0.9rem',
                                            display: 'flex', alignItems: 'center', justifyContent: 'center',
                                            color: active ? '#fff' : color,
                                            background: active ? color : bg,
                                            border: `1px solid ${active ? color : 'var(--border-color)'}`,
                                        });
                                        return (
                                        <div key={s.id || i} data-testid={`execute-step-${s.order_index}`}
                                            style={{
                                                display: 'flex', flexDirection: 'column', gap: 8, padding: '10px 12px', marginBottom: 8,
                                                background: v.status === 'FAIL' ? 'rgba(239,68,68,0.06)' : 'var(--bg-secondary)', borderRadius: 8,
                                                border: `1px solid ${v.status === 'FAIL' ? 'rgba(239,68,68,0.3)' : 'var(--border-color)'}`,
                                            }}>
                                            <div style={{ display: 'flex', gap: 12, alignItems: 'flex-start' }}>
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
                                                <div style={{ display: 'flex', gap: 4, flexShrink: 0 }}>
                                                    <button data-testid={`execute-step-pass-${s.order_index}`} title="Pass"
                                                        onClick={() => markStep(s.order_index, 'PASS')}
                                                        style={btn(v.status === 'PASS', 'var(--accent-green)', 'rgba(34,197,94,0.1)')}>✓</button>
                                                    <button data-testid={`execute-step-fail-${s.order_index}`} title="Fail"
                                                        onClick={() => markStep(s.order_index, 'FAIL')}
                                                        style={btn(v.status === 'FAIL', 'var(--accent-red)', 'rgba(239,68,68,0.1)')}>✕</button>
                                                    <button data-testid={`execute-step-skip-${s.order_index}`} title="Skip"
                                                        onClick={() => markStep(s.order_index, 'SKIP')}
                                                        style={btn(v.status === 'SKIP', '#9ca3af', 'rgba(156,163,175,0.1)')}>⊘</button>
                                                </div>
                                            </div>
                                            {v.status && (
                                                <input type="text" className="modern-input"
                                                    data-testid={`execute-step-note-${s.order_index}`}
                                                    placeholder="Note (optional)" value={v.note}
                                                    onChange={e => setStepNote(s.order_index, e.target.value)}
                                                    style={{ fontSize: '0.8rem', marginLeft: 34, width: 'calc(100% - 34px)' }} />
                                            )}
                                        </div>
                                        );
                                    })}
                                </div>
                            );
                        })()}

                        <div style={{ display: 'flex', gap: 8, marginTop: 18, flexWrap: 'wrap' }}>
                            <button data-testid="execute-pass" onClick={() => submitVerdict('PASS')}
                                style={{ padding: '9px 22px', borderRadius: 8, cursor: 'pointer', fontWeight: 700, fontSize: '0.9rem', color: 'var(--accent-green)', background: 'rgba(34,197,94,0.12)', border: '1px solid rgba(34,197,94,0.35)' }}>
                                ✓ Pass
                            </button>
                            <button data-testid="execute-fail" onClick={() => setFailPanelOpen(true)}
                                style={{ padding: '9px 22px', borderRadius: 8, cursor: 'pointer', fontWeight: 700, fontSize: '0.9rem', color: 'var(--accent-red)', background: 'rgba(239,68,68,0.12)', border: '1px solid rgba(239,68,68,0.35)' }}>
                                ✕ Fail
                            </button>
                            <button data-testid="execute-skip" onClick={() => submitVerdict('SKIP')}
                                style={{ padding: '9px 22px', borderRadius: 8, cursor: 'pointer', fontWeight: 700, fontSize: '0.9rem', color: '#9ca3af', background: 'rgba(156,163,175,0.12)', border: '1px solid rgba(156,163,175,0.35)' }}>
                                ⊘ Skip
                            </button>
                            <span style={{ alignSelf: 'center', marginLeft: 8, fontSize: '0.72rem', color: 'var(--text-secondary)', opacity: 0.7 }}>
                                Keys: P pass · F fail · S skip · ←/→ navigate
                            </span>
                        </div>

                        {failPanelOpen && (
                            <div style={{
                                marginTop: 12, padding: 14, borderRadius: 10,
                                background: 'rgba(239,68,68,0.05)', border: '1px solid rgba(239,68,68,0.25)',
                                display: 'flex', flexDirection: 'column', gap: 10,
                            }}>
                                <select className="modern-select" value={defectType}
                                    onChange={e => setDefectType(e.target.value)}
                                    data-testid="execute-defect-type" style={{ maxWidth: 260 }}>
                                    <option value="to_investigate">🔍 To Investigate</option>
                                    <option value="product_bug">🐞 Product Bug</option>
                                    <option value="automation_bug">🤖 Automation Bug</option>
                                    <option value="system_issue">⚙️ System Issue</option>
                                </select>
                                <textarea className="modern-input" rows={3} placeholder="What went wrong? (optional)"
                                    value={failNote} onChange={e => setFailNote(e.target.value)}
                                    data-testid="execute-fail-note" style={{ width: '100%', resize: 'vertical' }} />
                                <label
                                    data-testid="execute-attach-screenshots"
                                    style={{
                                        display: 'inline-flex', alignItems: 'center', gap: 6, alignSelf: 'flex-start',
                                        padding: '5px 12px', borderRadius: 7, cursor: uploadingFailShots ? 'default' : 'pointer',
                                        border: '1px solid var(--border-color)', background: 'var(--bg-secondary)',
                                        color: 'var(--text-secondary)', fontSize: '0.78rem', fontWeight: 600,
                                        opacity: uploadingFailShots ? 0.6 : 1,
                                    }}
                                >
                                    {"📎"} {uploadingFailShots ? 'Uploading…' : failShotCount > 0 ? `${failShotCount} attached — add more` : 'Attach screenshots'}
                                    <input
                                        type="file"
                                        accept="image/png,image/jpeg,image/gif,image/webp"
                                        multiple
                                        disabled={uploadingFailShots}
                                        data-testid="execute-attach-screenshots-input"
                                        style={{ display: 'none' }}
                                        onChange={async (e) => {
                                            const files = Array.from(e.target.files || []);
                                            e.target.value = '';
                                            const r = queue[currentIdx];
                                            if (files.length === 0 || !r) return;
                                            setUploadingFailShots(true);
                                            try {
                                                await uploadScreenshots(runId, r.id, files);
                                                setFailShotCount(c => c + files.length);
                                                toast.success(`${files.length} screenshot${files.length > 1 ? 's' : ''} attached`);
                                            } catch {
                                                toast.error('Failed to upload screenshots');
                                            } finally {
                                                setUploadingFailShots(false);
                                            }
                                        }}
                                    />
                                </label>
                                <div style={{ display: 'flex', gap: 8 }}>
                                    <button className="primary-btn" onClick={() => submitVerdict('FAIL')}
                                        data-testid="execute-fail-confirm" style={{ padding: '7px 18px', fontSize: '0.85rem' }}>
                                        Mark Failed
                                    </button>
                                    <button className="text-btn" onClick={() => setFailPanelOpen(false)}
                                        data-testid="execute-fail-cancel">
                                        Cancel
                                    </button>
                                </div>
                            </div>
                        )}

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
                </>
            )}
        </div>
    );
}
