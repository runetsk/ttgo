// Pure helpers for what TypeSafe.ai answered next to its verdict (the companion questions and the
// prompt-injection guard, stored as the analysis row's `signals`) and for the narrative transfer
// check on grouped results (`narrative_fit`, `narrative_split`). No React, no network.
// Thresholds match the server: SignalMin / InjectionMin in
// backend/pkg/tracker/failureanalysis/typesafe_questions.go and TransferFitMin in failureanalysis.
// Consumers: analysisMeta.js, api.js, components/RunResultDetail.jsx, components/AIVerdictBadge.jsx

export const SIGNAL_MIN = 0.8;
export const INJECTION_MIN = 0.8;
export const TRANSFER_FIT_MIN = 0.5;

export const INJECTION_LABEL = 'Possible prompt injection — review the raw failure';
export const INJECTION_NOTICE = 'No explanation was written: TypeSafe.ai found text in this failure that reads like instructions to an AI (possible prompt injection). The classification above still stands. Review the raw failure before sending it to the LLM.';
export const INJECTION_CONFIRM = 'This failure may contain instructions aimed at an AI (possible prompt injection). Send it to the LLM for an explanation anyway?';
export const FIT_NOTE = 'This explanation may not apply to this result';
export const SPLIT_NOTE = "Explained on its own: this explanation was written from this result's own failure, not its group's.";
export const SPLIT_PENDING_NOTE = 'This result is being explained on its own, from its own failure.';
export const SPLIT_FAILED_NOTE = "The explanation for this result alone could not be written; the group's explanation above may not apply.";

// The Explain 409 that asks for override_injection=true (spec §1.4, R9).
const INJECTION_CONFLICT = /possible prompt injection/i;

const num = (v) => (typeof v === 'number' && Number.isFinite(v) ? v : null);
const pct = (p) => `${Math.round(p * 100)}%`;

// parseSignals reads the stored signals: JSON text on REST rows and live events (an object is
// accepted too). Anything unreadable reads as "nothing was asked".
export function parseSignals(raw) {
    if (raw && typeof raw === 'object' && !Array.isArray(raw)) return raw;
    if (typeof raw !== 'string' || raw === '') return {};
    try {
        const v = JSON.parse(raw);
        return v && typeof v === 'object' && !Array.isArray(v) ? v : {};
    } catch {
        return {};
    }
}

// isInjectionFlagged: TypeSafe.ai judged the failure text (or its group's related failures) likely
// to contain instructions aimed at an AI, so the worker wrote no explanation for the group.
export function isInjectionFlagged(analysis) {
    const p = num(parseSignals(analysis?.signals).injection);
    return p !== null && p >= INJECTION_MIN;
}

// The yes/no companion questions, in chip order: [key, label, tone, what a "yes" means].
const COMPANIONS = [
    ['flaky_history', 'Flaky pattern in history', 'warn', "This test's recent outcomes alternate between passing and failing"],
    ['recurring', 'Seen before', 'info', 'This test failed with the same error condition in earlier runs'],
    ['outside_app', 'Outside the app', 'info', 'The error originates outside the application under test (network, CI runner, third-party service)'],
];

// signalChips lists the chips a result shows: the injection flag first, then every companion answer
// of SIGNAL_MIN or more. A question that was not asked has no key and shows nothing.
export function signalChips(analysis) {
    const s = parseSignals(analysis?.signals);
    const chips = [];
    const inj = num(s.injection);
    if (inj !== null && inj >= INJECTION_MIN) {
        chips.push({
            key: 'injection', label: INJECTION_LABEL, tone: 'danger',
            title: `TypeSafe.ai is ${pct(inj)} sure this failure contains instructions aimed at an AI. No explanation was written for its group.`,
        });
    }
    for (const [key, label, tone, what] of COMPANIONS) {
        const p = num(s[key]);
        if (p !== null && p >= SIGNAL_MIN) chips.push({ key, label, tone, title: `${what} — TypeSafe.ai ${pct(p)}` });
    }
    const kd = s.known_defect;
    const kc = num(kd?.confidence);
    if (kd && typeof kd.key === 'string' && kd.key !== '' && kd.key !== 'none' && kc !== null && kc >= SIGNAL_MIN) {
        chips.push({
            key: 'known_defect', label: `Matches ${kd.key}`, tone: 'info',
            title: `Linked defect ${kd.key} describes this same error — TypeSafe.ai ${pct(kc)}`,
        });
    }
    return chips;
}

// fitNote: what a grouped result says about the explanation copied from its group. After the
// group's explanation is written, TypeSafe.ai rates how well it fits each semantic clone
// (narrative_fit); below TRANSFER_FIT_MIN the clone is marked and can be explained on its own
// (Explain ?scope=result). The claim sets narrative_split, so a split row is being explained
// (pending), explained, or failed; a failed one can be retried with scope=result (R9).
// `action` names the button; null when there is nothing to say.
export function fitNote(analysis) {
    if (!analysis?.source_analysis_id) return null;
    if (analysis.narrative_split) {
        const status = analysis.narrative_status;
        if (status === 'unavailable' || status === 'unparseable') {
            return { state: 'split-failed', text: SPLIT_FAILED_NOTE, canExplain: true, action: 'Retry explanation' };
        }
        return { state: 'split', text: status === 'pending' ? SPLIT_PENDING_NOTE : SPLIT_NOTE, canExplain: false };
    }
    const fit = num(analysis.narrative_fit);
    if (fit === null || fit >= TRANSFER_FIT_MIN) return null;
    return {
        state: 'mismatch', text: FIT_NOTE,
        detail: `TypeSafe.ai rated the group's explanation ${pct(fit)} likely to describe this result's failure.`,
        canExplain: analysis.narrative_status !== 'pending', action: 'Explain this result',
    };
}

// isInjectionConflict: the server refused Explain until the user confirms sending a failure that
// may contain instructions aimed at an AI (409, override_injection=true required).
export function isInjectionConflict(err) {
    return err?.response?.status === 409 && INJECTION_CONFLICT.test(String(err.response.data?.error || ''));
}

// withInjectionConfirm sends once with `opts`; on the injection 409 it asks confirmFn and, on yes,
// resends once with overrideInjection. A declined question rethrows the 409 marked
// injectionDeclined so callers stay quiet. A request that already carried the override is never
// asked about again. Covers rows the page did not know were flagged: Explain this result checks
// the clone's own evidence first, and requires the override when TypeSafe is unavailable (R9).
export async function withInjectionConfirm(send, opts = {}, confirmFn = () => false) {
    try {
        return await send(opts);
    } catch (err) {
        if (opts.overrideInjection || !isInjectionConflict(err)) throw err;
        if (!confirmFn(INJECTION_CONFIRM)) {
            err.injectionDeclined = true;
            throw err;
        }
        return send({ ...opts, overrideInjection: true });
    }
}

// explainRequest decides what an Explain click sends. On a row flagged for possible prompt
// injection it asks confirmFn(INJECTION_CONFIRM) first and sends the override only on yes;
// null means the user declined and nothing is sent.
export function explainRequest(analysis, { scope = '' } = {}, confirmFn = () => false) {
    if (!isInjectionFlagged(analysis)) return { scope, overrideInjection: false };
    if (!confirmFn(INJECTION_CONFIRM)) return null;
    return { scope, overrideInjection: true };
}

// explainParams is the Explain query for an explainRequest result.
export function explainParams({ scope = '', overrideInjection = false } = {}) {
    return { ...(scope ? { scope } : {}), ...(overrideInjection ? { override_injection: true } : {}) };
}
