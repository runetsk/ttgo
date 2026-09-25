import React, { useCallback, useEffect, useRef, useState } from 'react';
import { aiGeneration } from '../../api';
import { toast } from '../../toast';
import { TEMPLATE_VARS, REQUIRED_TEMPLATE_VARS, PARENT_REQUIRED_VARS } from './constants';
import { placeholderStatus, insertText } from '../../utils/aiSettingsTabs';
import { ts } from './templateEditorStyles';

// The two prompt templates generation uses. Both drafts live here, so switching between them (or
// to another tab of Settings → AI) keeps unsaved edits; each saves and resets on its own.
const KINDS = {
    standard: {
        label: 'Standard',
        desc: 'Used when generating tests for a single requirement (no child issues).',
        required: REQUIRED_TEMPLATE_VARS,
        missingWhat: 'Requirement details',
        contentKey: 'content',
        save: aiGeneration.updateTemplate, reset: aiGeneration.resetTemplate,
        name: 'Template',
    },
    parent: {
        label: 'Parent (with child issues)',
        desc: 'Used for a parent requirement that has child issues. Lighter, and focused on coverage across the children rather than deep single-requirement rules.',
        required: PARENT_REQUIRED_VARS,
        missingWhat: 'Child issue context',
        contentKey: 'parent_content',
        save: aiGeneration.updateParentTemplate, reset: aiGeneration.resetParentTemplate,
        name: 'Parent template',
    },
};
const KIND_IDS = ['standard', 'parent'];

