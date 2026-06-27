import React from 'react';
import { ResponsiveContainer, AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip } from 'recharts';

function GrowthTooltip({ active, payload, label }) {
    if (!active || !payload?.length) return null;
    const d = payload[0]?.payload;
    return (
        <div style={{
            background: 'var(--bg-secondary)', border: '1px solid var(--border-color)',
            borderRadius: 6, padding: '10px 14px', fontSize: '0.82rem',
        }}>
            <div style={{ fontWeight: 600, marginBottom: 4 }}>{label}</div>
            <div>New: +{d?.delta || 0}</div>
            <div>Total: {d?.total_count || 0}</div>
        </div>
    );
}

export default function GrowthChart({ data }) {
    const points = data?.points || [];
    if (points.length === 0) {
        return (
            <div className="analytics-empty">
                <div className="analytics-empty-icon">🌱</div>
                <div className="analytics-empty-text">No test case growth data</div>
                <div className="analytics-empty-hint">Create test cases to see growth trends</div>
            </div>
        );
    }
    return (
        <div style={{ width: '100%', height: 300 }}>
            <ResponsiveContainer>
                <AreaChart data={points} margin={{ top: 5, right: 20, left: 0, bottom: 5 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.06)" />
                    <XAxis dataKey="date" tick={{ fontSize: 11, fill: 'var(--text-secondary)' }} />
                    <YAxis tick={{ fontSize: 11, fill: 'var(--text-secondary)' }} />
                    <Tooltip content={<GrowthTooltip />} />
                    <Area type="monotone" dataKey="delta" name="New Tests"
                        fill="rgba(99, 102, 241, 0.2)" stroke="#6366f1" strokeWidth={2} />
                </AreaChart>
            </ResponsiveContainer>
        </div>
    );
}
