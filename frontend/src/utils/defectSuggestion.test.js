import test from 'node:test';
import assert from 'node:assert/strict';
import { shouldShowSuggestion, suggestionLabel } from './defectSuggestion.js';

const failResult = (overrides = {}) => ({
    id: 'r1', status: 'FAIL', defect_type: 'to_investigate', ...overrides,
});
const analysis = (suggested) => ({ id: 'a1', verdict: 'product_bug', suggested_defect_type: suggested });

test('shows for an untriaged FAIL with a suggestion', () => {
    assert.equal(shouldShowSuggestion(failResult(), analysis('product_bug')), true);
});

test('shows when defect_type is empty or missing (also untriaged)', () => {
    assert.equal(shouldShowSuggestion(failResult({ defect_type: '' }), analysis('system_issue')), true);
    assert.equal(shouldShowSuggestion(failResult({ defect_type: undefined }), analysis('system_issue')), true);
    assert.equal(shouldShowSuggestion(failResult({ defect_type: null }), analysis('system_issue')), true);
});

test('hides once the row is triaged by a human', () => {
    for (const decided of ['product_bug', 'automation_bug', 'system_issue']) {
        assert.equal(
            shouldShowSuggestion(failResult({ defect_type: decided }), analysis('product_bug')), false,
            `${decided} is a decision — the chip must not nag`,
        );
    }
});

test('hides when there is no analysis or no suggestion', () => {
    assert.equal(shouldShowSuggestion(failResult(), undefined), false);
    assert.equal(shouldShowSuggestion(failResult(), null), false);
    assert.equal(shouldShowSuggestion(failResult(), {}), false);
    assert.equal(shouldShowSuggestion(failResult(), analysis('')), false, 'unknown verdict maps to ""');
    assert.equal(shouldShowSuggestion(failResult(), analysis('   ')), false, 'whitespace is not a suggestion');
    assert.equal(shouldShowSuggestion(failResult(), { suggested_defect_type: 123 }), false);
});

test('hides for non-failure results', () => {
    for (const status of ['PASS', 'SKIP', 'PENDING', 'RUNNING']) {
        assert.equal(
            shouldShowSuggestion(failResult({ status, defect_type: '' }), analysis('product_bug')), false,
            `${status} has no defect_type control`,
        );
    }
});

// ERROR results are triageable too (the analyzer already produces verdicts for
// them); the chip used to be FAIL-only, which is the gap these cover.
test('shows for an untriaged ERROR with a suggestion', () => {
    assert.equal(shouldShowSuggestion(failResult({ status: 'ERROR' }), analysis('system_issue')), true);
    assert.equal(
        shouldShowSuggestion(failResult({ status: 'ERROR', defect_type: '' }), analysis('system_issue')), true,
        'an empty defect_type is untriaged for ERROR just as it is for FAIL',
    );
});

test('hides once an ERROR row is triaged by a human', () => {
    for (const decided of ['product_bug', 'automation_bug', 'system_issue']) {
        assert.equal(
            shouldShowSuggestion(failResult({ status: 'ERROR', defect_type: decided }), analysis('product_bug')), false,
            `${decided} is a decision — the chip must not nag on ERROR rows either`,
        );
    }
});

test('hides for a missing result', () => {
    assert.equal(shouldShowSuggestion(null, analysis('product_bug')), false);
    assert.equal(shouldShowSuggestion(undefined, analysis('product_bug')), false);
});

test('suggestionLabel maps known keys and falls back to the raw value', () => {
    assert.equal(suggestionLabel('product_bug'), 'Product bug');
    assert.equal(suggestionLabel('automation_bug'), 'Automation bug');
    assert.equal(suggestionLabel('system_issue'), 'System issue');
    assert.equal(suggestionLabel('future_type'), 'future_type');
    assert.equal(suggestionLabel(''), '');
    assert.equal(suggestionLabel(undefined), '');
});
