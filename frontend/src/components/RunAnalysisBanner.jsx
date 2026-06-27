import React, { useCallback, useEffect, useState } from 'react';
import { getRunAnalysisJob, cancelRunAnalysisJob } from '../api';
import { useSubscription } from '../hooks/useSubscription';

export default function RunAnalysisBanner({ runId, refreshKey = 0 }) {
    const [job, setJob] = useState(null);
    const [covered, setCovered] = useState(0);

    useEffect(() => {
        if (!runId) return;
        getRunAnalysisJob(runId).then((j) => setJob(j || null)).catch(() => setJob(null));
    }, [runId, refreshKey]);

    useSubscription(runId ? `run:${runId}` : null, useCallback((event) => {
        if (event.type !== 'run_analysis.progress' && event.type !== 'run_analysis.completed') return;
        const d = event.data || {};
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
    }, []));

    if (!job) return null;
    if (job.status !== 'queued' && job.status !== 'running') return null;

    const pct = job.capped_at > 0 ? Math.min(100, (job.analyzed_count / job.capped_at) * 100) : 0;

    return (
        <div style={{
            padding: '8px 14px', background: 'rgba(99,102,241,0.06)',
            border: '1px solid rgba(99,102,241,0.25)', borderRadius: 8,
            display: 'flex', alignItems: 'center', gap: 10, fontSize: 13,
            marginBottom: 10,
        }}>
            <span style={{
                display: 'inline-block', width: 8, height: 8, borderRadius: '50%',
                background: 'var(--accent-indigo)', flexShrink: 0,
            }} />
            <span style={{ color: 'var(--text-primary)' }}>
                AI analyzing failures — {job.analyzed_count || 0} of {job.capped_at || 0} groups
                {covered ? ` (covers ${covered} of ${job.total_failures || 0} failed results)` : ''}
            </span>
            <div style={{ flex: 1, height: 3, background: 'var(--border-color)', borderRadius: 2, overflow: 'hidden', margin: '0 10px', minWidth: 80 }}>
                <div style={{ height: '100%', width: `${pct}%`, background: 'var(--accent-indigo)', transition: 'width 0.3s ease' }} />
            </div>
            <button
                onClick={() => cancelRunAnalysisJob(runId).catch(() => {})}
                style={{ padding: '2px 8px', fontSize: 11 }}
            >
                Cancel
            </button>
        </div>
    );
}
