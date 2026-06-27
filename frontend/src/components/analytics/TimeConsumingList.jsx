import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import { ResponsiveContainer, BarChart, Bar, XAxis, YAxis, Tooltip, Cell } from 'recharts';
import { formatDuration, relativeTime } from './utils';

const STATUS_COLORS = { PASS: '#22c55e', FAIL: '#ef4444', ERROR: '#ef4444', SKIP: '#9ca3af' };

export default function TimeConsumingList({ data }) {
    const [viewMode, setViewMode] = useState('table');
    const items = data?.test_cases || [];

    if (items.length === 0) {
        return (
            <div className="analytics-empty">
                <div className="analytics-empty-icon">⚡</div>
                <div className="analytics-empty-text">No time-consuming test data</div>
            </div>
        );
    }

    return (
        <div className="analytics-chart-container">
            <div className="analytics-chart-header">
                <h4 style={{ margin: 0, color: 'var(--text-primary)' }}>Slowest Test Cases</h4>
                <div className="analytics-chart-toggles">
                    <button className={`analytics-toggle-btn ${viewMode === 'table' ? 'active' : ''}`}
                        onClick={() => setViewMode('table')} type="button">Table</button>
                    <button className={`analytics-toggle-btn ${viewMode === 'bar' ? 'active' : ''}`}
                        onClick={() => setViewMode('bar')} type="button">Bar</button>
                </div>
            </div>

            {viewMode === 'bar' ? (
                <div style={{ width: '100%', height: Math.max(items.length * 30, 200) }}>
                    <ResponsiveContainer>
                        <BarChart data={items} layout="vertical" margin={{ top: 5, right: 20, left: 120, bottom: 5 }}>
                            <XAxis type="number" tickFormatter={v => formatDuration(v)}
                                tick={{ fontSize: 11, fill: 'var(--text-secondary)' }} />
                            <YAxis type="category" dataKey="test_case_name" width={110}
                                tick={{ fontSize: 10, fill: 'var(--text-secondary)' }} />
                            <Tooltip formatter={v => formatDuration(v)} />
                            <Bar dataKey="duration_ms" name="Duration">
                                {items.map((tc, i) => (
                                    <Cell key={i} fill={STATUS_COLORS[tc.status] || '#6366f1'} />
                                ))}
                            </Bar>
                        </BarChart>
                    </ResponsiveContainer>
                </div>
            ) : (
                <table className="analytics-table">
                    <thead>
                        <tr>
                            <th>Test Name</th>
                            <th>Status</th>
                            <th>Duration</th>
                            <th>Start Time</th>
                        </tr>
                    </thead>
                    <tbody>
                        {items.map(tc => (
                            <tr key={tc.test_case_id}>
                                <td>
                                    <Link to={`/test-cases/${tc.test_case_id}`}>
                                        {tc.test_case_name || tc.test_case_id?.slice(0, 8)}
                                    </Link>
                                </td>
                                <td>
                                    <span className={`status-badge ${tc.status?.toLowerCase()}`}>
                                        {tc.status}
                                    </span>
                                </td>
                                <td>{formatDuration(tc.duration_ms)}</td>
                                <td title={tc.start_time}>{relativeTime(tc.start_time)}</td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            )}
        </div>
    );
}
