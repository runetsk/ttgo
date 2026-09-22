// Pure text derivations for the AI analysis card and verdict badge. No React, no network.
// Consumers: AIVerdictBadge.jsx, RunResultDetail.jsx
import { suggestionLabel } from './defectSuggestion.js';

const fmt = (n) => (Number.isFinite(n) ? n.toFixed(2) : null);

export function badgeTitle(analysis) {
    if (analysis?.engine !== 'typesafe') return undefined;
    const parts = ['TypeSafe'];
    if (analysis.model_name) parts.push(analysis.model_name);
    const c = fmt(analysis.confidence_score);
    return `${parts.join(' ')}${c ? ` · confidence ${c}` : ''}`;
}

export function analysisMetaParts(analysis) {
    if (!analysis) return [];
    const isTS = analysis.engine === 'typesafe';
    const parts = [isTS ? 'Engine: TypeSafe' : 'Engine: LLM'];
    if (analysis.model_name) parts.push(analysis.model_name);
    if (!isTS) return parts;
    const c = fmt(analysis.confidence_score);
    if (c) parts.push(`confidence ${c}`);
    if (analysis.suggested_defect_type) {
        const dc = fmt(analysis.suggested_defect_type_confidence);
        const derived = analysis.suggestion_source === 'verdict';
        const detail = [dc, derived ? 'from the verdict' : null].filter(Boolean).join(', ');
        parts.push(`suggested defect type: ${suggestionLabel(analysis.suggested_defect_type)}${detail ? ` (${detail})` : ''}`);
    } else {
        parts.push('no defect type suggested');
    }
    return parts;
}

export function narrativeNotice(analysis) {
    switch (analysis?.narrative_status) {
        case 'unavailable': return 'The explanation could not be generated; the classification above still stands.';
        case 'unparseable': return 'The model returned an unreadable explanation; the raw text is under Rationale.';
        default: return null;
    }
}

export function groupingNote(analysis) {
    if (!analysis?.dedup_group_key) return null;
    if (analysis.dedup_method === 'semantic') {
        const p = fmt(analysis.dedup_p_same);
        return p ? `Grouped semantically (p = ${p})` : 'Grouped semantically';
    }
    return 'Grouped with an identical failure';
}
