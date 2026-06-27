import React from 'react';
import { ResponsiveContainer, BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip } from 'recharts';
import { formatDuration } from './utils';

function DurationTooltip({ active, payload, label }) {
    if (!active || !payload?.length) return null;
    const d = payload[0]?.payload;
    return (
        <div style={{
            background: 'var(--bg-secondary)', border: '1px solid var(--border-color)',
            borderRadius: 6, padding: '10px 14px', fontSize: '0.82rem',
        }}>
            <div style={{ fontWeight: 600, marginBottom: 4 }}>{label}</div>
            <div>Total: {formatDuration(d?.total_duration_ms)}</div>
            <div>Average: {formatDuration(d?.avg_duration_ms)}</div>
            <div>Runs: {d?.run_count}</div>
        </div>
    );
}

export default function DurationChart({ data }) {
    const points = data?.points || [];
    if (points.length === 0) {
        return (
            <div className="analytics-empty">
                <div className="analytics-empty-icon">⏱️</div>
                <div className="analytics-empty-text">No duration data available</div>
            </div>
        );
    }
    return (
        <div className="analytics-chart-container">
            <h4 style={{ margin: '0 0 12px', color: 'var(--text-primary)' }}>Run Duration per Day</h4>
            <div style={{ width: '100%', height: 280 }}>
                <ResponsiveContainer>
                    <BarChart data={points} margin={{ top: 5, right: 20, left: 0, bottom: 5 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.06)" />
                        <XAxis dataKey="date" tick={{ fontSize: 11, fill: 'var(--text-secondary)' }} />
                        <YAxis tick={{ fontSize: 11, fill: 'var(--text-secondary)' }}
                            tickFormatter={v => formatDuration(v)} />
                        <Tooltip content={<DurationTooltip />} />
                        <Bar dataKey="total_duration_ms" name="Duration" fill="#6366f1" radius={[4, 4, 0, 0]} />
                    </BarChart>
                </ResponsiveContainer>
            </div>
        </div>
    );
}
