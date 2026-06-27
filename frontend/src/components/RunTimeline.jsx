import React, { useState, useMemo, useRef, useEffect } from 'react';
import {
    ResponsiveContainer, BarChart, Bar, XAxis, YAxis,
    CartesianGrid, Tooltip, Cell,
} from 'recharts';

const STATUS_COLORS = {
    PASS: '#22c55e',
    FAIL: '#ef4444',
    ERROR: '#ef4444',
    SKIP: '#9ca3af',
    PENDING: '#f59e0b',
    RUNNING: '#6366f1',
};

const STATUS_ORDER = ['PASS', 'FAIL', 'ERROR', 'SKIP', 'PENDING', 'RUNNING'];

function formatTime(ts) {
    if (!ts || ts === '0001-01-01T00:00:00Z') return '—';
    return new Date(ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

function formatDuration(ms) {
    if (!ms) return '—';
    if (ms < 1000) return `${ms}ms`;
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
    return `${Math.floor(ms / 60000)}m ${((ms % 60000) / 1000).toFixed(0)}s`;
}

function TimelineTooltip({ active, payload }) {
    if (!active || !payload?.length) return null;
    const d = payload[0]?.payload;
    if (!d) return null;
    return (
        <div style={{
            background: 'var(--bg-secondary)', border: '1px solid var(--border-color)',
            borderRadius: 6, padding: '10px 14px', fontSize: '0.82rem', maxWidth: 320,
            boxShadow: '0 4px 12px rgba(0,0,0,0.3)',
        }}>
            <div style={{ fontWeight: 600, marginBottom: 6, wordBreak: 'break-word' }}>{d.name}</div>
            <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 6 }}>
                <span style={{
                    background: STATUS_COLORS[d.status] || '#9ca3af',
                    color: '#fff', padding: '1px 8px', borderRadius: 4,
                    fontSize: '0.72rem', fontWeight: 700,
                }}>{d.status}</span>
                <span style={{ fontSize: '0.78rem', color: 'var(--text-secondary)' }}>{formatDuration(d.durationMs)}</span>
            </div>
            <div style={{ fontSize: '0.78rem', color: 'var(--text-secondary)' }}>
                {formatTime(d.startTime)} → {formatTime(d.endTime)}
            </div>
            {d.attemptNumber > 1 && <div style={{ marginTop: 4, color: '#856404', fontSize: '0.78rem' }}>Attempt #{d.attemptNumber}</div>}
        </div>
    );
}

// Minimum visible bar width as a fraction of total span
const MIN_BAR_FRACTION = 0.006;

function TimeAxis({ totalSpan, formatTick }) {
    const tickCount = 6;
    const ticks = [];
    for (let i = 0; i <= tickCount; i++) {
        ticks.push((totalSpan / tickCount) * i);
    }
    return (
        <div style={{
            display: 'flex', borderTop: '1px solid var(--border-color)',
        }}>
            {/* Spacer matching YAxis width */}
            <div style={{ width: 200, flexShrink: 0 }} />
            {/* Tick labels area */}
            <div style={{
                flex: 1, display: 'flex', justifyContent: 'space-between',
                padding: '6px 24px 8px 0',
            }}>
                {ticks.map((val, i) => (
                    <span key={i} style={{
                        fontSize: 11, color: 'var(--text-secondary)', whiteSpace: 'nowrap',
                    }}>
                        {formatTick(val)}
                    </span>
                ))}
            </div>
        </div>
    );
}

export default function RunTimeline({ results, onNavigateToResult }) {
    const [hiddenStatuses, setHiddenStatuses] = useState({});
    const scrollRef = useRef(null);
    const [scrollHeight, setScrollHeight] = useState(400);

    // Filter to results with valid timing data, latest attempt per test case
    const allTimedResults = useMemo(() => {
        const byTestCase = {};
        const orphans = [];
        for (const rr of (results || [])) {
            if (!rr.start_time || rr.start_time === '0001-01-01T00:00:00Z') continue;
            if (!rr.end_time || rr.end_time === '0001-01-01T00:00:00Z') continue;
            if (!rr.test_case_id) { orphans.push(rr); continue; }
            if (!byTestCase[rr.test_case_id]) byTestCase[rr.test_case_id] = [];
            byTestCase[rr.test_case_id].push(rr);
        }
        const latest = [...orphans];
        for (const tcId in byTestCase) {
            byTestCase[tcId].sort((a, b) => b.attempt_number - a.attempt_number);
            latest.push(byTestCase[tcId][0]);
        }
        latest.sort((a, b) => new Date(a.start_time) - new Date(b.start_time));
        return latest;
    }, [results]);

    // Statuses present in data
    const presentStatuses = useMemo(() => {
        const s = new Set(allTimedResults.map(r => r.status));
        return STATUS_ORDER.filter(st => s.has(st));
    }, [allTimedResults]);

    // Filtered results based on hidden statuses
    const filteredResults = useMemo(() => {
        if (Object.keys(hiddenStatuses).length === 0) return allTimedResults;
        return allTimedResults.filter(r => !hiddenStatuses[r.status]);
    }, [allTimedResults, hiddenStatuses]);

    // Measure available space: from scroll div top to viewport bottom
    useEffect(() => {
        const measure = () => {
            if (scrollRef.current) {
                const rect = scrollRef.current.getBoundingClientRect();
                setScrollHeight(Math.max(200, window.innerHeight - rect.top - 82));
            }
        };
        const timer = setTimeout(measure, 0);
        window.addEventListener('resize', measure);
        return () => { clearTimeout(timer); window.removeEventListener('resize', measure); };
    }, [filteredResults.length]);

    if (allTimedResults.length === 0) {
        return (
            <div style={{ textAlign: 'center', padding: '60px 0', color: 'var(--text-secondary)' }}>
                <div style={{ fontSize: '2rem', marginBottom: 8 }}>&#9201;</div>
                <div style={{ fontSize: '0.85rem' }}>No timing data available</div>
                <div style={{ fontSize: '0.78rem', marginTop: 4, opacity: 0.6 }}>
                    Tests need start_time and end_time to appear on the timeline
                </div>
            </div>
        );
    }

    // Compute time range from ALL results (not filtered) so axis stays stable
    const minTime = new Date(allTimedResults[0].start_time).getTime();
    const maxTime = Math.max(...allTimedResults.map(r => new Date(r.end_time).getTime()));
    const totalSpan = maxTime - minTime || 1;
    const minBarDuration = totalSpan * MIN_BAR_FRACTION;

    // Build chart data
    const chartData = filteredResults.map(r => {
        const start = new Date(r.start_time).getTime();
        const end = new Date(r.end_time).getTime();
        const rawDuration = end - start;
        return {
            name: r.test_name_snapshot || 'Unknown',
            offset: start - minTime,
            duration: Math.max(rawDuration, minBarDuration),
            rawDuration,
            status: r.status,
            startTime: r.start_time,
            endTime: r.end_time,
            durationMs: r.duration_ms || rawDuration,
            attemptNumber: r.attempt_number,
            resultId: r.id,
        };
    });

    // Clock time axis formatter
    const formatTick = (value) => {
        const ts = new Date(minTime + value);
        return ts.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    };

    const toggleStatus = (status) => {
        setHiddenStatuses(prev => {
            const next = { ...prev };
            if (next[status]) delete next[status];
            else next[status] = true;
            return next;
        });
    };

    const handleBarClick = (data) => {
        if (data?.resultId && onNavigateToResult) {
            onNavigateToResult(data.resultId);
        }
    };

    const chartHeight = Math.max(200, chartData.length * 32);

    // Status summary counts
    const statusCounts = {};
    allTimedResults.forEach(r => { statusCounts[r.status] = (statusCounts[r.status] || 0) + 1; });

    return (
        <div style={{ width: '100%' }}>
            {/* Header bar */}
            <div style={{
                display: 'flex', alignItems: 'center', gap: 16, flexWrap: 'wrap',
                marginBottom: 6, fontSize: '0.75rem', color: 'var(--text-secondary)',
            }}>
                <span>{allTimedResults.length} test{allTimedResults.length !== 1 ? 's' : ''}</span>
                <span>Span: {formatDuration(totalSpan)}</span>

                {/* Status filter buttons */}
                <div style={{ display: 'flex', gap: 4, marginLeft: 'auto' }}>
                    {presentStatuses.map(status => {
                        const hidden = !!hiddenStatuses[status];
                        return (
                            <button
                                key={status}
                                onClick={() => toggleStatus(status)}
                                style={{
                                    display: 'flex', alignItems: 'center', gap: 5,
                                    padding: '3px 10px', borderRadius: 6, cursor: 'pointer',
                                    fontSize: '0.72rem', fontWeight: 600, border: 'none',
                                    background: hidden ? 'transparent' : `${STATUS_COLORS[status]}18`,
                                    color: hidden ? 'var(--text-secondary)' : STATUS_COLORS[status],
                                    opacity: hidden ? 0.45 : 1,
                                    transition: 'all 0.15s ease',
                                    outline: hidden ? '1px solid var(--border-color)' : `1px solid ${STATUS_COLORS[status]}40`,
                                }}
                            >
                                <span style={{
                                    width: 8, height: 8, borderRadius: 2,
                                    background: hidden ? 'var(--text-secondary)' : STATUS_COLORS[status],
                                    display: 'inline-block', opacity: hidden ? 0.4 : 1,
                                }} />
                                {status} ({statusCounts[status] || 0})
                            </button>
                        );
                    })}
                </div>
            </div>

            {filteredResults.length === 0 ? (
                <div style={{ textAlign: 'center', padding: '40px 0', color: 'var(--text-secondary)', fontSize: '0.85rem' }}>
                    All statuses are hidden. Click a status above to show tests.
                </div>
            ) : (
                <div style={{
                    border: '1px solid var(--border-color)', borderRadius: 8,
                    background: 'var(--bg-secondary)',
                }}>
                    <div ref={scrollRef} style={{
                        height: Math.min(chartHeight, scrollHeight),
                        overflowY: chartHeight > scrollHeight ? 'auto' : 'hidden',
                    }}>
                        <ResponsiveContainer width="100%" height={chartHeight}>
                            <BarChart
                                data={chartData}
                                layout="vertical"
                                margin={{ top: 0, right: 24, bottom: 0, left: 0 }}
                                barSize={22}
                                onClick={(state) => {
                                    if (state?.activePayload?.[0]?.payload) {
                                        handleBarClick(state.activePayload[0].payload);
                                    }
                                }}
                            >
                                <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.06)" horizontal={false} />
                                <XAxis type="number" domain={[0, totalSpan]} hide />
                                <YAxis
                                    type="category"
                                    dataKey="name"
                                    width={200}
                                    tick={{ fontSize: 11, fill: 'var(--text-secondary)' }}
                                    tickFormatter={(v) => v.length > 28 ? v.slice(0, 27) + '...' : v}
                                    stroke="var(--border-color)"
                                />
                                <Tooltip content={<TimelineTooltip />} cursor={{ fill: 'rgba(255,255,255,0.03)' }} />
                                <Bar dataKey="offset" stackId="timeline" fill="transparent" isAnimationActive={false} />
                                <Bar
                                    dataKey="duration"
                                    stackId="timeline"
                                    isAnimationActive={false}
                                    radius={[3, 3, 3, 3]}
                                    style={{ cursor: onNavigateToResult ? 'pointer' : 'default' }}
                                >
                                    {chartData.map((entry, index) => (
                                        <Cell key={index} fill={STATUS_COLORS[entry.status] || '#9ca3af'} />
                                    ))}
                                </Bar>
                            </BarChart>
                        </ResponsiveContainer>
                    </div>
                    {/* Fixed time axis — always visible below scroll */}
                    <TimeAxis totalSpan={totalSpan} formatTick={formatTick} />
                </div>
            )}
        </div>
    );
}
