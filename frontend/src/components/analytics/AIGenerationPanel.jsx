import React from 'react';
import {
    ResponsiveContainer, BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Cell,
} from 'recharts';

const OUTCOME_COLORS = {
    'Accepted as-is': '#34d399', 'Accepted edited': '#a7f3d0', Rejected: '#f87171',
    Superseded: '#a78bfa', Pending: '#94a3b8',
};

// AI generation outcome / cost panel (stage 6 learning & cost analytics).
export default function AIGenerationPanel({ report }) {
    if (!report) return null;
    const { runs, drafts, rejection_reasons: reasons, providers } = report;
    const accepted = drafts.accepted_unchanged + drafts.accepted_edited;
    const decided = accepted + drafts.rejected;
    const acceptanceRate = decided ? Math.round((accepted / decided) * 100) : null;

    const outcomeData = [
        { name: 'Accepted as-is', value: drafts.accepted_unchanged },
        { name: 'Accepted edited', value: drafts.accepted_edited },
        { name: 'Rejected', value: drafts.rejected },
        { name: 'Superseded', value: drafts.superseded },
        { name: 'Pending', value: drafts.pending },
    ].filter(d => d.value > 0);

    const stat = (label, value, hint) => (
        <div className="analytics-stat-card" style={{
            background: 'var(--bg-secondary)', border: '1px solid var(--border-color)',
            borderRadius: 10, padding: '10px 14px', minWidth: 130,
        }} title={hint}>
            <div style={{ fontSize: 11, color: 'var(--text-secondary)' }}>{label}</div>
            <div style={{ fontSize: 20, fontWeight: 700 }}>{value}</div>
        </div>
    );

    return (
        <div data-testid="ai-generation-analytics">
            <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', marginBottom: 16 }}>
                {stat('Runs', runs.total, `${runs.completed} completed / ${runs.failed} failed / ${runs.cancelled} cancelled`)}
                {stat('Acceptance', acceptanceRate == null ? '—' : `${acceptanceRate}%`, 'accepted / (accepted + rejected)')}
                {stat('Tokens', runs.total_tokens.toLocaleString())}
                {stat('Configured cost', `$${runs.total_cost_usd.toFixed(2)}`, 'sum of estimated_cost (needs provider pricing)')}
                {stat('p95 duration', `${(runs.p95_duration_ms / 1000).toFixed(1)}s`, `avg ${(runs.avg_duration_ms / 1000).toFixed(1)}s`)}
                {stat('Parse failures', runs.parse_failures, `${runs.retried_runs} runs retried`)}
            </div>

            {outcomeData.length > 0 && (
                <div style={{ width: '100%', height: 220 }}>
                    <ResponsiveContainer>
                        <BarChart data={outcomeData} margin={{ top: 5, right: 20, left: 0, bottom: 5 }}>
                            <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.06)" />
                            <XAxis dataKey="name" tick={{ fontSize: 11, fill: 'var(--text-secondary)' }} />
                            <YAxis allowDecimals={false} tick={{ fontSize: 11, fill: 'var(--text-secondary)' }} />
                            <Tooltip contentStyle={{ background: 'var(--bg-secondary)', border: '1px solid var(--border-color)' }} />
                            <Bar dataKey="value" name="Drafts">
                                {outcomeData.map(d => <Cell key={d.name} fill={OUTCOME_COLORS[d.name]} />)}
                            </Bar>
                        </BarChart>
                    </ResponsiveContainer>
                </div>
            )}

            {Object.keys(reasons || {}).length > 0 && (
                <div style={{ marginTop: 14, fontSize: 12.5, color: 'var(--text-secondary)' }}>
                    <b style={{ color: 'var(--text-primary)' }}>Rejection reasons:</b>{' '}
                    {Object.entries(reasons).sort((a, b) => b[1] - a[1])
                        .map(([k, v]) => `${k.replaceAll('_', ' ')} × ${v}`).join(' · ')}
                </div>
            )}

            {(providers || []).length > 0 && (
                <div style={{ overflowX: 'auto', marginTop: 14 }}>
                    <table className="analytics-table" style={{ width: '100%', fontSize: 12, borderCollapse: 'collapse' }}>
                        <thead>
                            <tr style={{ textAlign: 'left', color: 'var(--text-secondary)' }}>
                                <th style={{ padding: '4px 8px' }}>Provider</th>
                                <th style={{ padding: '4px 8px' }}>Model</th>
                                <th style={{ padding: '4px 8px' }}>Runs</th>
                                <th style={{ padding: '4px 8px' }}>Parse fails</th>
                                <th style={{ padding: '4px 8px' }}>Tokens</th>
                                <th style={{ padding: '4px 8px' }}>Cost</th>
                                <th style={{ padding: '4px 8px' }}>Avg dur</th>
                            </tr>
                        </thead>
                        <tbody>
                            {providers.map(p => (
                                <tr key={`${p.provider_label}:${p.model_name}`} style={{ borderTop: '1px solid var(--border-color)' }}>
                                    <td style={{ padding: '4px 8px' }}>{p.provider_label || '—'}</td>
                                    <td style={{ padding: '4px 8px' }}>{p.model_name}</td>
                                    <td style={{ padding: '4px 8px' }}>{p.runs}</td>
                                    <td style={{ padding: '4px 8px' }}>{p.parse_failures}</td>
                                    <td style={{ padding: '4px 8px' }}>{p.total_tokens.toLocaleString()}</td>
                                    <td style={{ padding: '4px 8px' }}>${p.cost_usd.toFixed(2)}</td>
                                    <td style={{ padding: '4px 8px' }}>{(p.avg_duration_ms / 1000).toFixed(1)}s</td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}
        </div>
    );
}
