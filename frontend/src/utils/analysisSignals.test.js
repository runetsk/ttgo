import test from 'node:test';
import assert from 'node:assert/strict';
import {
    SIGNAL_MIN, INJECTION_MIN, TRANSFER_FIT_MIN, INJECTION_LABEL, INJECTION_CONFIRM, FIT_NOTE, SPLIT_PENDING_NOTE, SPLIT_FAILED_NOTE,
    parseSignals, signalChips, isInjectionFlagged, fitNote, explainRequest, explainParams, isInjectionConflict, withInjectionConfirm,
} from './analysisSignals.js';

test('thresholds match the server', () => {
    assert.equal(SIGNAL_MIN, 0.8);
    assert.equal(INJECTION_MIN, 0.8);
    assert.equal(TRANSFER_FIT_MIN, 0.5);
});

test('parseSignals reads the stored JSON text, an object, or nothing', () => {
    assert.deepEqual(parseSignals('{"injection":0.03,"recurring":0.97}'), { injection: 0.03, recurring: 0.97 });
    assert.deepEqual(parseSignals({ outside_app: 0.9 }), { outside_app: 0.9 });
    assert.deepEqual(parseSignals(''), {});
    assert.deepEqual(parseSignals(null), {});
    assert.deepEqual(parseSignals('not json'), {});
    assert.deepEqual(parseSignals('[1,2]'), {});
    assert.deepEqual(parseSignals(42), {});
});

test('signalChips shows each answer at or above 0.80, the injection flag first', () => {
    const chips = signalChips({ signals: JSON.stringify({
        injection: 0.91, flaky_history: 0.91, recurring: 0.8, outside_app: 0.79,
        known_defect: { key: 'PAY-42', confidence: 0.88 },
    }) });
    assert.deepEqual(chips.map((c) => [c.key, c.label, c.tone]), [
        ['injection', 'Possible prompt injection — review the raw failure', 'danger'],
        ['flaky_history', 'Flaky pattern in history', 'warn'],
        ['recurring', 'Seen before', 'info'],
        ['known_defect', 'Matches PAY-42', 'info'],
    ]);
    assert.equal(INJECTION_LABEL, chips[0].label);
    assert.match(chips[1].title, /91%/);
    assert.match(chips[3].title, /PAY-42.*88%/);
});

test('signalChips leaves out weak answers, "none", unasked questions and junk', () => {
    assert.deepEqual(signalChips({ signals: '{"injection":0.79,"known_defect":{"key":"PAY-1","confidence":0.7}}' }), []);
    assert.deepEqual(signalChips({ signals: '{"known_defect":{"key":"none","confidence":0.99}}' }), []);
    assert.deepEqual(signalChips({ signals: '{"known_defect":{"key":"","confidence":0.99}}' }), []);
    assert.deepEqual(signalChips({ signals: '{"recurring":"0.9"}' }), [], 'a string is not a probability');
    assert.deepEqual(signalChips({ signals: '{"outside_app":0.95}' }).map((c) => c.label), ['Outside the app']);
    assert.deepEqual(signalChips({}), []);
    assert.deepEqual(signalChips(null), []);
});

test('isInjectionFlagged trips at 0.80', () => {
    assert.equal(isInjectionFlagged({ signals: '{"injection":0.8}' }), true);
    assert.equal(isInjectionFlagged({ signals: '{"injection":0.7999}' }), false);
    assert.equal(isInjectionFlagged({ signals: { injection: 0.95 } }), true);
    assert.equal(isInjectionFlagged({ signals: '' }), false);
    assert.equal(isInjectionFlagged(undefined), false);
});

test('fitNote flags a clone whose copied explanation may not fit it', () => {
    const clone = { source_analysis_id: 'rep', narrative_fit: 0.31, narrative_split: false, narrative_status: 'ok' };
    const note = fitNote(clone);
    assert.equal(note.state, 'mismatch');
    assert.equal(note.text, FIT_NOTE);
    assert.equal(FIT_NOTE, 'This explanation may not apply to this result');
    assert.match(note.detail, /31%/);
    assert.equal(note.canExplain, true);
    assert.equal(note.action, 'Explain this result');
    assert.equal(fitNote({ ...clone, narrative_status: 'pending' }).canExplain, false, 'not while an explanation is being written');
    assert.equal(fitNote({ ...clone, narrative_fit: 0.5 }), null, '0.50 fits');
    assert.equal(fitNote({ ...clone, narrative_fit: null }), null, 'unchecked');
    assert.equal(fitNote({ ...clone, source_analysis_id: null }), null, 'representatives are never checked');
    assert.equal(fitNote(null), null);
});

