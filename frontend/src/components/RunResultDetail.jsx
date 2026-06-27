import React, { useState } from 'react';
import DefectLinkPanel from './DefectLinkPanel';
import CommentsPanel from './CommentsPanel';
import ScreenshotGallery from './ScreenshotGallery';
import AIVerdictBadge from './AIVerdictBadge';
import { analyzeRunResult, listRunResultAnalyses } from '../api';
import { useAIGeneration } from '../contexts/AIGenerationContext';

const STATUS_DOT_COLORS = {
    PASS: '#10b981', FAIL: '#ef4444', ERROR: '#ef4444',
    PENDING: '#f59e0b', SKIP: '#94a3b8', RUNNING: '#3b82f6',
};

const RunResultDetail = ({ result, attempts }) => {
    if (!result) return null;

    const [showLogs, setShowLogs] = useState(false);
    const [showSteps, setShowSteps] = useState(false);
    const [selectedAttemptId, setSelectedAttemptId] = useState(null);
    const [detailTab, setDetailTab] = useState('failure');
    const [analyses, setAnalyses] = useState(null);
    const [selectedVersion, setSelectedVersion] = useState(null);
    const [analyzing, setAnalyzing] = useState(false);
    const { aiFeaturesEnabled } = useAIGeneration();

    const loadAnalyses = async (resultId) => {
        try {
            const list = await listRunResultAnalyses(resultId);
            const arr = Array.isArray(list) ? list : [];
            setAnalyses(arr);
            setSelectedVersion(arr[0]?.version || null);
        } catch {
            setAnalyses([]);
        }
    };

    // If attempts provided and user selected one, show that instead
    const hasMultipleAttempts = attempts && attempts.length > 1;
    const activeResult = hasMultipleAttempts && selectedAttemptId
        ? attempts.find(a => a.id === selectedAttemptId) || result
        : result;

    const {
        status,
        test_case_id,
        test_name_snapshot,
        error_message,
        stack_trace,
        failure_type,
        video,
        trace_url,
        log_text,
        steps,
        duration_ms,
        browser,
        os,
        app_version,
        environment,
        attempt_number
    } = activeResult;

    const isFailed = status === 'FAIL' || status === 'ERROR';

    const [galleryOpen, setGalleryOpen] = useState(false);
    const [galleryIndex, setGalleryIndex] = useState(0);

    // Parse screenshots JSON array
    const screenshotUrls = (() => {
        if (activeResult.screenshots) {
            try { return JSON.parse(activeResult.screenshots); } catch { /* fall through */ }
        }
        return [];
    })();

    const createDefectContext = (isFailed && test_case_id)
        ? { testName: test_name_snapshot, errorMessage: error_message, stackTrace: stack_trace }
        : null;

    const formatDuration = (ms) => {
        if (!ms) return '—';
        return ms < 1000 ? `${ms}ms` : `${(ms / 1000).toFixed(2)}s`;
    };

    return (
        <div data-testid="run-result-detail" style={{ padding: '12px 16px 16px', background: 'var(--bg-secondary)' }}>

            {/* Timeline strip for multiple attempts */}
            {hasMultipleAttempts && (
                <div style={{ display: 'flex', alignItems: 'center', gap: 0, marginBottom: 14, paddingBottom: 14, borderBottom: '1px solid var(--border-color)' }}>
                    {[...attempts].reverse().map((attempt, idx, arr) => (
                        <React.Fragment key={attempt.id}>
                            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 4 }}>
                                <div
                                    onClick={() => setSelectedAttemptId(attempt.id === result.id ? null : attempt.id)}
                                    title={`Attempt ${attempt.attempt_number} — ${attempt.status}`}
                                    style={{
                                        width: 28, height: 28, borderRadius: '50%',
                                        display: 'flex', alignItems: 'center', justifyContent: 'center',
                                        fontSize: 11, fontWeight: 700, color: '#fff', cursor: 'pointer',
                                        background: STATUS_DOT_COLORS[attempt.status] || '#94a3b8',
                                        transition: 'transform 0.15s',
                                        ...(attempt.id === activeResult.id ? {
                                            boxShadow: '0 0 0 3px rgba(79,70,229,0.3)',
                                            outline: '2px solid var(--accent-indigo)',
                                        } : {}),
                                    }}
                                    onMouseEnter={e => e.currentTarget.style.transform = 'scale(1.15)'}
                                    onMouseLeave={e => e.currentTarget.style.transform = 'scale(1)'}
                                >
                                    {attempt.attempt_number}
                                </div>
                                <span style={{
                                    fontSize: 10,
                                    color: attempt.id === activeResult.id ? 'var(--accent-indigo)' : 'var(--text-secondary)',
                                    fontWeight: attempt.id === activeResult.id ? 600 : 400,
                                }}>
                                    {attempt.id === result.id ? 'Current' : attempt.status}
                                </span>
                            </div>
                            {idx < arr.length - 1 && (
                                <div style={{ width: 28, height: 2, background: 'var(--border-color)', flexShrink: 0 }} />
                            )}
                        </React.Fragment>
                    ))}
                </div>
            )}

            {attempt_number > 1 && (
                <div style={{ marginBottom: 8, fontSize: '0.85em', color: '#856404', fontWeight: 600 }}>
                    ↻ Attempt {attempt_number}
                </div>
            )}

            {/* Failure Details + AI Analysis tabs (only for FAIL/ERROR) */}
            {isFailed && (
                <div style={{ marginBottom: 10 }}>
                    <div style={{
                        display: 'flex', gap: 0, borderBottom: '1px solid var(--border-color)', marginBottom: 10,
                    }}>
                        {[
                            { key: 'failure', label: 'Failure Details', color: 'var(--accent-red)' },
                            { key: 'ai',      label: 'AI Analysis',     color: 'var(--accent-indigo)' },
                        ].filter(tab => tab.key !== 'ai' || aiFeaturesEnabled).map(tab => {
                            const isActive = detailTab === tab.key;
                            return (
                                <button
                                    key={tab.key}
                                    data-testid={`result-tab-${tab.key}`}
                                    onClick={() => {
                                        setDetailTab(tab.key);
                                        if (tab.key === 'ai' && analyses === null) {
                                            loadAnalyses(activeResult.id);
                                        }
                                    }}
                                    style={{
                                        background: 'none', border: 'none', padding: '8px 14px',
                                        cursor: 'pointer', fontSize: '0.78rem', fontWeight: 600,
                                        textTransform: 'uppercase', letterSpacing: '0.06em',
                                        color: isActive ? tab.color : 'var(--text-secondary)',
                                        borderBottom: isActive ? `2px solid ${tab.color}` : '2px solid transparent',
                                        marginBottom: -1, transition: 'color 0.15s',
                                    }}
                                >
                                    {tab.label}
                                    {tab.key === 'ai' && analyses && analyses[0] && (
                                        <span style={{ marginLeft: 6, verticalAlign: 'middle' }}>
                                            <AIVerdictBadge verdict={analyses[0].verdict} confidence={analyses[0].confidence} dedupGroup={!!analyses[0].dedup_group_key} />
                                        </span>
                                    )}
                                </button>
                            );
                        })}
                    </div>

                    {detailTab === 'failure' && (
                        <div style={{
                            background: 'rgba(239,68,68,0.04)', border: '1px solid rgba(239,68,68,0.12)',
                            borderRadius: 8, padding: '14px 16px',
                        }}>
                            {(error_message || stack_trace || failure_type) ? (
                                <>
                                    {failure_type && (
                                        <div style={{ marginBottom: error_message || stack_trace ? 10 : 0 }}>
                                            <span style={{
                                                background: 'rgba(239,68,68,0.1)', color: 'var(--accent-red)',
                                                padding: '2px 8px', borderRadius: 10, fontSize: '0.68rem', fontWeight: 600,
                                            }}>
                                                {failure_type}
                                            </span>
                                        </div>
                                    )}
                                    {(error_message || stack_trace) && (
                                        <pre data-testid="error-message" style={{
                                            margin: 0, background: 'var(--bg-tertiary)', borderRadius: 6,
                                            padding: '12px 14px', fontFamily: 'monospace', fontSize: '0.78rem',
                                            color: 'var(--accent-red)', lineHeight: 1.7, whiteSpace: 'pre-wrap', wordBreak: 'break-word',
                                            overflowX: 'auto', maxHeight: 300,
                                        }}>
                                            {error_message}{error_message && stack_trace ? '\n\n' : ''}{stack_trace && <span data-testid="stack-trace">{stack_trace}</span>}
                                        </pre>
                                    )}
                                </>
                            ) : (
                                <div style={{ color: 'var(--text-secondary)', fontSize: '0.82rem' }}>
                                    No failure details available.
                                </div>
                            )}
                        </div>
                    )}

                    {detailTab === 'ai' && (
                        <div style={{
                            background: 'var(--glass-bg)', border: '1px solid var(--border-color)',
                            borderRadius: 6, padding: '12px 14px',
                        }}>
                            {analyses === null ? (
                                <div style={{ fontSize: 12, color: 'var(--text-secondary)' }}>Loading…</div>
                            ) : analyses.length === 0 ? (
                                <button
                                    disabled={analyzing}
                                    onClick={async () => {
                                        setAnalyzing(true);
                                        try {
                                            const row = await analyzeRunResult(activeResult.id);
                                            setAnalyses([row]);
                                            setSelectedVersion(row.version);
                                        } finally {
                                            setAnalyzing(false);
                                        }
                                    }}
                                >
                                    {analyzing ? 'Analyzing…' : 'Analyze'}
                                </button>
                            ) : (
                                <AIAnalysisCard
                                    analysis={analyses.find(a => a.version === selectedVersion) || analyses[0]}
                                    onReAnalyze={async () => {
                                        setAnalyzing(true);
                                        try {
                                            const row = await analyzeRunResult(activeResult.id);
                                            setAnalyses([row, ...analyses]);
                                            setSelectedVersion(row.version);
                                        } finally {
                                            setAnalyzing(false);
                                        }
                                    }}
                                    reAnalyzing={analyzing}
                                    versions={analyses}
                                    selectedVersion={selectedVersion}
                                    onSelectVersion={setSelectedVersion}
                                />
                            )}
                        </div>
                    )}
                </div>
            )}

            {/* Section B — Execution Context bar */}
            <div style={{
                display: 'flex', gap: 16, alignItems: 'center', padding: '8px 14px',
                background: 'var(--glass-bg)', border: '1px solid var(--border-color)',
                borderRadius: 6, marginBottom: 10, flexWrap: 'wrap',
            }}>
                <span style={{ fontSize: '0.78rem' }}>
                    <span style={{ color: 'var(--text-secondary)', opacity: 0.6 }}>Browser</span>{' '}
                    <span style={{ color: 'var(--text-primary)', opacity: 0.8 }}>{browser || '—'}</span>
                </span>
                <span style={{ fontSize: '0.78rem' }}>
                    <span style={{ color: 'var(--text-secondary)', opacity: 0.6 }}>OS</span>{' '}
                    <span style={{ color: 'var(--text-primary)', opacity: 0.8 }}>{os || '—'}</span>
                </span>
                <span style={{ fontSize: '0.78rem' }}>
                    <span style={{ color: 'var(--text-secondary)', opacity: 0.6 }}>Env</span>{' '}
                    <span style={{ color: 'var(--text-primary)', opacity: 0.8 }}>{environment || '—'}</span>
                </span>
                <span style={{ fontSize: '0.78rem' }}>
                    <span style={{ color: 'var(--text-secondary)', opacity: 0.6 }}>Ver</span>{' '}
                    <span style={{ color: 'var(--text-primary)', opacity: 0.8 }}>{app_version || '—'}</span>
                </span>
                <span style={{ fontSize: '0.78rem' }}>
                    <span style={{ color: 'var(--text-secondary)', opacity: 0.6 }}>Duration</span>{' '}
                    <span style={{ color: 'var(--text-primary)', opacity: 0.8 }}>{formatDuration(duration_ms)}</span>
                </span>
                {(screenshotUrls.length > 0 || video || trace_url) && (
                    <div style={{ marginLeft: 'auto', display: 'flex', gap: 8, alignItems: 'center' }}>
                        {screenshotUrls.length > 0 && (
                            <button
                                onClick={() => { setGalleryIndex(0); setGalleryOpen(true); }}
                                data-testid="artifact-screenshot"
                                title={`${screenshotUrls.length} screenshot${screenshotUrls.length > 1 ? 's' : ''}`}
                                style={{ background: 'none', border: 'none', color: 'var(--accent-indigo)', cursor: 'pointer', fontSize: '0.85rem', padding: 0 }}
                            >
                                {"📸"} {screenshotUrls.length > 1 && screenshotUrls.length}
                            </button>
                        )}
                        {video && /^(https?:\/\/|\/)/i.test(video) && <a href={video} target="_blank" rel="noopener noreferrer" data-testid="artifact-video" title="Video" style={{ color: 'var(--accent-indigo)', textDecoration: 'none', fontSize: '0.85rem' }}>🎥</a>}
                        {trace_url && /^(https?:\/\/|\/)/i.test(trace_url) && <a href={trace_url} target="_blank" rel="noopener noreferrer" data-testid="artifact-trace" title="Trace" style={{ color: 'var(--accent-indigo)', textDecoration: 'none', fontSize: '0.85rem' }}>🔗</a>}
                    </div>
                )}
            </div>

            {/* Screenshot inline strip */}
            {screenshotUrls.length > 0 && (
                <div style={{ marginBottom: 10 }}>
                    <div style={{ fontSize: '0.78rem', color: 'var(--text-secondary)', marginBottom: 6 }}>Screenshots</div>
                    <div style={{ display: 'flex', gap: 10, overflowX: 'auto', padding: '2px 2px 6px' }}>
                        {screenshotUrls.map((url, i) => (
                            <div key={i} style={{ flexShrink: 0 }}>
                                <div
                                    style={{
                                        border: '1px solid rgba(255,255,255,0.14)',
                                        borderRadius: 6,
                                        overflow: 'hidden',
                                        width: 280,
                                        height: 180,
                                        boxSizing: 'border-box',
                                        boxShadow: '0 0 0 1px rgba(0,0,0,0.32), 0 10px 20px rgba(0,0,0,0.18)',
                                        background: 'var(--bg-primary)',
                                        cursor: 'pointer',
                                        transition: 'border-color 0.2s, box-shadow 0.2s, transform 0.2s',
                                    }}
                                    onMouseEnter={e => {
                                        e.currentTarget.style.borderColor = 'var(--accent-indigo)';
                                        e.currentTarget.style.boxShadow = '0 0 0 1px rgba(99,102,241,0.35), 0 14px 28px rgba(0,0,0,0.24)';
                                        e.currentTarget.style.transform = 'translateY(-1px)';
                                    }}
                                    onMouseLeave={e => {
                                        e.currentTarget.style.borderColor = 'rgba(255,255,255,0.14)';
                                        e.currentTarget.style.boxShadow = '0 0 0 1px rgba(0,0,0,0.32), 0 10px 20px rgba(0,0,0,0.18)';
                                        e.currentTarget.style.transform = 'translateY(0)';
                                    }}
                                    onClick={() => { setGalleryIndex(i); setGalleryOpen(true); }}
                                >
                                    <img
                                        src={url}
                                        alt={`Step ${i + 1}`}
                                        style={{
                                            display: 'block',
                                            width: '100%',
                                            height: '100%',
                                            objectFit: 'cover',
                                            background: 'var(--bg-primary)',
                                        }}
                                    />
                                </div>
                                <div style={{ fontSize: '0.68rem', color: 'var(--text-secondary)', marginTop: 4, textAlign: 'center' }}>
                                    Screenshot {i + 1} of {screenshotUrls.length}
                                </div>
                            </div>
                        ))}
                    </div>
                </div>
            )}

            {galleryOpen && (
                <ScreenshotGallery
                    screenshots={screenshotUrls}
                    initialIndex={galleryIndex}
                    onClose={() => setGalleryOpen(false)}
                />
            )}

            {/* Section C — Logs & Steps (conditional, collapsible) */}
            {(log_text || steps) && (
                <div style={{ display: 'flex', gap: 8, marginBottom: 10 }}>
                    {log_text && (
                        <div style={{ flex: 1 }}>
                            <button
                                onClick={() => setShowLogs(!showLogs)}
                                style={{
                                    background: 'none', border: 'none', color: 'var(--accent-indigo)',
                                    cursor: 'pointer', fontSize: '0.75rem', fontWeight: 600, padding: '4px 0',
                                }}
                            >
                                {showLogs ? '▼ Hide Logs' : '▶ Show Logs'}
                            </button>
                            {showLogs && (
                                <pre style={{
                                    margin: '6px 0 0', background: 'rgba(0,0,0,0.2)', border: '1px solid var(--border-color)',
                                    borderRadius: 6, padding: 10, maxHeight: 200, overflowY: 'auto',
                                    fontSize: '0.75rem', fontFamily: 'monospace', color: 'var(--text-secondary)',
                                    whiteSpace: 'pre-wrap', wordBreak: 'break-word',
                                }}>
                                    {log_text}
                                </pre>
                            )}
                        </div>
                    )}
                    {steps && (
                        <div style={{ flex: 1 }}>
                            <button
                                onClick={() => setShowSteps(!showSteps)}
                                style={{
                                    background: 'none', border: 'none', color: 'var(--accent-indigo)',
                                    cursor: 'pointer', fontSize: '0.75rem', fontWeight: 600, padding: '4px 0',
                                }}
                            >
                                {showSteps ? '▼ Hide Steps' : '▶ Show Steps'}
                            </button>
                            {showSteps && (
                                <pre style={{
                                    margin: '6px 0 0', background: 'rgba(0,0,0,0.2)', border: '1px solid var(--border-color)',
                                    borderRadius: 6, padding: 10, maxHeight: 200, overflowY: 'auto',
                                    fontSize: '0.75rem', fontFamily: 'monospace', color: 'var(--text-secondary)',
                                    whiteSpace: 'pre-wrap',
                                }}>
                                    {JSON.stringify(steps, null, 2)}
                                </pre>
                            )}
                        </div>
                    )}
                </div>
            )}

            {/* Section E — Comments */}
            <div style={{
                background: 'var(--glass-bg)', border: '1px solid var(--border-color)',
                borderRadius: 6, padding: '8px 14px', marginBottom: 10,
            }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 8 }}>
                    <span style={{ fontSize: '0.7rem' }}>💬</span>
                    <span style={{ fontSize: '0.72rem', fontWeight: 700, color: 'var(--accent-indigo)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>
                        Comments
                    </span>
                </div>
                <CommentsPanel
                    targetType="result"
                    runId={activeResult.test_run_id}
                    resultId={activeResult.id}
                    compact={true}
                />
            </div>

            {/* Section D — Linked Defects */}
            {test_case_id && (
                <div style={{
                    background: 'var(--glass-bg)', border: '1px solid var(--border-color)',
                    borderRadius: 6, padding: '8px 14px',
                }}>
                    <DefectLinkPanel
                        resultId={activeResult.id}
                        runId={activeResult.test_run_id}
                        createDefectContext={createDefectContext}
                        containerStyle={{ marginTop: 0 }}
                    />
                </div>
            )}
        </div>
    );
};

