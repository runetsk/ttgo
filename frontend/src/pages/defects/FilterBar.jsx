import { SEVERITY_ORDER, SORT_OPTIONS, STATUS_TABS } from '../../utils/defectQueue';

// Status tabs · severity chips · sort. Every option comes from the tables in
// utils/defectQueue, so a tab key can never drift from a derived status and a
// chip can never name a severity the sort does not rank.
//
// Active tabs and chips carry aria-pressed. That is what the stylesheet keys
// off as well, so the "on" state is never signalled by colour alone.
export default function FilterBar({
    counts = {},
    status = 'all',
    onStatusChange,
    severities = [],
    onToggleSeverity,
    sort = 'priority',
    onSortChange,
}) {
    const tabCounts = counts.tabs || {};

    return (
        <div className="defects-filterbar">
            <div className="defects-tabs" role="group" aria-label="Filter by status">
                {STATUS_TABS.map(tab => (
                    <button
                        key={tab.key}
                        type="button"
                        className="defects-tab"
                        aria-pressed={status === tab.key}
                        onClick={() => onStatusChange(tab.key)}
                    >
                        {tab.label}
                        <span className="defects-tab-count">{tabCounts[tab.key] ?? 0}</span>
                    </button>
                ))}
            </div>

            <span className="defects-filter-divider" aria-hidden="true" />

            <div className="defects-sev-chips" role="group" aria-label="Filter by severity">
                {SEVERITY_ORDER.map(severity => (
                    <button
                        key={severity}
                        type="button"
                        className={`defects-sev-chip defects-sev--${severity}`}
                        aria-pressed={severities.includes(severity)}
                        onClick={() => onToggleSeverity(severity)}
                    >
                        <span className="defects-sev-dot" aria-hidden="true" />
                        {severity}
                    </button>
                ))}
            </div>

            <div className="defects-sort">
                <label className="defects-sort-label" htmlFor="defects-sort">Sort</label>
                <select
                    id="defects-sort"
                    className="defects-sort-select"
                    value={sort}
                    onChange={event => onSortChange(event.target.value)}
                >
                    {SORT_OPTIONS.map(option => (
                        <option key={option.value} value={option.value}>{option.label}</option>
                    ))}
                </select>
            </div>
        </div>
    );
}
