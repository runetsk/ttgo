import React, { useCallback, useEffect, useState } from 'react';
import { getRunAnalysisJob, cancelRunAnalysisJob, retryFailedRunAnalysis, analyzeRunFailures } from '../api';
import { useSubscription } from '../hooks/useSubscription';
import { jobSummary, showEndedJob, jobTelemetry, runningNote, newerJobState } from '../utils/analysisJob.js';

const dismissKey = (runId) => `ttgo.analysisBanner.dismissed.${runId}`;

function readDismissed(runId) {
    try {
        return localStorage.getItem(dismissKey(runId));
    } catch {
        return null;
    }
}

const TONES = {
    ok: { bg: 'rgba(34,197,94,0.06)', border: 'rgba(34,197,94,0.3)', dot: 'var(--accent-green, #22c55e)' },
    warn: { bg: 'rgba(234,179,8,0.08)', border: 'rgba(234,179,8,0.3)', dot: '#eab308' },
    error: { bg: 'rgba(239,68,68,0.06)', border: 'rgba(239,68,68,0.3)', dot: 'var(--accent-red)' },
};

export default function RunAnalysisBanner({ runId, refreshKey = 0 }) {
    const [job, setJob] = useState(null);
    const [covered, setCovered] = useState(0);
    const [watched, setWatched] = useState(false); // this page saw the job queued or running
    const [dismissedId, setDismissedId] = useState(() => readDismissed(runId));
    const [retrying, setRetrying] = useState(false);

    useEffect(() => {
        if (!runId) return;
        getRunAnalysisJob(runId).then((j) => {
            setJob(j || null);
            if (j && (j.status === 'queued' || j.status === 'running')) setWatched(true);
        }).catch(() => setJob(null));
    }, [runId, refreshKey]);

    useSubscription(runId ? `run:${runId}` : null, useCallback((event) => {
        if (event.type !== 'run_analysis.progress' && event.type !== 'run_analysis.completed') return;
        const d = event.data || {};
        if (event.type === 'run_analysis.progress') setWatched(true);
        setJob((prev) => ({
            ...(prev || {}),
            id: d.job_id,
            status: d.status,
            analyzed_count: d.analyzed_groups,
            unique_groups: d.unique_groups,
            capped_at: d.capped_groups,
            total_failures: d.total_failures,
        }));
        if (typeof d.covered_failures === 'number') setCovered(d.covered_failures);
        if (event.type === 'run_analysis.completed') {
            // The event carries progress only; the outcome counts come with the job.
            getRunAnalysisJob(runId).then((j) => setJob((prev) => newerJobState(prev, j))).catch(() => {});
        }
    }, [runId]));

    // While a job runs, its outcome counts (explanations still being written) come only with the job
    // itself, so refresh it, at most once a second, while the run's events arrive: decisions,
    // explanations written (.updated) and progress. The subscription ends with the job.
    const running = job?.status === 'queued' || job?.status === 'running';
    useSubscription(runId && running ? `run:${runId}` : null, useCallback(() => {
        getRunAnalysisJob(runId).then((j) => setJob((prev) => newerJobState(prev, j))).catch(() => {});
    }, [runId]), { debounceMs: 1000 });

    if (!job) return null;

    if (job.status !== 'queued' && job.status !== 'running') {
        const summary = jobSummary(job);
        if (!showEndedJob(summary, watched, dismissedId === job.id)) return null;
        const tone = TONES[summary.tone] || TONES.ok;
        const dismiss = () => {
            try { localStorage.setItem(dismissKey(runId), job.id); } catch { /* per-viewer convenience only */ }
            setDismissedId(job.id);
        };
        const retry = async () => {
            setRetrying(true);
            try {
                const next = await retryFailedRunAnalysis(runId);
                setJob(next);
                setWatched(true);
            } catch {
                // toasted by the API interceptor
            } finally {
                setRetrying(false);
            }
        };
        const runAnyway = async () => {
            setRetrying(true);
            try {
                const next = await analyzeRunFailures(runId, { acknowledgeBudget: true });
                setJob(next);
                setWatched(true);
            } catch {
                // toasted by the API interceptor
            } finally {
                setRetrying(false);
            }
        };
        const telemetry = jobTelemetry(job);
        return (
            <div style={{
                padding: '8px 14px', background: tone.bg, border: `1px solid ${tone.border}`, borderRadius: 8,
                display: 'flex', alignItems: 'center', gap: 10, fontSize: 13, marginBottom: 10, flexWrap: 'wrap',
            }} data-testid="run-analysis-summary">
                <span style={{ display: 'inline-block', width: 8, height: 8, borderRadius: '50%', background: tone.dot, flexShrink: 0 }} />
                <span style={{ color: 'var(--text-primary)', flex: '1 1 320px' }}>
                    {summary.text}
                    {job.pipeline_label && (
                        <span style={{ display: 'block', color: 'var(--text-secondary)', fontSize: 12 }} data-testid="run-analysis-pipeline">
                            {job.pipeline_label}
                        </span>
                    )}
                    {telemetry && (
                        <span style={{ display: 'block', color: 'var(--text-secondary)', fontSize: 12 }} data-testid="run-analysis-telemetry">
                            {telemetry}
                        </span>
                    )}
                    {summary.retryHint && (
                        <span style={{ display: 'block', color: 'var(--text-secondary)', fontSize: 12 }} data-testid="run-analysis-retry-hint">
                            {summary.retryHint}
                        </span>
                    )}
                </span>
                {summary.retryable && (
                    <button onClick={retry} disabled={retrying} title={summary.retryHint || undefined} className="action-btn" style={{ padding: '4px 12px', fontSize: '0.78rem' }} data-testid="run-analysis-retry-failed">
                        {retrying ? 'Queuing…' : 'Retry failed groups'}
                    </button>
                )}
                {summary.runAnyway && (
                    <button onClick={runAnyway} disabled={retrying} className="action-btn" style={{ padding: '4px 12px', fontSize: '0.78rem' }} data-testid="run-analysis-run-anyway">
                        {retrying ? 'Queuing…' : 'Run anyway'}
                    </button>
                )}
                <button onClick={dismiss} title="Hide until the next analysis" aria-label="Dismiss"
                    className="action-btn" style={{ padding: '4px 12px', fontSize: '0.78rem' }} data-testid="run-analysis-dismiss">
                    ✕
                </button>
            </div>
        );
    }

    const pct = job.capped_at > 0 ? Math.min(100, (job.analyzed_count / job.capped_at) * 100) : 0;
    const note = runningNote(job);

    return (
        <div style={{
            padding: '8px 14px', background: 'rgba(99,102,241,0.06)',
            border: '1px solid rgba(99,102,241,0.25)', borderRadius: 8,
            display: 'flex', alignItems: 'center', gap: 10, fontSize: 13,
            marginBottom: 10,
        }} title={job.pipeline_label || undefined}>
            <span style={{
                display: 'inline-block', width: 8, height: 8, borderRadius: '50%',
                background: 'var(--accent-indigo)', flexShrink: 0,
            }} />
            <span style={{ color: 'var(--text-primary)' }}>
                {job.retry_failed_only ? 'AI retrying failed groups' : 'AI analyzing failures'} — {job.analyzed_count || 0} of {job.capped_at || 0} groups
                {covered ? ` (covers ${covered} of ${job.total_failures || 0} failed results)` : ''}
                {note && <span data-testid="run-analysis-explanations-pending"> · {note}</span>}
            </span>
            <div style={{ flex: 1, height: 3, background: 'var(--border-color)', borderRadius: 2, overflow: 'hidden', margin: '0 10px', minWidth: 80 }}>
                <div style={{ height: '100%', width: `${pct}%`, background: 'var(--accent-indigo)', transition: 'width 0.3s ease' }} />
            </div>
            <button
                onClick={() => cancelRunAnalysisJob(runId).catch(() => {})}
                className="action-btn" style={{ padding: '4px 12px', fontSize: '0.78rem' }}
            >
                Cancel
            </button>
        </div>
    );
}
