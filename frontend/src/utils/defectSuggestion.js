// Pure helpers for the AI-suggested defect_type chip. No React, no network.
// The backend owns the verdict -> defect_type mapping (models.SuggestedDefectType);
// the frontend only decides whether to surface what it was handed.
// Consumers: frontend/src/pages/testRunDetail/ResultsTab.jsx
import { isFailureStatus } from './resultStatus.js';

// A result is "untriaged" while its defect_type is still the failure auto-default
// (to_investigate) or empty — neither is a human decision, so a suggestion helps.
const UNTRIAGED_DEFECT_TYPES = new Set(['', 'to_investigate']);

const SUGGESTION_LABELS = {
    product_bug: 'Product bug',
    automation_bug: 'Automation bug',
    system_issue: 'System issue',
};

// shouldShowSuggestion reports whether the AI's suggested defect_type is worth
// showing for a row. Deliberately narrow: the chip is an aid for undecided
// failures, not a nag — it disappears the moment a human triages the row.
export function shouldShowSuggestion(result, analysis) {
    if (!result || !isFailureStatus(result.status)) return false;
    if (!UNTRIAGED_DEFECT_TYPES.has(result.defect_type || '')) return false;
    const suggested = analysis?.suggested_defect_type;
    return typeof suggested === 'string' && suggested.trim() !== '';
}

// suggestionLabel renders a defect_type key as human text, falling back to the
// raw key so an unknown value from a newer backend still reads sensibly.
export function suggestionLabel(defectType) {
    return SUGGESTION_LABELS[defectType] || defectType || '';
}
