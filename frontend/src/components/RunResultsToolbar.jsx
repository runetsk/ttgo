import React from 'react';
import { GROUP_DIMENSIONS } from '../utils/runResultsGrouping';

const BTN_BASE = {
    padding: '5px 12px', fontSize: 12, border: 'none', cursor: 'pointer',
    fontWeight: 500, transition: 'background 0.1s ease',
};

export default function RunResultsToolbar({
    view, groupBy, onViewChange, onGroupByChange,
    onCollapseAll, onExpandAll, resultCount, groupCount,
    dimensions = GROUP_DIMENSIONS,
}) {
    const summary = view === 'grouped'
        ? `${resultCount} result${resultCount === 1 ? '' : 's'} · ${groupCount} group${groupCount === 1 ? '' : 's'}`
        : `${resultCount} result${resultCount === 1 ? '' : 's'}`;

    return (
        <div
            data-testid="run-results-toolbar"
            style={{
                display: 'flex', alignItems: 'center', gap: 12,
                padding: '8px 0', fontSize: 13, flexWrap: 'wrap',
            }}
        >
            <div style={{
                display: 'inline-flex', border: '1px solid var(--border-color)',
                borderRadius: 6, overflow: 'hidden', background: 'var(--bg-primary)',
            }}>
                <button
                    type="button"
                    onClick={() => onViewChange('list')}
                    data-testid="view-toggle-list"
                    style={{
                        ...BTN_BASE,
                        background: view === 'list' ? 'var(--accent-indigo)' : 'var(--bg-primary)',
                        color:      view === 'list' ? '#fff' : 'var(--text-primary)',
                    }}
                >☰ List</button>
                <button
                    type="button"
                    onClick={() => onViewChange('grouped')}
                    data-testid="view-toggle-grouped"
                    style={{
                        ...BTN_BASE,
                        background: view === 'grouped' ? 'var(--accent-indigo)' : 'var(--bg-primary)',
                        color:      view === 'grouped' ? '#fff' : 'var(--text-primary)',
                    }}
                >⊟ Grouped</button>
            </div>

            {view === 'grouped' && (
                <>
                    <div style={{ width: 1, height: 18, background: 'var(--border-color)' }} />
                    <label style={{ display: 'inline-flex', alignItems: 'center', gap: 6, color: 'var(--text-secondary)' }}>
                        Group by:
                        <select
                            value={groupBy}
                            onChange={(e) => onGroupByChange(e.target.value)}
                            data-testid="group-by-select"
                            style={{
                                padding: '4px 8px', border: '1px solid var(--border-color)',
                                borderRadius: 6, background: 'var(--bg-primary)',
                                color: 'var(--text-primary)', fontSize: 12,
                            }}
                        >
                            {dimensions.map(d => (
                                <option key={d.value} value={d.value}>{d.label}</option>
                            ))}
                        </select>
                    </label>
                    <button
                        type="button"
                        onClick={onCollapseAll}
                        data-testid="collapse-all"
                        style={{
                            padding: '4px 10px', border: '1px solid var(--border-color)',
                            borderRadius: 6, background: 'var(--bg-primary)',
                            color: 'var(--text-primary)', fontSize: 12, cursor: 'pointer',
                        }}
                    >Collapse all</button>
                    <button
                        type="button"
                        onClick={onExpandAll}
                        data-testid="expand-all"
                        style={{
                            padding: '4px 10px', border: '1px solid var(--border-color)',
                            borderRadius: 6, background: 'var(--bg-primary)',
                            color: 'var(--text-primary)', fontSize: 12, cursor: 'pointer',
                        }}
                    >Expand all</button>
                </>
            )}

            <div style={{ flex: 1 }} />
            <span style={{ color: 'var(--text-secondary)' }}>{summary}</span>
        </div>
    );
}
