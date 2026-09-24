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

// isFailedAnalysis: the attempt produced no decision. Rows stored before decision_status existed
// are recognised by their summary.
export function isFailedAnalysis(analysis) {
    if (!analysis) return false;
    if (analysis.decision_status) return analysis.decision_status === 'failed';
    return typeof analysis.summary === 'string' && /^analysis failed\s*:/i.test(analysis.summary);
}

const ERROR_LABELS = {
    timeout: 'timed out',
    rate_limit: 'rate limited',
    truncated: 'reply cut off twice',
    unparseable: 'unreadable reply twice',
    provider: 'provider error',
    network: 'network error',
    authentication: 'authentication failed',
    authorization: 'not authorized',
    configuration: 'TypeSafe.ai key missing',
};

export function failureHeading(analysis) {
    const c = analysis?.error_category;
    return c ? `Analysis failed · ${ERROR_LABELS[c] || c.replace(/_/g, ' ')}` : 'Analysis failed';
}

export function failureMessage(analysis) {
    return (analysis?.summary || '').replace(/^analysis failed\s*:\s*/i, '');
}

// explainAction names the button that asks for the missing explanation of a stored TypeSafe
// decision, or null when there is nothing to explain.
export function explainAction(analysis) {
    if (analysis?.engine !== 'typesafe' || isFailedAnalysis(analysis)) return null;
    switch (analysis.narrative_status) {
        case 'skipped': return 'Explain';
        case 'unavailable':
        case 'unparseable': return 'Retry explanation';
        default: return null;
    }
}

// shouldReplaceAnalysis decides whether a live analysis event replaces the one on screen for a
// result. Explain fills in an existing version in place and broadcasts it, and it can be run on
// an older version, so an event for a lower version than the one shown is ignored; the same
// version (an explanation filled in) or a newer one replaces it.
export function shouldReplaceAnalysis(current, incoming) {
    if (!current) return true;
    return (incoming?.version ?? 0) >= (current?.version ?? 0);
}

// takeoverNote says what TypeSafe decided when the LLM took over below the threshold.
export function takeoverNote(analysis) {
    if (!analysis?.takeover_from_verdict) return null;
    const c = fmt(analysis.takeover_from_confidence);
    return `TypeSafe said ${analysis.takeover_from_verdict.replace(/_/g, ' ')}${c ? ` with confidence ${c}` : ''}, below the takeover threshold, so the LLM decided.`;
}

export function narrativeNotice(analysis) {
    if (isFailedAnalysis(analysis)) return null;
    switch (analysis?.narrative_status) {
        case 'unavailable': return 'The explanation could not be generated; the classification above still stands.';
        case 'unparseable': return 'The model returned an unreadable explanation; the raw text is under Rationale.';
        case 'skipped': return 'No explanation was written because explanations are switched off in the TypeSafe.ai settings. The classification above is TypeSafe\'s decision.';
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
