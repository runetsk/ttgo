import React, { useEffect, useState, useCallback } from 'react';
import { useParams, Link, useNavigate, useSearchParams } from 'react-router-dom';
import { getTestRun, deleteTestRun, updateTestRun, addRunResult, getTests, listRunDefects, analyzeRunFailures, completeTestRun, reopenTestRun } from '../api';
import { activeColumns } from '../utils/columnFeatures';
import { toast } from '../toast';

import ColumnPicker from '../components/ColumnPicker';
import { useAIGeneration } from '../contexts/AIGenerationContext';
import { useColumnPreference } from '../hooks/useColumnPreference';
import { useColumnWidths } from '../hooks/useColumnWidths';
import { useSubscription } from '../hooks/useSubscription';
import { useWebSocket } from '../hooks/useWebSocket';
import { latestAttempts, applyResultDelta } from '../utils/runResults';
import DefectsTab from './testRunDetail/DefectsTab';
import TimelineTab from './testRunDetail/TimelineTab';
import CompareTab from './testRunDetail/CompareTab';
import ResultsTab from './testRunDetail/ResultsTab';

const RESULT_COLUMN_DEFS = [
    { key: 'test_case',    label: 'Test Case',    mandatory: true,  defaultVisible: true,  defaultWidth: 200 },
    { key: 'status',       label: 'Status',       mandatory: true,  defaultVisible: true,  defaultWidth: 140 },
    { key: 'defect_type',  label: 'Defect Type',  mandatory: false, defaultVisible: true,  defaultWidth: 180 },
    { key: 'defect_links', label: 'Defect Links', mandatory: false, defaultVisible: true,  defaultWidth: 100 },
    { key: 'categories',       label: 'Categories',       mandatory: false, defaultVisible: true,  defaultWidth: 150 },
    { key: 'result_id',    label: 'Result ID',    mandatory: false, defaultVisible: true,  defaultWidth: 180 },
    { key: 'duration',     label: 'Duration',     mandatory: false, defaultVisible: true,  defaultWidth: 120 },
    { key: 'environment',  label: 'Environment',  mandatory: false, defaultVisible: false, defaultWidth: 120 },
    { key: 'browser',      label: 'Browser',      mandatory: false, defaultVisible: false, defaultWidth: 120 },
    { key: 'os',           label: 'OS',           mandatory: false, defaultVisible: false, defaultWidth: 120 },
    { key: 'app_version',  label: 'App Version',  mandatory: false, defaultVisible: false, defaultWidth: 120 },
    { key: 'start_time',   label: 'Start Time',   mandatory: false, defaultVisible: false, defaultWidth: 120 },
    { key: 'end_time',     label: 'End Time',     mandatory: false, defaultVisible: false, defaultWidth: 120 },
    { key: 'failure_type', label: 'Failure Type', mandatory: false, defaultVisible: false, defaultWidth: 120 },
    { key: 'error_message',label: 'Error',        mandatory: false, defaultVisible: false, defaultWidth: 120 },
    { key: 'artifacts',    label: 'Artifacts',    mandatory: false, defaultVisible: false, defaultWidth: 120 },
    { key: 'log_text',     label: 'Log',          mandatory: false, defaultVisible: false, defaultWidth: 120 },
    { key: 'metadata',     label: 'Metadata',     mandatory: false, defaultVisible: false, defaultWidth: 120 },
    { key: 'ai_verdict',   label: 'AI Verdict',   mandatory: false, defaultVisible: false, defaultWidth: 160, feature: 'ai' },
    { key: 'updated_at',   label: 'Updated At',   mandatory: false, defaultVisible: false, defaultWidth: 160 },
    { key: 'attempt_number', label: 'Attempt', mandatory: false, defaultVisible: false, defaultWidth: 80 },
];

const OPTIONAL_COLUMN_KEYS = RESULT_COLUMN_DEFS.filter(c => !c.mandatory).map(c => c.key);

