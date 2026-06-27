import React, { useState } from 'react';

function getRateColor(pct) {
    if (pct >= 80) return '#22c55e';
    if (pct >= 50) return '#eab308';
    return '#ef4444';
}

export default function PassingRateSummary({ data }) {
    const [includeSkipped, setIncludeSkipped] = useState(true);

    if (!data) return null;
    const { pass_count = 0, fail_count = 0, skip_count = 0 } = data;

    const denominator = includeSkipped
        ? pass_count + fail_count + skip_count
        : pass_count + fail_count;

    const rate = denominator > 0 ? (pass_count / denominator) * 100 : 0;
    const ratePct = Math.round(rate);
    const color = getRateColor(ratePct);

    return (
        <div className="analytics-chart-container">
            <div className="analytics-chart-header">
                <h4 style={{ margin: 0, color: 'var(--text-primary)' }}>Passing Rate</h4>
                <label className="analytics-switch">
                    <input
                        type="checkbox"
                        checked={includeSkipped}
                        onChange={e => setIncludeSkipped(e.target.checked)}
                    />
                    Include skipped
                </label>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                <div className="analytics-rate-bar" style={{ flex: 1 }}>
                    <div
                        className="analytics-rate-bar-fill"
                        style={{
                            width: `${ratePct}%`,
                            background: color,
                        }}
                    />
                    <div className="analytics-rate-bar-label">{ratePct}%</div>
                </div>
                <div style={{
                    fontSize: '1.5em',
                    fontWeight: 700,
                    color,
                    minWidth: 60,
                    textAlign: 'right',
                }}>
                    {ratePct}%
                </div>
            </div>
            <div style={{ marginTop: 8, fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
                {pass_count} passed / {denominator} total
                {!includeSkipped && skip_count > 0 && ` (${skip_count} skipped excluded)`}
            </div>
        </div>
    );
}
