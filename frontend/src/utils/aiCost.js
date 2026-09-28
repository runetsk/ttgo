// Pure money and soft-budget helpers for AI cost. No React, no network.
// Consumers: api.js (budget confirmation), analysisJob.js, aiSettings/BudgetSettings.jsx,
// analytics/AIGenerationPanel.jsx

// usd shows cents, and four decimals for a non-zero amount under a cent (one analysis is often
// a fraction of a cent).
export function usd(n) {
    const v = Number(n) || 0;
    return v > 0 && v < 0.01 ? `$${v.toFixed(4)}` : `$${v.toFixed(2)}`;
}

// isBudgetConflict: the server's soft-budget 409 ({ category: 'budget', scope, ... }).
export function isBudgetConflict(err) {
    return err?.response?.status === 409 && err.response.data?.category === 'budget';
}

// budgetConfirmText turns the 409 budget payload into the question asked before resending. The server's estimate is worst case (every attempt, hedges included), and the question says so.
export function budgetConfirmText(data) {
    const est = usd(data?.estimated_cost_usd);
    if (data?.scope === 'month') {
        return `This analysis (worst case ~${est}) would exceed the monthly AI budget (${usd(data.month_spent_usd)} of ${usd(data.budget_usd)} spent). Run it anyway?`;
    }
    return `This analysis could cost up to ~${est} (worst case), above the per-request AI budget of ${usd(data?.budget_usd)}. Run it anyway?`;
}

// withBudgetConfirm sends once; on a soft-budget 409 it asks confirmFn and, if confirmed,
// resends with the acknowledgement. send(acknowledged) performs the request. A declined
// question rethrows the 409 marked budgetDeclined so callers can stay quiet.
// acknowledged=true (the user already agreed) skips straight to the acknowledged send.
export async function withBudgetConfirm(send, confirmFn, acknowledged = false) {
    if (acknowledged) return send(true);
    try {
        return await send(false);
    } catch (err) {
        if (!isBudgetConflict(err)) throw err;
        if (!confirmFn(budgetConfirmText(err.response.data))) {
            err.budgetDeclined = true;
            throw err;
        }
        return send(true);
    }
}

// spendSplit: the month's estimated spend by source for the budget card, or null when the
// server does not report the split.
export function spendSplit(cfg) {
    const gen = cfg?.month_spent_generation_usd;
    const analysis = cfg?.month_spent_analysis_usd;
    if (gen == null && analysis == null) return null;
    return `${usd(gen)} test generation · ${usd(analysis)} failure analysis`;
}

// analysisCostLine: the Analytics AI section's failure-analysis cost for the report window,
// or null when the window has no cost events.
export function analysisCostLine(a) {
    if (!a || !a.events) return null;
    let inner = `${usd(a.typesafe_cost_usd)} TypeSafe · ${usd(a.llm_cost_usd)} LLM`;
    if (a.explain_cost_usd > 0) inner += `, ${usd(a.explain_cost_usd)} of it explanations`;
    return `Failure analysis: ${usd(a.cost_usd)} (${inner}) over ${a.events} ${a.events === 1 ? 'call' : 'calls'}`;
}
