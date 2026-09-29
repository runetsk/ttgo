// Pure helpers for the auto-apply part of the AI Failure Analysis card: the minimum-confidence
// range, when auto-apply may be switched on, and the accuracy-gate status line. No React, no
// network. Ranges and gate constants match the server (the failure-analysis settings handler and
// failureanalysis.AutoApplyGateMinGraded / AutoApplyGateMinAccuracy / AutoApplyGateWindowDays).
// Consumers: utils/failureAnalysisSettings.js, components/AIFailureAnalysisSettings.jsx

export const AUTO_APPLY_MIN_CONFIDENCE_MIN = 80;
export const AUTO_APPLY_MIN_CONFIDENCE_MAX = 99;
export const AUTO_APPLY_DEFAULTS = { auto_apply_defect_type: false, auto_apply_min_confidence: 95 };
export const GATE_MIN_GRADED = 50;
export const GATE_MIN_ACCURACY = 0.95;
export const GATE_WINDOW_DAYS = 90;

const wholeIn = (n, min, max) => Number.isInteger(n) && n >= min && n <= max;
const joinList = (xs) => (xs.length <= 1 ? xs.join('') : `${xs.slice(0, -1).join(', ')} and ${xs[xs.length - 1]}`);
const needPct = Math.round(GATE_MIN_ACCURACY * 100);

// minConfidenceError returns the message for a threshold the server would reject, else undefined.
export function minConfidenceError(value) {
    return wholeIn(value, AUTO_APPLY_MIN_CONFIDENCE_MIN, AUTO_APPLY_MIN_CONFIDENCE_MAX)
        ? undefined
        : `Enter a whole number from ${AUTO_APPLY_MIN_CONFIDENCE_MIN} to ${AUTO_APPLY_MIN_CONFIDENCE_MAX} (percent).`;
}

// gatePct: the threshold the gate is checked at. It is the draft's when valid, else the saved one,
// else the default; null before the settings load.
export function gatePct(draft, saved) {
    if (!draft) return null;
    if (!minConfidenceError(draft.auto_apply_min_confidence)) return draft.auto_apply_min_confidence;
    if (!minConfidenceError(saved?.auto_apply_min_confidence)) return saved.auto_apply_min_confidence;
    return AUTO_APPLY_DEFAULTS.auto_apply_min_confidence;
}

const switchingOn = (draft, saved) => !!draft?.auto_apply_defect_type && !saved?.auto_apply_defect_type;

// autoApplyToggleLocked: the switch cannot be turned on until the gate is known to be open (the
// server refuses that change while it is closed). Turning it off is always possible.
export function autoApplyToggleLocked(draft, saved, gate) {
    return !draft?.auto_apply_defect_type && !saved?.auto_apply_defect_type && gate?.open !== true;
}

// autoApplyErrors: a draft that switches auto-apply on while the gate measured at its threshold is
// closed cannot be saved. Staying on is fine: each job then pauses auto-apply by itself.
export function autoApplyErrors(draft, saved, gate) {
    if (switchingOn(draft, saved) && gate && !gate.open) {
        return { auto_apply_defect_type: 'The accuracy gate is closed at this minimum confidence, so auto-apply cannot be switched on yet.' };
    }
    return {};
}

// thresholdPct reads the gate's threshold: 0..1 from the server, a whole percent tolerated.
function thresholdPct(gate) {
    const m = Number(gate?.min_confidence);
    if (!Number.isFinite(m) || m <= 0) return null;
    return Math.round(m <= 1 ? m * 100 : m);
}

// gateText is the one-line gate status, e.g. "98.2 % of 214 rows at ≥ 95 % — gate open".
export function gateText(gate) {
    if (!gate) return 'Checking the accuracy gate…';
    const graded = gate.graded || 0;
    const t = thresholdPct(gate);
    const at = t !== null ? ` at ≥ ${t} %` : '';
    if (graded === 0) return `No graded rows${at} yet (${GATE_MIN_GRADED} needed) — gate closed`;
    const acc = Number.isFinite(gate.accuracy) ? gate.accuracy : (gate.agreed || 0) / graded;
    const head = `${(acc * 100).toFixed(1)} % of ${graded} rows${at}`;
    if (gate.open) return `${head} — gate open`;
    if (graded < GATE_MIN_GRADED) return `${head} (${GATE_MIN_GRADED} rows needed) — gate closed`;
    return `${head} (${needPct} % needed) — gate closed`;
}

// gateScope says what the gate figures cover: only the current policy family (R4), and no
// confirmations of labels auto-apply wrote (R10: a Confirm is not an independent judgement).
// null before the gate loads.
export function gateScope(gate) {
    if (!gate) return null;
    const policies = Array.isArray(gate.policies) && gate.policies.length > 0 ? `, ${joinList(gate.policies)} decisions only` : '';
    return `People's triage of TypeSafe.ai's own defect-type suggestions over the last ${GATE_WINDOW_DAYS} days${policies}; confirmations of labels auto-apply set do not count. Opens at ${GATE_MIN_GRADED} rows and ${needPct} % agreement.`;
}
