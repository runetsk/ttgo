import React, { useEffect, useMemo, useState } from 'react';
import { getTypeSafeSettings, updateTypeSafeSettings, testTypeSafeConnection } from '../../api';
import { toast } from '../../toast';
import { s, m } from './styles';
import { formFromSettings, buildTypeSafePatch, keyStatusLabel, TIMEOUT_MIN, TIMEOUT_MAX } from '../../utils/typesafeSettings';

/* ── TypeSafe.ai Section: one hosted decision API used by failure analysis ── */
export default function TypeSafeSettingsCard({ isAdmin }) {
    const [settings, setSettings] = useState(null);
    const [form, setForm] = useState(null);
    const [saving, setSaving] = useState(false);
    const [testing, setTesting] = useState(false);
    const [testResult, setTestResult] = useState(null);

    useEffect(() => {
        let alive = true;
        getTypeSafeSettings()
            .then((data) => { if (alive) { setSettings(data); setForm(formFromSettings(data)); } })
            .catch(() => toast.error('Failed to load TypeSafe settings'));
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

    return (
        <section style={s.section} data-testid="typesafe-settings">
            <div style={s.sectionHead}>
                <div style={s.sectionHeadLeft}>
                    <span style={s.sectionDot} />
                    <h4 style={s.sectionTitle}>TypeSafe.ai</h4>
                </div>
            </div>
            <p style={{ color: 'var(--text-secondary)', fontSize: 13, margin: '0 0 12px' }}>
                A hosted decision model that classifies failing results and groups failures sharing a cause.
                When enabled, redacted failure text (error, stack head, log tail, steps and recent history) is sent to TypeSafe.ai.
            </p>

            <label style={m.toggle}>
                <input type="checkbox" style={m.checkbox} checked={!!form.enabled} disabled={!isAdmin}
                    onChange={(e) => update({ enabled: e.target.checked })} data-testid="typesafe-enabled" />
                <span style={m.toggleLabel}>Enable TypeSafe.ai</span>
            </label>

            <div style={m.field}>
                <label style={m.fieldLabel} htmlFor="typesafe-api-key">API key</label>
                <input id="typesafe-api-key" type="password" autoComplete="off" data-testid="typesafe-api-key"
                    placeholder="Leave blank to keep the current key" value={form.api_key} disabled={!isAdmin || form.clear_api_key}
                    onChange={(e) => update({ api_key: e.target.value })} />
                <div style={m.currentKeyNote} data-testid="typesafe-key-status">{keyStatusLabel(settings)}</div>
                {isAdmin && settings.api_key_status !== 'missing' && (
                    <label style={m.toggle}>
                        <input type="checkbox" style={m.checkbox} checked={!!form.clear_api_key}
                            onChange={(e) => update({ clear_api_key: e.target.checked, api_key: '' })} data-testid="typesafe-clear-key" />
                        <span style={m.toggleLabel}>Remove the stored key</span>
                    </label>
                )}
            </div>

            <div style={m.field}>
                <label style={m.fieldLabel} htmlFor="typesafe-model">Model</label>
                <input id="typesafe-model" type="text" value={form.model} disabled={!isAdmin} data-testid="typesafe-model"
                    onChange={(e) => update({ model: e.target.value })} />
                <div style={m.currentKeyNote}>Pinned by default (jev-1.13.0). Confidence thresholds are tuned per version; move deliberately.</div>
            </div>

            <div style={m.field}>
                <label style={m.fieldLabel} htmlFor="typesafe-timeout">Timeout (seconds)</label>
                <input id="typesafe-timeout" type="number" min={TIMEOUT_MIN} max={TIMEOUT_MAX} value={form.timeout_seconds} disabled={!isAdmin}
                    data-testid="typesafe-timeout" onChange={(e) => update({ timeout_seconds: Number(e.target.value) })} />
            </div>

            {[
                ['verdict_engine_enabled', 'Use for failure verdicts', 'TypeSafe decides the verdict and the suggested defect type; your LLM writes the explanation.'],
                ['semantic_dedup_enabled', 'Semantic failure grouping', 'Merge failure groups that share a cause even when the error text differs.'],
                ['allow_auto_failure_analysis', 'Allow on automatic analysis', 'Off by default: analyses started by run completion do not contact TypeSafe unless this is on.'],
            ].map(([key, label, desc]) => (
                <label key={key} style={m.toggle}>
                    <input type="checkbox" style={m.checkbox} checked={!!form[key]} disabled={!isAdmin}
                        onChange={(e) => update({ [key]: e.target.checked })} data-testid={`typesafe-${key}`} />
                    <span style={m.toggleLabel}>{label}<span style={{ display: 'block', color: 'var(--text-secondary)', fontSize: 12 }}>{desc}</span></span>
                </label>
            ))}

            {isAdmin && (
                <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginTop: 12, flexWrap: 'wrap' }}>
                    <button className="primary-btn" onClick={save} disabled={saving || !patch || !timeoutValid || !modelValid} data-testid="typesafe-save">
                        {saving ? 'Saving…' : 'Save'}
                    </button>
                    <button onClick={runTest} disabled={testing || settings.api_key_status === 'missing'} data-testid="typesafe-test">
                        {testing ? 'Testing…' : 'Test connection'}
                    </button>
                    {testResult && (
                        <span style={s.testResult} data-testid="typesafe-test-result">
                            {testResult.ok
                                ? `Connected · ${(testResult.models || []).map((model) => model.name).join(', ') || 'no models listed'}`
                                : `Failed (${testResult.category}): ${testResult.message}`}
                        </span>
                    )}
                </div>
            )}
        </section>
    );
}
