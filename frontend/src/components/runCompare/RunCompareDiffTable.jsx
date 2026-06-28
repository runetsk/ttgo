import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import StatusPill from './StatusPill';
import RunCompareRowDetail from './RunCompareRowDetail';

const GROUP_COLOR = {
    regressions: 'var(--accent-red)',
    fixed: 'var(--accent-green)',
    stillFailing: 'var(--text-secondary)',
    otherChanges: 'var(--text-secondary)',
    unchanged: 'var(--text-secondary)',
    onlyThis: 'var(--accent-indigo)',
    onlyCompared: 'var(--accent-indigo)',
};

function Banner({ tone, children }) {
    const c = tone === 'good' ? '34,197,94' : '99,102,241';
    return (
        <div style={{ padding: '10px 14px', marginBottom: 12, borderRadius: 8, fontSize: '0.82rem', background: `rgba(${c},0.08)`, border: `1px solid rgba(${c},0.25)`, color: 'var(--text-primary)' }}>
            {children}
        </div>
    );
}

export default function RunCompareDiffTable({ groups, summary, thisName, comparedName, analysesThis, analysesCompared, aiEnabled }) {
    const [expanded, setExpanded] = useState(() => new Set());
    const [collapsedGroups, setCollapsedGroups] = useState(() => new Set(['unchanged']));

    const visibleGroups = groups.filter((g) => g.rows.length > 0);
    const toggleRow = (id) => setExpanded((prev) => { const n = new Set(prev); n.has(id) ? n.delete(id) : n.add(id); return n; });
    const toggleGroup = (k) => setCollapsedGroups((prev) => { const n = new Set(prev); n.has(k) ? n.delete(k) : n.add(k); return n; });

    return (
        <div data-testid="compare-diff">
            {summary.counts.shared === 0 && (
                <Banner tone="info">These runs share no tests — only the run-specific sections below differ.</Banner>
            )}
            {summary.counts.shared > 0 && summary.counts.regressions === 0 && (
                <Banner tone="good">No regressions — every shared test held or improved.</Banner>
            )}

            <div style={{ border: '1px solid var(--border-color)', borderRadius: 8, overflow: 'hidden' }}>
                <div style={{ display: 'grid', gridTemplateColumns: '20px 1fr 90px 90px', padding: '7px 12px', fontSize: '0.72rem', color: 'var(--text-secondary)' }}>
                    <span />
                    <span>Test case</span>
                    <span style={{ textAlign: 'center' }}>This run</span>
                    <span style={{ textAlign: 'center' }}>Compared</span>
                </div>

                {visibleGroups.map((g) => {
                    const collapsed = collapsedGroups.has(g.key);
                    return (
                        <div key={g.key}>
                            <button
                                type="button"
                                data-testid={`compare-group-${g.key}`}
                                onClick={() => toggleGroup(g.key)}
                                style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', width: '100%', padding: '7px 12px', background: 'var(--bg-secondary)', border: 'none', borderTop: '1px solid var(--border-color)', cursor: 'pointer', textAlign: 'left' }}
                            >
                                <span style={{ fontSize: '0.74rem', fontWeight: 600, color: GROUP_COLOR[g.key] }}>
                                    {collapsed ? '▶' : '▼'} {g.label}
                                </span>
                                <span data-testid={`compare-group-${g.key}-count`} style={{ fontSize: '0.74rem', color: 'var(--text-secondary)' }}>{g.rows.length}</span>
                            </button>

                            {!collapsed && g.rows.map((row) => {
                                const id = row.testCaseId || row.name;
                                const expandable = !!(row.thisResult && row.comparedResult);
                                const isOpen = expanded.has(id);
                                return (
                                    <div key={id}>
                                        <div
                                            data-testid={`compare-row-${id}`}
                                            onClick={() => expandable && toggleRow(id)}
                                            style={{ display: 'grid', gridTemplateColumns: '20px 1fr 90px 90px', alignItems: 'center', padding: '7px 12px', borderTop: '1px solid var(--border-color)', cursor: expandable ? 'pointer' : 'default' }}
                                        >
                                            <span style={{ color: 'var(--text-secondary)', fontSize: '0.7rem' }}>{expandable ? (isOpen ? '▼' : '▶') : ''}</span>
                                            <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: '0.82rem' }} title={row.name}>
                                                {row.testCaseId
                                                    ? <Link to={`/library/tests/${row.testCaseId}`} onClick={(e) => e.stopPropagation()} className="result-test-link">{row.name}</Link>
                                                    : row.name}
                                            </span>
                                            <span style={{ textAlign: 'center' }}>{row.thisResult ? <StatusPill status={row.thisResult.status} /> : <span style={{ color: 'var(--text-secondary)' }}>—</span>}</span>
                                            <span style={{ textAlign: 'center' }}>{row.comparedResult ? <StatusPill status={row.comparedResult.status} /> : <span style={{ color: 'var(--text-secondary)' }}>—</span>}</span>
                                        </div>
                                        {expandable && isOpen && (
                                            <RunCompareRowDetail
                                                row={row}
                                                thisName={thisName}
                                                comparedName={comparedName}
                                                thisVerdict={analysesThis[row.thisResult.id]}
                                                comparedVerdict={analysesCompared[row.comparedResult.id]}
                                                aiEnabled={aiEnabled}
                                            />
                                        )}
                                    </div>
                                );
                            })}
                        </div>
                    );
                })}
            </div>
        </div>
    );
}
