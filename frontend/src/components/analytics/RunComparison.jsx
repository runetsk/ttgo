import React, { useState, useEffect, useMemo } from 'react';
import { getTestRuns, getAnalyticsCompareRuns } from '../../api';
import { formatDuration } from './utils';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer, Cell } from 'recharts';

const COLORS = { pass: '#22c55e', fail: '#ef4444', skip: '#9ca3af' };

function DiffIndicator({ v1, v2, invert = false, fmt }) {
    if (typeof v1 !== 'number' || typeof v2 !== 'number') return <span style={{ color: 'var(--text-secondary)' }}>—</span>;
    const diff = v2 - v1;
    if (diff === 0) return <span style={{ color: 'var(--text-secondary)', fontSize: '0.82rem' }}>No change</span>;
    const positive = invert ? diff < 0 : diff > 0;
    const color = positive ? '#22c55e' : '#ef4444';
    const arrow = diff > 0 ? '↑' : '↓';
    const display = fmt ? fmt(Math.abs(diff)) : Math.abs(diff);
    return (
        <span style={{ color, fontWeight: 600, fontSize: '0.85rem', display: 'inline-flex', alignItems: 'center', gap: 3 }}>
            <span>{arrow}</span>
            {display}
        </span>
    );
}

function RunDropdown({ value, onChange, runs, label }) {
    return (
        <div style={{ flex: 1, minWidth: 200 }}>
            <label style={{
                display: 'block',
                fontSize: '0.72rem',
                textTransform: 'uppercase',
                letterSpacing: '0.08em',
                fontWeight: 700,
                color: 'var(--text-secondary)',
                marginBottom: 6,
            }}>
                {label}
            </label>
            <select
                className="modern-select"
                value={value}
                onChange={e => onChange(e.target.value)}
                style={{ width: '100%', fontSize: '0.88rem' }}
            >
                <option value="">Select a run...</option>
                {runs.map(r => (
                    <option key={r.id} value={r.id}>
                        {r.name} — {new Date(r.created_at).toLocaleDateString()}
                    </option>
                ))}
            </select>
        </div>
    );
}

