import { diffWordsWithSpace } from 'diff';

// Word-level text diff -> [{value, added, removed}]. First in-repo use of the
// long-installed `diff` package (see CLAUDE.md stack notes).
export function diffText(a, b) {
    return diffWordsWithSpace(a || '', b || '');
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
    return {
        name: diffText(original.name, alternative.name),
        description: diffText(original.description, alternative.description),
        category: { from: original.category, to: alternative.category, changed: original.category !== alternative.category },
        sourceRefs: {
            added: aRefs.filter(r => !oRefs.includes(r)),
            removed: oRefs.filter(r => !aRefs.includes(r)),
        },
        steps,
    };
}
