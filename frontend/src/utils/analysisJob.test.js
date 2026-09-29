import test from 'node:test';
import assert from 'node:assert/strict';
import { isTerminalJob, jobSummary, showEndedJob, jobTelemetry, runningNote, newerJobState, jobNotes } from './analysisJob.js';

test('isTerminalJob', () => {
    for (const s of ['completed', 'failed', 'cancelled']) assert.equal(isTerminalJob({ status: s }), true, s);
    for (const s of ['queued', 'running', undefined]) assert.equal(isTerminalJob({ status: s }), false, String(s));
    assert.equal(jobSummary({ status: 'running' }), null);
});

test('jobSummary: a clean finish', () => {
    const s = jobSummary({ status: 'completed', outcomes: { groups: 12, decided: 12, explanation_skipped: 12 } });
    assert.equal(s.tone, 'ok');
    assert.equal(s.retryable, false);
    assert.equal(s.text, 'AI analysis finished: 12 groups decided, 12 with explanations off.');
});

test('jobSummary: failures are counted and retryable', () => {
    const s = jobSummary({ status: 'completed', outcomes: { groups: 10, decided: 8, failed: 2, unknown: 1, taken_over: 3, no_explanation: 1, failed_rows: 5 } });
    assert.equal(s.tone, 'warn');
    assert.equal(s.retryable, true);
    assert.equal(s.text, 'AI analysis finished: 8 groups decided, 2 failed, 1 unknown, 3 decided by the LLM below the threshold, 1 without an explanation. 5 failed results have no decision yet.');
    assert.match(jobSummary({ status: 'completed', outcomes: { decided: 1, failed: 1, failed_rows: 1 } }).text, /1 failed result has no decision yet/);
});

test('jobSummary: cancelled, failed and retry jobs', () => {
    const c = jobSummary({ status: 'cancelled', analyzed_count: 3, capped_at: 9, outcomes: { decided: 3 } });
    assert.equal(c.tone, 'warn');
    assert.match(c.text, /^AI analysis was cancelled after 3 of 9 groups: 3 groups decided\.$/);
    const f = jobSummary({ status: 'failed', error_message: 'AI features are switched off' });
    assert.equal(f.tone, 'error');
    assert.equal(f.text, 'AI analysis failed: AI features are switched off.');
    assert.match(jobSummary({ status: 'completed', retry_failed_only: true, outcomes: { decided: 1 } }).text, /^Retry of failed groups finished: 1 group decided\./);
});

test('showEndedJob: clean finishes only when watched, problems until dismissed', () => {
    const ok = { tone: 'ok' };
    const warn = { tone: 'warn' };
    assert.equal(showEndedJob(ok, false, false), false, 'an old clean job is not news');
    assert.equal(showEndedJob(ok, true, false), true);
    assert.equal(showEndedJob(warn, false, false), true);
    assert.equal(showEndedJob(warn, false, true), false);
    assert.equal(showEndedJob(null, true, false), false);
});

test('jobSummary: settings failures say retrying will not help yet', () => {
    const s = jobSummary({ status: 'completed', outcomes: { decided: 1, failed: 2, failed_rows: 3, failed_configuration: 2 } });
    assert.equal(s.retryable, true, 'Retry failed groups stays available');
    assert.equal(s.retryHint, "2 groups failed on a settings problem (model id, key). Retrying won't help until the settings are fixed.");
    assert.equal(jobSummary({ status: 'completed', outcomes: { decided: 1, failed: 1, failed_rows: 1 } }).retryHint, null);
    assert.match(jobSummary({ status: 'failed', error_message: 'x', outcomes: { failed_rows: 1, failed_configuration: 1 } }).retryHint,
        /^1 group failed on a settings problem/);
});

test('jobSummary: an automatic analysis skipped over the monthly budget offers Run anyway', () => {
    const job = { status: 'skipped', skip_reason: 'budget', skip_estimate_usd: 0.42, skip_spent_usd: 9.8, skip_budget_usd: 10 };
    assert.equal(isTerminalJob(job), true);
    const s = jobSummary(job);
    assert.equal(s.tone, 'warn');
    assert.equal(s.runAnyway, true);
    assert.equal(s.retryable, false);
    assert.equal(s.text, 'Automatic analysis skipped: this run (worst case ~$0.42) would exceed the monthly AI budget ($9.80 of $10.00 spent).');
    assert.equal(showEndedJob(s, false, false), true, 'shown until dismissed');
});

test('jobTelemetry: stage timing and rate-limit hits', () => {
    assert.equal(jobTelemetry({ outcomes: {} }), null);
    assert.equal(jobTelemetry({}), null);
    assert.equal(jobTelemetry({ outcomes: {
        decision_ms_avg: 1200, decision_ms_p50: 1100, decision_ms_max: 3400,
        llm_ms_avg: 4000, llm_ms_p50: 3200, llm_ms_max: 9800, rate_limit_hits: 3,
    } }), 'TypeSafe 1.2 s avg (p50 1.1 s, max 3.4 s) · LLM 4.0 s avg (p50 3.2 s, max 9.8 s) · 3 rate-limit hits');
    assert.equal(jobTelemetry({ outcomes: { rate_limit_hits: 1 } }), '1 rate-limit hit');
});

