import React from 'react';
import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer } from 'recharts';

const COLORS = {
    pass: '#22c55e',
    fail: '#ef4444',
    skip: '#9ca3af',
};

function getRateClass(pct) {
    if (pct >= 80) return 'pass-rate-high';
    if (pct >= 50) return 'pass-rate-medium';
    return 'pass-rate-low';
}

export default function SummaryCards({ data }) {
    if (!data) return null;

    const { total_runs = 0, pass_count = 0, fail_count = 0, skip_count = 0, pass_rate = 0 } = data;
    const passRatePct = Math.round(pass_rate * 100);

    // Empty state
    if (total_runs === 0) {
        return (
            <div className="analytics-empty">
                <div className="analytics-empty-icon">📊</div>
                <div className="analytics-empty-text">No test run data available</div>
                <div className="analytics-empty-hint">Run some tests to see analytics here</div>
            </div>
        );
    }

    const donutData = [
        { name: 'Passed', value: pass_count, color: COLORS.pass },
        { name: 'Failed', value: fail_count, color: COLORS.fail },
        { name: 'Skipped', value: skip_count, color: COLORS.skip },
    ].filter(d => d.value > 0);

    const CustomTooltip = ({ active, payload }) => {
        if (!active || !payload?.length) return null;
        const d = payload[0];
        const pct = total_runs > 0 ? ((d.value / total_runs) * 100).toFixed(1) : 0;
        return (
            <div style={{
                background: 'var(--bg-secondary)',
                border: '1px solid var(--border-color)',
                borderRadius: 6,
                padding: '8px 12px',
                fontSize: '0.82rem',
            }}>
                <div style={{ fontWeight: 600, color: d.payload.color }}>{d.name}</div>
                <div>{d.value} ({pct}%)</div>
            </div>
        );
    };

    return (
        <div className="analytics-summary-row">
            <div>
                <div className="analytics-stat-cards">
                    <div className="analytics-stat-card">
                        <div className="analytics-stat-value">{total_runs}</div>
                        <div className="analytics-stat-label">Total Runs</div>
                    </div>
                    <div className="analytics-stat-card">
                        <div className="analytics-stat-value" style={{ color: COLORS.pass }}>{pass_count}</div>
                        <div className="analytics-stat-label">Passed</div>
                    </div>
                    <div className="analytics-stat-card">
                        <div className="analytics-stat-value" style={{ color: COLORS.fail }}>{fail_count}</div>
                        <div className="analytics-stat-label">Failed</div>
                    </div>
                    <div className="analytics-stat-card">
                        <div className="analytics-stat-value" style={{ color: COLORS.skip }}>{skip_count}</div>
                        <div className="analytics-stat-label">Skipped</div>
                    </div>
                    <div className="analytics-stat-card">
                        <div className={`analytics-stat-value ${getRateClass(passRatePct)}`}>{passRatePct}%</div>
                        <div className="analytics-stat-label">Pass Rate</div>
                    </div>
                </div>
            </div>
            <div style={{ width: 200, height: 200 }}>
                <ResponsiveContainer width="100%" height="100%">
                    <PieChart>
                        <Pie
                            data={donutData}
                            cx="50%"
                            cy="50%"
                            innerRadius={55}
                            outerRadius={85}
                            dataKey="value"
                            stroke="none"
                        >
                            {donutData.map((entry, i) => (
                                <Cell key={i} fill={entry.color} />
                            ))}
                        </Pie>
                        <Tooltip content={<CustomTooltip />} />
                    </PieChart>
                </ResponsiveContainer>
            </div>
        </div>
    );
}
