// Maps an AIGenerationRun API object onto the legacy `debug` shape that
// LlmFeedbackPanel renders, so the panel needs no rework during the
// lifecycle-endpoint migration.
export function runToDebug(run) {
    if (!run) return null;
    const debug = {
        duration_ms: run.duration_ms,
        model: run.model_name,
        finish_reason: run.finish_reason,
        max_tokens_budget: run.max_tokens_budget,
        retried: (run.retry_count || 0) > 0,
        provider_label: run.provider_label,
        provider_type: run.provider_type,
        request_context: run.request_context,
        template_type: run.template_type,
    };
    if ((run.prompt_tokens || 0) > 0 || (run.completion_tokens || 0) > 0 || (run.total_tokens || 0) > 0) {
        debug.usage = {
            prompt_tokens: run.prompt_tokens,
            completion_tokens: run.completion_tokens,
            total_tokens: run.total_tokens,
        };
    }
    if (run.estimated_cost != null) {
        debug.estimated_cost = run.estimated_cost;
    }
    return debug;
}
