import React, { useState } from 'react';
import {
    ResponsiveContainer, BarChart, Bar, XAxis, YAxis, Tooltip, Cell,
    PieChart, Pie,
} from 'recharts';

const COLORS = { pass: '#22c55e', fail: '#ef4444' };

function PRTooltip({ active, payload }) {
    if (!active || !payload?.length) return null;
    const d = payload[0]?.payload;
    return (
        <div style={{
            background: 'var(--bg-secondary)', border: '1px solid var(--border-color)',
            borderRadius: 6, padding: '10px 14px', fontSize: '0.82rem',
        }}>
            <div style={{ fontWeight: 600, marginBottom: 4 }}>{d?.folder_name}</div>
            <div style={{ color: COLORS.pass }}>Passed: {d?.passed_count}</div>
            <div style={{ color: COLORS.fail }}>Failed: {d?.failed_count}</div>
            <div>Rate: {Math.round(d?.passing_rate || 0)}%</div>
        </div>
    );
}

export default function PassingRatePerFolder({ data, onExcludeSkippedChange }) {
    const [chartType, setChartType] = useState('bar');
    const [excludeSkipped, setExcludeSkipped] = useState(false);

    const folders = data?.folders || [];

    const handleToggle = (checked) => {
        setExcludeSkipped(checked);
        if (onExcludeSkippedChange) onExcludeSkippedChange(checked);
    };

    if (folders.length === 0) {
        return (
            <div className="analytics-empty">
                <div className="analytics-empty-icon">📁</div>
                <div className="analytics-empty-text">No folder data available</div>
                <div className="analytics-empty-hint">Assign runs to folders to see per-folder rates</div>
            </div>
        );
    }

    return (
        <div>
            <div className="analytics-chart-header">
                <div className="analytics-chart-toggles">
                    <button className={`analytics-toggle-btn ${chartType === 'bar' ? 'active' : ''}`}
                        onClick={() => setChartType('bar')} type="button">Bar</button>
                    <button className={`analytics-toggle-btn ${chartType === 'donut' ? 'active' : ''}`}
                        onClick={() => setChartType('donut')} type="button">Donut</button>
                </div>
                <label className="analytics-switch">
                    <input type="checkbox" checked={excludeSkipped}
                        onChange={e => handleToggle(e.target.checked)} />
                    Exclude skipped
                </label>
            </div>

            {chartType === 'bar' ? (
                <div style={{ width: '100%', height: 300 }}>
                    <ResponsiveContainer>
                        <BarChart data={folders} margin={{ top: 5, right: 20, left: 0, bottom: 5 }}>
                            <XAxis dataKey="folder_name" tick={{ fontSize: 10, fill: 'var(--text-secondary)' }} />
                            <YAxis domain={[0, 100]} tick={{ fontSize: 11, fill: 'var(--text-secondary)' }} />
                            <Tooltip content={<PRTooltip />} />
                            <Bar dataKey="passing_rate" name="Pass Rate" radius={[4, 4, 0, 0]}>
                                {folders.map((f, i) => (
                                    <Cell key={i} fill={f.passing_rate >= 80 ? COLORS.pass : f.passing_rate >= 50 ? '#eab308' : COLORS.fail} />
                                ))}
                            </Bar>
                        </BarChart>
                    </ResponsiveContainer>
                </div>
            ) : (
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))', gap: 12 }}>
                    {folders.map(f => {
                        const donutData = [
                            { name: 'Passed', value: f.passed_count, fill: COLORS.pass },
                            { name: 'Failed', value: f.failed_count, fill: COLORS.fail },
                        ];
                        return (
                            <div key={f.folder_id} style={{ textAlign: 'center' }}>
                                <div style={{ width: 120, height: 120, margin: '0 auto' }}>
                                    <ResponsiveContainer>
                                        <PieChart>
                                            <Pie data={donutData} innerRadius={30} outerRadius={50}
                                                dataKey="value" stroke="none">
                                                {donutData.map((d, i) => (
                                                    <Cell key={i} fill={d.fill} />
                                                ))}
                                            </Pie>
                                        </PieChart>
                                    </ResponsiveContainer>
                                </div>
                                <div style={{ fontSize: '0.8rem', fontWeight: 600 }}>{f.folder_name}</div>
                                <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                                    {Math.round(f.passing_rate)}%
                                </div>
                            </div>
                        );
                    })}
                </div>
            )}
        </div>
    );
}