export default function TemplateEditor({ isAdmin, onStatusChange }) {
    const [template, setTemplate] = useState(null);
    const [drafts, setDrafts] = useState({ standard: '', parent: '' });
    const [kind, setKind] = useState('standard');
    const [busy, setBusy] = useState(null); // 'save' | 'reset' | null
    const editorRef = useRef(null);

    const load = useCallback(() => {
        aiGeneration.getTemplate()
            .then((t) => { setTemplate(t); setDrafts({ standard: t.content || '', parent: t.parent_content || '' }); })
            .catch(() => {});
    }, []);
    useEffect(() => { load(); }, [load]);

    const saved = (k) => (template ? template[KINDS[k].contentKey] || '' : '');
    const modified = {
        standard: !!template && drafts.standard !== saved('standard'),
        parent: !!template && drafts.parent !== saved('parent'),
    };
    const standardCustom = !!template && saved('standard') !== (template.default_content || '');
    const parentCustom = !!template && saved('parent') !== (template.default_parent_content || '');
    const dirty = modified.standard || modified.parent;
    useEffect(() => {
        if (template) onStatusChange?.({ standardCustom, parentCustom, dirty });
    }, [template, standardCustom, parentCustom, dirty, onStatusChange]);

    const cfg = KINDS[kind];
    const content = drafts[kind];
    const rows = placeholderStatus(content, TEMPLATE_VARS, cfg.required);
    const missing = rows.filter((r) => r.required && !r.present).map((r) => r.name);
    const setContent = (value) => setDrafts((d) => ({ ...d, [kind]: value }));

    const handleSave = async () => {
        setBusy('save');
        try {
            const t = await cfg.save(content);
            setTemplate(t);
            if (t.warnings?.length) t.warnings.forEach((w) => toast.error(w));
            else toast.success(`${cfg.name} saved`);
        } catch (err) {
            toast.error(err?.response?.data?.error || `Failed to save ${cfg.name.toLowerCase()}`);
        } finally {
            setBusy(null);
        }
    };

    const handleReset = async () => {
        if (!window.confirm(`Reset the ${cfg.name.toLowerCase()} to the built-in default?`)) return;
        setBusy('reset');
        try {
            const t = await cfg.reset();
            setTemplate(t);
            setDrafts((d) => ({ ...d, [kind]: t[cfg.contentKey] || '' }));
            toast.success(`${cfg.name} reset to default`);
        } catch (err) {
            toast.error(err?.response?.data?.error || `Failed to reset ${cfg.name.toLowerCase()}`);
        } finally {
            setBusy(null);
        }
    };

    const insert = (name) => {
        const el = editorRef.current;
        if (!el || !isAdmin) return;
        const next = insertText(el.value, el.selectionStart, el.selectionEnd, name);
        setContent(next.value);
        requestAnimationFrame(() => { el.focus(); el.setSelectionRange(next.caret, next.caret); });
    };

    return (
        <section style={ts.section}>
            <div style={ts.head}>
                <div style={ts.segmented} role="group" aria-label="Prompt template">
                    {KIND_IDS.map((k) => (
                        <button key={k} type="button" aria-pressed={kind === k} data-testid={`template-kind-${k}`}
                            onClick={() => setKind(k)} style={{ ...ts.segment, ...(kind === k ? ts.segmentOn : null) }}>
                            {KINDS[k].label}
                            {modified[k] && <span style={ts.dirtyDot} aria-label="unsaved changes" />}
                        </button>
                    ))}
                </div>
                <div style={ts.actions}>
                    {modified[kind] && <span style={ts.unsaved} data-testid="template-unsaved">Unsaved changes</span>}
                    {isAdmin && (
                        <>
                            <button type="button" className="action-btn" data-testid="template-reset" onClick={handleReset}
                                disabled={busy !== null} style={{ fontSize: '0.82rem' }}>
                                {busy === 'reset' ? 'Resetting…' : 'Reset to default'}
                            </button>
                            <button type="button" className="primary-btn" data-testid="template-save" onClick={handleSave}
                                disabled={busy !== null || !modified[kind]} style={{ fontSize: '0.82rem' }}>
                                {busy === 'save' ? 'Saving…' : 'Save template'}
                            </button>
                        </>
                    )}
                </div>
            </div>

            <p style={ts.desc}>{cfg.desc}</p>

            {missing.length > 0 && (
                <div role="alert" style={ts.missing}>
                    <strong>Missing required placeholders: </strong>
                    <span style={{ fontFamily: 'monospace' }}>{missing.join(', ')}</span>. {cfg.missingWhat} will <strong>not</strong> be sent to the LLM without them.
                </div>
            )}

            <div style={ts.grid}>
                <div style={ts.editorWrap}>
                    {!isAdmin && <div style={ts.readOnly}>View only: an admin can edit templates</div>}
                    <textarea ref={editorRef} className="modern-input" data-testid="template-editor" aria-label={`${cfg.label} prompt template`}
                        style={{ ...ts.editor, opacity: isAdmin ? 1 : 0.7 }} value={content} onChange={(e) => setContent(e.target.value)}
                        disabled={!isAdmin} placeholder="Loading template…" spellCheck={false} />
                    <div style={ts.footer}>
                        <span>{content.length} chars</span>
                        <span>{rows.filter((r) => r.present).length} of {rows.length} placeholders</span>
                    </div>
                </div>
                <div style={ts.side}>
                    <div style={ts.sideTitle}>Placeholders</div>
                    {rows.map((r) => {
                        const bad = r.required && !r.present;
                        return (
                            <button key={r.name} type="button" style={ts.ph} onClick={() => insert(r.name)} disabled={!isAdmin}
                                data-testid={`template-placeholder-${r.name.replace(/[{}]/g, '')}`}
                                aria-label={`Insert ${r.name}${r.required ? ', required' : ''}${r.present ? ', used' : ', not used'}`}>
                                <span aria-hidden="true" style={{ width: 14, textAlign: 'center', ...(bad ? ts.phMissing : null) }}>{r.present ? '✓' : bad ? '!' : '○'}</span>
                                <span style={{ ...ts.phName, ...(bad ? ts.phMissing : null) }}>{r.name}</span>
                                {r.required && <span style={ts.phReq}>required</span>}
                            </button>
                        );
                    })}
                    <p style={ts.sideNote}>Click a placeholder to insert it at the cursor.</p>
                </div>
            </div>
        </section>
    );
}
