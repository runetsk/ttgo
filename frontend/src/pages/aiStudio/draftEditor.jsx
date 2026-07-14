import { useEffect, useMemo, useRef, useState } from 'react';
import { AIC, Icon } from './constants';
import { AIBtn, SectionLabel } from './primitives';

const AUTOSAVE_MS = 800;

// Map server findings (validator + rubric use "steps[i].field" / plain field
// names) to a per-field lookup for inline display under the matching input.
function findingsByField(draft) {
    const out = {};
    for (const f of draft.findings || []) {
        (out[f.field] = out[f.field] || []).push(f);
    }
    for (const dim of draft.quality || []) {
        for (const f of dim.findings || []) {
            (out[f.field] = out[f.field] || []).push(f);
        }
    }
    return out;
}

function FieldFindings({ findings }) {
    if (!findings || !findings.length) return null;
    return (
        <div style={{ marginTop: 3, display: 'flex', flexDirection: 'column', gap: 2 }}>
            {findings.map((f, i) => (
                <span key={i} style={{
                    fontSize: 10.5, lineHeight: 1.4,
                    color: f.severity === 'error' ? 'var(--aig-danger-fg)' : 'var(--aig-tone-amber-fg)',
                }}>
                    {f.severity === 'error' ? '✕' : '⚠'} {f.message}
                </span>
            ))}
        </div>
    );
}

const contentOf = (draft) => ({
    name: draft.name || '',
    category: draft.category || '',
    description: draft.description || '',
    source_refs: draft.source_refs || [],
    steps: (draft.steps || []).map(s => ({ action: s.action || '', expected_result: s.expected_result || '' })),
});

