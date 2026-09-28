import test from 'node:test';
import assert from 'node:assert/strict';
import {
    badgeTitle, analysisMetaParts, narrativeNotice, groupingNote, isFailedAnalysis, failureHeading, failureMessage,
    failureAdvice, explainAction, takeoverNote, mergeAnalysis, mergeAnalysisList, analysisFromEvent,
    isPendingNarrative, PENDING_EXPLANATION,
} from './analysisMeta.js';

const ts = {
    engine: 'typesafe', model_name: 'jev-1.13.0', confidence_score: 0.87, confidence: 'medium',
    suggested_defect_type: 'automation_bug', suggested_defect_type_confidence: 0.91,
    narrative_status: 'ok', dedup_method: '', dedup_p_same: null,
};

test('badgeTitle names the engine and numeric confidence for TypeSafe rows only', () => {
    assert.equal(badgeTitle(ts), 'TypeSafe jev-1.13.0 · confidence 0.87');
    assert.equal(badgeTitle({ engine: 'generative', confidence: 'high' }), undefined);
    assert.equal(badgeTitle({}), undefined);
});

test('analysisMetaParts lists engine, model, confidence and the suggestion', () => {
    assert.deepEqual(analysisMetaParts(ts), [
        'Engine: TypeSafe', 'jev-1.13.0', 'confidence 0.87', 'suggested defect type: Automation bug (0.91)',
    ]);
    assert.deepEqual(analysisMetaParts({ ...ts, suggested_defect_type: '' }), [
        'Engine: TypeSafe', 'jev-1.13.0', 'confidence 0.87', 'no defect type suggested',
    ]);
    assert.deepEqual(analysisMetaParts({ engine: 'generative', model_name: 'gpt-x', confidence: 'high' }), ['Engine: LLM', 'gpt-x']);
});

test('narrativeNotice explains missing or unparseable narratives', () => {
    assert.equal(narrativeNotice(ts), null);
    assert.equal(narrativeNotice({ ...ts, narrative_status: 'unavailable' }), 'The explanation could not be generated; the classification above still stands.');
    assert.equal(narrativeNotice({ ...ts, narrative_status: 'unparseable' }), 'The model returned an unreadable explanation; the raw text is under Rationale.');
    assert.match(narrativeNotice({ ...ts, narrative_status: 'skipped' }), /^No explanation was written because explanations are switched off/);
    assert.equal(narrativeNotice({ ...ts, narrative_status: 'unavailable', decision_status: 'failed' }), null, 'a failed attempt has its own card');
});

test('groupingNote distinguishes signature and semantic clones', () => {
    assert.equal(groupingNote({ dedup_group_key: null }), null);
    assert.equal(groupingNote({ dedup_group_key: 'k', dedup_method: 'signature' }), 'Grouped with an identical failure');
    assert.equal(groupingNote({ dedup_group_key: 'k', dedup_method: '' }), 'Grouped with an identical failure');
    assert.equal(groupingNote({ dedup_group_key: 'k', dedup_method: 'semantic', dedup_p_same: 0.93 }), 'Grouped semantically (p = 0.93)');
});

test('analysisMetaParts says when the suggestion was derived from the verdict', () => {
    const parts = analysisMetaParts({
        engine: 'typesafe', model_name: 'jev', confidence_score: 0.96,
        suggested_defect_type: 'product_bug', suggested_defect_type_confidence: 0.96, suggestion_source: 'verdict',
    });
    assert.ok(parts.some((p) => p.includes('from the verdict')), JSON.stringify(parts));
    assert.ok(parts.some((p) => p.includes('(0.96, from the verdict)')), JSON.stringify(parts));
    const asked = analysisMetaParts({ engine: 'typesafe', suggested_defect_type: 'product_bug', suggested_defect_type_confidence: 0.7 });
    assert.ok(!asked.some((p) => p.includes('from the verdict')), 'a question-answered suggestion is not marked');
});