function AIAnalysisCard({ analysis, onReAnalyze, reAnalyzing, versions, selectedVersion, onSelectVersion }) {
    const [showRationale, setShowRationale] = useState(false);
    if (!analysis) return null;

    const failed = typeof analysis.summary === 'string' && /^analysis failed\s*:/i.test(analysis.summary);
    const failureMessage = failed ? analysis.summary.replace(/^analysis failed\s*:\s*/i, '') : null;
    const hasNextAction = !!(analysis.next_action && analysis.next_action.trim() && analysis.next_action.trim() !== '—');
    const hasRationale = !!(analysis.rationale && analysis.rationale.trim() && analysis.rationale.trim() !== '—');

    return (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            {/* Meta row: verdict + confidence on left, model · time on right */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: 8 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
                    <AIVerdictBadge verdict={analysis.verdict} confidence={analysis.confidence} dedupGroup={!!analysis.dedup_group_key} />
                    {analysis.dedup_group_key && analysis.source_analysis_id && (
                        <span title="Grouped from representative analysis" style={{ color: 'var(--text-secondary)', fontSize: 11 }}>
                            ↳ Grouped
                        </span>
                    )}
                </div>
                <div style={{ color: 'var(--text-secondary)', fontSize: 11, display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
                    {analysis.model_name && <span>{analysis.model_name}</span>}
                    {analysis.model_name && analysis.created_at && <span style={{ opacity: 0.5 }}>•</span>}
                    {analysis.created_at && <span>{new Date(analysis.created_at).toLocaleString()}</span>}
                </div>
            </div>

            {/* Summary (or failure card) */}
            {failed ? (
                <div style={{
                    background: 'rgba(239,68,68,0.06)', border: '1px solid rgba(239,68,68,0.25)',
                    borderRadius: 6, padding: '10px 12px',
                }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 4, color: 'var(--accent-red)', fontSize: '0.72rem', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.06em' }}>
                        <span>⚠️</span> Analysis failed
                    </div>
                    <div style={{ fontSize: '0.82rem', fontFamily: 'monospace', color: 'var(--text-secondary)', whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                        {failureMessage}
                    </div>
                </div>
            ) : (
                <div>
                    <div style={{ fontSize: '0.68rem', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.06em', color: 'var(--text-secondary)', marginBottom: 4 }}>Summary</div>
                    <div style={{ fontSize: '0.88rem', lineHeight: 1.5, color: 'var(--text-primary)' }}>{analysis.summary || <span style={{ color: 'var(--text-secondary)', fontStyle: 'italic' }}>No summary provided.</span>}</div>
                </div>
            )}

            {/* Next action callout */}
            {!failed && (
                <div style={{
                    background: 'rgba(99,102,241,0.06)', border: '1px solid rgba(99,102,241,0.22)',
                    borderRadius: 6, padding: '10px 12px',
                }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 4, fontSize: '0.68rem', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.06em', color: 'var(--accent-indigo)' }}>
                        <span>💡</span> Suggested next action
                    </div>
                    <div style={{ fontSize: '0.88rem', lineHeight: 1.5, color: 'var(--text-primary)' }}>
                        {hasNextAction ? analysis.next_action : <span style={{ color: 'var(--text-secondary)', fontStyle: 'italic' }}>No suggestion provided.</span>}
                    </div>
                </div>
            )}

            {/* Rationale collapsible */}
            {hasRationale && (
                <div style={{ border: '1px solid var(--border-color)', borderRadius: 6, overflow: 'hidden' }}>
                    <button
                        onClick={() => setShowRationale(!showRationale)}
                        style={{
                            display: 'flex', alignItems: 'center', gap: 8, width: '100%',
                            padding: '8px 12px', background: 'transparent', border: 'none',
                            cursor: 'pointer', textAlign: 'left',
                            color: 'var(--text-secondary)', fontSize: '0.72rem', fontWeight: 700,
                            textTransform: 'uppercase', letterSpacing: '0.06em',
                        }}
                    >
                        <span style={{ color: 'var(--accent-indigo)' }}>{showRationale ? '▾' : '▸'}</span>
                        Rationale
                    </button>
                    {showRationale && (
                        <div style={{
                            padding: '0 12px 10px', fontSize: '0.82rem', lineHeight: 1.5,
                            color: 'var(--text-secondary)', whiteSpace: 'pre-wrap',
                        }}>
                            {analysis.rationale}
                        </div>
                    )}
                </div>
            )}

            {/* Footer: Re-analyze + version pills */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap', paddingTop: 4 }}>
                <button
                    onClick={onReAnalyze}
                    disabled={reAnalyzing}
                    style={{
                        padding: '6px 14px', fontSize: '0.78rem', fontWeight: 600,
                        color: 'var(--accent-indigo)', background: 'rgba(99,102,241,0.06)',
                        border: '1px solid rgba(99,102,241,0.3)', borderRadius: 6,
                        cursor: reAnalyzing ? 'wait' : 'pointer',
                        opacity: reAnalyzing ? 0.6 : 1,
                        display: 'inline-flex', alignItems: 'center', gap: 6,
                    }}
                >
                    <span style={{ fontSize: '0.9rem' }}>↻</span>
                    {reAnalyzing ? 'Re-analyzing…' : 'Re-analyze'}
                </button>
                {versions.length > 1 && (
                    <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
                        <span style={{ color: 'var(--text-secondary)', fontSize: 11, textTransform: 'uppercase', letterSpacing: '0.05em', fontWeight: 600 }}>Version</span>
                        {versions.map((v) => {
                            const active = v.version === selectedVersion;
                            return (
                                <button
                                    key={v.id}
                                    onClick={() => onSelectVersion(v.version)}
                                    title={v.created_at ? new Date(v.created_at).toLocaleString() : ''}
                                    style={{
                                        padding: '3px 9px', fontSize: 11, fontWeight: 600,
                                        borderRadius: 99, cursor: 'pointer',
                                        background: active ? 'var(--accent-indigo)' : 'transparent',
                                        color: active ? '#fff' : 'var(--text-secondary)',
                                        border: `1px solid ${active ? 'var(--accent-indigo)' : 'var(--border-color)'}`,
                                        transition: 'all 0.15s',
                                    }}
                                >
                                    v{v.version}
                                </button>
                            );
                        })}
                    </div>
                )}
            </div>
        </div>
    );
}

export default RunResultDetail;
