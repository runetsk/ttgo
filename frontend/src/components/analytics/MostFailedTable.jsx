import React from 'react';
import { Link } from 'react-router-dom';
import { relativeTime } from './utils';

function getRateBadge(rate) {
    const pct = Math.round(rate * 100);
    const cls = pct >= 80 ? 'low' : pct >= 50 ? 'medium' : 'high'; // inverted: high failure = bad
    return <span className={`analytics-rate-badge ${cls}`}>{pct}%</span>;
}

export default function MostFailedTable({ data }) {
    const items = data?.test_cases || [];
    if (items.length === 0) {
        return (
            <div className="analytics-empty">
                <div className="analytics-empty-icon">✅</div>
                <div className="analytics-empty-text">No failed tests in this period</div>
            </div>
        );
    }
    return (
        <table className="analytics-table">
            <thead>
                <tr>
                    <th>Test Name</th>
                    <th>Failed</th>
                    <th>Total</th>
                    <th>Failure Rate</th>
                    <th>Last Failure</th>
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
                        <td style={{ color: 'var(--accent-red)', fontWeight: 600 }}>{tc.failed_count}</td>
                        <td>{tc.total_runs}</td>
                        <td>{getRateBadge(tc.failure_rate)}</td>
                        <td title={tc.last_failure_at}>{relativeTime(tc.last_failure_at)}</td>
                    </tr>
                ))}
            </tbody>
        </table>
    );
}
