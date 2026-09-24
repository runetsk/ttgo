import React, { useEffect, useMemo, useState } from 'react';
import { getTypeSafeSettings, updateTypeSafeSettings, testTypeSafeConnection, aiGeneration } from '../../api';
import { toast } from '../../toast';
import { s } from './styles';
import { ToggleCard, InlineSwitch, FieldRow, SuffixInput, UnsavedBadge } from './SettingsControls';
import { cs } from './settingsControlStyles';
import {
    formFromSettings, buildTypeSafePatch, keyStatusLabel, canTestConnection, escalationValid, escalationNote,
    defaultProvider, describeRoute, typesafeStatus, verdictDependencyNote, TIMEOUT_MIN, TIMEOUT_MAX,
} from '../../utils/typesafeSettings';

const icon = (paths) => (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">{paths}</svg>
);
const ICONS = {
    power: icon(<><path d="M18.36 6.64a9 9 0 1 1-12.73 0" /><line x1="12" y1="2" x2="12" y2="12" /></>),
    target: icon(<><circle cx="12" cy="12" r="10" /><circle cx="12" cy="12" r="6" /><circle cx="12" cy="12" r="2" /></>),
    layers: icon(<><polygon points="12 2 2 7 12 12 22 7 12 2" /><polyline points="2 17 12 22 22 17" /><polyline points="2 12 12 17 22 12" /></>),
    activity: icon(<polyline points="22 12 18 12 15 21 9 3 6 12 2 12" />),
    message: icon(<path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />),
    fallback: icon(<><polyline points="1 4 1 10 7 10" /><path d="M3.51 15a9 9 0 1 0 2.13-9.36L1 10" /></>),
};

const TONES = {
    green: { color: 'var(--aig-tone-green-fg)', background: 'rgba(34,197,94,0.1)', border: 'rgba(34,197,94,0.25)' },
    amber: { color: 'var(--aig-tone-amber-fg)', background: 'rgba(234,179,8,0.1)', border: 'rgba(234,179,8,0.25)' },
    red: { color: 'var(--aig-tone-red-fg)', background: 'rgba(239,68,68,0.1)', border: 'rgba(239,68,68,0.25)' },
    muted: { color: 'var(--text-secondary)', background: 'transparent', border: 'var(--border-color)' },
};

function StatusPill({ status }) {
    const t = TONES[status.tone] || TONES.muted;
    return (
        <span data-testid="typesafe-status" style={{
            fontSize: '0.7rem', fontWeight: 600, padding: '1px 8px', borderRadius: 20,
            color: t.color, background: t.background, border: `1px solid ${t.border}`,
        }}>{status.label}</span>
    );
}

