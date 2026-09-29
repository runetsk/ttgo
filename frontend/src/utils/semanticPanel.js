// Pure text for the semantic-merge panel on a grouped result's AI card and the job banner's
// semantic-grouping line (TypeSafe backlog Wave 4). No React, no network.
// Consumers: components/RunResultDetail.jsx, components/RunAnalysisBanner.jsx

// Sent on window after a split queues its analysis, so the run's job banner fetches the new job.
export const ANALYSIS_QUEUED_EVENT = 'ttgo:run-analysis-queued';

const pct = (p) => `${Math.round(p * 100)}%`;
const plural = (n, one, many) => `${n} ${n === 1 ? one : many}`;

// formatDay renders an ISO timestamp as a short local date; '' when missing or invalid.
function formatDay(iso) {
    if (!iso) return '';
    const d = new Date(iso);
    return Number.isNaN(d.getTime()) ? '' : d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
}

// semanticPanelText derives the panel's lines from GET …/semantic:
// - probability: how sure TypeSafe was that the two failures share a cause;
// - provenance: asked in this analysis, or remembered from an earlier one (and when);
// - model: the configured model, the one that answered, and the question policy;
// - splitSummary: what a split takes out and keeps apart.
export function semanticPanelText(view) {
    if (!view) return null;
    const p = view.pair?.p_same;
    const probability = Number.isFinite(p)
        ? `TypeSafe.ai is ${pct(p)} sure these failures share a cause.`
        : 'TypeSafe.ai judged these failures to share a cause.';
    let provenance = 'Decided in this analysis.';
    if (view.pair?.source === 'memory') {
        const day = formatDay(view.pair.remembered_from);
        provenance = day ? `Remembered from an analysis on ${day}; not asked again.` : 'Remembered from an earlier analysis; not asked again.';
    }
    const pair = view.pair || {};
    const model = [pair.answered_model || pair.model, pair.policy_version].filter(Boolean).join(' · ');
    const n = view.split_group_size || 0;
    const others = view.other_groups || 0;
    const splitSummary = n > 0
        ? `Splitting takes out ${plural(n, 'result', 'results')} with this error and analyzes ${n === 1 ? 'it' : 'them'} on ${n === 1 ? 'its' : 'their'} own. Later analyses keep ${n === 1 ? 'it' : 'them'} apart from the ${plural(others, 'other error', 'other errors')} in this group.`
        : null;
    return { probability, provenance, model, splitSummary };
}

// splitConfirmText is the question asked before a split (it starts a paid analysis).
export function splitConfirmText(view) {
    const n = view?.split_group_size || 0;
    return `Split ${plural(n, 'result', 'results')} from this group and analyze ${n === 1 ? 'it' : 'them'} on ${n === 1 ? 'its' : 'their'} own? This runs a new analysis, and later analyses will not merge ${n === 1 ? 'it' : 'them'} with this group again.`;
}

function parseReport(raw) {
    if (raw && typeof raw === 'object') return raw;
    if (typeof raw !== 'string' || raw === '') return null;
    try {
        const v = JSON.parse(raw);
        return v && typeof v === 'object' ? v : null;
    } catch {
        return null;
    }
}

// semanticReportLine is the banner's semantic-grouping line from the job's semantic_report, or
// null when the pass decided no pair (or did not run).
export function semanticReportLine(job) {
    const r = parseReport(job?.semantic_report);
    if (!r) return null;
    const asked = r.asked || 0;
    const remembered = r.remembered || 0;
    const blocked = r.human_blocked || 0;
    const pairs = asked + remembered + blocked;
    if (pairs === 0) return null;
    const merges = r.merged || 0;
    const extras = [
        remembered ? `${remembered} remembered` : null,
        blocked ? `${blocked} kept apart by a person` : null,
    ].filter(Boolean);
    return `Semantic grouping: ${plural(merges, 'merge', 'merges')} from ${plural(pairs, 'pair', 'pairs')}${extras.length ? ` (${extras.join(', ')})` : ''}`;
}
