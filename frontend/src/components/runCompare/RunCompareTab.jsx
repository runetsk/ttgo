import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { getTestRun, getTestRuns, getCurrentRunAnalyses } from '../../api';
import { useAIGeneration } from '../../contexts/AIGenerationContext';
import { diffRuns } from '../../utils/runDiff';
import RunCompareSummary from './RunCompareSummary';
import RunCompareDiffTable from './RunCompareDiffTable';

function Empty({ testid, children }) {
    return <div data-testid={testid} style={{ padding: '28px 16px', textAlign: 'center', color: 'var(--text-secondary)', fontSize: '0.86rem' }}>{children}</div>;
}

export default function RunCompareTab({ run }) {
    const { aiFeaturesEnabled } = useAIGeneration();
    const [searchParams, setSearchParams] = useSearchParams();
    const compareWith = searchParams.get('compareWith') || '';

    const [runs, setRuns] = useState([]);
    const [comparedRun, setComparedRun] = useState(null);
    const [analysesThis, setAnalysesThis] = useState({});
    const [analysesCompared, setAnalysesCompared] = useState({});
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState(null);
    const defaultedRef = useRef(false);

    // Recent runs for the picker (exclude the opened run).
    useEffect(() => {
        getTestRuns(null, null, 'created_at', 'desc', 1, 50)
            .then((d) => setRuns(((d && d.runs) || []).filter((r) => r.id !== run.id)))
            .catch(() => setRuns([]));
    }, [run.id]);

    // Auto-pick the most recent other run once, only if nothing is selected yet.
    useEffect(() => {
        if (!defaultedRef.current && !compareWith && runs.length > 0) {
            defaultedRef.current = true;
            setSearchParams((prev) => { const n = new URLSearchParams(prev); n.set('compareWith', runs[0].id); return n; }, { replace: true });
        }
    }, [compareWith, runs, setSearchParams]);

    // Fetch the compared run.
    useEffect(() => {
        if (!compareWith || compareWith === run.id) { setComparedRun(null); return; } // eslint-disable-line react-hooks/set-state-in-effect
        setLoading(true); setError(null);
        getTestRun(compareWith)
            .then((d) => { setComparedRun(d && d.id ? d : null); setLoading(false); })
            .catch((e) => { setError((e && e.message) || 'Failed to load run'); setLoading(false); });
    }, [compareWith, run.id]);

    // AI verdicts for both runs (only when enabled).
    useEffect(() => {
        if (!aiFeaturesEnabled) { setAnalysesThis({}); return; } // eslint-disable-line react-hooks/set-state-in-effect
        getCurrentRunAnalyses(run.id).then(setAnalysesThis).catch(() => setAnalysesThis({}));
    }, [aiFeaturesEnabled, run.id]);
    useEffect(() => {
        if (!aiFeaturesEnabled || !comparedRun) { setAnalysesCompared({}); return; } // eslint-disable-line react-hooks/set-state-in-effect
        getCurrentRunAnalyses(comparedRun.id).then(setAnalysesCompared).catch(() => setAnalysesCompared({}));
    }, [aiFeaturesEnabled, comparedRun]);

    // Recomputes when the opened run updates live (WebSocket) or the selection changes.
    const diff = useMemo(() => (comparedRun ? diffRuns(run, comparedRun) : null), [run, comparedRun]);

    const onPick = (e) => {
        const id = e.target.value;
        setSearchParams((prev) => {
            const n = new URLSearchParams(prev);
            if (id) n.set('compareWith', id); else n.delete('compareWith');
            return n;
        }, { replace: true });
    };

    return (
        <div data-testid="run-compare-tab">
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap', marginBottom: 16 }}>
                <span style={{ fontSize: '0.82rem', color: 'var(--text-secondary)' }}>Comparing</span>
                <span style={{ fontSize: '0.88rem', fontWeight: 600 }}>{run.name}</span>
                <span style={{ color: 'var(--text-secondary)' }}>⇄</span>
                <select className="modern-select" data-testid="compare-run-select" value={compareWith} onChange={onPick} style={{ minWidth: 240, fontSize: '0.85rem' }}>
                    <option value="">Select a run to compare…</option>
                    {runs.map((r) => (
                        <option key={r.id} value={r.id}>{r.name} — {new Date(r.created_at).toLocaleDateString()}</option>
                    ))}
                </select>
            </div>

            {!compareWith && <Empty testid="compare-empty">Pick a run to compare against.</Empty>}
            {compareWith && compareWith === run.id && <Empty testid="compare-same-run">Select a different run to compare.</Empty>}
            {loading && <div style={{ padding: 16, color: 'var(--text-secondary)' }}>Loading comparison…</div>}
            {error && <div data-testid="compare-error" style={{ padding: '12px 16px', borderRadius: 8, background: 'rgba(239,68,68,0.06)', border: '1px solid rgba(239,68,68,0.2)', color: 'var(--accent-red)', fontSize: '0.85rem' }}>Error: {error}</div>}

            {diff && !loading && (
                <>
                    <RunCompareSummary summary={diff.summary} />
                    <RunCompareDiffTable
                        groups={diff.groups}
                        summary={diff.summary}
                        thisName={run.name}
                        comparedName={comparedRun.name}
                        analysesThis={analysesThis}
                        analysesCompared={analysesCompared}
                        aiEnabled={aiFeaturesEnabled}
                    />
                </>
            )}
        </div>
    );
}
