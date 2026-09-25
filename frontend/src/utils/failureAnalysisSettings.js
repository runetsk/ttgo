// Pure helpers for the AI Failure Analysis settings card. No React, no network.
// Ranges match the server (backend/internal/api/ai/failure_analysis.go).
// Consumers: components/AIFailureAnalysisSettings.jsx, utils/analysisFlow.js (via the card's state).

export const MAX_ANALYSES_MIN = 1;
export const MAX_ANALYSES_MAX = 500;
export const PARALLEL_MIN = 1;
export const PARALLEL_MAX = 8;

// The fields the card saves; anything else in the settings object (id, timestamps) is ignored.
export const FA_FIELDS = ['enabled_on_completion', 'max_analyses_per_run', 'parallel_groups', 'dedup_enabled', 'redaction_enabled', 'prompt_template'];

const wholeIn = (n, min, max) => Number.isInteger(n) && n >= min && n <= max;

// validateFailureAnalysisDraft returns a message per field the server would reject.
export function validateFailureAnalysisDraft(draft) {
    const errors = {};
    if (!wholeIn(draft?.max_analyses_per_run, MAX_ANALYSES_MIN, MAX_ANALYSES_MAX)) {
        errors.max_analyses_per_run = `Enter a whole number from ${MAX_ANALYSES_MIN} to ${MAX_ANALYSES_MAX}.`;
    }
    if (!wholeIn(draft?.parallel_groups, PARALLEL_MIN, PARALLEL_MAX)) {
        errors.parallel_groups = `Enter a whole number from ${PARALLEL_MIN} to ${PARALLEL_MAX}.`;
    }
    if (typeof draft?.prompt_template !== 'string' || draft.prompt_template.trim() === '') {
        errors.prompt_template = 'The prompt template cannot be empty.';
    }
    return errors;
}

// isFailureAnalysisDirty reports whether the draft differs from the saved settings.
export function isFailureAnalysisDirty(draft, saved) {
    return FA_FIELDS.some((f) => draft?.[f] !== saved?.[f]);
}

// parseWholeNumber reads a number input. An emptied or non-numeric input stays NaN so validation
// flags it, instead of silently becoming 0 (which the server answers with a 400).
export function parseWholeNumber(value) {
    if (value === '' || value == null) return NaN;
    const n = Number(value);
    return Number.isFinite(n) ? n : NaN;
}
