import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import { deleteRunResult, updateRunResult, retryRunResult, bulkUpdateRunResults, listRunComments, getCategories } from '../../api';
import DateRangeFilter from '../../components/filters/DateRangeFilter';
import CategoryFilter from '../../components/filters/CategoryFilter';
import { inDateRange } from '../../utils/dateFilter';
import RunResultDetail from '../../components/RunResultDetail';
import CommentsPanel from '../../components/CommentsPanel';
import AIVerdictBadge from '../../components/AIVerdictBadge';
import RunAnalysisBanner from '../../components/RunAnalysisBanner';
import RunResultsToolbar from '../../components/RunResultsToolbar';
import { useRunViewPreference } from '../../hooks/useRunViewPreference';
import { groupResults, GROUP_DIMENSIONS } from '../../utils/runResultsGrouping';
import { shouldShowSuggestion, suggestionLabel } from '../../utils/defectSuggestion';

// Theme-readable text tones for the AI suggestion chip, echoing the defect_type
// <select> colors below. --aig-tone-*-fg and --text-secondary are defined for
// BOTH themes; --surface-bg / --accent-purple are not, so they are avoided here.
const SUGGESTION_FG = {
    product_bug: 'var(--aig-tone-red-fg)',
    automation_bug: 'var(--aig-tone-purple-fg)',
    system_issue: 'var(--text-secondary)',
};

// DefectSuggestionChip is an aid, not a nag: it renders only for untriaged FAILs that have a
// suggestion, and Accept reuses the normal triage write, so the backend snapshot/calibration
// path is identical to a human picking the value from the <select> beside it.
//
// Renders nothing when AI features are off. Every other AI surface on this page is gated the
// same way (the analysis banner, the ai_verdict column) — without this, disabling AI would
// still leave live Accept buttons behind on previously-stored analyses.
function DefectSuggestionChip({ result, analysis, enabled, onAccept }) {
    if (!enabled || !shouldShowSuggestion(result, analysis)) return null;
    const suggested = analysis.suggested_defect_type;
    return (
        <div
            data-testid={`defect-suggestion-${result.test_case_id}`}
            title={`AI failure analysis suggests "${suggestionLabel(suggested)}" — accept it, or pick another value above`}
            style={{
                display: 'flex', alignItems: 'center', gap: 4,
                marginTop: 3, padding: '1px 4px',
                background: 'var(--bg-secondary)',
                border: '1px solid var(--border-color)',
                borderRadius: 6, fontSize: '0.68rem',
                maxWidth: '100%', overflow: 'hidden',
            }}
        >
            <span style={{
                color: SUGGESTION_FG[suggested] || 'var(--text-secondary)',
                whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
            }}>
                AI: {suggestionLabel(suggested)}
            </span>
            <button
                type="button"
                onClick={() => onAccept(result.id, suggested)}
                data-testid={`defect-suggestion-accept-${result.test_case_id}`}
                style={{
                    marginLeft: 'auto', flexShrink: 0,
                    padding: '0 5px', borderRadius: 5,
                    border: '1px solid var(--border-color)',
                    background: 'transparent',
                    color: 'var(--text-primary)',
                    fontSize: '0.68rem', lineHeight: '15px',
                    cursor: 'pointer',
                }}
            >
                Accept
            </button>
        </div>
    );
}

export default function ResultsTab({
    runId,
    latestResults,
    attemptsByTestCase,
    selectedResults,
    setSelectedResults,
    aiFeaturesEnabled,
    analysisBannerRefresh,
    currentAnalyses,
    setCurrentAnalyses,
    showFilters,
    isVisible,
    columnWidths,
    columnActions,
    optionalColumnKeys,
}) {
    const { startResize, resetColumnWidth, isResizing } = columnActions;
    const [expandedResults, setExpandedResults] = useState(new Set());
    const lastClickedRef = React.useRef(null);
    const [sortConfig, setSortConfig] = useState({ key: null, direction: 'asc' });
    const [showRunComments, setShowRunComments] = useState(false);
    const [runCommentCount, setRunCommentCount] = useState(0);
    const [latestCommentTime, setLatestCommentTime] = useState(null);
    const { view, groupBy, setView, setGroupBy } = useRunViewPreference();
    const [collapsedGroups, setCollapsedGroups] = useState(new Set());
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

    React.useEffect(() => {
        if (runId) {
            listRunComments(runId).then(data => {
                const list = data || [];
                setRunCommentCount(list.length);
                if (list.length > 0) setLatestCommentTime(list[list.length - 1].created_at);
            }).catch(() => {});
        }
    }, [runId]);

    React.useEffect(() => {
        getCategories(1, 200).then(d => setResultCategories(d.categories || [])).catch(() => setResultCategories([]));
    }, []);

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
                const isFailed = r.status === 'FAIL' || r.status === 'ERROR';
                const dt = isFailed ? (r.defect_type || 'to_investigate') : '';
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

    const formatDuration = (ms) => {
        if (!ms) return '-';
        return ms < 1000 ? `${ms}ms` : `${(ms / 1000).toFixed(2)}s`;
    };

    const formatDateTime = (ts) => {
        if (!ts || ts === '0001-01-01T00:00:00Z') return '—';
        return new Date(ts).toLocaleString();
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
            const ids = filteredResults.map(r => r.id);
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

    return (<>
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
                    onCountChange={setRunCommentCount}
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
                        <label data-testid="select-all-results" style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer', width: 20, height: 20, borderRadius: 5, border: selectedResults.size > 0 ? '2px solid var(--accent-indigo)' : '2px solid var(--border-color)', background: selectedResults.size === filteredResults.length && filteredResults.length > 0 ? 'var(--accent-indigo)' : selectedResults.size > 0 ? 'rgba(99,102,241,0.2)' : 'transparent', transition: 'all 0.15s ease', position: 'relative' }}>
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
                                <label data-testid={`select-result-${result.test_case_id}`} style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer', width: 18, height: 18, borderRadius: 4, border: selectedResults.has(result.id) ? '2px solid var(--accent-indigo)' : '2px solid var(--border-color)', background: selectedResults.has(result.id) ? 'var(--accent-indigo)' : 'transparent', transition: 'all 0.15s ease' }}>
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
                                        <>
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
                                        <DefectSuggestionChip
                                            result={result}
                                            analysis={currentAnalyses?.[result.id]}
                                            enabled={aiFeaturesEnabled}
                                            onAccept={handleUpdateDefectType}
                                        />
                                        </>
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
                                            const { analyzeRunResult } = await import('../../api');
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
                                <td colSpan={3 + optionalColumnKeys.filter(k => isVisible(k)).length + 1} style={{ padding: 0 }}>
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
                            <td colSpan={3 + optionalColumnKeys.filter(k => isVisible(k)).length + 1} style={{ textAlign: 'center', padding: '32px 0', color: 'var(--text-secondary)' }}>
                                {sortedResults.length === 0 ? 'No results logged yet.' : 'No results match the current filters.'}
                            </td>
                        </tr>
                    );
                  }
                  if (view !== 'grouped') {
                    return filteredResults.map(renderResultRow);
                  }
                  const totalCols = 3 + optionalColumnKeys.filter(k => isVisible(k)).length + 1;
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
    </>);
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
