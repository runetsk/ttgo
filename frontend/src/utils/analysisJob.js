// Pure text for the run's AI-analysis banner once its job has ended. No React, no network.
// Consumer: RunAnalysisBanner.jsx
import { usd } from './aiCost.js';

const plural = (n, one, many) => `${n} ${n === 1 ? one : many}`;

export function isTerminalJob(job) {
    return ['completed', 'failed', 'cancelled', 'skipped'].includes(job?.status);
}

// jobSummary describes an ended job from its outcome counts (GET /runs/{id}/analysis-job).
// tone: 'ok' when every group got a decision, 'warn' when some did not, 'error' when the job
// itself failed. retryable: some failed results still have no decision. retryHint: some of
// those failed on a settings problem, which a retry cannot fix (the button stays).
export function jobSummary(job) {
    if (!isTerminalJob(job)) return null;
    if (job.status === 'skipped') {
        // An automatic analysis the monthly budget held back; "Run anyway" starts it acknowledged.
        return {
            tone: 'warn', retryable: false, retryHint: null, runAnyway: true,
            text: `Automatic analysis skipped: this run (worst case ~${usd(job.skip_estimate_usd)}) would exceed the monthly AI budget (${usd(job.skip_spent_usd)} of ${usd(job.skip_budget_usd)} spent).`,
        };
    }
    const o = job.outcomes || {};
    const failedRows = o.failed_rows || 0;
    const retryable = failedRows > 0;
    const configFailed = o.failed_configuration || 0;
    const retryHint = retryable && configFailed > 0
        ? `${plural(configFailed, 'group', 'groups')} failed on a settings problem (model id, key). Retrying won't help until the settings are fixed.`
        : null;
    if (job.status === 'failed') {
        return { tone: 'error', retryable, retryHint, text: `AI analysis failed: ${job.error_message || 'unknown error'}.` };
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
    // Groups held back from the LLM, or auto-apply paused by its gate, need a look: the banner
    // stays until dismissed, like failures.
    const paused = (o.auto_apply_state ?? job.auto_apply_state) === 'paused';
    const tone = job.status === 'cancelled' || o.failed || o.injection_flagged > 0 || paused ? 'warn' : 'ok';
    return { tone, retryable, retryHint, text };
}

// showEndedJob: a clean finish is shown only when this page watched the job run; anything that
// needs attention (failures, a failed or cancelled job) stays until dismissed.
export function showEndedJob(summary, watchedItRun, dismissed) {
    if (!summary || dismissed) return false;
    return summary.tone !== 'ok' || watchedItRun;
}

const secs = (ms) => `${(ms / 1000).toFixed(1)} s`;

// jobTelemetry: how long the job's stages took, how often a provider rate-limited it, how many LLM
// calls hit the per-call timeout and how many hedged requests fired and won (outcomes of
// GET /runs/{id}/analysis-job, else the job's own columns), or null when there is nothing to report.
export function jobTelemetry(job) {
    const o = job?.outcomes || {};
    const parts = [];
    if (o.decision_ms_max > 0) parts.push(`TypeSafe ${secs(o.decision_ms_avg)} avg (p50 ${secs(o.decision_ms_p50)}, max ${secs(o.decision_ms_max)})`);
    if (o.llm_ms_max > 0) parts.push(`LLM ${secs(o.llm_ms_avg)} avg (p50 ${secs(o.llm_ms_p50)}, max ${secs(o.llm_ms_max)})`);
    if (o.rate_limit_hits > 0) parts.push(plural(o.rate_limit_hits, 'rate-limit hit', 'rate-limit hits'));
    const timeouts = o.call_timeouts ?? job?.call_timeouts ?? 0;
    if (timeouts > 0) parts.push(plural(timeouts, 'LLM call timed out', 'LLM calls timed out'));
    const fired = o.hedges_fired ?? job?.hedges_fired ?? 0;
    if (fired > 0) parts.push(`${plural(fired, 'hedge', 'hedges')} fired, ${o.hedges_won ?? job?.hedges_won ?? 0} won`);
    return parts.length ? parts.join(' · ') : null;
}

// runningNote: what the running banner adds while decisions are already shown and their
// explanations are still being written (outcomes.explanation_pending exists only while the job
// runs), or null.
export function runningNote(job) {
    const n = job?.outcomes?.explanation_pending || 0;
    return n > 0 ? `${plural(n, 'explanation', 'explanations')} in progress` : null;
}

// newerJobState picks what the banner shows when a refetched job lands: a response that left
// while the job was still running must not replace the same job already known to have ended.
export function newerJobState(prev, next) {
    if (!next) return prev;
    if (prev && prev.id === next.id && isTerminalJob(prev) && !isTerminalJob(next)) return prev;
    return next;
}

// jobNotes: extra lines for an ended job, about what the TypeSafe checks found and what auto-apply
// did. It covers groups held back from the LLM for possible prompt injection, grouped results their
// group's explanation may not fit (and checks that failed or were skipped over the per-group limit),
// labels written by auto-apply, and auto-apply paused because its accuracy gate was closed.
export function jobNotes(job) {
    const o = job?.outcomes || {};
    const notes = [];
    if (o.injection_flagged > 0) {
        notes.push({ key: 'injection', text: `${plural(o.injection_flagged, 'group was', 'groups were')} flagged for possible prompt injection: no explanation was written. Review the raw failure.` });
    }
    if (o.transfer_mismatch > 0) {
        const one = o.transfer_mismatch === 1;
        notes.push({ key: 'transfer', text: `${plural(o.transfer_mismatch, 'grouped result', 'grouped results')} may not match ${one ? 'its' : 'their'} group's explanation; open ${one ? 'it' : 'them'} to explain ${one ? 'it' : 'them'} on ${one ? 'its' : 'their'} own.` });
    }
    if (o.transfer_check_failed > 0) {
        notes.push({ key: 'transfer-failed', text: `The explanation check failed for ${plural(o.transfer_check_failed, 'group', 'groups')}.` });
    }
    if (o.transfer_unchecked > 0) {
        notes.push({ key: 'transfer-unchecked', text: `${plural(o.transfer_unchecked, 'grouped result was', 'grouped results were')} not checked against the explanation (40 per group at most).` });
    }
    if (o.auto_applied > 0) notes.push({ key: 'auto-applied', text: `${plural(o.auto_applied, 'label', 'labels')} set by AI` });
    if ((o.auto_apply_state ?? job?.auto_apply_state) === 'paused') {
        notes.push({ key: 'auto-apply-paused', text: 'Auto-apply paused: accuracy gate closed' });
    }
    return notes;
}
