import { diffWordsWithSpace } from 'diff';

// Word-level text diff -> [{value, added, removed}]. First in-repo use of the
// long-installed `diff` package (see CLAUDE.md stack notes).
export function diffText(a, b) {
    return diffWordsWithSpace(a || '', b || '');
}

// One side of a word-diff. leftParts is the ORIGINAL text (drop insertions);
// rightParts is the NEW text (drop deletions). Common segments stay on both.
export function leftParts(parts) {
    return (parts || []).filter(p => !p.added);
}
export function rightParts(parts) {
    return (parts || []).filter(p => !p.removed);
}

const stepEqual = (a, b) => a.action === b.action && a.expected_result === b.expected_result;

// Concise field/step diff between a draft and its regenerated alternative
// (spec: "Show a concise field and step diff before choosing a replacement").
// Steps align by index; length differences classify as added/removed tail.
export function buildDraftDiff(original, alternative) {
    const oSteps = original.steps || [];
    const aSteps = alternative.steps || [];
    const steps = [];
    const max = Math.max(oSteps.length, aSteps.length);
    for (let i = 0; i < max; i++) {
        const o = oSteps[i];
        const a = aSteps[i];
        if (o && a) {
            steps.push(stepEqual(o, a)
                ? { type: 'unchanged', index: i, action: null, expected: null }
                : { type: 'changed', index: i, action: diffText(o.action, a.action), expected: diffText(o.expected_result, a.expected_result) });
        } else if (a) {
            steps.push({ type: 'added', index: i, action: [{ value: a.action, added: true }], expected: [{ value: a.expected_result, added: true }] });
        } else {
            steps.push({ type: 'removed', index: i, action: [{ value: o.action, removed: true }], expected: [{ value: o.expected_result, removed: true }] });
        }
    }
    const oRefs = original.source_refs || [];
    const aRefs = alternative.source_refs || [];
    const refsAdded = aRefs.filter(r => !oRefs.includes(r));
    const refsRemoved = oRefs.filter(r => !aRefs.includes(r));

    const name = diffText(original.name, alternative.name);
    const description = diffText(original.description, alternative.description);
    const categoryChanged = original.category !== alternative.category;
    const partsChanged = (parts) => parts.some(p => p.added || p.removed);

    const summary = {
        nameChanged: partsChanged(name),
        categoryChanged,
        descriptionChanged: partsChanged(description),
        stepsChanged: steps.filter(s => s.type === 'changed').length,
        stepsAdded: steps.filter(s => s.type === 'added').length,
        stepsRemoved: steps.filter(s => s.type === 'removed').length,
        stepsUnchanged: steps.filter(s => s.type === 'unchanged').length,
        refsAdded: refsAdded.length,
        refsRemoved: refsRemoved.length,
    };

    return {
        name,
        description,
        category: { from: original.category, to: alternative.category, changed: categoryChanged },
        sourceRefs: { added: refsAdded, removed: refsRemoved },
        steps,
        summary,
    };
}
