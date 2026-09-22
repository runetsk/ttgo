import test from 'node:test';
import assert from 'node:assert/strict';
import { badgeTitle, analysisMetaParts, narrativeNotice, groupingNote } from './analysisMeta.js';

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
