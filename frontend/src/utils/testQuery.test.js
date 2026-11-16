import test from 'node:test';
import assert from 'node:assert/strict';
import { localRangeToUTC, buildTestPageParams } from './testQuery.js';

test('localRangeToUTC maps a day range to [localMidnight(from), localMidnight(to+1))', () => {
    const r = localRangeToUTC({ from: '2026-06-15', to: '2026-06-17' });
    assert.equal(r.from, new Date(2026, 5, 15).toISOString(), 'lower bound is local midnight of from');
    assert.equal(r.to, new Date(2026, 5, 18).toISOString(), 'upper bound is exclusive: local midnight after to');
});

test('localRangeToUTC spans a whole calendar day (DST-aware)', () => {
    const r = localRangeToUTC({ from: '2026-03-08', to: '2026-03-08' });
    const hours = (Date.parse(r.to) - Date.parse(r.from)) / 3600000;
    assert.ok([23, 24, 25].includes(hours), `a single local day is 23/24/25h, got ${hours}`);
});

test('localRangeToUTC handles partial and empty ranges', () => {
    assert.deepEqual(localRangeToUTC({ from: null, to: null }), {});
    assert.deepEqual(localRangeToUTC(null), {});
    const onlyFrom = localRangeToUTC({ from: '2026-01-10', to: null });
    assert.ok(onlyFrom.from && !('to' in onlyFrom));
    const onlyTo = localRangeToUTC({ from: null, to: '2026-01-10' });
    assert.ok(onlyTo.to && !('from' in onlyTo));
});

const grid = (over = {}) => ({
    folderIds: ['f1', 'f2'],
    search: 'hi',
    columnFilters: {
        id: 'x', name: '', description: 'd', categories: ['c1', 'c2'],
        steps_count: '3', open_defects: '', reverification_flagged: 'yes', linked_requirements: 'PROJ',
        created_at: { from: '2026-01-01', to: null }, updated_at: { from: null, to: null },
    },
    sort: { key: 'name', direction: 'desc' },
    pageSize: 50, page: 3,
    ...over,
});

test('buildTestPageParams maps grid state to query params', () => {
    const p = buildTestPageParams(grid());
    assert.equal(p.folder_ids, 'f1,f2');
    assert.equal(p.search, 'hi');
    assert.equal(p.id, 'x');
    assert.equal('name' in p, false, 'empty filters are omitted');
    assert.equal(p.description, 'd');
    assert.equal(p.category_ids, 'c1,c2');
    assert.equal(p.steps_count, '3');
    assert.equal('open_defects' in p, false);
    assert.equal(p.reverification, 'yes');
    assert.equal(p.linked_requirement, 'PROJ');
    assert.equal(p.sort, 'name');
    assert.equal(p.dir, 'desc');
    assert.equal(p.limit, 50);
    assert.equal(p.offset, 100, 'page 3 of 50 → offset 100');
    assert.ok(p.created_from);
    assert.equal('created_to' in p, false);
    assert.equal('updated_from' in p, false);
});

test('buildTestPageParams defaults sort to name asc and omits empty search', () => {
    const p = buildTestPageParams(grid({ search: '', sort: {}, columnFilters: { categories: [] } }));
    assert.equal('search' in p, false);
    assert.equal(p.sort, 'name');
    assert.equal(p.dir, 'asc');
    assert.equal('category_ids' in p, false);
});
