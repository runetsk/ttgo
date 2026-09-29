// Pure helpers for the AI Failure Analysis settings card. No React, no network.
// Ranges match the server (backend/internal/api/ai/failure_analysis.go).
// Consumers: components/AIFailureAnalysisSettings.jsx, utils/analysisFlow.js (via the card's state). Auto-apply rules: autoApplySettings.js.

import { AUTO_APPLY_DEFAULTS, minConfidenceError } from './autoApplySettings.js';

export const MAX_ANALYSES_MIN = 1;
export const MAX_ANALYSES_MAX = 500;
export const PARALLEL_MIN = 1;
export const PARALLEL_MAX = 8;
export const LLM_TIMEOUT_MIN = 10;
export const LLM_TIMEOUT_MAX = 120;
export const HEDGE_MIN = 3; // 0 = off
export const FEW_SHOT_MIN = 0;
export const FEW_SHOT_MAX = 8;

// Server defaults of the fields added with the tail-latency and few-shot settings. A settings
// object without them (an older server, a test mock) reads as these values instead of failing
// validation or looking unsaved.
export const FA_DEFAULTS = { llm_call_timeout_seconds: 45, hedge_after_seconds: 0, few_shot_examples: 4, ...AUTO_APPLY_DEFAULTS };

// The fields the card saves; anything else in the settings object (id, timestamps) is ignored.
export const FA_FIELDS = [
    'enabled_on_completion', 'max_analyses_per_run', 'parallel_groups', 'dedup_enabled', 'redaction_enabled', 'prompt_template',
    'llm_call_timeout_seconds', 'hedge_after_seconds', 'few_shot_examples', 'auto_apply_defect_type', 'auto_apply_min_confidence',
];

const wholeIn = (n, min, max) => Number.isInteger(n) && n >= min && n <= max;

// withFailureAnalysisDefaults fills the FA_DEFAULTS fields the settings object lacks.
export function withFailureAnalysisDefaults(settings) {
    if (!settings) return settings;
    const out = { ...settings };
    for (const [k, v] of Object.entries(FA_DEFAULTS)) if (out[k] === undefined || out[k] === null) out[k] = v;
    return out;
}

// hedgeMax: the longest hedge delay for a call timeout (one second below it). With an invalid
// timeout the widest range applies; the timeout field reports its own error.
export function hedgeMax(timeout) {
    return wholeIn(timeout, LLM_TIMEOUT_MIN, LLM_TIMEOUT_MAX) ? timeout - 1 : LLM_TIMEOUT_MAX - 1;
}

// validateFailureAnalysisDraft returns a message per field the server would reject.
export function validateFailureAnalysisDraft(draft) {
    const errors = {};
    if (!wholeIn(draft?.max_analyses_per_run, MAX_ANALYSES_MIN, MAX_ANALYSES_MAX)) {
        errors.max_analyses_per_run = `Enter a whole number from ${MAX_ANALYSES_MIN} to ${MAX_ANALYSES_MAX}.`;
    }
    if (!wholeIn(draft?.parallel_groups, PARALLEL_MIN, PARALLEL_MAX)) {
        errors.parallel_groups = `Enter a whole number from ${PARALLEL_MIN} to ${PARALLEL_MAX}.`;
    }
    if (!wholeIn(draft?.llm_call_timeout_seconds, LLM_TIMEOUT_MIN, LLM_TIMEOUT_MAX)) {
        errors.llm_call_timeout_seconds = `Enter a whole number of seconds from ${LLM_TIMEOUT_MIN} to ${LLM_TIMEOUT_MAX}.`;
    }
    const hedge = draft?.hedge_after_seconds;
    const maxHedge = hedgeMax(draft?.llm_call_timeout_seconds);
    if (hedge !== 0 && !wholeIn(hedge, HEDGE_MIN, maxHedge)) {
        errors.hedge_after_seconds = `Enter 0 (off) or a whole number of seconds from ${HEDGE_MIN} to ${maxHedge}, below the LLM call timeout.`;
    }
    if (!wholeIn(draft?.few_shot_examples, FEW_SHOT_MIN, FEW_SHOT_MAX)) {
        errors.few_shot_examples = `Enter a whole number from ${FEW_SHOT_MIN} to ${FEW_SHOT_MAX}.`;
    }
    const minConfidence = minConfidenceError(draft?.auto_apply_min_confidence);
    if (minConfidence) errors.auto_apply_min_confidence = minConfidence;
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