test('fitNote says when a clone was explained on its own, is being explained, or failed and can be retried', () => {
    const base = { source_analysis_id: 'rep', narrative_fit: 0.31, narrative_split: true };
    const split = fitNote({ ...base, narrative_status: 'ok' });
    assert.equal(split.state, 'split');
    assert.match(split.text, /own/);
    assert.equal(split.canExplain, false);
    const writing = fitNote({ ...base, narrative_status: 'pending' }); // the claim sets narrative_split
    assert.equal(writing.state, 'split');
    assert.equal(writing.text, SPLIT_PENDING_NOTE);
    assert.equal(writing.canExplain, false);
    for (const status of ['unavailable', 'unparseable']) {
        const failed = fitNote({ ...base, narrative_status: status });
        assert.equal(failed.state, 'split-failed', status);
        assert.equal(failed.text, SPLIT_FAILED_NOTE);
        assert.equal(failed.canExplain, true, 'R9: a failed own explanation can be retried');
        assert.equal(failed.action, 'Retry explanation');
    }
});

test('isInjectionConflict recognises the Explain 409 that asks for the override', () => {
    const err = (status, error) => ({ response: { status, data: { error } } });
    assert.equal(isInjectionConflict(err(409, 'possible prompt injection: confirm to send this failure to the LLM')), true);
    assert.equal(isInjectionConflict(err(409, 'already being explained')), false);
    assert.equal(isInjectionConflict(err(500, 'possible prompt injection: confirm to send this failure to the LLM')), false);
    assert.equal(isInjectionConflict({}), false);
    assert.equal(isInjectionConflict(undefined), false);
});

test('withInjectionConfirm asks on the 409 and resends once with the override', async () => {
    const conflict = { response: { status: 409, data: { error: 'possible prompt injection: confirm to send this failure to the LLM' } } };
    const sent = [];
    const send = async (opts) => {
        sent.push(opts);
        if (!opts.overrideInjection) throw conflict;
        return 'row';
    };
    const asked = [];
    assert.equal(await withInjectionConfirm(send, { scope: 'result' }, (m) => { asked.push(m); return true; }), 'row');
    assert.deepEqual(sent, [{ scope: 'result' }, { scope: 'result', overrideInjection: true }]);
    assert.deepEqual(asked, [INJECTION_CONFIRM]);

    sent.length = 0;
    await assert.rejects(withInjectionConfirm(send, { scope: 'result' }, () => false), (e) => e === conflict && e.injectionDeclined === true);
    assert.equal(sent.length, 1, 'declined: nothing is resent');

    const other = { response: { status: 409, data: { error: 'already being explained' } } };
    await assert.rejects(withInjectionConfirm(async () => { throw other; }, {}, () => true), (e) => e === other && !e.injectionDeclined);
    // A request that already carried the override is never asked about again.
    let calls = 0;
    await assert.rejects(withInjectionConfirm(async () => { calls++; throw conflict; }, { overrideInjection: true }, () => true));
    assert.equal(calls, 1);
});

test('explainRequest asks before sending a flagged failure, and only then overrides', () => {
    const asked = [];
    const yes = (m) => { asked.push(m); return true; };
    const no = (m) => { asked.push(m); return false; };
    const flagged = { signals: '{"injection":0.93}' };
    assert.deepEqual(explainRequest(flagged, {}, yes), { scope: '', overrideInjection: true });
    assert.equal(explainRequest(flagged, {}, no), null, 'declined: nothing is sent');
    assert.deepEqual(asked, [INJECTION_CONFIRM, INJECTION_CONFIRM]);
    assert.deepEqual(explainRequest({ signals: '{"injection":0.1}' }, { scope: 'result' }, no), { scope: 'result', overrideInjection: false });
    assert.equal(asked.length, 2, 'an unflagged row is not asked about');
    assert.match(INJECTION_CONFIRM, /possible prompt injection/);
});

test('explainParams builds the Explain query', () => {
    assert.deepEqual(explainParams({}), {});
    assert.deepEqual(explainParams(), {});
    assert.deepEqual(explainParams({ scope: 'result' }), { scope: 'result' });
    assert.deepEqual(explainParams({ overrideInjection: true }), { override_injection: true });
    assert.deepEqual(explainParams({ scope: 'result', overrideInjection: false }), { scope: 'result' });
});