/* ── TypeSafe.ai Section: one hosted decision API used by failure analysis ── */
export default function TypeSafeSettingsCard({ isAdmin }) {
    const [settings, setSettings] = useState(null);
    const [form, setForm] = useState(null);
    const [saving, setSaving] = useState(false);
    const [testing, setTesting] = useState(false);
    const [testResult, setTestResult] = useState(null);
    const [providers, setProviders] = useState(null); // null: not loaded (or not visible to this user)

    useEffect(() => {
        let alive = true;
        getTypeSafeSettings()
            .then((data) => { if (alive) { setSettings(data); setForm(formFromSettings(data)); } })
            .catch(() => toast.error('Failed to load TypeSafe settings'));
        aiGeneration.listProviders()
            .then((list) => { if (alive) setProviders(Array.isArray(list) ? list : []); })
            .catch(() => {});
        return () => { alive = false; };
    }, []);

    const patch = useMemo(() => (form && settings ? buildTypeSafePatch(form, settings) : null), [form, settings]);

    if (!settings || !form) {
        return (
            <section style={s.section}>
                <div style={s.loadingState}><span style={s.loadingSpinner} />Loading TypeSafe settings…</div>
            </section>
        );
    }

    const update = (p) => setForm((prev) => ({ ...prev, ...p }));

    const save = async () => {
        if (!patch) return;
        setSaving(true);
        try {
            const next = await updateTypeSafeSettings(patch);
            setSettings(next);
            setForm(formFromSettings(next));
            setTestResult(null);
            toast.success('TypeSafe settings saved');
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Failed to save TypeSafe settings');
        } finally {
            setSaving(false);
        }
    };

    const runTest = async () => {
        setTesting(true);
        setTestResult(null);
        try {
            setTestResult(await testTypeSafeConnection());
        } catch (err) {
            setTestResult({ ok: false, category: 'network', message: err?.response?.data?.error || err.message });
        } finally {
            setTesting(false);
        }
    };

    const timeoutValid = Number.isInteger(form.timeout_seconds) && form.timeout_seconds >= TIMEOUT_MIN && form.timeout_seconds <= TIMEOUT_MAX;
    const modelValid = typeof form.model === 'string' && form.model.trim() !== '';
    const escalationOk = escalationValid(form.escalate_below_pct ?? 0);
    const canSave = !!patch && timeoutValid && modelValid && escalationOk;
    const testAllowed = canTestConnection(form, settings);
    const route = providers ? describeRoute(form, settings.api_key_status === 'ok', defaultProvider(providers)) : null;
    const unsavedKeyChange = !!form.clear_api_key || (typeof form.api_key === 'string' && form.api_key.trim() !== '');
    const verdictNote = verdictDependencyNote(form);
    const locked = !isAdmin;

    return (
        <section style={cs.section} data-testid="typesafe-settings">
            <div style={cs.sectionHead}>
                <div style={cs.sectionHeadLeft}>
                    <span style={cs.sectionDot} />
                    <h4 style={cs.sectionTitle}>TypeSafe.ai</h4>
                    <StatusPill status={typesafeStatus(settings)} />
                    {patch && <UnsavedBadge />}
                </div>
                {isAdmin && (
                    <button className="primary-btn" onClick={save} disabled={saving || !canSave} data-testid="typesafe-save" style={{ fontSize: '0.82rem' }}>
                        {saving ? 'Saving…' : 'Save'}
                    </button>
                )}
            </div>

            <p style={cs.desc}>
                A hosted decision model that classifies failing results and groups failures sharing a cause.
                When enabled, redacted failure text (error, stack head, log tail, steps and recent history) is sent to TypeSafe.ai.
            </p>

            <ToggleCard
                icon={ICONS.power} iconColor="#14b8a6"
                label="Enable TypeSafe.ai"
                desc={form.enabled
                    ? 'TypeSafe is used for the features switched on below.'
                    : 'Off: nothing is sent to TypeSafe. The settings below are kept for when you switch it on.'}
                checked={form.enabled} disabled={locked} testId="typesafe-enabled"
                onChange={(v) => update({ enabled: v })}
            />

            <div style={cs.groupTitle}>Connection</div>
            <FieldRow
                label="API key" htmlFor="typesafe-api-key" testId="typesafe-key-row"
                hint={(
                    <>
                        <span data-testid="typesafe-key-status">{keyStatusLabel(settings)}</span>
                        {unsavedKeyChange && (
                            <span data-testid="typesafe-test-hint" style={{ display: 'block' }}>
                                {form.clear_api_key ? 'The key is removed when you save.' : 'Save the new key before testing it.'}
                            </span>
                        )}
                        {testResult && (
                            <span data-testid="typesafe-test-result" style={{ display: 'block', fontWeight: 600, color: testResult.ok ? 'var(--aig-tone-green-fg)' : 'var(--aig-tone-red-fg)' }}>
                                {testResult.ok
                                    ? `Connected · ${(testResult.models || []).map((model) => model.name).join(', ') || 'no models listed'}`
                                    : `Failed (${testResult.category}): ${testResult.message}`}
                            </span>
                        )}
                        {isAdmin && settings.api_key_status !== 'missing' && (
                            <span style={{ display: 'block', marginTop: 6 }}>
                                <InlineSwitch label="Remove the stored key" checked={form.clear_api_key} testId="typesafe-clear-key"
                                    onChange={(v) => update({ clear_api_key: v, api_key: '' })} />
                            </span>
                        )}
                    </>
                )}
            >
                <input id="typesafe-api-key" className="modern-input" type="password" autoComplete="off" data-testid="typesafe-api-key"
                    placeholder={settings.api_key_status === 'ok' ? 'Leave blank to keep the current key' : 'Paste your TypeSafe.ai or OpenRouter key'}
                    value={form.api_key} disabled={locked || form.clear_api_key}
                    onChange={(e) => update({ api_key: e.target.value })}
                    style={{ width: 280, padding: '8px 12px', fontSize: '0.85rem' }} />
                {isAdmin && (
                    <button onClick={runTest} disabled={testing || !testAllowed} data-testid="typesafe-test"
                        title={testAllowed ? 'Check the stored key with one small request' : 'Save a key first'}
                        style={{ ...cs.secondaryBtn, opacity: testing || !testAllowed ? 0.55 : 1, cursor: testing || !testAllowed ? 'not-allowed' : 'pointer' }}>
                        {testing ? 'Testing…' : 'Test connection'}
                    </button>
                )}
            </FieldRow>
            <FieldRow label="Model" htmlFor="typesafe-model"
                hint="Pinned by default (jev-1.13.0). Confidence thresholds are tuned per version, so change it deliberately.">
                <input id="typesafe-model" className="modern-input" type="text" value={form.model} disabled={locked} data-testid="typesafe-model"
                    onChange={(e) => update({ model: e.target.value })} style={{ width: 200, padding: '8px 12px', fontSize: '0.85rem' }} />
            </FieldRow>
            <FieldRow label="Timeout" htmlFor="typesafe-timeout"
                hint={timeoutValid ? 'How long to wait for one TypeSafe answer before counting it as unavailable.' : `Enter a whole number of seconds from ${TIMEOUT_MIN} to ${TIMEOUT_MAX}.`}>
                <SuffixInput id="typesafe-timeout" suffix="s" min={TIMEOUT_MIN} max={TIMEOUT_MAX} value={form.timeout_seconds} disabled={locked}
                    data-testid="typesafe-timeout" onChange={(e) => update({ timeout_seconds: Number(e.target.value) })} />
            </FieldRow>

            <div style={cs.groupTitle}>What TypeSafe does</div>
            <div style={cs.togglesGrid}>
                <ToggleCard icon={ICONS.target} iconColor="#818cf8"
                    label="Use for failure verdicts"
                    desc="TypeSafe decides the verdict and the suggested defect type, with a real confidence score."
                    checked={form.verdict_engine_enabled} disabled={locked} testId="typesafe-verdict_engine_enabled"
                    onChange={(v) => update({ verdict_engine_enabled: v })} />
                <ToggleCard icon={ICONS.layers} iconColor="#60a5fa"
                    label="Semantic failure grouping"
                    desc="Merge failure groups that share a cause even when the error text differs."
                    checked={form.semantic_dedup_enabled} disabled={locked} testId="typesafe-semantic_dedup_enabled"
                    onChange={(v) => update({ semantic_dedup_enabled: v })} />
                <ToggleCard icon={ICONS.activity} iconColor="#14b8a6"
                    label="Allow on automatic analysis"
                    desc="Off by default: analyses started by run completion do not contact TypeSafe unless this is on."
                    checked={form.allow_auto_failure_analysis} disabled={locked} testId="typesafe-allow_auto_failure_analysis"
                    onChange={(v) => update({ allow_auto_failure_analysis: v })} />
            </div>

            <div style={cs.groupTitle}>How your LLM helps</div>
            <div style={cs.togglesGrid}>
                <ToggleCard icon={ICONS.message} iconColor="#f472b6"
                    label="Write an explanation with the default LLM"
                    desc="After TypeSafe decides, your LLM writes the summary, next action and rationale. Off: decisions only, in well under a second per group; each analysis has an Explain button."
                    checked={form.narrative_enabled} disabled={locked || !form.verdict_engine_enabled} note={verdictNote}
                    testId="typesafe-narrative_enabled" onChange={(v) => update({ narrative_enabled: v })} />
                <ToggleCard icon={ICONS.fallback} iconColor="#fbbf24"
                    label="Use the default LLM when TypeSafe is unavailable"
                    desc="On: if TypeSafe cannot be reached, the LLM decides and the analysis says so. Off: the attempt is recorded as failed and can be retried; failure data never goes to the LLM in its place."
                    checked={form.llm_fallback_enabled} disabled={locked || !form.verdict_engine_enabled} note={verdictNote}
                    testId="typesafe-llm_fallback_enabled" onChange={(v) => update({ llm_fallback_enabled: v })} />
            </div>
            <FieldRow label="Ask the LLM when TypeSafe's confidence is below" htmlFor="typesafe-escalate" disabled={!form.verdict_engine_enabled}
                hint={<span data-testid="typesafe-escalate-note">{form.verdict_engine_enabled ? escalationNote(form.escalate_below_pct ?? 0) : verdictNote}</span>}>
                <SuffixInput id="typesafe-escalate" suffix="%" min={0} max={100} step={1} value={form.escalate_below_pct ?? 0}
                    disabled={locked || !form.verdict_engine_enabled} data-testid="typesafe-escalate_below_pct"
                    onChange={(e) => update({ escalate_below_pct: e.target.value === '' ? NaN : Number(e.target.value) })} />
            </FieldRow>

            {route && (
                <div style={routePanel} data-testid="typesafe-route">
                    <div style={cs.groupTitle}>
                        What an analysis will do{patch ? ' (with the unsaved changes above)' : ''}
                    </div>
                    <div style={routeColumns}>
                        {[['Started by hand', route.manual, 'manual'], ['Started by run completion', route.auto, 'auto']].map(([title, lines, key]) => (
                            <div key={key} data-testid={`typesafe-route-${key}`} style={{ minWidth: 0 }}>
                                <div style={routeTitle}>{title}</div>
                                <ul style={routeList}>
                                    {lines.map((line) => <li key={line} style={routeItem}>{line}</li>)}
                                </ul>
                            </div>
                        ))}
                    </div>
                </div>
            )}
        </section>
    );
}

const routePanel = {
    display: 'flex', flexDirection: 'column', gap: 10, padding: '12px 14px', borderRadius: 10,
    border: '1px solid rgba(99,102,241,0.25)', background: 'rgba(99,102,241,0.04)',
};
const routeColumns = { display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))', gap: 16 };
const routeTitle = { fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-primary)', marginBottom: 4 };
const routeList = { margin: 0, paddingLeft: 18, display: 'flex', flexDirection: 'column', gap: 3 };
const routeItem = { fontSize: '0.78rem', color: 'var(--text-secondary)', lineHeight: 1.5 };
