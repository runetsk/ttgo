import { TRIAGE_TILES, tileHint, tilePressed } from '../../utils/defectQueue';

// The four tiles above the queue. They are the page's answer to "what do I do
// next": each one names a backlog and, clicked, drops the table into exactly the
// rows it counted — and clicked again while pressed, releases it. The release
// itself is the page's (DefectsPage.pickTile); the tile only reports which of the
// two a click means.
//
// The tile table itself (labels, counts, filter presets) and both derivations
// below it (the hint line and the pressed state) live in utils/defectQueue so
// they are unit-tested against the same filtering the table uses — this file
// only renders them.

export default function TriageStrip({ counts = {}, view, onPick }) {
    return (
        <div className="defects-triage-strip">
            {TRIAGE_TILES.map(tile => {
                // Computed once and used twice on purpose: the same value that
                // renders aria-pressed is the one handed to onPick, so what the
                // tile says about itself and what clicking it does cannot drift.
                const pressed = tilePressed(tile, view);
                return (
                    <button
                        key={tile.key}
                        type="button"
                        className={`defects-tile${tile.tone === 'alert' ? ' defects-tile--alert' : ''}`}
                        aria-pressed={pressed}
                        // Copied, never handed out by reference: the page stores these
                        // straight into state and must not be able to mutate the table.
                        onClick={() => onPick({ ...tile.filters, severities: [...tile.filters.severities] }, pressed)}
                    >
                        <span className="defects-tile-label">{tile.label}</span>
                        <span className="defects-tile-value">
                            <span className="defects-tile-count">{counts[tile.countKey] ?? 0}</span>
                            <span className="defects-tile-hint">{tileHint(tile, counts)}</span>
                        </span>
                    </button>
                );
            })}
        </div>
    );
}
