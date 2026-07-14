import test from 'node:test';
import assert from 'node:assert/strict';
import { draftFlags, isDraftClean, filterDrafts, REVIEW_FILTERS } from './draftReview.js';

const clean = { id: 'a', status: 'pending', edited: false, source_refs: ['AC-1'] };
const invalid = { id: 'b', status: 'pending', findings: [{ severity: 'error', code: 'no_steps' }], source_refs: ['AC-1'] };
const warned = { id: 'c', status: 'pending', quality: [{ key: 'specificity', findings: [{ severity: 'warning' }] }], source_refs: ['AC-1'] };
const dupHigh = { id: 'd', status: 'pending', duplicates: [{ kind: 'existing', similarity: 0.95 }], source_refs: ['AC-1'] };
const dupLow = { id: 'e', status: 'pending', duplicates: [{ kind: 'batch', similarity: 0.6 }], source_refs: ['AC-1'] };
const edited = { id: 'f', status: 'pending', edited: true, source_refs: ['AC-1'] };
const accepted = { id: 'g', status: 'accepted', source_refs: ['AC-1'] };
const rejected = { id: 'h', status: 'rejected', source_refs: ['AC-1'] };
const uncovered = { id: 'i', status: 'pending', source_refs: [] };
const all = [clean, invalid, warned, dupHigh, dupLow, edited, accepted, rejected, uncovered];

test('draftFlags classifies findings, duplicates, refs, and edits', () => {
    assert.deepEqual(draftFlags(invalid).invalid, true);
    assert.equal(draftFlags(clean).invalid, false);
    assert.equal(draftFlags(warned).warnings, true);
    assert.equal(draftFlags(dupHigh).highConfidenceDuplicate, true);
    assert.equal(draftFlags(dupLow).highConfidenceDuplicate, false);
    assert.equal(draftFlags(dupLow).possibleDuplicate, true);
    assert.equal(draftFlags(uncovered).uncovered, true);
    assert.equal(draftFlags(edited).edited, true);
});

test('isDraftClean excludes invalid and high-confidence duplicates only', () => {
    assert.equal(isDraftClean(clean), true);
    assert.equal(isDraftClean(warned), true, 'plain warnings do not block accept-all-clean');
    assert.equal(isDraftClean(dupLow), true, 'low-similarity candidates do not block');
    assert.equal(isDraftClean(invalid), false);
    assert.equal(isDraftClean(dupHigh), false);
    assert.equal(isDraftClean(accepted), false, 'only pending drafts are acceptable');
});

test('filterDrafts implements every REVIEW_FILTERS key', () => {
    assert.equal(filterDrafts(all, 'all').length, all.length);
    assert.deepEqual(filterDrafts(all, 'invalid').map(d => d.id), ['b']);
    assert.deepEqual(filterDrafts(all, 'duplicates').map(d => d.id), ['d', 'e']);
    assert.deepEqual(filterDrafts(all, 'uncovered').map(d => d.id), ['i']);
    assert.deepEqual(filterDrafts(all, 'edited').map(d => d.id), ['f']);
    assert.deepEqual(filterDrafts(all, 'accepted').map(d => d.id), ['g']);
    assert.deepEqual(filterDrafts(all, 'rejected').map(d => d.id), ['h']);
    assert.ok(filterDrafts(all, 'pending').every(d => d.status === 'pending'));
    for (const f of REVIEW_FILTERS) {
        assert.ok(Array.isArray(filterDrafts(all, f.key)), `filter ${f.key} returns an array`);
    }
});