export default function RunComparison() {
    const [runs, setRuns] = useState([]);
    const [run1, setRun1] = useState('');
    const [run2, setRun2] = useState('');
    const [comparison, setComparison] = useState(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState(null);

    useEffect(() => {
        getTestRuns(null, null, 'created_at', 'desc', 1, 50)
            .then(data => setRuns(data?.runs || []))
            .catch(() => setRuns([]));
    }, []);

    // Auto-select the two most recent runs
    useEffect(() => {
        if (runs.length >= 2 && !run1 && !run2) {
            setRun1(runs[0].id);
            setRun2(runs[1].id);
        }
    }, [runs, run1, run2]);

    useEffect(() => {
        if (!run1 || !run2 || run1 === run2) {
            setComparison(null);
            return;
        }
        setLoading(true);
        setError(null);
        getAnalyticsCompareRuns(run1, run2)
            .then(data => { setComparison(data); setLoading(false); })
            .catch(err => { setError(err?.message || 'Failed to compare'); setLoading(false); });
    }, [run1, run2]);

    const swapRuns = () => {
        setRun1(run2);
        setRun2(run1);
    };

    const chartData = useMemo(() => {
        if (!comparison) return [];
        return [
            {
                name: comparison.run1.run_name || 'Run 1',
                Passed: comparison.run1.passed,
                Failed: comparison.run1.failed,
                Skipped: comparison.run1.skipped,
            },
            {
                name: comparison.run2.run_name || 'Run 2',
                Passed: comparison.run2.passed,
                Failed: comparison.run2.failed,
                Skipped: comparison.run2.skipped,
            },
        ];
    }, [comparison]);

    const metrics = useMemo(() => {
        if (!comparison) return [];
        const r1 = comparison.run1;
        const r2 = comparison.run2;
        return [
            { label: 'Total Tests', v1: r1.total_tests, v2: r2.total_tests },
            { label: 'Passed', v1: r1.passed, v2: r2.passed, color: COLORS.pass },
            { label: 'Failed', v1: r1.failed, v2: r2.failed, color: COLORS.fail, invert: true },
            { label: 'Skipped', v1: r1.skipped, v2: r2.skipped, color: COLORS.skip, invert: true },
            { label: 'Pass Rate', v1: r1.pass_rate, v2: r2.pass_rate, fmt: v => v.toFixed(1) + '%' },
            { label: 'Duration', v1: r1.total_dur_ms, v2: r2.total_dur_ms, fmt: formatDuration, invert: true },
        ];
    }, [comparison]);

    return (
        <div>
            {/* Run pickers */}
            <div style={{
                display: 'flex',
                gap: 12,
                marginBottom: 20,
                flexWrap: 'wrap',
                alignItems: 'flex-end',
            }}>
                <RunDropdown value={run1} onChange={setRun1} runs={runs} label="Baseline Run" />
                <button
                    type="button"
                    onClick={swapRuns}
                    disabled={!run1 || !run2}
                    title="Swap runs"
                    style={{
                        padding: '8px 12px',
                        border: '1px solid var(--border-color)',
                        borderRadius: 'var(--radius-md)',
                        background: 'var(--bg-tertiary)',
                        color: 'var(--text-secondary)',
                        cursor: run1 && run2 ? 'pointer' : 'not-allowed',
                        fontSize: '1rem',
                        lineHeight: 1,
                        marginBottom: 1,
                        transition: 'all 0.15s',
                        opacity: run1 && run2 ? 1 : 0.4,
                    }}
                >
                    ⇄
                </button>
                <RunDropdown value={run2} onChange={setRun2} runs={runs} label="Compare With" />
            </div>

            {run1 && run2 && run1 === run2 && (
                <div style={{
                    padding: '10px 16px',
                    marginBottom: 16,
                    borderRadius: 'var(--radius-md)',
                    background: 'rgba(239,68,68,0.06)',
                    border: '1px solid rgba(239,68,68,0.2)',
                    color: '#ef4444',
                    fontSize: '0.84rem',
                }}>
                    Please select two different runs to compare.
                </div>
            )}

            {loading && <div className="analytics-loading">Comparing runs...</div>}

            {error && (
                <div style={{
                    padding: '12px 16px',
                    marginBottom: 16,
                    borderRadius: 'var(--radius-md)',
                    background: 'rgba(239,68,68,0.06)',
                    border: '1px solid rgba(239,68,68,0.2)',
                    color: '#ef4444',
                    fontSize: '0.85rem',
                }}>
                    Error: {error}
                </div>
            )}

            {comparison && !loading && (
                <div style={{ display: 'flex', gap: 20, flexWrap: 'wrap' }}>
                    {/* Chart */}
                    <div style={{
                        flex: '1 1 380px',
                        minWidth: 320,
                        background: 'rgba(255,255,255,0.02)',
                        border: '1px solid var(--border-color)',
                        borderRadius: 'var(--radius-lg)',
                        padding: 20,
                    }}>
                        <h4 style={{ margin: '0 0 16px', color: 'var(--text-primary)', fontSize: '0.9rem', fontWeight: 600 }}>
                            Results Distribution
                        </h4>
                        <div style={{ width: '100%', height: 220 }}>
                            <ResponsiveContainer>
                                <BarChart data={chartData} margin={{ top: 5, right: 10, left: -10, bottom: 5 }} barCategoryGap="25%">
                                    <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.06)" vertical={false} />
                                    <XAxis
                                        dataKey="name"
                                        tick={{ fontSize: 11, fill: 'var(--text-secondary)' }}
                                        axisLine={{ stroke: 'var(--border-color)' }}
                                        tickLine={false}
                                    />
                                    <YAxis
                                        tick={{ fontSize: 11, fill: 'var(--text-secondary)' }}
                                        axisLine={false}
                                        tickLine={false}
                                    />
                                    <Tooltip
                                        contentStyle={{
                                            background: 'var(--bg-secondary)',
                                            border: '1px solid var(--border-color)',
                                            borderRadius: 8,
                                            fontSize: '0.82rem',
                                        }}
                                    />
                                    <Legend iconType="circle" iconSize={8} wrapperStyle={{ fontSize: '0.8rem' }} />
                                    <Bar dataKey="Passed" fill={COLORS.pass} radius={[4, 4, 0, 0]} />
                                    <Bar dataKey="Failed" fill={COLORS.fail} radius={[4, 4, 0, 0]} />
                                    <Bar dataKey="Skipped" fill={COLORS.skip} radius={[4, 4, 0, 0]} />
                                </BarChart>
                            </ResponsiveContainer>
                        </div>
                    </div>

                    {/* Metrics table */}
                    <div style={{
                        flex: '1 1 360px',
                        minWidth: 300,
                        background: 'rgba(255,255,255,0.02)',
                        border: '1px solid var(--border-color)',
                        borderRadius: 'var(--radius-lg)',
                        padding: 20,
                    }}>
                        <h4 style={{ margin: '0 0 16px', color: 'var(--text-primary)', fontSize: '0.9rem', fontWeight: 600 }}>
                            Metric Comparison
                        </h4>
                        <table className="analytics-table" style={{ fontSize: '0.84rem' }}>
                            <thead>
                                <tr>
                                    <th>Metric</th>
                                    <th style={{ textAlign: 'center' }}>Baseline</th>
                                    <th style={{ textAlign: 'center' }}>Compare</th>
                                    <th style={{ textAlign: 'center' }}>Change</th>
                                </tr>
                            </thead>
                            <tbody>
                                {metrics.map(m => {
                                    const display = m.fmt || (v => v);
                                    return (
                                        <tr key={m.label}>
                                            <td style={{ fontWeight: 500 }}>{m.label}</td>
                                            <td style={{ textAlign: 'center', color: m.color || 'var(--text-primary)' }}>
                                                {display(m.v1)}
                                            </td>
                                            <td style={{ textAlign: 'center', color: m.color || 'var(--text-primary)' }}>
                                                {display(m.v2)}
                                            </td>
                                            <td style={{ textAlign: 'center' }}>
                                                <DiffIndicator v1={m.v1} v2={m.v2} invert={m.invert} fmt={m.fmt} />
                                            </td>
                                        </tr>
                                    );
                                })}
                            </tbody>
                        </table>
                    </div>
                </div>
            )}

            {!comparison && !loading && !error && (
                <div className="analytics-empty">
                    <div className="analytics-empty-icon">⚖️</div>
                    <div className="analytics-empty-text">Select two runs to compare</div>
                    <div className="analytics-empty-hint">Pick a baseline and a comparison run from the dropdowns above</div>
                </div>
            )}
        </div>
    );
}
