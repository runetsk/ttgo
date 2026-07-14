// Pure review-state logic for AI generation drafts (Stage 4 reviewer workflow).
// Server draft shape: { id, status, edited, findings, quality, duplicates, source_refs }.

export const HIGH_CONFIDENCE_DUPLICATE = 0.9; // mirrors backend aigen.DupHighConfidence

export const REVIEW_FILTERS = [
    { key: 'all', label: 'All' },
    { key: 'pending', label: 'Pending' },
    { key: 'edited', label: 'Edited' },
    { key: 'invalid', label: 'Invalid' },
    { key: 'warnings', label: 'Warnings' },
    { key: 'duplicates', label: 'Duplicates' },
    { key: 'uncovered', label: 'No refs' },
    { key: 'accepted', label: 'Accepted' },
    { key: 'rejected', label: 'Rejected' },
];

export function draftFlags(draft) {
    const findings = draft.findings || [];
    const quality = draft.quality || [];
    const duplicates = draft.duplicates || [];
    return {
        invalid: findings.some(f => f.severity === 'error'),
        warnings: findings.some(f => f.severity === 'warning') || quality.length > 0,
        possibleDuplicate: duplicates.length > 0,
        highConfidenceDuplicate: duplicates.some(d => (d.similarity || 0) >= HIGH_CONFIDENCE_DUPLICATE),
        uncovered: (draft.source_refs || []).length === 0,
        edited: !!draft.edited,
    };
}

// "Clean" per spec: pending, structurally valid, and no unresolved
// high-confidence duplicate. Plain warnings never block.
export function isDraftClean(draft) {
    if (draft.status !== 'pending') return false;
    const flags = draftFlags(draft);
    return !flags.invalid && !flags.highConfidenceDuplicate;
}

export function filterDrafts(drafts, filterKey) {
    switch (filterKey) {
        case 'pending': return drafts.filter(d => d.status === 'pending');
        case 'accepted': return drafts.filter(d => d.status === 'accepted');
        case 'rejected': return drafts.filter(d => d.status === 'rejected');
        case 'edited': return drafts.filter(d => draftFlags(d).edited);
        case 'invalid': return drafts.filter(d => draftFlags(d).invalid);
        case 'warnings': return drafts.filter(d => draftFlags(d).warnings);
        case 'duplicates': return drafts.filter(d => draftFlags(d).possibleDuplicate);
        case 'uncovered': return drafts.filter(d => draftFlags(d).uncovered);
        default: return drafts;
    }
}
