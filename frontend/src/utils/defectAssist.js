// Pure text for TypeSafe.ai defect assist (TypeSafe backlog Wave 5): the severity suggestion and
// the possible duplicates POST /defects/assist returns. No React, no network.
// Consumer: components/DefectAssistPanel.jsx

const pct = (p) => `${Math.round((p || 0) * 100)}%`;

// severityText: "Suggested severity: major (84%)", or null without a suggestion.
export function severityText(assist) {
    const s = assist?.severity;
    if (!s?.value) return null;
    return `Suggested severity: ${s.value} (${pct(s.confidence)})`;
}

// canApplySeverity: the suggestion differs from what the form holds.
export function canApplySeverity(assist, current) {
    const v = assist?.severity?.value;
    return !!v && v !== current;
}

// duplicateLabel: "PAY-42 · Checkout total wrong (91% likely the same problem)".
export function duplicateLabel(d) {
    const head = d?.external_key ? `${d.external_key} · ` : '';
    return `${head}${d?.title || 'Untitled defect'} (${pct(d?.p_same)} likely the same problem)`;
}

// duplicatesSummary: the line above the list, or the all-clear.
export function duplicatesSummary(assist) {
    const n = assist?.duplicates?.length || 0;
    if (n === 0) return 'No open defect looks like the same problem.';
    return n === 1 ? 'This may duplicate an open defect:' : `This may duplicate ${n} open defects:`;
}

// defectLink opens a defect on the Defects page.
export const defectLink = (id) => `/defects?focus=${encodeURIComponent(id)}`;