export default function TestRunDetail() {
    const { runId } = useParams();
    const navigate = useNavigate();
    const { aiFeaturesEnabled } = useAIGeneration();
    const [run, setRun] = useState(null);
    const [visibleKeys, toggleColumn, resetColumns] = useColumnPreference('run-detail-results', RESULT_COLUMN_DEFS);
    const { columnWidths, startResize, resetWidths, resetColumnWidth, isResizing } = useColumnWidths('run-detail-results', RESULT_COLUMN_DEFS);
    const featureColumnDefs = activeColumns(RESULT_COLUMN_DEFS, { ai: aiFeaturesEnabled });
    const isVisible = (key) => visibleKeys.has(key) && featureColumnDefs.some(c => c.key === key);
    const handleResetAll = useCallback(() => { resetColumns(); resetWidths(); }, [resetColumns, resetWidths]);
    const [loading, setLoading] = useState(true);
    const [allTests, setAllTests] = useState([]); // For Add Test dropdown
    const [isAddMode, setIsAddMode] = useState(false);
    const [selectedTestToAdd, setSelectedTestToAdd] = useState("");
    const [analysisBannerRefresh, setAnalysisBannerRefresh] = useState(0);
    const [searchParams] = useSearchParams();
    const [activeTab, setActiveTab] = useState(searchParams.get('compareWith') ? 'compare' : 'results');
    const [runDefectLinks, setRunDefectLinks] = useState([]);
    const [defectsLoading, setDefectsLoading] = useState(false);
    const [currentAnalyses, setCurrentAnalyses] = useState({});
    const [showFilters, setShowFilters] = useState(false);

    // 018-websocket-realtime: subscribe to real-time run updates instead of polling
    const { registerRefresh, unregisterRefresh } = useWebSocket();
    useSubscription(runId ? `run:${runId}` : null, useCallback((event) => {
        if (event.type === 'run_result_analysis.created') {
            const d = event.data || {};
            if (!d.run_result_id) return;
            setCurrentAnalyses((prev) => ({
                ...prev,
                [d.run_result_id]: {
                    id: d.analysis_id,
                    version: d.version,
                    verdict: d.verdict,
                    confidence: d.confidence,
                    dedup_group_key: d.dedup_group_key || null,
                },
            }));
            return;
        }
        const d = event.data;
        // Result-level events carry deltas (affected rows + run summary);
        // run-level events still carry the full run.
        if (d && d.run_id && d.run) {
            setRun(prev => applyResultDelta(prev, d));
            return;
        }
        if (d && d.id) {
            setRun(d);
        }
    }, []), { debounceMs: 300, buffer: true });

    // Declared before the mount effect below — its dependency array reads
    // these bindings during render, so declaring them later is a TDZ crash.
    const loadCurrentAnalyses = useCallback(() => {
        if (!runId) return;
        import('../api').then(({ getCurrentRunAnalyses }) =>
            getCurrentRunAnalyses(runId).then(setCurrentAnalyses).catch(() => {})
        );
    }, [runId]);

    const loadRun = useCallback(() => {
        getTestRun(runId)
            .then(data => {
                if (data && data.id) setRun(data);
                else setRun(null);
                setLoading(false);
            })
            .catch(err => {
                console.error(err);
                setLoading(false);
            });
    }, [runId]);

    useEffect(() => {
        loadRun();
        loadAllTests(); // Pre-fetch for "Add Test"
        loadCurrentAnalyses();
        // Register loadRun as refresh callback for reconnection
        registerRefresh('testRunDetail', loadRun);
        return () => unregisterRefresh('testRunDetail');
    }, [runId, loadRun, loadCurrentAnalyses, registerRefresh, unregisterRefresh]);

    const loadRunDefectLinks = useCallback(() => {
        setDefectsLoading(true);
        listRunDefects(runId)
            .then(data => setRunDefectLinks(Array.isArray(data) ? data : []))
            .catch(() => setRunDefectLinks([]))
            .finally(() => setDefectsLoading(false));
    }, [runId]);

    useEffect(() => {
        if (activeTab === 'defects') loadRunDefectLinks();
    }, [activeTab, loadRunDefectLinks]);

    const loadAllTests = () => {
        getTests([], undefined, { view: 'list' })
            .then(data => setAllTests(Array.isArray(data) ? data : []))
            .catch(() => { });
    };

    // Derive latest-attempt-only view and group attempts by test case
    const { latestResults, attemptsByTestCase } = React.useMemo(() => {
        if (!run?.run_results) return { latestResults: [], attemptsByTestCase: {} };
        const byTestCase = {};
        for (const rr of run.run_results) {
            if (!rr.test_case_id) continue;
            (byTestCase[rr.test_case_id] = byTestCase[rr.test_case_id] || []).push(rr);
        }
        // Each test case's attempts newest-first, matching the prior derivation —
        // RunResultDetail's timeline strip reverses this for chronological display.
        for (const tcId in byTestCase) {
            byTestCase[tcId].sort((a, b) => (b.attempt_number || 0) - (a.attempt_number || 0));
        }
        return { latestResults: latestAttempts(run.run_results), attemptsByTestCase: byTestCase };
    }, [run?.run_results]);


    // Collect unique categories from all test results
    const runCategories = React.useMemo(() => {
        if (!run?.run_results) return [];
        const categoryMap = new Map();
        run.run_results.forEach(result => {
            (result.test_case?.categories || []).forEach(s => {
                if (!categoryMap.has(s.id)) categoryMap.set(s.id, s);
            });
        });
        return Array.from(categoryMap.values());
    }, [run]);

    if (loading) return <div>Loading run...</div>;
    if (!run) return <div>Run not found</div>;

    // Calculate stats (latest attempts only)
    const total = latestResults.length;
    const allPassed = latestResults.filter(r => r.status === 'PASS').length;
    const passedAfterRetry = latestResults.filter(r =>
        r.status === 'PASS' && r.test_case_id && attemptsByTestCase[r.test_case_id]?.length > 1
    ).length;
    const passed = allPassed;
    const passedFirstTry = allPassed - passedAfterRetry;
    const failed = latestResults.filter(r => r.status === 'FAIL' || r.status === 'ERROR').length;
    const skipped = latestResults.filter(r => r.status === 'SKIP').length;
    const pending = latestResults.filter(r => r.status === 'PENDING').length;

    const handleStatusChange = async (newStatus) => {
        if (newStatus !== run.status) {
            await updateTestRun(runId, run.name, run.category_id, newStatus);
        }
    };

    const handleComplete = async () => {
        try {
            await completeTestRun(runId);
        } catch {
            toast.error('Failed to complete run');
        }
    };

    const handleReopen = async () => {
        try {
            await reopenTestRun(runId);
        } catch {
            toast.error('Failed to reopen run');
        }
    };

    const handleDelete = async () => {
        if (window.confirm('Are you sure you want to delete this test run?')) {
            await deleteTestRun(runId);
            navigate('/runs');
        }
    };

    const handleRename = async () => {
        const newName = window.prompt("Enter new name:", run.name);
        if (newName && newName !== run.name) {
            await updateTestRun(runId, newName, run.category_id);
        }
    };

    const handleAddTest = async () => {
        if (!selectedTestToAdd) return;
        await addRunResult(runId, selectedTestToAdd);
        setIsAddMode(false);
        setSelectedTestToAdd("");
    };

    // Progress bar segment widths
    const progressSegments = total > 0 ? {
        pass: `${(passedFirstTry / total) * 100}%`,
        passAfterRetry: `${(passedAfterRetry / total) * 100}%`,
        fail: `${(failed / total) * 100}%`,
        pending: `${(pending / total) * 100}%`,
        skip: `${(skipped / total) * 100}%`,
    } : { pass: '0%', passAfterRetry: '0%', fail: '0%', pending: '0%', skip: '0%' };

    return (
        <div className="test-grid-container">
            {/* Row 1 — Title bar */}
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 12 }}>
                <Link to="/runs" style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', textDecoration: 'none' }}>← Runs</Link>
                <span style={{ color: 'var(--text-secondary)', opacity: 0.4 }}>/</span>
                <h2 style={{ fontSize: '1.2rem', fontWeight: 700, margin: 0, letterSpacing: '-0.01em', cursor: 'pointer' }} onClick={handleRename} title="Click to rename" data-testid="run-title">
                    {run.name}
                </h2>
                <select
                    className={`status-select ${run.status.toLowerCase()}`}
                    value={run.status}
                    onChange={e => handleStatusChange(e.target.value)}
                    data-testid="run-status-select"
                >
                    <option value="PENDING">PENDING</option>
                    <option value="RUNNING">RUNNING</option>
                    <option value="PASS">PASS</option>
                    <option value="FAIL">FAIL</option>
                    <option value="SKIP">SKIP</option>
                    <option value="ERROR">ERROR</option>
                </select>
                <div style={{ marginLeft: 'auto', display: 'flex', gap: 6, alignItems: 'center' }}>
                    {(run.status === 'PENDING' || run.status === 'RUNNING' || run.status === 'SKIP') && (
                        <button
                            className="action-btn"
                            onClick={handleComplete}
                            title="Derive PASS/FAIL from the results and finalize the run"
                            style={{ color: 'var(--accent-green)', borderColor: 'rgba(34,197,94,0.3)', background: 'rgba(34,197,94,0.05)', padding: '5px 12px', fontSize: '0.8rem' }}
                            data-testid="complete-run-button"
                        >
                            ✓ Complete Run
                        </button>
                    )}
                    {(run.status === 'PASS' || run.status === 'FAIL') && (
                        <button
                            className="action-btn"
                            onClick={handleReopen}
                            title="Set the run back to RUNNING to keep updating results"
                            style={{ color: 'var(--accent-indigo)', borderColor: 'rgba(99,102,241,0.3)', background: 'rgba(99,102,241,0.05)', padding: '5px 12px', fontSize: '0.8rem' }}
                            data-testid="reopen-run-button"
                        >
                            ↻ Reopen
                        </button>
                    )}
                    <ColumnPicker
                        columnDefs={featureColumnDefs}
                        visibleKeys={visibleKeys}
                        onToggle={toggleColumn}
                        onReset={handleResetAll}
                    />
                    <button
                        className={`action-btn ${showFilters ? 'active' : ''}`}
                        onClick={() => setShowFilters(s => !s)}
                        style={{ padding: '8px 12px', background: showFilters ? 'var(--bg-tertiary)' : 'transparent' }}
                        title="Column Filters"
                    >
                        {showFilters ? 'Hide Filters' : 'Column Filters'}
                    </button>
                    <button
                        className="action-btn"
                        onClick={() => navigate(`/runs/run/${runId}/execute`)}
                        title="Execute tests one by one"
                        style={{ color: 'var(--accent-indigo)', borderColor: 'rgba(99,102,241,0.3)', background: 'rgba(99,102,241,0.05)', padding: '5px 12px', fontSize: '0.8rem' }}
                        data-testid="execute-run-button"
                    >
                        ▶ Execute
                    </button>
                    <button
                        className="action-btn"
                        onClick={() => setIsAddMode(!isAddMode)}
                        style={{ color: 'var(--accent-indigo)', borderColor: 'rgba(99,102,241,0.3)', background: 'rgba(99,102,241,0.05)', padding: '5px 12px', fontSize: '0.8rem' }}
                        data-testid="add-test-to-run-button"
                    >
                        + Add Test
                    </button>
                    {aiFeaturesEnabled && (
                    <button
                        className="action-btn"
                        onClick={async () => {
                            try {
                                await analyzeRunFailures(runId);
                                toast.success('Analysis queued');
                                setAnalysisBannerRefresh(n => n + 1);
                            } catch (e) {
                                if (e.response?.status === 409) {
                                    toast.info ? toast.info('Analysis is already in progress') : toast.success('Analysis is already in progress');
                                    setAnalysisBannerRefresh(n => n + 1);
                                } else {
                                    toast.error('Failed to queue analysis: ' + (e.response?.data?.error || e.message));
                                }
                            }
                        }}
                        disabled={failed === 0}
                        title={failed === 0 ? 'No failed results to analyze' : 'Queue AI analysis for failed results'}
                        style={{ color: 'var(--accent-indigo)', borderColor: 'rgba(99,102,241,0.3)', background: 'rgba(99,102,241,0.05)', padding: '5px 12px', fontSize: '0.8rem', opacity: failed === 0 ? 0.5 : 1 }}
                        data-testid="analyze-failures-button"
                    >
                        Analyze failures
                    </button>
                    )}
                    <button
                        className="action-btn"
                        onClick={handleDelete}
                        style={{ color: 'var(--accent-red)', borderColor: 'rgba(239,68,68,0.2)', background: 'transparent', padding: '5px 12px', fontSize: '0.8rem' }}
                        data-testid="delete-run-button"
                    >
                        Delete
                    </button>
                </div>
            </div>

            {/* Add Test inline form */}
            {isAddMode && (
                <div style={{ marginBottom: 12, display: 'flex', gap: 8, padding: 12, background: 'var(--bg-secondary)', borderRadius: 'var(--radius-md)' }}>
                    <select
                        className="modern-select"
                        value={selectedTestToAdd}
                        onChange={e => setSelectedTestToAdd(e.target.value)}
                        style={{ flex: 1 }}
                        data-testid="add-test-select"
                    >
                        <option value="">Select Test Case to Add...</option>
                        {allTests.filter(t => !run.run_results.find(r => r.test_case_id === t.id)).map(t => (
                            <option key={t.id} value={t.id}>{t.name}</option>
                        ))}
                    </select>
                    <button className="primary-btn" onClick={handleAddTest} disabled={!selectedTestToAdd} data-testid="confirm-add-test-button">Add</button>
                    <button className="text-btn" onClick={() => setIsAddMode(false)} data-testid="cancel-add-test-button">Cancel</button>
                </div>
            )}

            {/* Row 2 — Compact stats bar */}
            <div style={{
                display: 'flex', alignItems: 'center', gap: 14, padding: '10px 16px',
                background: 'var(--glass-bg)', border: '1px solid var(--border-color)',
                borderRadius: 8, marginBottom: 20, flexWrap: 'wrap',
            }}>
                {/* Segmented progress bar */}
                <div style={{ width: 140, height: 7, background: 'rgba(255,255,255,0.06)', borderRadius: 4, overflow: 'hidden', flexShrink: 0, display: 'flex' }} data-testid="stats-progress-bar">
                    {passedFirstTry > 0 && <div style={{ width: progressSegments.pass, height: '100%', background: 'var(--accent-green)' }} />}
                    {passedAfterRetry > 0 && <div style={{ width: progressSegments.passAfterRetry, height: '100%', background: '#856404' }} />}
                    {failed > 0 && <div style={{ width: progressSegments.fail, height: '100%', background: 'var(--accent-red)' }} />}
                    {pending > 0 && <div style={{ width: progressSegments.pending, height: '100%', background: 'var(--warning-color)' }} />}
                    {skipped > 0 && <div style={{ width: progressSegments.skip, height: '100%', background: 'rgba(255,255,255,0.15)' }} />}
                </div>
                <span style={{ fontSize: '0.8rem', fontWeight: 600 }} data-testid="stats-passed">
                    <span style={{ color: 'var(--accent-green)' }}>{passed}</span>
                    <span style={{ color: 'var(--text-secondary)' }}> / {total}</span>
                </span>
                <div style={{ width: 1, height: 16, background: 'var(--border-color)' }} />
                {failed > 0 && <span style={{ fontSize: '0.8rem', color: 'var(--accent-red)', fontWeight: 600 }} data-testid="stats-failed">{failed} failed</span>}
                {failed === 0 && <span style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }} data-testid="stats-failed">0 failed</span>}
                {passedAfterRetry > 0 && <span style={{ fontSize: '0.8rem', color: '#856404', fontWeight: 600 }} data-testid="stats-retried">↻ {passedAfterRetry} passed after retry</span>}
                {skipped > 0 && <span style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }} data-testid="stats-skipped">{skipped} skipped</span>}
                {pending > 0 && <span style={{ fontSize: '0.8rem', color: 'var(--warning-color)' }} data-testid="stats-pending">{pending} pending</span>}
                {pending === 0 && skipped === 0 && (
                    <span style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>0 pending</span>
                )}
                {runCategories.length > 0 && (
                    <>
                        <div style={{ width: 1, height: 16, background: 'var(--border-color)' }} />
                        <div style={{ display: 'flex', gap: 4 }} data-testid="run-categories">
                            {runCategories.map(s => (
                                <span key={s.id} className="category-tag" style={{ fontSize: '0.7rem', padding: '1px 8px' }}>
                                    {s.name}
                                </span>
                            ))}
                        </div>
                    </>
                )}
                <span style={{ marginLeft: 'auto', fontSize: '0.72rem', color: 'var(--text-secondary)', opacity: 0.6 }}>
                    Updated {new Date(run.updated_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                </span>
            </div>

            {/* Tab bar */}
            <div style={{ display: 'flex', gap: 0, marginBottom: 12, borderBottom: '1px solid var(--border-color)' }}>
                {['results', 'defects', 'timeline', 'compare'].map(tab => (
                    <button
                        key={tab}
                        data-testid={`run-tab-${tab}`}
                        onClick={() => setActiveTab(tab)}
                        style={{
                            padding: '8px 20px', fontSize: '0.82rem', fontWeight: 600,
                            background: 'none', border: 'none', cursor: 'pointer',
                            color: activeTab === tab ? 'var(--accent-indigo)' : 'var(--text-secondary)',
                            borderBottom: activeTab === tab ? '2px solid var(--accent-indigo)' : '2px solid transparent',
                            marginBottom: -1, textTransform: 'capitalize',
                        }}
                    >
                        {tab === 'defects' ? `Defects${runDefectLinks.length > 0 ? ` (${new Set(runDefectLinks.map(l => l.id)).size})` : ''}` : tab === 'timeline' ? 'Timeline' : tab === 'compare' ? 'Compare' : 'Results'}
                    </button>
                ))}
            </div>

            {activeTab === 'results' && (
                <ResultsTab
                    runId={runId}
                    latestResults={latestResults}
                    attemptsByTestCase={attemptsByTestCase}
                    aiFeaturesEnabled={aiFeaturesEnabled}
                    analysisBannerRefresh={analysisBannerRefresh}
                    currentAnalyses={currentAnalyses}
                    setCurrentAnalyses={setCurrentAnalyses}
                    showFilters={showFilters}
                    isVisible={isVisible}
                    columnWidths={columnWidths}
                    columnActions={{ startResize, resetColumnWidth, isResizing }}
                    optionalColumnKeys={OPTIONAL_COLUMN_KEYS}
                />
            )}

            {activeTab === 'defects' && (
                <DefectsTab runDefectLinks={runDefectLinks} defectsLoading={defectsLoading} />
            )}

            {activeTab === 'timeline' && (
                <TimelineTab
                    run={run}
                    onNavigateToResult={(resultId) => {
                        setActiveTab('results');
                        setTimeout(() => {
                            const row = document.querySelector(`[data-result-id="${resultId}"]`);
                            if (row) {
                                row.scrollIntoView({ behavior: 'smooth', block: 'center' });
                                row.style.transition = 'background 0.3s';
                                row.style.background = 'rgba(99,102,241,0.15)';
                                setTimeout(() => { row.style.background = ''; }, 1500);
                            }
                        }, 100);
                    }}
                />
            )}
            {activeTab === 'compare' && (
                <CompareTab run={run} />
            )}
        </div>
    );
}
