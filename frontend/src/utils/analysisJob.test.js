import test from 'node:test';
import assert from 'node:assert/strict';
import { isTerminalJob, jobSummary, showEndedJob } from './analysisJob.js';

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