// Hosted with `key={draft.id}` by StudioDraftDetail (see drafts.jsx), so
// selecting a *different* draft remounts this component and re-initializes
// `form` from scratch via the lazy useState initializer below. A version
// bump on the SAME draft id only ever comes from this component's own
// `onSave` calls (autosave / restore-original), so it must never force a
// reset — that would clobber whatever the user typed while the save
// round-trip was still in flight. Keying on id alone (not id+version) gets
// the "reset on new draft" behavior for free without that race, so no
// reset-effect is needed at all.
export function DraftEditor({ draft, onSave, disabled }) {
    const [form, setForm] = useState(() => contentOf(draft));
    const [saveState, setSaveState] = useState('saved'); // saved | dirty | saving | error
    const timerRef = useRef(null);
    const pendingRef = useRef(null); // latest UNSAVED snapshot; null when clean/saved
    const seqRef = useRef(0);        // increments per scheduled save; only the latest may write the badge

    useEffect(() => () => {
        clearTimeout(timerRef.current);
        if (pendingRef.current) onSave(draft.id, pendingRef.current); // don't drop unsaved edits on navigate
    }, [draft.id, onSave]);

    const byField = useMemo(() => findingsByField(draft), [draft]);

    const flush = async (snapshot, seq) => {
        setSaveState('saving');
        try {
            await onSave(draft.id, snapshot);
            if (pendingRef.current === snapshot) pendingRef.current = null; // no newer edit queued -> clean
            if (seq === seqRef.current) setSaveState('saved');              // stale save must not clobber a newer 'dirty'
        } catch {
            if (seq === seqRef.current) setSaveState('error');
        }
    };

    const update = (changes) => {
        if (disabled) return;
        const next = { ...form, ...changes };
        setForm(next);
        pendingRef.current = next;
        setSaveState('dirty');
        clearTimeout(timerRef.current);
        const seq = ++seqRef.current;
        timerRef.current = setTimeout(() => flush(next, seq), AUTOSAVE_MS);
    };

    const updateStep = (i, field, value) => {
        update({ steps: form.steps.map((s, j) => (j === i ? { ...s, [field]: value } : s)) });
    };
    const addStep = () => update({ steps: [...form.steps, { action: '', expected_result: '' }] });
    const removeStep = (i) => update({ steps: form.steps.filter((_, j) => j !== i) });
    const moveStep = (i, dir) => {
        const j = i + dir;
        if (j < 0 || j >= form.steps.length) return;
        const steps = [...form.steps];
        [steps[i], steps[j]] = [steps[j], steps[i]];
        update({ steps });
    };

    const undoLocal = () => {
        clearTimeout(timerRef.current);
        pendingRef.current = null;
        setForm(contentOf(draft));
        setSaveState('saved');
    };
    const restoreOriginal = () => {
        if (!draft.original) return;
        const orig = {
            name: draft.original.name, category: draft.original.category,
            description: draft.original.description,
            source_refs: draft.original.source_refs || [],
            steps: draft.original.steps || [],
        };
        setForm(orig);
        pendingRef.current = orig;
        setSaveState('dirty');
        clearTimeout(timerRef.current);
        const seq = ++seqRef.current;
        timerRef.current = setTimeout(() => flush(orig, seq), 100);
    };

    const saveLabel = { saved: 'Saved', dirty: 'Unsaved…', saving: 'Saving…', error: 'Save failed' }[saveState];

    const inputStyle = { width: '100%', fontSize: 12.5 };

    return (
        <div data-testid="draft-editor">
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
                {draft.edited && (
                    <span style={{
                        padding: '1px 7px', borderRadius: 999, fontSize: 10, fontWeight: 700,
                        background: 'var(--aig-accent-soft-bg)', border: '1px solid var(--aig-accent-soft-border)',
                        color: AIC.indigoSoft,
                    }}>EDITED · v{draft.version}</span>
                )}
                <span data-testid="save-state" style={{
                    marginLeft: 'auto', fontSize: 10.5,
                    color: saveState === 'error' ? 'var(--aig-danger-fg)' : AIC.muted,
                }}>{saveLabel}</span>
            </div>

            <SectionLabel>Name</SectionLabel>
            <input className="modern-input" style={inputStyle} value={form.name}
                onChange={e => update({ name: e.target.value })} disabled={disabled} />
            <FieldFindings findings={byField.name} />

            <SectionLabel>Category</SectionLabel>
            <input className="modern-input" style={inputStyle} value={form.category}
                onChange={e => update({ category: e.target.value })} disabled={disabled} />
            <FieldFindings findings={byField.category} />

            <SectionLabel>Description</SectionLabel>
            <textarea className="modern-input" rows={3} style={{ ...inputStyle, resize: 'vertical' }}
                value={form.description} onChange={e => update({ description: e.target.value })} disabled={disabled} />
            <FieldFindings findings={byField.description} />

            <SectionLabel>Source refs</SectionLabel>
            <input className="modern-input" style={inputStyle}
                value={form.source_refs.join(', ')}
                placeholder="AC-1, PROJ-12"
                onChange={e => update({ source_refs: e.target.value.split(',').map(s => s.trim()).filter(Boolean) })}
                disabled={disabled} />
            <FieldFindings findings={byField.source_refs} />

            <SectionLabel right={
                <AIBtn onClick={addStep} disabled={disabled} style={{ padding: '2px 8px', fontSize: 10.5 }}>
                    {Icon.plus(11)} Step
                </AIBtn>
            }>Steps</SectionLabel>
            {form.steps.map((s, i) => (
                <div key={i} style={{
                    border: `1px solid ${AIC.border}`, borderRadius: 8,
                    padding: '8px 10px', marginBottom: 8, background: 'var(--aig-surface-tint)',
                }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 4, marginBottom: 5 }}>
                        <span style={{ fontSize: 10.5, fontWeight: 700, color: AIC.muted }}>#{i + 1}</span>
                        <div style={{ marginLeft: 'auto', display: 'flex', gap: 2 }}>
                            <button className="action-btn" title="Move up" onClick={() => moveStep(i, -1)} disabled={disabled || i === 0}>↑</button>
                            <button className="action-btn" title="Move down" onClick={() => moveStep(i, 1)} disabled={disabled || i === form.steps.length - 1}>↓</button>
                            <button className="action-btn" title="Remove step" onClick={() => removeStep(i)} disabled={disabled}>✕</button>
                        </div>
                    </div>
                    <textarea className="modern-input" rows={2} style={{ ...inputStyle, resize: 'vertical' }}
                        placeholder="Action" value={s.action}
                        onChange={e => updateStep(i, 'action', e.target.value)} disabled={disabled} />
                    <FieldFindings findings={byField[`steps[${i}].action`]} />
                    <textarea className="modern-input" rows={2} style={{ ...inputStyle, resize: 'vertical', marginTop: 6 }}
                        placeholder="Expected result" value={s.expected_result}
                        onChange={e => updateStep(i, 'expected_result', e.target.value)} disabled={disabled} />
                    <FieldFindings findings={byField[`steps[${i}].expected_result`]} />
                </div>
            ))}
            <FieldFindings findings={byField.steps} />

            <div style={{ display: 'flex', gap: 8, marginTop: 10 }}>
                <AIBtn onClick={undoLocal} disabled={disabled || saveState === 'saved'}>Undo local changes</AIBtn>
                <AIBtn onClick={restoreOriginal} disabled={disabled || !draft.original}>Restore generated version</AIBtn>
            </div>
        </div>
    );
}
