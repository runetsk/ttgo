import React from 'react';
import { Link } from 'react-router-dom';
import { relativeTime } from './utils';

export default function FlakyTestsTable({ data }) {
    const items = data?.flaky_tests || [];
    if (items.length === 0) {
        return (
            <div className="analytics-empty">
                <div className="analytics-empty-icon">🎉</div>
                <div className="analytics-empty-text">No flaky tests detected</div>
                <div className="analytics-empty-hint">Test results are consistent</div>
            </div>
        );
    }
    return (
        <table className="analytics-table">
            <thead>
                <tr>
                    <th>Test Name</th>
                    <th>Switches</th>
                    <th>Switch %</th>
                    <th>Runs</th>
                    <th>Last Switch</th>
                </tr>
            </thead>
            <tbody>
                {items.map(tc => (
                    <tr key={tc.test_case_id} className={tc.current_status === 'PASS' ? 'flaky-pass' : 'flaky-fail'}>
                        <td>
                            <Link to={`/test-cases/${tc.test_case_id}`}>
                                {tc.test_case_name || tc.test_case_id?.slice(0, 8)}
                            </Link>
                        </td>
                        <td>{tc.switch_count} / {tc.possible_switches}</td>
                        <td style={{ fontWeight: 600, color: tc.switch_percentage >= 50 ? 'var(--accent-red)' : 'var(--warning-color, #eab308)' }}>
                            {Math.round(tc.switch_percentage)}%
                        </td>
                        <td>{tc.total_runs}</td>
                        <td title={tc.last_switch_at}>{relativeTime(tc.last_switch_at)}</td>
                    </tr>
                ))}
            </tbody>
        </table>
    );
}
