// Pure helpers for defect types that AI auto-apply wrote (run_results.defect_type_source = 'ai';
// 'human' = a person's explicit triage and '' = the automatic default both render as ordinary
// labels). No React, no network. An AI label is not a triage decision: the server never stamps decided_at
// for it and leaves it out of accuracy, examples and history. Confirm turns it into one.
// Consumer: pages/testRunDetail/ResultsTab.jsx
import { isFailureStatus } from './resultStatus.js';
import { suggestionLabel } from './defectSuggestion.js';

export const DEFECT_TYPE_SOURCE_AI = 'ai';

// Auto-apply only ever writes one of the three conclusive types.
const CONCLUSIVE = new Set(['product_bug', 'automation_bug', 'system_issue']);

export function isAIApplied(result) {
    return !!result && result.defect_type_source === DEFECT_TYPE_SOURCE_AI
        && isFailureStatus(result.status) && CONCLUSIVE.has(result.defect_type);
}

// aiBadgeTitle is the AI badge's tooltip. `analysis` (the result's current analysis) adds the
// confidence only when it suggested the value the row carries.
export function aiBadgeTitle(result, analysis) {
    if (!isAIApplied(result)) return undefined;
    const label = suggestionLabel(result.defect_type);
    const score = analysis?.suggested_defect_type === result.defect_type ? analysis?.suggested_defect_type_confidence : null;
    const conf = Number.isFinite(score) ? ` with confidence ${Math.round(score * 100)}%` : '';
    return `Set by AI: TypeSafe.ai suggested "${label}"${conf} and auto-apply wrote it. Confirm keeps it as your decision; pick another value to correct it.`;
}