test('isFailedAnalysis reads decision_status, and the summary on older rows', () => {
    assert.equal(isFailedAnalysis({ decision_status: 'failed' }), true);
    assert.equal(isFailedAnalysis({ decision_status: 'ok', summary: 'analysis failed: looks like one' }), false, 'the status wins');
    assert.equal(isFailedAnalysis({ summary: 'analysis failed: provider 502' }), true);
    assert.equal(isFailedAnalysis({ verdict: 'unknown', summary: 'Not enough evidence.' }), false, 'unknown is an answer');
    assert.equal(isFailedAnalysis(null), false);
});

test('failureHeading and failureMessage describe a failed attempt', () => {
    assert.equal(failureHeading({ error_category: 'timeout' }), 'Analysis failed · timed out');
    assert.equal(failureHeading({ error_category: 'some_new_kind' }), 'Analysis failed · some new kind');
    assert.equal(failureHeading({}), 'Analysis failed');
    assert.equal(failureMessage({ summary: 'analysis failed: upstream 502' }), 'upstream 502');
    assert.equal(failureMessage({ summary: 'AI returned unparseable response after retry' }), 'AI returned unparseable response after retry');
});

test('explainAction offers Explain or Retry only on unexplained TypeSafe decisions', () => {
    assert.equal(explainAction({ ...ts, narrative_status: 'skipped' }), 'Explain');
    assert.equal(explainAction({ ...ts, narrative_status: 'unavailable' }), 'Retry explanation');
    assert.equal(explainAction({ ...ts, narrative_status: 'unparseable' }), 'Retry explanation');
    assert.equal(explainAction(ts), null);
    assert.equal(explainAction({ ...ts, narrative_status: 'skipped', decision_status: 'failed' }), null);
    assert.equal(explainAction({ engine: 'generative', narrative_status: 'unavailable' }), null);
});

test('takeoverNote records what TypeSafe said before the LLM decided', () => {
    assert.equal(takeoverNote({ takeover_from_verdict: 'flaky_test', takeover_from_confidence: 0.48 }),
        'TypeSafe said flaky test with confidence 0.48, below the takeover threshold, so the LLM decided.');
    assert.equal(takeoverNote({ takeover_from_verdict: '' }), null);
    assert.equal(takeoverNote(ts), null);
});

test('failureHeading names a settings problem', () => {
    assert.equal(failureHeading({ error_category: 'configuration' }), 'Analysis failed · settings problem');
});

test('failureAdvice points a settings failure at the settings instead of Re-analyze', () => {
    assert.equal(failureAdvice({ error_category: 'timeout' }), 'No decision was made. Re-analyze to try again.');
    assert.equal(failureAdvice({ engine: 'typesafe', error_category: 'configuration' }),
        'No decision was made. Check the TypeSafe.ai settings (model id, key), then Re-analyze.');
    assert.equal(failureAdvice({ engine: 'generative', error_category: 'configuration' }),
        'No decision was made. Check the LLM provider settings (model, key), then Re-analyze.');
    assert.equal(failureAdvice(null), 'No decision was made. Re-analyze to try again.');
});

test('a pending explanation reads "Explanation being written…" with no Explain button', () => {
    const pending = { ...ts, narrative_status: 'pending' };
    assert.equal(PENDING_EXPLANATION, 'Explanation being written…');
    assert.equal(isPendingNarrative(pending), true);
    assert.equal(isPendingNarrative(ts), false);
    assert.equal(narrativeNotice(pending), 'Explanation being written…');
    assert.equal(explainAction(pending), null, 'nothing to ask for while it is being written');
    assert.equal(badgeTitle(pending), 'TypeSafe jev-1.13.0 · confidence 0.87 · Explanation being written…');
    const failed = { ...pending, decision_status: 'failed' };
    assert.equal(isPendingNarrative(failed), false, 'a failed attempt has its own card');
    assert.equal(narrativeNotice(failed), null);
});

