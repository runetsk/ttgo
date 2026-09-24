// Pure text for the run's AI-analysis banner once its job has ended. No React, no network.
// Consumer: RunAnalysisBanner.jsx

const plural = (n, one, many) => `${n} ${n === 1 ? one : many}`;

export function isTerminalJob(job) {
    return ['completed', 'failed', 'cancelled'].includes(job?.status);
}

// jobSummary describes an ended job from its outcome counts (GET /runs/{id}/analysis-job).
// tone: 'ok' when every group got a decision, 'warn' when some did not, 'error' when the job
// itself failed. retryable: some failed results still have no decision.
export function jobSummary(job) {
    if (!isTerminalJob(job)) return null;
    const o = job.outcomes || {};
    const failedRows = o.failed_rows || 0;
    const retryable = failedRows > 0;
    if (job.status === 'failed') {
        return { tone: 'error', retryable, text: `AI analysis failed: ${job.error_message || 'unknown error'}.` };
    }
    const parts = [];
    const decided = o.decided || 0;
    parts.push(`${plural(decided, 'group', 'groups')} decided`);
    if (o.failed) parts.push(`${o.failed} failed`);
    if (o.unknown) parts.push(`${o.unknown} unknown`);
    if (o.taken_over) parts.push(`${o.taken_over} decided by the LLM below the threshold`);
    if (o.no_explanation) parts.push(`${o.no_explanation} without an explanation`);
    if (o.explanation_skipped) parts.push(`${o.explanation_skipped} with explanations off`);
    const head = job.status === 'cancelled'
        ? `AI analysis was cancelled after ${job.analyzed_count || 0} of ${job.capped_at || 0} groups`
        : job.retry_failed_only ? 'Retry of failed groups finished' : 'AI analysis finished';
    let text = `${head}: ${parts.join(', ')}.`;
    if (retryable) text += ` ${plural(failedRows, 'failed result has', 'failed results have')} no decision yet.`;
    const tone = job.status === 'cancelled' || o.failed ? 'warn' : 'ok';
    return { tone, retryable, text };
}

// showEndedJob: a clean finish is shown only when this page watched the job run; anything that
// needs attention (failures, a failed or cancelled job) stays until dismissed.
export function showEndedJob(summary, watchedItRun, dismissed) {
    if (!summary || dismissed) return false;
    return summary.tone !== 'ok' || watchedItRun;
}
