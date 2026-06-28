import React from 'react';
import StatusPill from './StatusPill';
import AIVerdictBadge from '../AIVerdictBadge';
import { formatDuration } from '../analytics/utils';

const DEFECT_LABELS = {
    product_bug: 'Product bug',
    automation_bug: 'Automation bug',
    system_issue: 'System issue',
    to_investigate: 'To investigate',
};

function KV({ k, children }) {
    return (
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 10, padding: '4px 0', fontSize: '0.78rem', borderTop: '1px solid var(--border-color)' }}>
            <span style={{ color: 'var(--text-secondary)' }}>{k}</span>
            <span style={{ textAlign: 'right', minWidth: 0 }}>{children}</span>
        </div>
    );
}

function Pane({ title, result, verdict, aiEnabled }) {
    return (
        <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: '0.72rem', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: 6, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={title}>{title}</div>
            <KV k="Status"><StatusPill status={result && result.status} /></KV>
            <KV k="Duration">{result ? formatDuration(result.duration_ms) : '—'}</KV>
            <KV k="Defect type">{result && result.defect_type ? (DEFECT_LABELS[result.defect_type] || result.defect_type) : '—'}</KV>
            {aiEnabled && (
                <KV k="AI verdict">{verdict ? <AIVerdictBadge verdict={verdict.verdict} confidence={verdict.confidence} dedupGroup={!!verdict.dedup_group_key} /> : '—'}</KV>
            )}
            <KV k="Error">{result && result.error_message
                ? <code style={{ fontSize: '0.72rem', color: 'var(--accent-red)', whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>{result.error_message}</code>
                : '—'}</KV>
        </div>
    );
}

export default function RunCompareRowDetail({ row, thisName, comparedName, thisVerdict, comparedVerdict, aiEnabled }) {
    return (
        <div
            data-testid={`compare-detail-${row.testCaseId || row.name}`}
            style={{ display: 'flex', gap: 16, padding: '12px 14px', background: 'var(--bg-secondary)', borderTop: '1px solid var(--border-color)' }}
        >
            <Pane title={`${thisName} · this run`} result={row.thisResult} verdict={thisVerdict} aiEnabled={aiEnabled} />
            <div style={{ width: 1, background: 'var(--border-color)' }} />
            <Pane title={`${comparedName} · compared`} result={row.comparedResult} verdict={comparedVerdict} aiEnabled={aiEnabled} />
        </div>
    );
}
