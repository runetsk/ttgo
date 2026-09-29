import test from 'node:test';
import assert from 'node:assert/strict';
import { isAIApplied, aiBadgeTitle, DEFECT_TYPE_SOURCE_AI } from './defectTypeSource.js';

const ai = { id: 'r1', status: 'FAIL', defect_type: 'automation_bug', defect_type_source: 'ai' };

test('isAIApplied: a conclusive label auto-apply wrote on a failure', () => {
    assert.equal(DEFECT_TYPE_SOURCE_AI, 'ai');
    assert.equal(isAIApplied(ai), true);
    assert.equal(isAIApplied({ ...ai, status: 'ERROR' }), true);
    assert.equal(isAIApplied({ ...ai, defect_type_source: '' }), false, 'the automatic default, or unknown');
    assert.equal(isAIApplied({ ...ai, defect_type_source: 'human' }), false, 'a person set it (Amendment 1)');
    assert.equal(isAIApplied({ ...ai, defect_type_source: undefined }), false, 'an older server');
    assert.equal(isAIApplied({ ...ai, defect_type: 'to_investigate' }), false, 'reset to the default');
    assert.equal(isAIApplied({ ...ai, status: 'PASS' }), false);
    assert.equal(isAIApplied(null), false);
});

test('aiBadgeTitle names the label, its confidence when known, and what Confirm does', () => {
    assert.equal(aiBadgeTitle(ai, { suggested_defect_type: 'automation_bug', suggested_defect_type_confidence: 0.97 }),
        'Set by AI: TypeSafe.ai suggested "Automation bug" with confidence 97% and auto-apply wrote it. Confirm keeps it as your decision; pick another value to correct it.');
    assert.equal(aiBadgeTitle(ai),
        'Set by AI: TypeSafe.ai suggested "Automation bug" and auto-apply wrote it. Confirm keeps it as your decision; pick another value to correct it.');
    assert.match(aiBadgeTitle(ai, { suggested_defect_type: 'product_bug', suggested_defect_type_confidence: 0.99 }), /suggested "Automation bug" and/,
        'an analysis that suggests another value lends no confidence');
    assert.equal(aiBadgeTitle({ ...ai, defect_type_source: '' }), undefined);
});
