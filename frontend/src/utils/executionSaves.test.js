import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createExecutionSaves } from './executionSaves.js';

test('switching cases flushes each latest draft without cancelling another case', async () => {
    const writes = [];
    const saves = createExecutionSaves(async (id, patch) => writes.push([id, patch]), { delay: 60000 });
    saves.schedule('a', { steps: ['old'] });
    saves.schedule('a', { steps: ['latest'] });
    saves.schedule('b', { steps: ['other'] });
    await saves.flush();
    assert.deepEqual(writes, [['a', { steps: ['latest'] }], ['b', { steps: ['other'] }]]);
    assert.equal(saves.hasPending(), false);
});

test('a final verdict waits for an in-flight step save and replaces a pending draft', async () => {
    const writes = [];
    let release;
    let entered;
    const first = new Promise(resolve => { release = resolve; });
    const started = new Promise(resolve => { entered = resolve; });
    const saves = createExecutionSaves(async (id, patch) => {
        writes.push([id, patch]);
        if (writes.length === 1) { entered(); await first; }
    }, { delay: 60000 });
    const old = saves.save('a', { steps: ['old'] });
    await started;
    saves.schedule('a', { steps: ['draft'] });
    const final = saves.save('a', { status: 'PASS', steps: ['latest'] });
    assert.equal(writes.length, 1);
    release();
    await Promise.all([old, final]);
    assert.deepEqual(writes, [['a', { steps: ['old'] }], ['a', { status: 'PASS', steps: ['latest'] }]]);
    await saves.flush();
    assert.equal(writes.length, 2);
});

test('failed saves block navigation until a retry succeeds', async () => {
    let fail = true;
    const saves = createExecutionSaves(async () => { if (fail) throw new Error('offline'); }, { delay: 60000 });
    saves.schedule('a', { steps: ['draft'] });
    await assert.rejects(saves.flush('a'), /offline/);
    assert.equal(saves.hasPending(), true);
    fail = false;
    await saves.flush('a');
    assert.equal(saves.hasPending(), false);
});

for (const lateFailure of [false, true]) {
    test(`navigation never replays an older failure over a newer verdict (late failure: ${lateFailure})`, async () => {
        const writes = [];
        let failOld;
        let finishNew;
        let startedOld;
        let startedNew;
        const oldStarted = new Promise(resolve => { startedOld = resolve; });
        const newStarted = new Promise(resolve => { startedNew = resolve; });
        const oldGate = new Promise((_, reject) => { failOld = reject; });
        const newGate = new Promise(resolve => { finishNew = resolve; });
        const saves = createExecutionSaves(async (id, patch) => {
            writes.push(patch);
            if (writes.length === 1) { startedOld(); await oldGate; }
            else if (writes.length === 2) { startedNew(); await newGate; }
        });
        const old = saves.save('a', { steps: ['old'] });
        const oldRejected = assert.rejects(old, /offline/);
        await oldStarted;
        if (!lateFailure) { failOld(new Error('offline')); await oldRejected; }
        const verdict = saves.save('a', { status: 'PASS', steps: ['new'] });
        if (lateFailure) { failOld(new Error('offline')); await oldRejected; }
        await newStarted;
        const exit = saves.flush();
        finishNew();
        await Promise.all([verdict, exit]);
        assert.deepEqual(writes, [{ steps: ['old'] }, { status: 'PASS', steps: ['new'] }]);
        assert.equal(saves.hasPending(), false);
    });
}
