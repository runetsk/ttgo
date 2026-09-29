import test from 'node:test';
import assert from 'node:assert/strict';
import { semanticPanelText, splitConfirmText, semanticReportLine, ANALYSIS_QUEUED_EVENT } from './semanticPanel.js';

const VIEW = {
    representative: { result_id: 'r1', test_name: 'Pay', failure_type: 'http', error_message: 'POST returned 503' },
    result: { result_id: 'r2', test_name: 'Capture', failure_type: 'http', error_message: 'gateway unavailable (503)' },
    pair: { p_same: 0.86, source: 'typesafe', model: 'jev-latest', answered_model: 'jev-1.13.0', policy_version: 'fa-semantic-v1', remembered_from: null },
    group_size: 9, split_group_size: 3, other_groups: 1, can_split: true, split_blocked_reason: '',
};

test('the panel says how sure TypeSafe was and where the answer came from', () => {
    const t = semanticPanelText(VIEW);
    assert.equal(t.probability, 'TypeSafe.ai is 86% sure these failures share a cause.');
    assert.equal(t.provenance, 'Decided in this analysis.');
    assert.equal(t.model, 'jev-1.13.0 · fa-semantic-v1');
    assert.equal(t.splitSummary, 'Splitting takes out 3 results with this error and analyzes them on their own. Later analyses keep them apart from the 1 other error in this group.');
});

test('a remembered answer names the analysis it came from', () => {
    const t = semanticPanelText({ ...VIEW, pair: { ...VIEW.pair, source: 'memory', remembered_from: '2026-09-20T10:00:00Z' } });
    assert.match(t.provenance, /^Remembered from an analysis on .*2026.*; not asked again\.$/);
    const noDate = semanticPanelText({ ...VIEW, pair: { ...VIEW.pair, source: 'memory', remembered_from: null } });
    assert.equal(noDate.provenance, 'Remembered from an earlier analysis; not asked again.');
});

test('the panel copes with a merge from before the pair record and a single result', () => {
    const t = semanticPanelText({ ...VIEW, pair: null, split_group_size: 1, other_groups: 2 });
    assert.equal(t.probability, 'TypeSafe.ai judged these failures to share a cause.');
    assert.equal(t.model, '');
    assert.equal(t.splitSummary, 'Splitting takes out 1 result with this error and analyzes it on its own. Later analyses keep it apart from the 2 other errors in this group.');
    assert.equal(semanticPanelText({ ...VIEW, split_group_size: 0 }).splitSummary, null);
    assert.equal(semanticPanelText(null), null);
});

test('the split confirmation says it runs an analysis and is permanent', () => {
    assert.equal(splitConfirmText(VIEW), 'Split 3 results from this group and analyze them on their own? This runs a new analysis, and later analyses will not merge them with this group again.');
    assert.match(splitConfirmText({ split_group_size: 1 }), /^Split 1 result .* analyze it on its own\?/);
});

test('the banner line counts merges, pairs, remembered and person-kept-apart pairs', () => {
    assert.equal(semanticReportLine({ semantic_report: '{"asked":6,"remembered":5,"human_blocked":1,"merged":3}' }),
        'Semantic grouping: 3 merges from 12 pairs (5 remembered, 1 kept apart by a person)');
    assert.equal(semanticReportLine({ semantic_report: { asked: 1, merged: 1 } }), 'Semantic grouping: 1 merge from 1 pair');
    assert.equal(semanticReportLine({ semantic_report: '{"asked":0,"remembered":0,"human_blocked":0,"merged":0}' }), null);
    assert.equal(semanticReportLine({ semantic_report: '' }), null);
    assert.equal(semanticReportLine({ semantic_report: 'nope' }), null);
    assert.equal(semanticReportLine(null), null);
    assert.equal(ANALYSIS_QUEUED_EVENT, 'ttgo:run-analysis-queued');
});
