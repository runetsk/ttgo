import test from 'node:test';
import assert from 'node:assert/strict';
import { applyResultDelta } from './runResults.js';

const prev = () => ({
    id: 'run1', name: 'Run', status: 'RUNNING', updated_at: 't0',
    run_results: [
        { id: 'r1', status: 'FAIL', defect_type: 'to_investigate', test_case: { id: 'tc1', name: 'Case 1' }, open_defect_link_count: 2 },
        { id: 'r2', status: 'PENDING' },
    ],
});

test('upserts full rows: merges existing, appends new', () => {
    const next = applyResultDelta(prev(), {
        run_id: 'run1',
        run: { id: 'run1', status: 'RUNNING', updated_at: 't1' },
        results: [
            { id: 'r1', status: 'PASS', defect_type: '', open_defect_link_count: 2 }, // no test_case key
            { id: 'r3', status: 'PENDING', test_name_snapshot: 'New Case' },
        ],
    });
    assert.equal(next.run_results.length, 3);
    const r1 = next.run_results.find(r => r.id === 'r1');
    assert.equal(r1.status, 'PASS');
    assert.equal(r1.test_case.name, 'Case 1', 'missing keys must not clobber merged row fields');
    assert.equal(next.updated_at, 't1');
    assert.equal(next.run_results.find(r => r.id === 'r3').test_name_snapshot, 'New Case');
});

test('applies patch to listed ids only', () => {
    const next = applyResultDelta(prev(), {
        run_id: 'run1',
        run: { id: 'run1', status: 'RUNNING' },
        result_ids: ['r2', 'missing'],
        patch: { status: 'PASS', defect_type: '', updated_at: 't1' },
    });
    assert.equal(next.run_results.find(r => r.id === 'r2').status, 'PASS');
    assert.equal(next.run_results.find(r => r.id === 'r1').status, 'FAIL');
    assert.equal(next.run_results.length, 2, 'unknown patched ids are ignored');
});

test('removes deleted ids', () => {
    const next = applyResultDelta(prev(), {
        run_id: 'run1',
        run: { id: 'run1', status: 'RUNNING' },
        deleted_result_ids: ['r1'],
    });
    assert.deepEqual(next.run_results.map(r => r.id), ['r2']);
});

test('run summary fields merge without clobbering run_results', () => {
    const next = applyResultDelta(prev(), {
        run_id: 'run1',
        run: { id: 'run1', status: 'PASS', updated_at: 't2' }, // summary has no run_results key
    });
    assert.equal(next.status, 'PASS');
    assert.equal(next.run_results.length, 2);
});

test('returns prev unchanged when inapplicable', () => {
    assert.equal(applyResultDelta(null, { run_id: 'run1', run: { id: 'run1' } }), null);
    const p = prev();
    assert.equal(applyResultDelta(p, { run_id: 'other', run: { id: 'other' } }), p);
    assert.equal(applyResultDelta(p, null), p);
});
