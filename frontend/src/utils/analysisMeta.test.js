import test from 'node:test';
import assert from 'node:assert/strict';
import { badgeTitle, analysisMetaParts, narrativeNotice, groupingNote, isFailedAnalysis, failureHeading, failureMessage, failureAdvice, explainAction, takeoverNote, shouldReplaceAnalysis } from './analysisMeta.js';

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

test('shouldReplaceAnalysis keeps the newest version on screen', () => {
    assert.equal(shouldReplaceAnalysis(undefined, { version: 1 }), true);
    assert.equal(shouldReplaceAnalysis({ version: 2 }, { version: 3 }), true, 'a new analysis');
    assert.equal(shouldReplaceAnalysis({ version: 2 }, { version: 2 }), true, 'an explanation filled in on the current version');
    assert.equal(shouldReplaceAnalysis({ version: 2 }, { version: 1 }), false, 'explaining an older version does not roll the grid back');
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
