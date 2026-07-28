import { TRIAGE_TILES } from '../../utils/defectQueue';

// The four tiles above the queue. They are the page's answer to "what do I do
// next": each one names a backlog and, clicked, drops the table into it.
//
// The tile table itself (labels, counts, filter presets) lives in
// utils/defectQueue so it is unit-tested against the same derivation the table
// uses — this file only renders it.

// Hint text under each count. Only "Critical open" is dynamic: it reports how
// much work the criticals are blocking, which is the reason to look at them
// first. Keys match TRIAGE_TILES entries.
const HINTS = {
    triage: 'unassigned · no owner yet',
    stale: 'no update in 7 days',
    fixed: 'ready to verify',
};

function tileHint(tile, counts) {
    if (tile.key !== 'critical') return HINTS[tile.key] || '';
    const blocked = counts.blockingTests || 0;
    return `blocking ${blocked} test${blocked === 1 ? '' : 's'}`;
}

// A tile reads as pressed when the filters it sets are the ones in force.
// Non-pressable tiles (see TRIAGE_TILES) get no aria-pressed at all rather than
// a permanent "not pressed" — they are plain actions, not toggles.
function pressedState(tile, status, severities) {
    if (tile.pressable === false) return undefined;
    if (tile.filters.status !== status) return false;
    const want = tile.filters.severities;
    return want.length === severities.length && want.every(sev => severities.includes(sev));
}

export default function TriageStrip({ counts = {}, status = 'all', severities = [], onPick }) {
    return (
        <div className="defects-triage-strip">
            {TRIAGE_TILES.map(tile => (
                <button
                    key={tile.key}
                    type="button"
                    className={`defects-tile${tile.tone === 'alert' ? ' defects-tile--alert' : ''}`}
                    aria-pressed={pressedState(tile, status, severities)}
                    // Copied, never handed out by reference: the page stores these
                    // straight into state and must not be able to mutate the table.
                    onClick={() => onPick({ ...tile.filters, severities: [...tile.filters.severities] })}
                >
                    <span className="defects-tile-label">{tile.label}</span>
                    <span className="defects-tile-value">
                        <span className="defects-tile-count">{counts[tile.countKey] ?? 0}</span>
                        <span className="defects-tile-hint">{tileHint(tile, counts)}</span>
                    </span>
                </button>
            ))}
        </div>
    );
}
