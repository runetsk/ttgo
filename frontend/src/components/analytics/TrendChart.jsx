import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
    ResponsiveContainer, AreaChart, Area, BarChart, Bar,
    XAxis, YAxis, CartesianGrid, Tooltip, Legend, Line, ComposedChart,
} from 'recharts';

const COLORS = { pass: '#22c55e', fail: '#ef4444', skip: '#9ca3af' };

function CustomTooltip({ active, payload, label }) {
    if (!active || !payload?.length) return null;
    const d = payload[0]?.payload;
    if (!d) return null;
    const passRate = d.total_runs > 0 ? ((d.pass_count / d.total_runs) * 100).toFixed(1) : '0.0';
    return (
        <div style={{
            background: 'var(--bg-secondary)', border: '1px solid var(--border-color)',
            borderRadius: 6, padding: '10px 14px', fontSize: '0.82rem',
        }}>
            <div style={{ fontWeight: 600, marginBottom: 4 }}>{label}</div>
            <div style={{ color: COLORS.pass }}>Passed: {d.pass_count}</div>
            <div style={{ color: COLORS.fail }}>Failed: {d.fail_count}</div>
            <div style={{ color: COLORS.skip }}>Skipped: {d.skip_count}</div>
            <div style={{ marginTop: 4 }}>Total: {d.total_runs} | Rate: {passRate}%</div>
        </div>
    );
}

export default function TrendChart({ data, onTimeRangeChange }) {
    const navigate = useNavigate();
    const [chartType, setChartType] = useState('bar');
    const [hiddenSeries, setHiddenSeries] = useState({});

    const points = data?.points || [];

    if (points.length === 0) {
        return (
            <div className="analytics-empty">
                <div className="analytics-empty-icon">📈</div>
                <div className="analytics-empty-text">No trend data available</div>
                <div className="analytics-empty-hint">Run some tests to see trends here</div>
            </div>
        );
    }

    const toggleSeries = (dataKey) => {
        setHiddenSeries(prev => ({ ...prev, [dataKey]: !prev[dataKey] }));
    };

    const handleLegendClick = (e) => {
        if (e?.dataKey) toggleSeries(e.dataKey);
    };

    const handleChartClick = (chartData) => {
        if (chartData?.activePayload?.[0]?.payload?.date) {
            const date = chartData.activePayload[0].payload.date;
            navigate(`/runs?date=${encodeURIComponent(date)}`);
        }
    };

    const opacity = (key) => hiddenSeries[key] ? 0 : 1;

    const timeRanges = [7, 14, 30, 60, 90];

    const renderChart = () => {
        const commonProps = {
            data: points,
            margin: { top: 5, right: 20, left: 0, bottom: 5 },
        };

        if (chartType === 'area') {
            return (
                <ComposedChart {...commonProps} onClick={handleChartClick} style={{ cursor: 'pointer' }}>
                    <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.06)" />
                    <XAxis dataKey="date" tick={{ fontSize: 11, fill: 'var(--text-secondary)' }} />
                    <YAxis tick={{ fontSize: 11, fill: 'var(--text-secondary)' }} />
                    <Tooltip content={<CustomTooltip />} />
                    <Legend onClick={handleLegendClick} wrapperStyle={{ cursor: 'pointer' }} />
                    <Area type="monotone" dataKey="pass_count" name="Passed" stackId="1"
                        fill={COLORS.pass} stroke={COLORS.pass} fillOpacity={opacity('pass_count') * 0.6} strokeOpacity={opacity('pass_count')} />
                    <Area type="monotone" dataKey="fail_count" name="Failed" stackId="1"
                        fill={COLORS.fail} stroke={COLORS.fail} fillOpacity={opacity('fail_count') * 0.6} strokeOpacity={opacity('fail_count')} />
                    <Area type="monotone" dataKey="skip_count" name="Skipped" stackId="1"
                        fill={COLORS.skip} stroke={COLORS.skip} fillOpacity={opacity('skip_count') * 0.4} strokeOpacity={opacity('skip_count')} />
                    <Line type="monotone" dataKey="fail_count" name="Failed Trend" stroke="#fbbf24"
                        strokeWidth={2} dot={false} strokeDasharray="5 5" opacity={opacity('fail_count')} />
                </ComposedChart>
            );
        }
        return (
            <BarChart {...commonProps} onClick={handleChartClick} style={{ cursor: 'pointer' }}>
                <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.06)" />
                <XAxis dataKey="date" tick={{ fontSize: 11, fill: 'var(--text-secondary)' }} />
                <YAxis tick={{ fontSize: 11, fill: 'var(--text-secondary)' }} />
                <Tooltip content={<CustomTooltip />} />
                <Legend onClick={handleLegendClick} wrapperStyle={{ cursor: 'pointer' }} />
                <Bar dataKey="pass_count" name="Passed" fill={COLORS.pass} opacity={opacity('pass_count')} />
                <Bar dataKey="fail_count" name="Failed" fill={COLORS.fail} opacity={opacity('fail_count')} />
                <Bar dataKey="skip_count" name="Skipped" fill={COLORS.skip} opacity={opacity('skip_count')} />
            </BarChart>
        );
    };

    return (
        <div>
            <div className="analytics-chart-header">
                <div className="analytics-chart-toggles">
                    {timeRanges.map(d => (
                        <button key={d} className="analytics-toggle-btn" onClick={() => onTimeRangeChange?.(d)} type="button">
                            {d}d
                        </button>
                    ))}
                </div>
                <div className="analytics-chart-toggles">
                    <button className={`analytics-toggle-btn ${chartType === 'area' ? 'active' : ''}`}
                        onClick={() => setChartType('area')} type="button">Area</button>
                    <button className={`analytics-toggle-btn ${chartType === 'bar' ? 'active' : ''}`}
                        onClick={() => setChartType('bar')} type="button">Bar</button>
                </div>
            </div>
            <div style={{ width: '100%', height: 350 }}>
                <ResponsiveContainer>{renderChart()}</ResponsiveContainer>
            </div>
        </div>
    );
}
