import React, { useMemo } from 'react';
import { formatDuration } from '../analytics/utils';

// Change column compares current (this run) against baseline (compared run):
// diff = current - baseline. `invert` marks metrics where lower is better.
function Delta({ baseline, current, invert = false, fmt }) {
    if (typeof baseline !== 'number' || typeof current !== 'number') {
        return <span style={{ color: 'var(--text-secondary)' }}>—</span>;
    }
    const diff = current - baseline;
    if (diff === 0) return <span style={{ color: 'var(--text-secondary)', fontSize: '0.8rem' }}>No change</span>;
    const good = invert ? diff < 0 : diff > 0;
    const color = good ? 'var(--accent-green)' : 'var(--accent-red)';
    const arrow = diff > 0 ? '↑' : '↓';
    const mag = fmt ? fmt(Math.abs(diff)) : Math.abs(diff);
    return <span style={{ color, fontWeight: 600, fontSize: '0.82rem' }}>{arrow} {mag}</span>;
}

function Bar({ passed, failed, skipped, total }) {
    const pct = (n) => (total ? (n / total) * 100 : 0);
    return (
        <div style={{ flex: 1, display: 'flex', height: 12, borderRadius: 6, overflow: 'hidden', background: 'rgba(255,255,255,0.06)' }}>
            <div style={{ width: `${pct(passed)}%`, background: 'var(--accent-green)' }} />
            <div style={{ width: `${pct(failed)}%`, background: 'var(--accent-red)' }} />
            <div style={{ width: `${pct(skipped)}%`, background: 'rgba(255,255,255,0.15)' }} />
        </div>
    );
}

function Chip({ label, value, color, testid }) {
    return (
        <span data-testid={testid} style={{ fontSize: '0.78rem', fontWeight: 600, padding: '4px 10px', borderRadius: 6, background: 'var(--bg-tertiary)', color: color || 'var(--text-secondary)' }}>
            {value} {label}
        </span>
    );
}

export default function RunCompareSummary({ summary }) {
    const a = summary.this;
    const b = summary.compared;
    const c = summary.counts;
    const metrics = useMemo(() => [
        { label: 'Total tests', v1: a.total, v2: b.total },
        { label: 'Passed', v1: a.passed, v2: b.passed },
        { label: 'Failed', v1: a.failed, v2: b.failed, invert: true },
        { label: 'Skipped', v1: a.skipped, v2: b.skipped, invert: true },
        { label: 'Pass rate', v1: a.passRate, v2: b.passRate, fmt: (v) => `${v.toFixed(1)}%` },
        { label: 'Duration', v1: a.durationMs, v2: b.durationMs, fmt: formatDuration, invert: true },
    ], [a, b]);

    return (
        <div data-testid="compare-summary" style={{ marginBottom: 16 }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8, marginBottom: 14 }}>
                {[{ s: a }, { s: b }].map(({ s }, i) => (
                    <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                        <span style={{ fontSize: '0.78rem', color: 'var(--text-secondary)', width: 140, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={s.name}>{s.name}</span>
                        <Bar passed={s.passed} failed={s.failed} skipped={s.skipped} total={s.total} />
                        <span style={{ fontSize: '0.78rem', fontWeight: 600, width: 54, textAlign: 'right' }}>{s.passRate.toFixed(0)}%</span>
                    </div>
                ))}
            </div>

            <table className="analytics-table" style={{ fontSize: '0.82rem', marginBottom: 14 }}>
                <thead>
                    <tr>
                        <th>Metric</th>
                        <th style={{ textAlign: 'center' }}>This run</th>
                        <th style={{ textAlign: 'center' }}>Compared</th>
                        <th style={{ textAlign: 'center' }}>Change</th>
                    </tr>
                </thead>
                <tbody>
                    {metrics.map((m) => {
                        const disp = m.fmt || ((v) => v);
                        return (
                            <tr key={m.label}>
                                <td style={{ fontWeight: 500 }}>{m.label}</td>
                                <td style={{ textAlign: 'center' }}>{disp(m.v1)}</td>
                                <td style={{ textAlign: 'center' }}>{disp(m.v2)}</td>
                                <td style={{ textAlign: 'center' }}><Delta baseline={m.v2} current={m.v1} invert={m.invert} fmt={m.fmt} /></td>
                            </tr>
                        );
                    })}
                </tbody>
            </table>

            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                <Chip label="regressions" value={c.regressions} color="var(--accent-red)" testid="compare-count-regressions" />
                <Chip label="fixed" value={c.fixed} color="var(--accent-green)" testid="compare-count-fixed" />
                <Chip label="shared" value={c.shared} testid="compare-count-shared" />
                <Chip label="only in this run" value={c.onlyThis} color="var(--accent-indigo)" testid="compare-count-onlyThis" />
                <Chip label="only in compared" value={c.onlyCompared} color="var(--accent-indigo)" testid="compare-count-onlyCompared" />
            </div>
        </div>
    );
}
