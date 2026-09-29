import test from 'node:test';
import assert from 'node:assert/strict';
import {
    AUTO_APPLY_MIN_CONFIDENCE_MIN, AUTO_APPLY_MIN_CONFIDENCE_MAX, AUTO_APPLY_DEFAULTS, GATE_MIN_GRADED, GATE_MIN_ACCURACY, GATE_WINDOW_DAYS,
    minConfidenceError, gatePct, autoApplyToggleLocked, autoApplyErrors, gateText, gateScope,
} from './autoApplySettings.js';

const OFF = { auto_apply_defect_type: false, auto_apply_min_confidence: 95 };
const ON = { ...OFF, auto_apply_defect_type: true };
const OPEN = { graded: 214, agreed: 210, accuracy: 210 / 214, open: true, policies: ['fa-verdict-v7', 'fa-verdict-v8'], min_confidence: 0.95 };
const CLOSED = { graded: 214, agreed: 190, accuracy: 190 / 214, open: false, policies: ['fa-verdict-v7'], min_confidence: 0.95 };

test('constants match the server', () => {
    assert.deepEqual([AUTO_APPLY_MIN_CONFIDENCE_MIN, AUTO_APPLY_MIN_CONFIDENCE_MAX], [80, 99]);
    assert.deepEqual(AUTO_APPLY_DEFAULTS, { auto_apply_defect_type: false, auto_apply_min_confidence: 95 });
    assert.deepEqual([GATE_MIN_GRADED, GATE_MIN_ACCURACY, GATE_WINDOW_DAYS], [50, 0.95, 90]);
});

test('the minimum confidence is a whole percent from 80 to 99', () => {
    for (const ok of [80, 95, 99]) assert.equal(minConfidenceError(ok), undefined, String(ok));
    for (const bad of [79, 100, 95.5, NaN, undefined, 0.95]) assert.match(minConfidenceError(bad), /from 80 to 99/, String(bad));
});

test('gatePct checks the draft threshold when it is valid, else the saved one', () => {
    assert.equal(gatePct({ ...OFF, auto_apply_min_confidence: 90 }, OFF), 90);
    assert.equal(gatePct({ ...OFF, auto_apply_min_confidence: NaN }, { ...OFF, auto_apply_min_confidence: 97 }), 97);
    assert.equal(gatePct({ ...OFF, auto_apply_min_confidence: 50 }, {}), 95, 'the default when nothing is valid');
    assert.equal(gatePct(null, OFF), null, 'nothing to check before the settings load');
});

test('auto-apply can be switched on only while the gate is known open; off is always allowed', () => {
    assert.equal(autoApplyToggleLocked(OFF, OFF, null), true, 'gate still loading');
    assert.equal(autoApplyToggleLocked(OFF, OFF, CLOSED), true);
    assert.equal(autoApplyToggleLocked(OFF, OFF, OPEN), false);
    assert.equal(autoApplyToggleLocked(ON, ON, CLOSED), false, 'already on: it can be switched off');
    assert.equal(autoApplyToggleLocked(ON, OFF, CLOSED), false, 'switched on in this draft: it can be switched back off');
});

test('switching on against a closed gate is an error the save bar reports', () => {
    assert.deepEqual(autoApplyErrors(ON, OFF, OPEN), {});
    assert.match(autoApplyErrors(ON, OFF, CLOSED).auto_apply_defect_type, /gate is closed/);
    assert.deepEqual(autoApplyErrors(ON, ON, CLOSED), {}, 'staying on is allowed: each job pauses instead');
    assert.deepEqual(autoApplyErrors(ON, OFF, null), {}, 'unknown gate: the server decides (409)');
    assert.deepEqual(autoApplyErrors(OFF, OFF, CLOSED), {});
});

test('gateText reports the figures the gate measured', () => {
    assert.equal(gateText({ ...OPEN, accuracy: 0.9822 }), '98.2 % of 214 rows at ≥ 95 % — gate open');
    assert.equal(gateText(CLOSED), '88.8 % of 214 rows at ≥ 95 % (95 % needed) — gate closed');
    assert.equal(gateText({ graded: 12, agreed: 12, accuracy: 1, open: false, min_confidence: 0.9 }), '100.0 % of 12 rows at ≥ 90 % (50 rows needed) — gate closed');
    assert.equal(gateText({ graded: 0, agreed: 0, accuracy: 0, open: false, min_confidence: 0.95 }), 'No graded rows at ≥ 95 % yet (50 needed) — gate closed');
    assert.equal(gateText({ graded: 60, agreed: 59, open: true, min_confidence: 97 }), '98.3 % of 60 rows at ≥ 97 % — gate open',
        'without an accuracy field: agreed / graded; a whole-percent threshold reads as is');
    assert.equal(gateText(null), 'Checking the accuracy gate…');
});

test('gateScope names the window and the policies the figures cover', () => {
    assert.equal(gateScope(OPEN),
        "People's triage of TypeSafe.ai's own defect-type suggestions over the last 90 days, fa-verdict-v7 and fa-verdict-v8 decisions only; confirmations of labels auto-apply set do not count. Opens at 50 rows and 95 % agreement.");
    assert.equal(gateScope({ ...OPEN, policies: [] }),
        "People's triage of TypeSafe.ai's own defect-type suggestions over the last 90 days; confirmations of labels auto-apply set do not count. Opens at 50 rows and 95 % agreement.");
    assert.equal(gateScope(null), null);
});