test('runningNote counts explanations still being written', () => {
    assert.equal(runningNote({ status: 'running', outcomes: { explanation_pending: 3 } }), '3 explanations in progress');
    assert.equal(runningNote({ status: 'running', outcomes: { explanation_pending: 1 } }), '1 explanation in progress');
    assert.equal(runningNote({ status: 'running', outcomes: {} }), null);
    assert.equal(runningNote({ status: 'running' }), null);
    assert.equal(runningNote(null), null);
});

test('jobTelemetry: LLM call timeouts and hedges', () => {
    assert.equal(jobTelemetry({ outcomes: { call_timeouts: 2, hedges_fired: 3, hedges_won: 1 } }),
        '2 LLM calls timed out · 3 hedges fired, 1 won');
    assert.equal(jobTelemetry({ outcomes: { call_timeouts: 1 } }), '1 LLM call timed out');
    assert.equal(jobTelemetry({ outcomes: { hedges_fired: 1 } }), '1 hedge fired, 0 won');
    assert.equal(jobTelemetry({ call_timeouts: 2, outcomes: {} }), '2 LLM calls timed out', 'falls back to the job columns');
    assert.equal(jobTelemetry({ outcomes: { call_timeouts: 0, hedges_fired: 0, hedges_won: 0 } }), null);
    assert.equal(jobTelemetry({ outcomes: {
        llm_ms_avg: 4000, llm_ms_p50: 3200, llm_ms_max: 9800, rate_limit_hits: 1, call_timeouts: 1, hedges_fired: 2, hedges_won: 2,
    } }), 'LLM 4.0 s avg (p50 3.2 s, max 9.8 s) · 1 rate-limit hit · 1 LLM call timed out · 2 hedges fired, 2 won');
});

test('newerJobState never rolls an ended job back to running', () => {
    const ended = { id: 'j1', status: 'completed' };
    const running = { id: 'j1', status: 'running' };
    assert.equal(newerJobState(ended, running), ended, 'a refetch that left before the job ended');
    assert.equal(newerJobState(running, ended), ended);
    assert.equal(newerJobState(ended, { id: 'j2', status: 'queued' }).id, 'j2', 'a new job replaces an ended one');
    assert.equal(newerJobState(running, null), running);
    assert.equal(newerJobState(null, running), running);
});

test('jobNotes: injection flags, transfer checks, labels set by AI and a paused auto-apply', () => {
    assert.deepEqual(jobNotes({ status: 'completed', outcomes: {} }), []);
    assert.deepEqual(jobNotes(null), []);
    const notes = jobNotes({ status: 'completed', outcomes: {
        injection_flagged: 1, transfer_mismatch: 2, transfer_check_failed: 1, transfer_unchecked: 3, auto_applied: 4, auto_apply_state: 'paused',
    } });
    assert.deepEqual(notes.map((n) => n.key), ['injection', 'transfer', 'transfer-failed', 'transfer-unchecked', 'auto-applied', 'auto-apply-paused']);
    assert.deepEqual(notes.map((n) => n.text), [
        '1 group was flagged for possible prompt injection: no explanation was written. Review the raw failure.',
        "2 grouped results may not match their group's explanation; open them to explain them on their own.",
        'The explanation check failed for 1 group.',
        '3 grouped results were not checked against the explanation (40 per group at most).',
        '4 labels set by AI',
        'Auto-apply paused: accuracy gate closed',
    ]);
    assert.equal(jobNotes({ outcomes: { transfer_mismatch: 1, auto_applied: 1, injection_flagged: 2 } }).map((n) => n.text).join(' | '),
        "2 groups were flagged for possible prompt injection: no explanation was written. Review the raw failure. | 1 grouped result may not match its group's explanation; open it to explain it on its own. | 1 label set by AI");
    assert.deepEqual(jobNotes({ auto_apply_state: 'paused', outcomes: {} }).map((n) => n.key), ['auto-apply-paused'], 'falls back to the job column');
    assert.deepEqual(jobNotes({ outcomes: { auto_apply_state: 'on', auto_applied: 0 } }), []);
});

test('jobSummary: a flagged group or a paused auto-apply keeps the banner up until dismissed', () => {
    const flagged = jobSummary({ status: 'completed', outcomes: { decided: 2, injection_flagged: 1 } });
    assert.equal(flagged.tone, 'warn');
    assert.equal(flagged.text, 'AI analysis finished: 2 groups decided.');
    assert.equal(showEndedJob(flagged, false, false), true);
    assert.equal(jobSummary({ status: 'completed', outcomes: { decided: 2, auto_apply_state: 'paused' } }).tone, 'warn');
    assert.equal(jobSummary({ status: 'completed', auto_apply_state: 'paused', outcomes: { decided: 2 } }).tone, 'warn');
    assert.equal(jobSummary({ status: 'completed', outcomes: { decided: 2, auto_apply_state: 'on', auto_applied: 3 } }).tone, 'ok');
});
