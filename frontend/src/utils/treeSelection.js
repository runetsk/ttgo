// Pure tri-state selection helpers for the test-library folder tree.
// A folder node is { id, name, sub_folders: [...], test_cases: [{id, name}] }.

export function collectTestIds(node) {
    const ids = (node.test_cases || []).map(t => t.id);
    for (const sub of (node.sub_folders || [])) {
        ids.push(...collectTestIds(sub));
    }
    return ids;
}

// 'checked' when the folder has ≥1 test and all descendant tests are selected;
// 'partial' when some but not all are; 'empty' otherwise (including no tests).
export function folderCheckState(node, selected) {
    const ids = collectTestIds(node);
    if (ids.length === 0) return 'empty';
    const n = ids.filter(id => selected.has(id)).length;
    if (n === 0) return 'empty';
    return n === ids.length ? 'checked' : 'partial';
}

// Returns a new Set: clears all descendant test ids when the folder is fully
// checked, otherwise adds them all.
export function toggleFolderSelection(node, selected) {
    const ids = collectTestIds(node);
    const next = new Set(selected);
    if (folderCheckState(node, selected) === 'checked') {
        for (const id of ids) next.delete(id);
    } else {
        for (const id of ids) next.add(id);
    }
    return next;
}
