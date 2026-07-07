import test from 'node:test';
import assert from 'node:assert/strict';
import { collectTestIds, folderCheckState, toggleFolderSelection } from './treeSelection.js';

const tree = {
    id: 'root', name: 'Root',
    test_cases: [{ id: 't1', name: 'a' }],
    sub_folders: [
        { id: 'sub', name: 'Sub', test_cases: [{ id: 't2', name: 'b' }, { id: 't3', name: 'c' }], sub_folders: [] },
    ],
};

test('collectTestIds gathers own + descendant test ids', () => {
    assert.deepEqual(collectTestIds(tree).sort(), ['t1', 't2', 't3']);
    assert.deepEqual(collectTestIds({ id: 'x', name: 'x', test_cases: [], sub_folders: [] }), []);
});

test('folderCheckState reflects selection', () => {
    assert.equal(folderCheckState(tree, new Set()), 'empty');
    assert.equal(folderCheckState(tree, new Set(['t1'])), 'partial');
    assert.equal(folderCheckState(tree, new Set(['t1', 't2', 't3'])), 'checked');
    assert.equal(folderCheckState({ id: 'x', name: 'x', test_cases: [], sub_folders: [] }, new Set()), 'empty');
});

test('toggleFolderSelection adds all when not full, clears when full', () => {
    const added = toggleFolderSelection(tree, new Set(['t1']));
    assert.deepEqual([...added].sort(), ['t1', 't2', 't3']);
    const cleared = toggleFolderSelection(tree, new Set(['t1', 't2', 't3']));
    assert.equal(cleared.size, 0);
});
