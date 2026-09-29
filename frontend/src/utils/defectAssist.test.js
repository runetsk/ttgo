import test from 'node:test';
import assert from 'node:assert/strict';
import { severityText, canApplySeverity, duplicateLabel, duplicatesSummary, defectLink } from './defectAssist.js';

const ASSIST = {
    severity: { value: 'major', confidence: 0.84, probabilities: { major: 0.84 } },
    duplicates: [{ defect_id: 'd1', title: 'Checkout total wrong', status: 'open', external_key: 'PAY-42', p_same: 0.91 }],
};

test('the severity suggestion reads with its confidence and can be applied when it differs', () => {
    assert.equal(severityText(ASSIST), 'Suggested severity: major (84%)');
    assert.equal(severityText({ severity: null }), null);
    assert.equal(canApplySeverity(ASSIST, 'minor'), true);
    assert.equal(canApplySeverity(ASSIST, 'major'), false);
    assert.equal(canApplySeverity({}, 'minor'), false);
});

test('duplicates name the key, the title and how likely they are the same problem', () => {
    assert.equal(duplicateLabel(ASSIST.duplicates[0]), 'PAY-42 · Checkout total wrong (91% likely the same problem)');
    assert.equal(duplicateLabel({ title: 'No key', p_same: 0.7 }), 'No key (70% likely the same problem)');
    assert.equal(duplicatesSummary(ASSIST), 'This may duplicate an open defect:');
    assert.equal(duplicatesSummary({ duplicates: [{}, {}] }), 'This may duplicate 2 open defects:');
    assert.equal(duplicatesSummary({ duplicates: [] }), 'No open defect looks like the same problem.');
    assert.equal(defectLink('a b'), '/defects?focus=a%20b');
});
