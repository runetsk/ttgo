// Pure tri-state selection helpers for the test-library folder tree.
// A folder node is { id, name, sub_folders: [...], test_cases: [{id, name}] }.

const EMPTY = new Set();

export function collectTestIds(node) {
    const ids = (node.test_cases || []).map(t => t.id);
    for (const sub of (node.sub_folders || [])) {
        ids.push(...collectTestIds(sub));
    }
    return ids;
}

// 'checked' when the folder has ≥1 test and every descendant test is selected
// or locked; 'partial' when some but not all are; 'empty' otherwise. Locked ids
// (fixed selections, e.g. tests already in a run) always count as selected.
export function folderCheckState(node, selected, locked = EMPTY) {
    const ids = collectTestIds(node);
    if (ids.length === 0) return 'empty';
    const n = ids.filter(id => selected.has(id) || locked.has(id)).length;
    if (n === 0) return 'empty';
    return n === ids.length ? 'checked' : 'partial';
}

// Returns a new Set: clears all *non-locked* descendant test ids when the folder
// is fully checked, otherwise adds them all. Locked ids are never added or
// removed — the caller owns them elsewhere.
export function toggleFolderSelection(node, selected, locked = EMPTY) {
    const ids = collectTestIds(node);
    const next = new Set(selected);
    if (folderCheckState(node, selected, locked) === 'checked') {
        for (const id of ids) if (!locked.has(id)) next.delete(id);
    } else {
        for (const id of ids) if (!locked.has(id)) next.add(id);
    }
    return next;
}
