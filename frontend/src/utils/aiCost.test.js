import test from 'node:test';
import assert from 'node:assert/strict';
import { usd, isBudgetConflict, budgetConfirmText, withBudgetConfirm, spendSplit, analysisCostLine } from './aiCost.js';

const budget409 = (data) => {
    const err = new Error('Request failed with status code 409');
    err.response = { status: 409, data: { category: 'budget', ...data } };
    return err;
};

test('usd shows cents, and four decimals under a cent', () => {
    assert.equal(usd(12.5), '$12.50');
    assert.equal(usd(0), '$0.00');
    assert.equal(usd(0.00067), '$0.0007');
    assert.equal(usd(undefined), '$0.00');
});

test('isBudgetConflict only matches a 409 budget payload', () => {
    assert.equal(isBudgetConflict(budget409({ scope: 'month' })), true);
    assert.equal(isBudgetConflict({ response: { status: 409, data: { error: 'analysis already running' } } }), false);
    assert.equal(isBudgetConflict({ response: { status: 500, data: { category: 'budget' } } }), false);
    assert.equal(isBudgetConflict(null), false);
});

test('budgetConfirmText names the budget that would be exceeded', () => {
    assert.equal(budgetConfirmText({ scope: 'month', estimated_cost_usd: 0.42, month_spent_usd: 9.8, budget_usd: 10 }),
        'This analysis (~$0.42) would exceed the monthly AI budget ($9.80 of $10.00 spent). Run it anyway?');
    assert.equal(budgetConfirmText({ scope: 'request', estimated_cost_usd: 0.08, budget_usd: 0.05 }),
        'This analysis is estimated at ~$0.08, above the per-request AI budget of $0.05. Run it anyway?');
});

test('withBudgetConfirm: asks once and resends acknowledged', async () => {
    const sent = [];
    const send = async (ack) => {
        sent.push(ack);
        if (!ack) throw budget409({ scope: 'month', estimated_cost_usd: 1, month_spent_usd: 1, budget_usd: 1 });
        return 'ok';
    };
    const asked = [];
    assert.equal(await withBudgetConfirm(send, (msg) => { asked.push(msg); return true; }), 'ok');
    assert.deepEqual(sent, [false, true]);
    assert.equal(asked.length, 1);
});

test('withBudgetConfirm: a declined question rethrows the 409 marked budgetDeclined', async () => {
    const send = async () => { throw budget409({ scope: 'request', estimated_cost_usd: 1, budget_usd: 0.5 }); };
    await assert.rejects(withBudgetConfirm(send, () => false), (err) => err.budgetDeclined === true);
});

test('withBudgetConfirm: other errors pass through, and acknowledged skips the first try', async () => {
    const boom = new Error('boom');
    await assert.rejects(withBudgetConfirm(async () => { throw boom; }, () => true), boom);
    const sent = [];
    await withBudgetConfirm(async (ack) => { sent.push(ack); return 'ok'; }, () => true, true);
    assert.deepEqual(sent, [true]);
});

test('spendSplit and analysisCostLine', () => {
    assert.equal(spendSplit({ month_spent_generation_usd: 1.25, month_spent_analysis_usd: 0.5 }), '$1.25 test generation · $0.50 failure analysis');
    assert.equal(spendSplit({ month_spent_usd: 3 }), null, 'an older server without the split');
    assert.equal(analysisCostLine({ events: 0 }), null);
    assert.equal(analysisCostLine(undefined), null);
    assert.equal(analysisCostLine({ cost_usd: 0.36, typesafe_cost_usd: 0.11, llm_cost_usd: 0.25, explain_cost_usd: 0.05, events: 5 }),
        'Failure analysis: $0.36 ($0.11 TypeSafe · $0.25 LLM, $0.05 of it explanations) over 5 calls');
    assert.equal(analysisCostLine({ cost_usd: 0.1, typesafe_cost_usd: 0.1, llm_cost_usd: 0, explain_cost_usd: 0, events: 1 }),
        'Failure analysis: $0.10 ($0.10 TypeSafe · $0.00 LLM) over 1 call');
});
