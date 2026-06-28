import React, { useEffect, useState, useCallback } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import { getTestRun, deleteTestRun, updateTestRun, deleteRunResult, addRunResult, updateRunResult, retryRunResult, bulkUpdateRunResults, getTests, listRunComments, listRunDefectLinks, analyzeRunFailures, getCategories } from '../api';
import DateRangeFilter from '../components/filters/DateRangeFilter';
import CategoryFilter from '../components/filters/CategoryFilter';
import { inDateRange } from '../utils/dateFilter';
import { activeColumns } from '../utils/columnFeatures';
import { toast } from '../toast';

import RunResultDetail from '../components/RunResultDetail';
import RunTimeline from '../components/RunTimeline';
import CommentsPanel from '../components/CommentsPanel';
import ColumnPicker from '../components/ColumnPicker';
import AIVerdictBadge from '../components/AIVerdictBadge';
import RunAnalysisBanner from '../components/RunAnalysisBanner';
import { useAIGeneration } from '../contexts/AIGenerationContext';
import { useColumnPreference } from '../hooks/useColumnPreference';
import { useColumnWidths } from '../hooks/useColumnWidths';
import { useSubscription } from '../hooks/useSubscription';
import { useWebSocket } from '../hooks/useWebSocket';
import RunResultsToolbar from '../components/RunResultsToolbar';
import { useRunViewPreference } from '../hooks/useRunViewPreference';
import { groupResults, GROUP_DIMENSIONS } from '../utils/runResultsGrouping';

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
    const [expandedResults, setExpandedResults] = useState(new Set());
    const [analysisBannerRefresh, setAnalysisBannerRefresh] = useState(0);
    const [selectedResults, setSelectedResults] = useState(new Set());
    const lastClickedRef = React.useRef(null);
    const [sortConfig, setSortConfig] = useState({ key: null, direction: 'asc' });
    const [activeTab, setActiveTab] = useState('results');
    const [showRunComments, setShowRunComments] = useState(false);
    const [runCommentCount, setRunCommentCount] = useState(0);
    const [latestCommentTime, setLatestCommentTime] = useState(null);
    const [runDefectLinks, setRunDefectLinks] = useState([]);
    const [defectsLoading, setDefectsLoading] = useState(false);
    const [expandedDefects, setExpandedDefects] = useState(new Set());
    const [currentAnalyses, setCurrentAnalyses] = useState({});
    const { view, groupBy, setView, setGroupBy } = useRunViewPreference();
    const [collapsedGroups, setCollapsedGroups] = useState(new Set());
    const [showFilters, setShowFilters] = useState(false);
    const [resultCategories, setResultCategories] = useState([]);
    const [resultFilters, setResultFilters] = useState({
        test_case: '', status: '', defect_type: '', result_id: '',
        categories: [],
        start_time: { from: null, to: null },
        end_time: { from: null, to: null },
        updated_at: { from: null, to: null },
    });
    const toggleGroup = (key) => setCollapsedGroups(prev => {
        const next = new Set(prev);
        if (next.has(key)) next.delete(key); else next.add(key);
        return next;
    });

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
        if (event.data && event.data.id) {
            setRun(event.data);
        }
    }, []), { debounceMs: 300 });

    useEffect(() => {
        loadRun();
        loadAllTests(); // Pre-fetch for "Add Test"
        loadCurrentAnalyses();
        // Register loadRun as refresh callback for reconnection
        registerRefresh('testRunDetail', loadRun);
        return () => unregisterRefresh('testRunDetail');
    }, [runId]);

    useEffect(() => {
        if (runId) {
            listRunComments(runId).then(data => {
                const list = data || [];
                setRunCommentCount(list.length);
                if (list.length > 0) setLatestCommentTime(list[list.length - 1].created_at);
            }).catch(() => {});
        }
    }, [runId]);

    useEffect(() => {
        getCategories(1, 200).then(d => setResultCategories(d.categories || [])).catch(() => setResultCategories([]));
    }, []);

    const loadRunDefectLinks = useCallback(() => {
        setDefectsLoading(true);
        listRunDefectLinks(runId)
            .then(data => setRunDefectLinks(Array.isArray(data) ? data : []))
            .catch(() => setRunDefectLinks([]))
            .finally(() => setDefectsLoading(false));
    }, [runId]);

    const loadCurrentAnalyses = useCallback(() => {
        if (!runId) return;
        import('../api').then(({ getCurrentRunAnalyses }) =>
            getCurrentRunAnalyses(runId).then(setCurrentAnalyses).catch(() => {})
        );
    }, [runId]);

    useEffect(() => {
        if (activeTab === 'defects') loadRunDefectLinks();
    }, [activeTab, loadRunDefectLinks]);

    const relativeTime = (isoStr) => {
        if (!isoStr) return '';
        const diff = Date.now() - new Date(isoStr).getTime();
        const mins = Math.floor(diff / 60000);
        if (mins < 1) return 'just now';
        if (mins < 60) return `${mins}m ago`;
        const hrs = Math.floor(mins / 60);
        if (hrs < 24) return `${hrs}h ago`;
        return `${Math.floor(hrs / 24)}d ago`;
    };

    const loadAllTests = () => {
        getTests([], undefined, { view: 'list' })
            .then(data => setAllTests(Array.isArray(data) ? data : []))
            .catch(() => { });
    };

    const loadRun = () => {
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
    };

    const toggleResult = (id) => {
        const newSet = new Set(expandedResults);
        if (newSet.has(id)) newSet.delete(id);
        else newSet.add(id);
        setExpandedResults(newSet);
    };

    const handleSort = (key) => {
        let direction = 'asc';
        if (sortConfig.key === key && sortConfig.direction === 'asc') {
            direction = 'desc';
        }
        setSortConfig({ key, direction });
    };

    // Derive latest-attempt-only view and group attempts by test case
    const { latestResults, attemptsByTestCase } = React.useMemo(() => {
        if (!run?.run_results) return { latestResults: [], attemptsByTestCase: {} };
        const byTestCase = {};
        const orphans = [];
        for (const rr of run.run_results) {
            if (!rr.test_case_id) { orphans.push(rr); continue; }
            if (!byTestCase[rr.test_case_id]) byTestCase[rr.test_case_id] = [];
            byTestCase[rr.test_case_id].push(rr);
        }
        const latest = [...orphans];
        for (const tcId in byTestCase) {
            byTestCase[tcId].sort((a, b) => b.attempt_number - a.attempt_number);
            latest.push(byTestCase[tcId][0]);
        }
        return { latestResults: latest, attemptsByTestCase: byTestCase };
    }, [run?.run_results]);


    const sortedResults = React.useMemo(() => {
        if (!latestResults.length) return [];
        let sortableItems = [...latestResults];
        if (sortConfig.key) {
            sortableItems.sort((a, b) => {
                let aValue = a[sortConfig.key];
                let bValue = b[sortConfig.key];

                // Handle special cases
                if (sortConfig.key === 'duration_ms') {
                    aValue = Number(aValue || 0);
                    bValue = Number(bValue || 0);
                }

                if (aValue < bValue) return sortConfig.direction === 'asc' ? -1 : 1;
                if (aValue > bValue) return sortConfig.direction === 'asc' ? 1 : -1;
                return 0;
            });
        }
        return sortableItems;
    }, [latestResults, sortConfig]);

    const filteredResults = React.useMemo(() => {
        const f = resultFilters;
        const nameQ = f.test_case.toLowerCase();
        const idQ = f.result_id.toLowerCase();
        return sortedResults.filter(r => {
            if (nameQ && !(r.test_name_snapshot || '').toLowerCase().includes(nameQ)) return false;
            if (idQ && !(r.id || '').toLowerCase().includes(idQ)) return false;
            if (f.status && r.status !== f.status) return false;
            if (f.defect_type) {
                const dt = r.defect_type || 'to_investigate';
                if (dt !== f.defect_type) return false;
            }
            if (f.categories.length > 0) {
                const cats = r.test_case?.categories || [];
                if (!cats.some(c => f.categories.includes(c.id))) return false;
            }
            if (!inDateRange(r.start_time, f.start_time)) return false;
            if (!inDateRange(r.end_time, f.end_time)) return false;
            if (!inDateRange(r.updated_at, f.updated_at)) return false;
            return true;
        });
    }, [sortedResults, resultFilters]);

    // Run-results grouping (List vs Grouped view). The ai_verdict dimension is
    // hidden when AI features are off, mirroring the column-visibility gating.
    const groupDimensions = aiFeaturesEnabled
        ? GROUP_DIMENSIONS
        : GROUP_DIMENSIONS.filter(d => d.value !== 'ai_verdict');
    const effectiveGroupBy = (!aiFeaturesEnabled && groupBy === 'ai_verdict') ? 'status' : groupBy;
    const groupedResults = React.useMemo(
        () => (view === 'grouped' ? groupResults(filteredResults, effectiveGroupBy, currentAnalyses) : []),
        [view, effectiveGroupBy, filteredResults, currentAnalyses],
    );
    const collapseAll = () => setCollapsedGroups(new Set(groupedResults.map(g => g.key)));
    const expandAll = () => setCollapsedGroups(new Set());

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

    const formatDuration = (ms) => {
        if (!ms) return '-';
        return ms < 1000 ? `${ms}ms` : `${(ms / 1000).toFixed(2)}s`;
    };

    const formatDateTime = (ts) => {
        if (!ts || ts === '0001-01-01T00:00:00Z') return '—';
        return new Date(ts).toLocaleString();
    };

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
    const failedResults = latestResults.filter(r => r.status === 'FAIL' || r.status === 'ERROR');
    const productBug = failedResults.filter(r => r.defect_type === 'product_bug').length;
    const automationBug = failedResults.filter(r => r.defect_type === 'automation_bug').length;
    const systemIssue = failedResults.filter(r => r.defect_type === 'system_issue').length;
    const toInvestigate = failedResults.filter(r => r.defect_type === 'to_investigate' || !r.defect_type).length;

    const handleStatusChange = async (newStatus) => {
        if (newStatus !== run.status) {
            await updateTestRun(runId, run.name, run.category_id, newStatus);
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

    const handleDeleteResult = async (resultId) => {
        if (window.confirm('Remove this test from the run?')) {
            await deleteRunResult(runId, resultId);
        }
    };

    const handleUpdateResult = async (resultId, status) => {
        await updateRunResult(runId, resultId, status);
    };

    const handleUpdateDefectType = async (resultId, defectType) => {
        await updateRunResult(runId, resultId, { defect_type: defectType });
    };

    const handleRetryResult = async (resultId) => {
        try {
            await retryRunResult(runId, resultId);
        } catch (err) {
            console.error('Failed to retry result:', err);
        }
    };

    const toggleSelectResult = (id, e) => {
        // Shift-click range selection
        if (e?.shiftKey && lastClickedRef.current && lastClickedRef.current !== id) {
            const ids = sortedResults.map(r => r.id);
            const from = ids.indexOf(lastClickedRef.current);
            const to = ids.indexOf(id);
            if (from !== -1 && to !== -1) {
                const [start, end] = from < to ? [from, to] : [to, from];
                setSelectedResults(prev => {
                    const next = new Set(prev);
                    for (let i = start; i <= end; i++) next.add(ids[i]);
                    return next;
                });
                lastClickedRef.current = id;
                return;
            }
        }
        lastClickedRef.current = id;
        setSelectedResults(prev => {
            const next = new Set(prev);
            if (next.has(id)) next.delete(id);
            else next.add(id);
            return next;
        });
    };

    const toggleSelectAll = () => {
        if (selectedResults.size === filteredResults.length) {
            setSelectedResults(new Set());
        } else {
            setSelectedResults(new Set(filteredResults.map(r => r.id)));
        }
    };

    const handleBulkUpdateStatus = async (status) => {
        if (selectedResults.size === 0) return;
        await bulkUpdateRunResults(runId, Array.from(selectedResults), status);
        setSelectedResults(new Set());
    };

    const handleBulkDelete = async () => {
        if (selectedResults.size === 0) return;
        if (!window.confirm(`Remove ${selectedResults.size} test${selectedResults.size > 1 ? 's' : ''} from this run?`)) return;
        for (const id of selectedResults) {
            await deleteRunResult(runId, id);
        }
        setSelectedResults(new Set());
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
                {['results', 'defects', 'timeline'].map(tab => (
                    <button
                        key={tab}
                        onClick={() => setActiveTab(tab)}
                        style={{
                            padding: '8px 20px', fontSize: '0.82rem', fontWeight: 600,
                            background: 'none', border: 'none', cursor: 'pointer',
                            color: activeTab === tab ? 'var(--accent-indigo)' : 'var(--text-secondary)',
                            borderBottom: activeTab === tab ? '2px solid var(--accent-indigo)' : '2px solid transparent',
                            marginBottom: -1, textTransform: 'capitalize',
                        }}
                    >
                        {tab === 'defects' ? `Defects${runDefectLinks.length > 0 ? ` (${new Set(runDefectLinks.map(l => l.jira_issue_key)).size})` : ''}` : tab === 'timeline' ? 'Timeline' : 'Results'}
                    </button>
                ))}
            </div>

            {activeTab === 'results' && (<>
            {aiFeaturesEnabled && <RunAnalysisBanner runId={runId} refreshKey={analysisBannerRefresh} />}
            {/* Collapsible Run Comments */}
            <div style={{
                border: '1px solid rgba(99,102,241,0.12)', borderRadius: 8,
                marginBottom: 12, overflow: 'hidden',
            }}>
                <button
                    onClick={() => setShowRunComments(!showRunComments)}
                    style={{
                        display: 'flex', alignItems: 'center', gap: 8, width: '100%',
                        padding: '8px 14px', background: 'rgba(99,102,241,0.03)',
                        border: 'none', cursor: 'pointer', textAlign: 'left',
                    }}
                >
                    <span style={{ fontSize: '0.72rem', color: 'var(--accent-indigo)' }}>
                        {showRunComments ? '▼' : '▶'}
                    </span>
                    <span style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--accent-indigo)' }}>
                        Comments
                    </span>
                    {runCommentCount > 0 && (
                        <span style={{
                            background: 'rgba(99,102,241,0.15)', color: 'var(--accent-indigo)',
                            padding: '1px 7px', borderRadius: 10, fontSize: '0.68rem', fontWeight: 600,
                        }}>
                            {runCommentCount}
                        </span>
                    )}
                    {latestCommentTime && (
                        <span style={{ marginLeft: 'auto', fontSize: '0.68rem', color: 'var(--text-secondary)', opacity: 0.6 }}>
                            latest {relativeTime(latestCommentTime)}
                        </span>
                    )}
                </button>
                {showRunComments && (
                    <div style={{ padding: '10px 14px', borderTop: '1px solid var(--border-color)' }}>
                        <CommentsPanel
                            targetType="run"
                            runId={runId}
                            compact={false}
                            onCountChange={(count) => setRunCommentCount(count)}
                        />
                    </div>
                )}
            </div>

            {selectedResults.size > 0 && (
                <div style={{
                    display: 'flex', alignItems: 'center', gap: 12, padding: '10px 16px',
                    background: 'rgba(99,102,241,0.1)', border: '1px solid rgba(99,102,241,0.3)',
                    borderRadius: 10, marginBottom: 12,
                    position: 'sticky', top: 0, zIndex: 20,
                    backdropFilter: 'blur(12px)', WebkitBackdropFilter: 'blur(12px)',
                    boxShadow: '0 4px 16px rgba(0,0,0,0.25)',
                }}>
                    <span style={{
                        fontSize: '0.78rem', fontWeight: 700, color: '#fff',
                        background: 'var(--accent-indigo)', padding: '3px 10px',
                        borderRadius: 99, minWidth: 24, textAlign: 'center',
                    }}>
                        {selectedResults.size}
                    </span>
                    <span style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-primary)' }}>
                        selected
                    </span>
                    <div style={{ width: 1, height: 22, background: 'rgba(99,102,241,0.3)' }} />
                    {[
                        { value: 'PASS', label: 'Pass', icon: '✓', bg: 'rgba(34,197,94,0.12)', border: 'rgba(34,197,94,0.35)', color: 'var(--accent-green)' },
                        { value: 'FAIL', label: 'Fail', icon: '✕', bg: 'rgba(239,68,68,0.12)', border: 'rgba(239,68,68,0.35)', color: 'var(--accent-red)' },
                        { value: 'SKIP', label: 'Skip', icon: '⊘', bg: 'rgba(156,163,175,0.12)', border: 'rgba(156,163,175,0.35)', color: '#9ca3af' },
                        { value: 'PENDING', label: 'Pending', icon: '○', bg: 'rgba(234,179,8,0.12)', border: 'rgba(234,179,8,0.35)', color: 'var(--warning-color)' },
                    ].map(opt => (
                        <button
                            key={opt.value}
                            onClick={() => handleBulkUpdateStatus(opt.value)}
                            style={{
                                display: 'flex', alignItems: 'center', gap: 5,
                                color: opt.color, fontSize: '0.8rem', fontWeight: 600,
                                padding: '5px 12px', borderRadius: 7,
                                border: `1px solid ${opt.border}`, background: opt.bg,
                                cursor: 'pointer', transition: 'all 0.15s ease',
                            }}
                            onMouseEnter={e => { e.currentTarget.style.transform = 'translateY(-1px)'; e.currentTarget.style.boxShadow = `0 2px 8px ${opt.border}`; }}
                            onMouseLeave={e => { e.currentTarget.style.transform = 'none'; e.currentTarget.style.boxShadow = 'none'; }}
                        >
                            <span style={{ fontSize: '0.85rem', lineHeight: 1 }}>{opt.icon}</span>
                            {opt.label}
                        </button>
                    ))}
                    <div style={{ width: 1, height: 22, background: 'rgba(99,102,241,0.3)' }} />
                    <button
                        onClick={handleBulkDelete}
                        style={{
                            display: 'flex', alignItems: 'center', gap: 5,
                            color: 'var(--accent-red)', fontSize: '0.8rem', fontWeight: 600,
                            padding: '5px 12px', borderRadius: 7,
                            border: '1px solid rgba(239,68,68,0.25)', background: 'rgba(239,68,68,0.06)',
                            cursor: 'pointer', transition: 'all 0.15s ease', opacity: 0.8,
                        }}
                        onMouseEnter={e => { e.currentTarget.style.opacity = '1'; e.currentTarget.style.background = 'rgba(239,68,68,0.12)'; }}
                        onMouseLeave={e => { e.currentTarget.style.opacity = '0.8'; e.currentTarget.style.background = 'rgba(239,68,68,0.06)'; }}
                        title="Remove selected tests from run"
                    >
                        Remove
                    </button>
                    <button
                        onClick={() => setSelectedResults(new Set())}
                        style={{
                            marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: 4,
                            color: 'var(--text-secondary)', fontSize: '0.78rem', fontWeight: 500,
                            padding: '5px 10px', borderRadius: 7,
                            border: '1px solid var(--border-color)', background: 'transparent',
                            cursor: 'pointer', transition: 'all 0.15s ease',
                        }}
                        onMouseEnter={e => e.currentTarget.style.color = 'var(--text-primary)'}
                        onMouseLeave={e => e.currentTarget.style.color = 'var(--text-secondary)'}
                    >
                        Clear
                    </button>
                </div>
            )}

            <RunResultsToolbar
                view={view}
                groupBy={effectiveGroupBy}
                onViewChange={setView}
                onGroupByChange={setGroupBy}
                onCollapseAll={collapseAll}
                onExpandAll={expandAll}
                resultCount={filteredResults.length}
                groupCount={groupedResults.length}
                dimensions={groupDimensions}
            />
            <div className="table-scroll-x">
                <table className="modern-table resizable run-results-table">
                    <thead>
                        <tr>
                            <th style={{ width: 32, textAlign: 'center', padding: '6px 0', position: 'relative' }}>
                                <label style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer', width: 20, height: 20, borderRadius: 5, border: selectedResults.size > 0 ? '2px solid var(--accent-indigo)' : '2px solid var(--border-color)', background: selectedResults.size === filteredResults.length && filteredResults.length > 0 ? 'var(--accent-indigo)' : selectedResults.size > 0 ? 'rgba(99,102,241,0.2)' : 'transparent', transition: 'all 0.15s ease', position: 'relative' }}>
                                    <input
                                        type="checkbox"
                                        checked={filteredResults.length > 0 && selectedResults.size === filteredResults.length}
                                        onChange={toggleSelectAll}
                                        style={{ position: 'absolute', opacity: 0, width: 0, height: 0 }}
                                    />
                                    {selectedResults.size === filteredResults.length && filteredResults.length > 0 && (
                                        <span style={{ color: '#fff', fontSize: '0.7rem', fontWeight: 700, lineHeight: 1 }}>✓</span>
                                    )}
                                    {selectedResults.size > 0 && selectedResults.size < filteredResults.length && (
                                        <span style={{ color: 'var(--accent-indigo)', fontSize: '0.85rem', fontWeight: 700, lineHeight: 1 }}>—</span>
                                    )}
                                </label>
                            </th>
                            <th className="col-resize-th" onClick={() => handleSort('test_name_snapshot')} style={{ width: columnWidths['test_case'], cursor: 'pointer', userSelect: 'none' }}>
                                Test Case {sortConfig.key === 'test_name_snapshot' && (sortConfig.direction === 'asc' ? '↑' : '↓')}
                                <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('test_case', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('test_case'); }} />
                            </th>
                            <th className="col-resize-th" onClick={() => handleSort('status')} style={{ width: columnWidths['status'], cursor: 'pointer', userSelect: 'none' }}>
                                Status {sortConfig.key === 'status' && (sortConfig.direction === 'asc' ? '↑' : '↓')}
                                <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('status', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('status'); }} />
                            </th>
                            {isVisible('defect_type')  && (
                                <th className="col-resize-th" style={{ width: columnWidths['defect_type'], cursor: 'pointer', userSelect: 'none' }}>
                                    Defect Type
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('defect_type', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('defect_type'); }} />
                                </th>
                            )}
                            {isVisible('defect_links') && (
                                <th className="col-resize-th" style={{ width: columnWidths['defect_links'] }}>
                                    Defect Links
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('defect_links', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('defect_links'); }} />
                                </th>
                            )}
                            {isVisible('categories')       && (
                                <th className="col-resize-th" style={{ width: columnWidths['categories'], cursor: 'pointer', userSelect: 'none' }}>
                                    Categories
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('categories', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('categories'); }} />
                                </th>
                            )}
                            {isVisible('result_id')       && (
                                <th className="col-resize-th" onClick={() => handleSort('id')} style={{ width: columnWidths['result_id'], cursor: 'pointer', userSelect: 'none' }}>
                                    Result ID {sortConfig.key === 'id' && (sortConfig.direction === 'asc' ? '↑' : '↓')}
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('result_id', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('result_id'); }} />
                                </th>
                            )}
                            {isVisible('duration')     && (
                                <th className="col-resize-th" onClick={() => handleSort('duration_ms')} style={{ width: columnWidths['duration'], cursor: 'pointer', userSelect: 'none' }}>
                                    Duration {sortConfig.key === 'duration_ms' && (sortConfig.direction === 'asc' ? '↑' : '↓')}
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('duration', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('duration'); }} />
                                </th>
                            )}
                            {isVisible('environment')  && (
                                <th className="col-resize-th" style={{ width: columnWidths['environment'] }}>
                                    Environment
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('environment', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('environment'); }} />
                                </th>
                            )}
                            {isVisible('browser')      && (
                                <th className="col-resize-th" style={{ width: columnWidths['browser'] }}>
                                    Browser
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('browser', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('browser'); }} />
                                </th>
                            )}
                            {isVisible('os')           && (
                                <th className="col-resize-th" style={{ width: columnWidths['os'] }}>
                                    OS
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('os', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('os'); }} />
                                </th>
                            )}
                            {isVisible('app_version')  && (
                                <th className="col-resize-th" style={{ width: columnWidths['app_version'] }}>
                                    App Version
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('app_version', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('app_version'); }} />
                                </th>
                            )}
                            {isVisible('start_time')   && (
                                <th className="col-resize-th" style={{ width: columnWidths['start_time'] }}>
                                    Start Time
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('start_time', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('start_time'); }} />
                                </th>
                            )}
                            {isVisible('end_time')     && (
                                <th className="col-resize-th" style={{ width: columnWidths['end_time'] }}>
                                    End Time
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('end_time', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('end_time'); }} />
                                </th>
                            )}
                            {isVisible('failure_type') && (
                                <th className="col-resize-th" style={{ width: columnWidths['failure_type'] }}>
                                    Failure Type
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('failure_type', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('failure_type'); }} />
                                </th>
                            )}
                            {isVisible('error_message')&& (
                                <th className="col-resize-th" style={{ width: columnWidths['error_message'] }}>
                                    Error
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('error_message', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('error_message'); }} />
                                </th>
                            )}
                            {isVisible('artifacts')    && (
                                <th className="col-resize-th" style={{ width: columnWidths['artifacts'] }}>
                                    Artifacts
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('artifacts', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('artifacts'); }} />
                                </th>
                            )}
                            {isVisible('log_text')     && (
                                <th className="col-resize-th" style={{ width: columnWidths['log_text'] }}>
                                    Log
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('log_text', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('log_text'); }} />
                                </th>
                            )}
                            {isVisible('metadata')     && (
                                <th className="col-resize-th" style={{ width: columnWidths['metadata'] }}>
                                    Metadata
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('metadata', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('metadata'); }} />
                                </th>
                            )}
                            {isVisible('ai_verdict')   && (
                                <th className="col-resize-th" style={{ width: columnWidths['ai_verdict'] }}>
                                    AI Verdict
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('ai_verdict', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('ai_verdict'); }} />
                                </th>
                            )}
                            {isVisible('updated_at')   && (
                                <th className="col-resize-th" style={{ width: columnWidths['updated_at'] }}>
                                    Updated At
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('updated_at', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('updated_at'); }} />
                                </th>
                            )}
                            {isVisible('attempt_number') && (
                                <th className="col-resize-th" style={{ width: columnWidths['attempt_number'] }}>
                                    Attempt
                                    <div className={`col-resize-handle${isResizing ? ' active' : ''}`} onMouseDown={(e) => startResize('attempt_number', e)} onDoubleClick={(e) => { e.stopPropagation(); resetColumnWidth('attempt_number'); }} />
                                </th>
                            )}
                            <th style={{ width: 76, textAlign: 'center', fontSize: '0.72rem', color: 'var(--text-secondary)' }}>Actions</th>
                        </tr>
                        {showFilters && (
                            <tr className="filter-row" style={{ background: 'var(--bg-secondary)' }}>
                                <th></th>
                                {/* test_case (mandatory) */}
                                <th>
                                    <input className="modern-input" placeholder="Test case…" data-testid="filter-result-test_case"
                                        style={{ width: '100%', fontSize: '0.75rem', padding: '4px 8px' }}
                                        value={resultFilters.test_case}
                                        onChange={(e) => setResultFilters(p => ({ ...p, test_case: e.target.value }))} />
                                </th>
                                {/* status (mandatory) */}
                                <th>
                                    <select className="col-filter-select" data-testid="filter-result-status"
                                        value={resultFilters.status}
                                        onChange={(e) => setResultFilters(p => ({ ...p, status: e.target.value }))}>
                                        <option value="">All</option>
                                        <option value="PASS">Pass</option>
                                        <option value="FAIL">Fail</option>
                                        <option value="ERROR">Error</option>
                                        <option value="SKIP">Skip</option>
                                        <option value="PENDING">Pending</option>
                                    </select>
                                </th>
                                {isVisible('defect_type') && (
                                    <th>
                                        <select className="col-filter-select" data-testid="filter-result-defect_type"
                                            value={resultFilters.defect_type}
                                            onChange={(e) => setResultFilters(p => ({ ...p, defect_type: e.target.value }))}>
                                            <option value="">All</option>
                                            <option value="product_bug">Product Bug</option>
                                            <option value="automation_bug">Automation Bug</option>
                                            <option value="system_issue">System Issue</option>
                                            <option value="to_investigate">To Investigate</option>
                                        </select>
                                    </th>
                                )}
                                {isVisible('defect_links') && <th></th>}
                                {isVisible('categories') && (
                                    <th>
                                        <CategoryFilter categories={resultCategories} value={resultFilters.categories}
                                            onChange={(ids) => setResultFilters(p => ({ ...p, categories: ids }))}
                                            testId="filter-result-categories" />
                                    </th>
                                )}
                                {isVisible('result_id') && (
                                    <th>
                                        <input className="modern-input" placeholder="Result ID…" data-testid="filter-result-result_id"
                                            style={{ width: '100%', fontSize: '0.75rem', padding: '4px 8px' }}
                                            value={resultFilters.result_id}
                                            onChange={(e) => setResultFilters(p => ({ ...p, result_id: e.target.value }))} />
                                    </th>
                                )}
                                {isVisible('duration') && <th></th>}
                                {isVisible('environment') && <th></th>}
                                {isVisible('browser') && <th></th>}
                                {isVisible('os') && <th></th>}
                                {isVisible('app_version') && <th></th>}
                                {isVisible('start_time') && (
                                    <th>
                                        <DateRangeFilter value={resultFilters.start_time}
                                            onChange={(v) => setResultFilters(p => ({ ...p, start_time: v }))}
                                            testId="filter-result-start_time" />
                                    </th>
                                )}
                                {isVisible('end_time') && (
                                    <th>
                                        <DateRangeFilter value={resultFilters.end_time}
                                            onChange={(v) => setResultFilters(p => ({ ...p, end_time: v }))}
                                            testId="filter-result-end_time" />
                                    </th>
                                )}
                                {isVisible('failure_type') && <th></th>}
                                {isVisible('error_message') && <th></th>}
                                {isVisible('artifacts') && <th></th>}
                                {isVisible('log_text') && <th></th>}
                                {isVisible('metadata') && <th></th>}
                                {isVisible('ai_verdict') && <th></th>}
                                {isVisible('updated_at') && (
                                    <th>
                                        <DateRangeFilter value={resultFilters.updated_at}
                                            onChange={(v) => setResultFilters(p => ({ ...p, updated_at: v }))}
                                            testId="filter-result-updated_at" />
                                    </th>
                                )}
                                {isVisible('attempt_number') && <th></th>}
                                <th></th>
                            </tr>
                        )}
                    </thead>
                    <tbody>
                        {(() => {
                          const renderResultRow = (result) => (
                            <React.Fragment key={result.id}>
                                <tr
                                    data-result-id={result.id}
                                    onClick={() => toggleResult(result.id)}
                                    style={{ cursor: 'pointer', background: selectedResults.has(result.id) ? 'rgba(99,102,241,0.08)' : expandedResults.has(result.id) ? 'var(--bg-secondary)' : 'transparent', borderLeft: selectedResults.has(result.id) ? '3px solid var(--accent-indigo)' : '3px solid transparent', transition: 'background 0.1s ease' }}
                                >
                                    <td style={{ textAlign: 'center', padding: '6px 0', width: 32 }} onClick={e => e.stopPropagation()}>
                                        <label style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer', width: 18, height: 18, borderRadius: 4, border: selectedResults.has(result.id) ? '2px solid var(--accent-indigo)' : '2px solid var(--border-color)', background: selectedResults.has(result.id) ? 'var(--accent-indigo)' : 'transparent', transition: 'all 0.15s ease' }}>
                                            <input
                                                type="checkbox"
                                                checked={selectedResults.has(result.id)}
                                                onChange={(e) => toggleSelectResult(result.id, e)}
                                                style={{ position: 'absolute', opacity: 0, width: 0, height: 0 }}
                                            />
                                            {selectedResults.has(result.id) && (
                                                <span style={{ color: '#fff', fontSize: '0.65rem', fontWeight: 700, lineHeight: 1 }}>✓</span>
                                            )}
                                        </label>
                                    </td>
                                    <td style={{ fontWeight: 500, maxWidth: columnWidths['test_case'], overflow: 'hidden' }}>
                                        <div style={{ display: 'flex', alignItems: 'center', minWidth: 0 }}>
                                        <span style={{ marginRight: 8, flexShrink: 0 }}>{expandedResults.has(result.id) ? '▼' : '▶'}</span>
                                        <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={result.test_name_snapshot}>
                                        {result.test_case_id ? (
                                            <Link
                                                to={`/library/tests/${result.test_case_id}`}
                                                onClick={e => e.stopPropagation()}
                                                className="result-test-link"
                                            >
                                                {result.test_name_snapshot}
                                            </Link>
                                        ) : (
                                            result.test_name_snapshot
                                        )}
                                        </span>
                                        {result.attempt_number > 1 && (
                                            <span
                                                title={`Attempt ${result.attempt_number} — click to view history`}
                                                onClick={(e) => { e.stopPropagation(); toggleResult(result.id); }}
                                                style={{
                                                    display: 'inline-block', padding: '1px 6px', marginLeft: 6, flexShrink: 0,
                                                    fontSize: '11px', borderRadius: 8, background: '#fff3cd',
                                                    color: '#856404', cursor: 'pointer',
                                                }}
                                            >
                                                ↻ {result.attempt_number}
                                            </span>
                                        )}
                                        {(() => {
                                            const open = result.test_case?.open_defect_count || 0;
                                            const closed = result.test_case?.closed_defect_count || 0;
                                            if (!open && !closed) return null;
                                            return (
                                                <span
                                                    title={`${open} open, ${closed} resolved defect${open + closed !== 1 ? 's' : ''}`}
                                                    style={{
                                                        marginLeft: 6, fontSize: '0.68rem', fontWeight: 700, flexShrink: 0,
                                                        padding: '0 5px', borderRadius: 99, lineHeight: '16px',
                                                        background: open > 0 ? 'rgba(239,68,68,0.18)' : 'rgba(52,211,153,0.15)',
                                                        color: open > 0 ? '#f87171' : '#34d399',
                                                        border: `1px solid ${open > 0 ? 'rgba(239,68,68,0.35)' : 'rgba(52,211,153,0.35)'}`,
                                                    }}
                                                >
                                                    {open > 0 ? `${open}🐛` : `✓${closed}`}
                                                </span>
                                            );
                                        })()}
                                        </div>
                                    </td>
                                    <td>
                                        <select
                                            value={result.status}
                                            onClick={e => e.stopPropagation()}
                                            onChange={(e) => handleUpdateResult(result.id, e.target.value)}
                                            className={`status-select ${result.status.toLowerCase()}`}
                                            data-testid={`test-status-select-${result.test_case_id}`}
                                            style={{ width: '100%' }}
                                        >
                                            <option value="PENDING">PENDING</option>
                                            <option value="PASS">PASSED</option>
                                            <option value="FAIL">FAILED</option>
                                            <option value="SKIP">SKIPPED</option>
                                            <option value="RUNNING">RUNNING</option>
                                            <option value="ERROR">ERROR</option>
                                        </select>
                                    </td>
                                    {isVisible('defect_type') && (
                                        <td onClick={e => e.stopPropagation()} style={{ overflow: 'hidden' }}>
                                            {result.status === 'FAIL' ? (
                                                <select
                                                    value={result.defect_type || 'to_investigate'}
                                                    onChange={(e) => handleUpdateDefectType(result.id, e.target.value)}
                                                    data-testid={`defect-type-select-${result.test_case_id}`}
                                                    style={{
                                                        fontSize: '0.75rem',
                                                        padding: '2px 6px',
                                                        borderRadius: 6,
                                                        border: '1px solid var(--border-color)',
                                                        width: '100%',
                                                        maxWidth: '100%',
                                                        background: result.defect_type === 'product_bug' ? 'rgba(239,68,68,0.1)'
                                                            : result.defect_type === 'automation_bug' ? 'rgba(139,92,246,0.1)'
                                                            : result.defect_type === 'system_issue' ? 'rgba(100,116,139,0.1)'
                                                            : 'rgba(245,158,11,0.1)',
                                                        color: result.defect_type === 'product_bug' ? '#dc2626'
                                                            : result.defect_type === 'automation_bug' ? '#7c3aed'
                                                            : result.defect_type === 'system_issue' ? '#64748b'
                                                            : '#d97706',
                                                        cursor: 'pointer',
                                                    }}
                                                >
                                                    <option value="to_investigate">🔍 To Investigate</option>
                                                    <option value="product_bug">🐞 Product Bug</option>
                                                    <option value="automation_bug">🤖 Automation Bug</option>
                                                    <option value="system_issue">⚙️ System Issue</option>
                                                </select>
                                            ) : (
                                                <span style={{ color: 'var(--text-secondary)', fontSize: '0.75rem' }}>—</span>
                                            )}
                                        </td>
                                    )}
                                    {isVisible('defect_links') && (() => {
                                        const open = result.open_defect_link_count || 0;
                                        const closed = result.closed_defect_link_count || 0;
                                        const total = open + closed;
                                        return (
                                            <td>
                                                {total > 0 ? (
                                                    <span style={{ display: 'flex', gap: 6, alignItems: 'center', fontSize: '0.75rem' }}>
                                                        {open > 0 && <span style={{ color: 'var(--accent-red)', fontWeight: 600 }}>🐞 {open}</span>}
                                                        {closed > 0 && <span style={{ color: 'var(--accent-green)', fontWeight: 600 }}>✅ {closed}</span>}
                                                    </span>
                                                ) : (
                                                    <span style={{ color: 'var(--text-secondary)', fontSize: '0.75rem' }}>—</span>
                                                )}
                                            </td>
                                        );
                                    })()}
                                    {isVisible('categories') && (
                                        <td style={{ whiteSpace: 'normal', overflow: 'hidden' }}>
                                            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
                                                {(result.test_case?.categories || []).length > 0
                                                    ? result.test_case.categories.map(s => (
                                                        <span key={s.id} className="category-tag" style={{ fontSize: '0.7rem', padding: '1px 6px' }}>
                                                            {s.name}
                                                        </span>
                                                    ))
                                                    : <span style={{ color: 'var(--text-secondary)', fontSize: '0.75rem' }}>—</span>
                                                }
                                            </div>
                                        </td>
                                    )}
                                    {isVisible('result_id') && (
                                        <td style={{ fontFamily: 'monospace', fontSize: '0.78rem', color: 'var(--text-secondary)' }}>
                                            {result.id}
                                        </td>
                                    )}
                                    {isVisible('duration') && (
                                        <td style={{ fontFamily: 'monospace', fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
                                            {formatDuration(result.duration_ms)}
                                        </td>
                                    )}
                                    {isVisible('environment') && (
                                        <td style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
                                            {result.environment || '—'}
                                        </td>
                                    )}
                                    {isVisible('browser') && (
                                        <td style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
                                            {result.browser || '—'}
                                        </td>
                                    )}
                                    {isVisible('os') && (
                                        <td style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
                                            {result.os || '—'}
                                        </td>
                                    )}
                                    {isVisible('app_version') && (
                                        <td style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
                                            {result.app_version || '—'}
                                        </td>
                                    )}
                                    {isVisible('start_time') && (
                                        <td style={{ fontFamily: 'monospace', fontSize: '0.78rem', color: 'var(--text-secondary)', whiteSpace: 'nowrap' }}>
                                            {formatDateTime(result.start_time)}
                                        </td>
                                    )}
                                    {isVisible('end_time') && (
                                        <td style={{ fontFamily: 'monospace', fontSize: '0.78rem', color: 'var(--text-secondary)', whiteSpace: 'nowrap' }}>
                                            {formatDateTime(result.end_time)}
                                        </td>
                                    )}
                                    {isVisible('failure_type') && (
                                        <td style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
                                            {result.failure_type || '—'}
                                        </td>
                                    )}
                                    {isVisible('error_message') && (
                                        <td style={{ fontSize: '0.78rem', color: 'var(--accent-red)', maxWidth: 200 }}>
                                            {result.error_message ? (
                                                <span title={result.error_message} style={{ display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                                                    {result.error_message}
                                                </span>
                                            ) : '—'}
                                        </td>
                                    )}
                                    {isVisible('artifacts') && (
                                        <td style={{ fontSize: '0.78rem' }}>
                                            <div style={{ display: 'flex', gap: 6 }}>
                                                {(() => {
                                                    const urls = result.screenshots ? (() => { try { return JSON.parse(result.screenshots); } catch { return []; } })() : [];
                                                    return urls.length > 0 && /^(https?:\/\/|\/)/i.test(urls[0]) && (
                                                        <a
                                                            href={urls[0]}
                                                            target="_blank"
                                                            rel="noreferrer"
                                                            onClick={e => e.stopPropagation()}
                                                            title={`${urls.length} screenshot${urls.length > 1 ? 's' : ''}`}
                                                            style={{ color: 'var(--accent-indigo)', fontSize: '0.85rem', textDecoration: 'none' }}
                                                        >
                                                            {"📸"}{urls.length > 1 && <span style={{ fontSize: '0.7rem', marginLeft: 2 }}>{urls.length}</span>}
                                                        </a>
                                                    );
                                                })()}
                                                {result.video      && /^(https?:\/\/|\/)/i.test(result.video)     && <a href={result.video}      target="_blank" rel="noreferrer" onClick={e => e.stopPropagation()} title="Video"      style={{ color: 'var(--accent-indigo)' }}>🎥</a>}
                                                {result.trace_url  && /^(https?:\/\/|\/)/i.test(result.trace_url) && <a href={result.trace_url}  target="_blank" rel="noreferrer" onClick={e => e.stopPropagation()} title="Trace"       style={{ color: 'var(--accent-indigo)' }}>🔗</a>}
                                                {!result.screenshots && !result.video && !result.trace_url && <span style={{ color: 'var(--text-secondary)' }}>—</span>}
                                            </div>
                                        </td>
                                    )}
                                    {isVisible('log_text') && (
                                        <td style={{ fontFamily: 'monospace', fontSize: '0.75rem', color: 'var(--text-secondary)', maxWidth: 200 }}>
                                            {result.log_text ? (
                                                <span title={result.log_text} style={{ display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                                                    {result.log_text}
                                                </span>
                                            ) : '—'}
                                        </td>
                                    )}
                                    {isVisible('metadata') && (
                                        <td style={{ fontFamily: 'monospace', fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
                                            {result.metadata ? JSON.stringify(result.metadata) : '—'}
                                        </td>
                                    )}
                                    {isVisible('ai_verdict') && (
                                        <td style={{ width: columnWidths['ai_verdict'] }} onClick={e => e.stopPropagation()}>
                                            <AIVerdictCell
                                                result={result}
                                                analysis={currentAnalyses[result.id]}
                                                onAnalyze={async () => {
                                                    const { analyzeRunResult } = await import('../api');
                                                    const row = await analyzeRunResult(result.id);
                                                    setCurrentAnalyses((prev) => ({ ...prev, [result.id]: row }));
                                                }}
                                            />
                                        </td>
                                    )}
                                    {isVisible('updated_at') && (
                                        <td style={{ fontFamily: 'monospace', fontSize: '0.78rem', color: 'var(--text-secondary)', whiteSpace: 'nowrap' }}>
                                            {formatDateTime(result.updated_at)}
                                        </td>
                                    )}
                                    {isVisible('attempt_number') && (
                                        <td style={{ fontFamily: 'monospace', fontSize: '0.85rem', textAlign: 'center' }}>
                                            {result.attempt_number}
                                        </td>
                                    )}
                                    <td style={{ whiteSpace: 'nowrap', textAlign: 'center' }}>
                                        <div style={{ display: 'inline-flex', gap: 4, alignItems: 'center' }}>
                                        {result.test_case_id && (
                                            <button
                                                onClick={(e) => { e.stopPropagation(); handleRetryResult(result.id); }}
                                                title="Retry this test"
                                                style={{
                                                    display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
                                                    width: 28, height: 28, borderRadius: 6,
                                                    border: '1px solid transparent', background: 'transparent',
                                                    color: 'var(--text-secondary)', fontSize: '1rem',
                                                    cursor: 'pointer', transition: 'all 0.15s ease',
                                                }}
                                                onMouseEnter={e => { e.currentTarget.style.background = 'rgba(99,102,241,0.1)'; e.currentTarget.style.borderColor = 'rgba(99,102,241,0.3)'; e.currentTarget.style.color = 'var(--accent-indigo)'; }}
                                                onMouseLeave={e => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.borderColor = 'transparent'; e.currentTarget.style.color = 'var(--text-secondary)'; }}
                                            >
                                                ↻
                                            </button>
                                        )}
                                        <button
                                            onClick={(e) => { e.stopPropagation(); handleDeleteResult(result.id); }}
                                            title="Remove from run"
                                            data-testid={`remove-result-button-${result.test_case_id}`}
                                            style={{
                                                display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
                                                width: 28, height: 28, borderRadius: 6,
                                                border: '1px solid transparent', background: 'transparent',
                                                color: 'var(--text-secondary)', fontSize: '0.85rem',
                                                cursor: 'pointer', transition: 'all 0.15s ease',
                                            }}
                                            onMouseEnter={e => { e.currentTarget.style.background = 'rgba(239,68,68,0.1)'; e.currentTarget.style.borderColor = 'rgba(239,68,68,0.25)'; e.currentTarget.style.color = 'var(--accent-red)'; }}
                                            onMouseLeave={e => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.borderColor = 'transparent'; e.currentTarget.style.color = 'var(--text-secondary)'; }}
                                        >
                                            ✕
                                        </button>
                                        </div>
                                    </td>
                                </tr>
                                {expandedResults.has(result.id) && (
                                    <tr>
                                        <td colSpan={3 + OPTIONAL_COLUMN_KEYS.filter(k => isVisible(k)).length + 1} style={{ padding: 0 }}>
                                            <RunResultDetail
                                                result={result}
                                                attempts={result.test_case_id ? attemptsByTestCase[result.test_case_id] : null}
                                            />
                                        </td>
                                    </tr>
                                )}
                            </React.Fragment>
                          );
                          if (!filteredResults || filteredResults.length === 0) {
                            return (
                                <tr>
                                    <td colSpan={3 + OPTIONAL_COLUMN_KEYS.filter(k => isVisible(k)).length + 1} style={{ textAlign: 'center', padding: '32px 0', color: 'var(--text-secondary)' }}>
                                        {sortedResults.length === 0 ? 'No results logged yet.' : 'No results match the current filters.'}
                                    </td>
                                </tr>
                            );
                          }
                          if (view !== 'grouped') {
                            return filteredResults.map(renderResultRow);
                          }
                          const totalCols = 3 + OPTIONAL_COLUMN_KEYS.filter(k => isVisible(k)).length + 1;
                          return groupedResults.map(group => (
                            <React.Fragment key={group.key}>
                                <tr
                                    data-testid="group-header"
                                    onClick={() => toggleGroup(group.key)}
                                    style={{ cursor: 'pointer', background: 'var(--bg-secondary)', borderTop: '2px solid var(--border-color)' }}
                                >
                                    <td colSpan={totalCols} style={{ padding: '8px 12px' }}>
                                        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}>
                                            <span style={{ width: 12, display: 'inline-block', fontSize: 11, color: 'var(--text-secondary)' }}>
                                                {collapsedGroups.has(group.key) ? '▶' : '▼'}
                                            </span>
                                            <span style={{ display: 'inline-block', padding: '2px 10px', borderRadius: 12, background: group.color.pill, color: '#fff', fontSize: 12, fontWeight: 600 }}>
                                                {group.label}
                                            </span>
                                            <span style={{ color: 'var(--text-secondary)', fontSize: 12 }}>
                                                {group.rows.length} result{group.rows.length === 1 ? '' : 's'}{group.summary ? ` · ${group.summary}` : ''}
                                            </span>
                                        </span>
                                    </td>
                                </tr>
                                {!collapsedGroups.has(group.key) && group.rows.map(renderResultRow)}
                            </React.Fragment>
                          ));
                        })()}
                    </tbody>
                </table>
            </div>
            </>)}

            {activeTab === 'defects' && (
                <div>
                    {defectsLoading && <p style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', padding: 16 }}>Loading defect links...</p>}
                    {!defectsLoading && runDefectLinks.length === 0 && (
                        <p style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', padding: 16, textAlign: 'center' }}>
                            No defect links in this run yet. Link defects from individual test results.
                        </p>
                    )}
                    {!defectsLoading && runDefectLinks.length > 0 && (() => {
                        // Group links by jira_issue_key
                        const grouped = new Map();
                        runDefectLinks.forEach(link => {
                            const key = link.jira_issue_key;
                            if (!grouped.has(key)) {
                                grouped.set(key, {
                                    jira_issue_key: key,
                                    last_known_url: link.last_known_url,
                                    last_known_summary: link.last_known_summary,
                                    last_known_status: link.last_known_status,
                                    last_known_priority: link.last_known_priority,
                                    status_category: link.status_category,
                                    results: [],
                                });
                            }
                            grouped.get(key).results.push({
                                id: link.id,
                                test_case_id: link.test_case_id,
                                test_name_snapshot: link.test_name_snapshot,
                                result_status: link.result_status,
                            });
                        });
                        return (
                            <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                                {Array.from(grouped.values()).map(defect => {
                                    const statusStyle = defect.status_category === 'done'
                                        ? { background: 'rgba(52,211,153,0.15)', color: '#34d399', border: '1px solid rgba(52,211,153,0.35)' }
                                        : defect.status_category === 'indeterminate'
                                        ? { background: 'rgba(251,191,36,0.15)', color: '#fbbf24', border: '1px solid rgba(251,191,36,0.35)' }
                                        : { background: 'rgba(148,163,184,0.15)', color: '#94a3b8', border: '1px solid rgba(148,163,184,0.35)' };
                                    return (
                                        <div key={defect.jira_issue_key} style={{
                                            border: '1px solid var(--border-color)', borderRadius: 8,
                                            overflow: 'hidden', background: 'var(--bg-primary)',
                                        }}>
                                            {/* Defect header — clickable to expand/collapse */}
                                            <div
                                                onClick={() => setExpandedDefects(prev => {
                                                    const next = new Set(prev);
                                                    next.has(defect.jira_issue_key) ? next.delete(defect.jira_issue_key) : next.add(defect.jira_issue_key);
                                                    return next;
                                                })}
                                                style={{
                                                    display: 'flex', alignItems: 'center', gap: 12,
                                                    padding: '10px 16px', background: 'var(--bg-secondary)',
                                                    borderBottom: expandedDefects.has(defect.jira_issue_key) ? '1px solid var(--border-color)' : 'none',
                                                    cursor: 'pointer', userSelect: 'none',
                                                }}>
                                                <span style={{ fontSize: '0.68rem', color: 'var(--text-secondary)', flexShrink: 0, width: 12, textAlign: 'center' }}>
                                                    {expandedDefects.has(defect.jira_issue_key) ? '▼' : '▶'}
                                                </span>
                                                <a
                                                    href={defect.last_known_url || '#'}
                                                    target="_blank"
                                                    rel="noopener noreferrer"
                                                    onClick={e => e.stopPropagation()}
                                                    style={{ fontWeight: 700, color: 'var(--accent-purple, #a78bfa)', fontSize: '0.88rem', textDecoration: 'none', flexShrink: 0 }}
                                                >
                                                    {defect.jira_issue_key}
                                                </a>
                                                <span style={{ fontSize: '0.82rem', color: 'var(--text-primary)', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                                                    {defect.last_known_summary || '—'}
                                                </span>
                                                <span style={{ ...statusStyle, padding: '2px 10px', borderRadius: 99, fontSize: '0.72rem', fontWeight: 600, whiteSpace: 'nowrap', flexShrink: 0 }}>
                                                    {defect.last_known_status || '—'}
                                                </span>
                                                {defect.last_known_priority && (
                                                    <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', flexShrink: 0 }}>
                                                        {defect.last_known_priority}
                                                    </span>
                                                )}
                                                <span style={{
                                                    background: 'rgba(239,68,68,0.15)', color: 'var(--accent-red)',
                                                    padding: '1px 8px', borderRadius: 99, fontSize: '0.7rem', fontWeight: 700, flexShrink: 0,
                                                }}>
                                                    {defect.results.length} test{defect.results.length !== 1 ? 's' : ''}
                                                </span>
                                            </div>
                                            {/* Affected test cases — collapsed by default */}
                                            {expandedDefects.has(defect.jira_issue_key) && <div style={{ padding: '6px 16px' }}>
                                                {defect.results.map((res, idx) => (
                                                    <div key={res.id} style={{
                                                        display: 'flex', alignItems: 'center', gap: 10,
                                                        padding: '6px 0',
                                                        borderBottom: idx < defect.results.length - 1 ? '1px solid var(--border-color)' : 'none',
                                                    }}>
                                                        <span style={{ color: 'var(--text-secondary)', fontSize: '0.72rem', width: 16, textAlign: 'center', flexShrink: 0 }}>
                                                            {idx + 1}
                                                        </span>
                                                        <span style={{ flex: 1, fontSize: '0.82rem' }}>
                                                            {res.test_case_id ? (
                                                                <Link
                                                                    to={`/library/tests/${res.test_case_id}`}
                                                                    style={{ color: 'var(--accent-indigo)', textDecoration: 'none' }}
                                                                >
                                                                    {res.test_name_snapshot}
                                                                </Link>
                                                            ) : (
                                                                res.test_name_snapshot
                                                            )}
                                                        </span>
                                                        <span className={`status-badge ${(res.result_status || '').toLowerCase()}`} style={{ fontSize: '0.7rem', padding: '2px 8px' }}>
                                                            {res.result_status || '—'}
                                                        </span>
                                                    </div>
                                                ))}
                                            </div>}
                                        </div>
                                    );
                                })}
                            </div>
                        );
                    })()}
                </div>
            )}

            {activeTab === 'timeline' && (
                <RunTimeline
                    results={run.run_results}
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
        </div>
    );
}

function AIVerdictCell({ result, analysis, onAnalyze }) {
    const [loading, setLoading] = useState(false);
    const isFailure = result.status === 'FAIL' || result.status === 'ERROR';
    if (!isFailure) {
        return <span style={{ color: 'var(--text-secondary)', fontSize: '0.75rem' }}>—</span>;
    }
    if (!analysis) {
        return (
            <button
                onClick={async (e) => {
                    e.stopPropagation();
                    setLoading(true);
                    try { await onAnalyze(); } finally { setLoading(false); }
                }}
                disabled={loading}
                style={{ padding: '2px 10px', fontSize: 11 }}
            >
                {loading ? 'Analyzing…' : 'Analyze'}
            </button>
        );
    }
    return (
        <AIVerdictBadge
            verdict={analysis.verdict}
            confidence={analysis.confidence}
            dedupGroup={!!analysis.dedup_group_key}
        />
    );
}
