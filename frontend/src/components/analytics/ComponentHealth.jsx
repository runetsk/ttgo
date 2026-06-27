import React, { useState, useMemo } from 'react';

function getHealthColor(rate) {
    if (rate >= 90) return '#22c55e';
    if (rate >= 70) return '#84cc16';
    if (rate >= 50) return '#eab308';
    if (rate >= 30) return '#ef4444';
    return '#991b1b';
}

export default function ComponentHealth({ data, threshold: initialThreshold, onThresholdChange }) {
    const [threshold, setThreshold] = useState(initialThreshold || 80);
    const [sortKey, setSortKey] = useState('passing_rate');
    const [sortAsc, setSortAsc] = useState(true);

    const components = data?.components || [];
    const totals = data?.totals;

    const sorted = useMemo(() => {
        return [...components].sort((a, b) => {
            const av = a[sortKey] ?? 0;
            const bv = b[sortKey] ?? 0;
            return sortAsc ? av - bv : bv - av;
        });
    }, [components, sortKey, sortAsc]);

    const passing = sorted.filter(c => c.passing_rate >= threshold);
    const failing = sorted.filter(c => c.passing_rate < threshold);

    if (components.length === 0) {
        return (
            <div className="analytics-empty">
                <div className="analytics-empty-icon">🏥</div>
                <div className="analytics-empty-text">No component health data</div>
                <div className="analytics-empty-hint">Run tests in folders to see health</div>
            </div>
        );
    }

    const handleSort = (key) => {
        if (sortKey === key) setSortAsc(!sortAsc);
        else { setSortKey(key); setSortAsc(true); }
    };

    const handleThreshold = (val) => {
        const v = Math.max(50, Math.min(100, Number(val)));
        setThreshold(v);
        if (onThresholdChange) onThresholdChange(v);
    };

    const sortIcon = (key) => sortKey === key ? (sortAsc ? ' ↑' : ' ↓') : '';

    return (
        <div>
            {/* Threshold control */}
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 16 }}>
                <span style={{ fontSize: '0.82rem', color: 'var(--text-secondary)' }}>Pass threshold:</span>
                <input type="number" min={50} max={100}
                    className="modern-input" style={{ width: 70, padding: '4px 8px', fontSize: '0.82rem' }}
                    value={threshold} onChange={e => handleThreshold(e.target.value)} />
                <span style={{ fontSize: '0.82rem', color: 'var(--text-secondary)' }}>%</span>
            </div>

            {/* Health Cards */}
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, marginBottom: 20 }}>
                <div>
                    <h5 style={{ margin: '0 0 8px', color: 'var(--accent-green)', fontSize: '0.82rem' }}>
                        Passing ({passing.length})
                    </h5>
                    <div className="analytics-health-cards">
                        {passing.map(c => (
                            <div key={c.folder_id || 'ungrouped'} className="analytics-health-card"
                                style={{ borderColor: getHealthColor(c.passing_rate) }}>
                                <div className="analytics-health-card-name">{c.folder_name}</div>
                                <div className="analytics-health-card-rate" style={{ color: getHealthColor(c.passing_rate) }}>
                                    {Math.round(c.passing_rate)}%
                                </div>
                                <div className="analytics-health-card-count">{c.total_tests} tests</div>
                            </div>
                        ))}
                    </div>
                </div>
                <div>
                    <h5 style={{ margin: '0 0 8px', color: 'var(--accent-red)', fontSize: '0.82rem' }}>
                        Failing ({failing.length})
                    </h5>
                    <div className="analytics-health-cards">
                        {failing.map(c => (
                            <div key={c.folder_id || 'ungrouped'} className="analytics-health-card"
                                style={{ borderColor: getHealthColor(c.passing_rate) }}>
                                <div className="analytics-health-card-name">{c.folder_name}</div>
                                <div className="analytics-health-card-rate" style={{ color: getHealthColor(c.passing_rate) }}>
                                    {Math.round(c.passing_rate)}%
                                </div>
                                <div className="analytics-health-card-count">{c.total_tests} tests</div>
                            </div>
                        ))}
                    </div>
                </div>
            </div>

            {/* Health Table */}
            <table className="analytics-table">
                <thead>
                    <tr>
                        <th className="analytics-sortable-th" onClick={() => handleSort('folder_name')}>
                            Folder{sortIcon('folder_name')}
                        </th>
                        <th className="analytics-sortable-th" onClick={() => handleSort('passing_rate')}>
                            Pass Rate{sortIcon('passing_rate')}
                        </th>
                        <th className="analytics-sortable-th" onClick={() => handleSort('total_tests')}>
                            Total{sortIcon('total_tests')}
                        </th>
                        <th>Passed</th>
                        <th>Failed</th>
                        <th>Skipped</th>
                    </tr>
                </thead>
                <tbody>
                    {sorted.map(c => (
                        <tr key={c.folder_id || 'ungrouped'}>
                            <td>{c.folder_name}</td>
                            <td style={{ color: getHealthColor(c.passing_rate), fontWeight: 600 }}>
                                {Math.round(c.passing_rate)}%
                            </td>
                            <td>{c.total_tests}</td>
                            <td style={{ color: 'var(--accent-green)' }}>{c.passed_count}</td>
                            <td style={{ color: 'var(--accent-red)' }}>{c.failed_count}</td>
                            <td style={{ color: 'var(--text-secondary)' }}>{c.skipped_count}</td>
                        </tr>
                    ))}
                    {totals && (
                        <tr style={{ fontWeight: 700, borderTop: '2px solid var(--border-color)' }}>
                            <td>Totals</td>
                            <td style={{ color: getHealthColor(totals.passing_rate) }}>
                                {Math.round(totals.passing_rate)}%
                            </td>
                            <td>{totals.total_tests}</td>
                            <td style={{ color: 'var(--accent-green)' }}>{totals.passed_count}</td>
                            <td style={{ color: 'var(--accent-red)' }}>{totals.failed_count}</td>
                            <td style={{ color: 'var(--text-secondary)' }}>{totals.skipped_count}</td>
                        </tr>
                    )}
                </tbody>
            </table>
        </div>
    );
}