test('mergeAnalysis keeps the newest version, and within a version the latest explanation', () => {
    const pending = { id: 'a', version: 2, narrative_status: 'pending', narrative_revision: 0, summary: '' };
    const written = { id: 'a', version: 2, narrative_status: 'ok', narrative_revision: 1, summary: 'S' };
    assert.equal(mergeAnalysis(undefined, pending), pending);
    assert.equal(mergeAnalysis(pending, undefined), pending);
    assert.deepEqual(mergeAnalysis(pending, written), written, 'the explanation fills in the decision');
    assert.equal(mergeAnalysis(written, pending), written, 'a stale pending never overwrites a written explanation');
    assert.equal(mergeAnalysis(written, { ...written, summary: 'S2' }).summary, 'S2', 'an equal revision replaces (a REST refresh of the same state)');
    assert.equal(mergeAnalysis({ id: 'b', version: 3, narrative_revision: 0 }, { id: 'a', version: 2, narrative_revision: 5 }).id, 'b',
        'explaining an older version does not roll the row back');
    assert.equal(mergeAnalysis({ id: 'a', version: 2, narrative_revision: 4 }, { id: 'b', version: 3, narrative_revision: 0 }).id, 'b',
        'a newer version wins whatever its revision');
    assert.equal(mergeAnalysis({ version: 1 }, { version: 1, summary: 'x' }).summary, 'x', 'missing revisions count as 0');
});

test('mergeAnalysis keeps fields the incoming copy of the same analysis lacks', () => {
    const current = { id: 'a', version: 1, narrative_revision: 0, raw_response: 'r', summary: '' };
    const incoming = { id: 'a', version: 1, narrative_revision: 1, summary: 'S' };
    assert.deepEqual(mergeAnalysis(current, incoming), { id: 'a', version: 1, narrative_revision: 1, raw_response: 'r', summary: 'S' });
});

test('mergeAnalysisList updates a version in place and adds new versions newest first', () => {
    const v1 = { id: 'a1', version: 1, narrative_status: 'skipped', narrative_revision: 0 };
    const v2 = { id: 'a2', version: 2, narrative_status: 'ok', narrative_revision: 1, summary: 'S' };
    const list = [v2, v1];
    assert.equal(mergeAnalysisList(list, { ...v2, narrative_status: 'pending', narrative_revision: 0 }), list, 'a stale event changes nothing');
    const explained = mergeAnalysisList(list, { ...v1, narrative_status: 'ok', narrative_revision: 2, summary: 'old version explained' });
    assert.deepEqual(explained.map((a) => [a.version, a.summary]), [[2, 'S'], [1, 'old version explained']]);
    const v3 = { id: 'a3', version: 3, narrative_status: 'pending', narrative_revision: 0 };
    assert.deepEqual(mergeAnalysisList(list, v3).map((a) => a.id), ['a3', 'a2', 'a1']);
    assert.deepEqual(mergeAnalysisList(null, v3), [v3]);
    assert.equal(mergeAnalysisList(list, { version: 4 }), list, 'a row without an id is ignored');
});

test('analysisFromEvent reads the shared payload and leaves out fields an older payload lacks', () => {
    const row = analysisFromEvent({
        run_result_id: 'r1', analysis_id: 'a1', version: 2, verdict: 'flaky_test', confidence: 'high', engine: 'typesafe',
        narrative_status: 'pending', narrative_revision: 0, summary: '', next_action: '', rationale: '',
        created_at: '2026-09-28T10:00:00Z', policy_version: 'fa-verdict-v6', history_available: true, source_analysis_id: null,
    });
    assert.equal(row.id, 'a1');
    assert.equal(row.run_result_id, 'r1');
    assert.equal(row.narrative_status, 'pending');
    assert.equal(row.narrative_revision, 0);
    assert.equal(row.policy_version, 'fa-verdict-v6');
    assert.equal(row.history_available, true);
    assert.equal(row.source_analysis_id, null);
    assert.equal(row.summary, '');
    const old = analysisFromEvent({ run_result_id: 'r1', analysis_id: 'a1', version: 1, verdict: 'unknown', confidence: 'low' });
    assert.equal(old.engine, 'generative');
    assert.equal(old.narrative_status, 'ok');
    assert.equal(old.decision_status, 'ok');
    assert.ok(!('summary' in old), 'an older payload does not blank the summary');
    assert.equal(analysisFromEvent({}), null);
    assert.equal(analysisFromEvent(undefined), null);
});
