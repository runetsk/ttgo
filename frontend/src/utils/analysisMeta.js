// Pure text derivations for the AI analysis card and verdict badge. No React, no network.
// Consumers: AIVerdictBadge.jsx, RunResultDetail.jsx, TestRunDetail.jsx (signals: analysisSignals.js)
import { suggestionLabel } from './defectSuggestion.js';
import { signalChips, isInjectionFlagged, INJECTION_NOTICE } from './analysisSignals.js';

const fmt = (n) => (Number.isFinite(n) ? n.toFixed(2) : null);

// PENDING_EXPLANATION is what a TypeSafe decision shows while its explanation is still being
// written (the worker narrates after publishing the decision; Explain claims the row while it runs).
export const PENDING_EXPLANATION = 'Explanation being written…';

export function isPendingNarrative(analysis) {
    return analysis?.narrative_status === 'pending' && !isFailedAnalysis(analysis);
}

// badgeTitle is the verdict badge's tooltip: the engine, model and confidence of a TypeSafe
// decision, then the chips its companion answers earned, then the pending note.
export function badgeTitle(analysis) {
    const extras = [
        ...signalChips(analysis).map((c) => c.label),
        ...(isPendingNarrative(analysis) ? [PENDING_EXPLANATION] : []),
    ];
    if (analysis?.engine !== 'typesafe') return extras.length ? extras.join(' · ') : undefined;
    const parts = ['TypeSafe'];
    if (analysis.model_name) parts.push(analysis.model_name);
    const c = fmt(analysis.confidence_score);
    const base = `${parts.join(' ')}${c ? ` · confidence ${c}` : ''}`;
    return [base, ...extras].join(' · ');
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
    configuration: 'settings problem',
};

export function failureHeading(analysis) {
    const c = analysis?.error_category;
    return c ? `Analysis failed · ${ERROR_LABELS[c] || c.replace(/_/g, ' ')}` : 'Analysis failed';
}

export function failureMessage(analysis) {
    return (analysis?.summary || '').replace(/^analysis failed\s*:\s*/i, '');
}

// failureAdvice is the closing line of a failed-attempt card. A settings problem (missing or
// undecryptable key, unknown model id: error_category "configuration") fails again on every
// retry, so it points at the settings of the engine that failed instead of at Re-analyze.
export function failureAdvice(analysis) {
    if (analysis?.error_category !== 'configuration') return 'No decision was made. Re-analyze to try again.';
    const where = analysis.engine === 'generative'
        ? 'the LLM provider settings (model, key)'
        : 'the TypeSafe.ai settings (model id, key)';
    return `No decision was made. Check ${where}, then Re-analyze.`;
}

// explainAction names the button that asks for the missing explanation of a stored TypeSafe
// decision, or null when there is nothing to explain. A row flagged for possible prompt injection
// reads "Explain anyway…": the click asks for confirmation first (analysisSignals.explainRequest).
// A clone explained on its own (narrative_split) is retried from its fit note with scope=result,
// never with a group-scope Explain, so it gets no action here.
export function explainAction(analysis) {
    if (analysis?.engine !== 'typesafe' || isFailedAnalysis(analysis) || analysis.narrative_split) return null;
    let action;
    switch (analysis.narrative_status) {
        case 'skipped': action = 'Explain'; break;
        case 'unavailable':
        case 'unparseable': action = 'Retry explanation'; break;
        default: return null;
    }
    return isInjectionFlagged(analysis) ? 'Explain anyway…' : action;
}

// mergeAnalysis decides which copy of a result's analysis to show when a live event or a REST
// refresh brings another one. A newer version wins. Within one version, the later explanation
// wins: the incoming copy replaces the shown one only if its narrative_revision is at least as
// high, so a late `pending` never overwrites a written explanation. Explain can fill in an older
// version and broadcasts it; that event never rolls the row back. Two copies of the same analysis
// are merged, so a field the incoming copy lacks is kept. Returns `current` itself when nothing
// changes.
export function mergeAnalysis(current, incoming) {
    if (!incoming) return current;
    if (!current) return incoming;
    const cv = current.version ?? 0;
    const iv = incoming.version ?? 0;
    if (iv < cv) return current;
    if (iv === cv && (incoming.narrative_revision ?? 0) < (current.narrative_revision ?? 0)) return current;
    return incoming.id && incoming.id === current.id ? { ...current, ...incoming } : incoming;
}

// mergeAnalysisList applies one incoming analysis to a result's version list (newest first):
// the same analysis is merged in place with mergeAnalysis, and a new one is added in version
// order. Returns `list` itself when nothing changes.
export function mergeAnalysisList(list, incoming) {
    if (!incoming?.id) return list;
    const rows = Array.isArray(list) ? list : [];
    const i = rows.findIndex((r) => r.id === incoming.id);
    if (i >= 0) {
        const merged = mergeAnalysis(rows[i], incoming);
        if (merged === rows[i]) return list;
        const next = rows.slice();
        next[i] = merged;
        return next;
    }
    return [...rows, incoming].sort((a, b) => (b.version ?? 0) - (a.version ?? 0));
}

// Fields of the shared `run_result_analysis.created` / `.updated` payload that older servers did
// not send. They are copied only when present, so an event never blanks a field the row has.
const EVENT_OPTIONAL = [
    'summary', 'next_action', 'rationale', 'narrative_revision', 'source_analysis_id', 'created_at', 'policy_version', 'history_available',
    'signals', 'narrative_fit', 'narrative_split',
];

// analysisFromEvent turns a live analysis event's data into the row shape the REST endpoints
// return, or null when it names no result.
export function analysisFromEvent(d) {
    if (!d?.run_result_id) return null;
    const row = {
        id: d.analysis_id ?? d.id,
        run_result_id: d.run_result_id,
        version: d.version,
        verdict: d.verdict,
        suggested_defect_type: d.suggested_defect_type,
        suggested_defect_type_confidence: d.suggested_defect_type_confidence ?? null,
        suggestion_source: d.suggestion_source || '',
        confidence: d.confidence,
        confidence_score: d.confidence_score ?? null,
        engine: d.engine || 'generative',
        model_name: d.model_name || '',
        narrative_status: d.narrative_status || 'ok',
        decision_status: d.decision_status || 'ok',
        error_category: d.error_category || '',
        takeover_from_verdict: d.takeover_from_verdict || '',
        takeover_from_confidence: d.takeover_from_confidence ?? null,
        job_id: d.job_id || null,
        dedup_group_key: d.dedup_group_key || null,
        dedup_method: d.dedup_method || '',
        dedup_p_same: d.dedup_p_same ?? null,
    };
    for (const k of EVENT_OPTIONAL) if (d[k] !== undefined) row[k] = d[k];
    return row;
}

// takeoverNote says what TypeSafe decided when the LLM took over below the threshold.
export function takeoverNote(analysis) {
    if (!analysis?.takeover_from_verdict) return null;
    const c = fmt(analysis.takeover_from_confidence);
    return `TypeSafe said ${analysis.takeover_from_verdict.replace(/_/g, ' ')}${c ? ` with confidence ${c}` : ''}, below the takeover threshold, so the LLM decided.`;
}

export function narrativeNotice(analysis) {
    if (isFailedAnalysis(analysis)) return null;
    if (analysis?.narrative_status === 'unavailable' && isInjectionFlagged(analysis)) return INJECTION_NOTICE;
    switch (analysis?.narrative_status) {
        case 'pending': return PENDING_EXPLANATION;
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
