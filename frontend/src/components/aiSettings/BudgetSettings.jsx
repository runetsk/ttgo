import React, { useState, useEffect, useCallback } from 'react';
import { aiGeneration } from '../../api';
import { toast } from '../../toast';
import { monthMeter } from '../../utils/aiSettingsTabs';
import { s } from './styles';

const METER_FILL = { neutral: 'var(--accent-indigo)', warn: 'var(--aig-tone-amber-fg)', bad: 'var(--aig-tone-red-fg)' };
const FIELD_LABEL = { display: 'flex', flexDirection: 'column', gap: 4, fontSize: '0.78rem', color: 'var(--text-secondary)', fontWeight: 500 };
const asField = (usd) => (usd > 0 ? String(usd) : '');

/* ── Soft Cost Budgets Section ── */
export default function BudgetSettings({ isAdmin, onStatusChange }) {
    const [saved, setSaved] = useState(null); // { perRequest, monthly, monthlyUsd, spentUsd }
    const [perRequest, setPerRequest] = useState('');
    const [monthly, setMonthly] = useState('');
    const [saving, setSaving] = useState(false);

    const apply = useCallback((cfg) => {
        const next = {
            perRequest: asField(cfg.per_request_usd),
            monthly: asField(cfg.monthly_usd),
            monthlyUsd: cfg.monthly_usd > 0 ? cfg.monthly_usd : 0,
            spentUsd: cfg.month_spent_usd ?? 0,
        };
        setSaved(next);
        setPerRequest(next.perRequest);
        setMonthly(next.monthly);
    }, []);

    useEffect(() => {
        aiGeneration.getBudgetSettings().then(apply).catch(() => {});
    }, [apply]);

    const dirty = !!saved && (perRequest !== saved.perRequest || monthly !== saved.monthly);
    useEffect(() => {
        if (saved) onStatusChange?.({ monthlyUsd: saved.monthlyUsd, spentUsd: saved.spentUsd, dirty });
    }, [saved, dirty, onStatusChange]);

    const save = async () => {
        setSaving(true);
        try {
            apply(await aiGeneration.updateBudgetSettings({
                per_request_usd: perRequest === '' ? 0 : parseFloat(perRequest),
                monthly_usd: monthly === '' ? 0 : parseFloat(monthly),
            }));
            toast.success('Budgets saved');
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Failed to save budgets');
        } finally {
            setSaving(false);
        }
    };

    const meter = saved ? monthMeter(saved.spentUsd, saved.monthlyUsd) : null;

    return (
        <section style={s.section}>
            <div style={s.sectionHead}>
                <div style={s.sectionHeadLeft}>
                    <span style={s.sectionDot} />
                    <h4 style={s.sectionTitle}>Soft cost budgets</h4>
                    {dirty && <span style={s.modifiedBadge}>Unsaved changes</span>}
                </div>
                {isAdmin && (
                    <button className="primary-btn" data-testid="budget-save" onClick={save} disabled={saving || !dirty} style={{ fontSize: '0.82rem' }}>
                        {saving ? 'Saving…' : 'Save'}
                    </button>
                )}
            </div>
            <p style={s.templateDesc}>
                Warnings only: over a budget, generation asks for confirmation instead of cutting the request down.
                Requires provider pricing. Empty or zero means off.
            </p>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12, marginTop: 8, maxWidth: 520 }}>
                <label style={FIELD_LABEL}>
                    Per request (USD)
                    <input className="modern-input" data-testid="budget-per-request" type="number" min="0" step="0.01" value={perRequest}
                        onChange={(e) => setPerRequest(e.target.value)} disabled={!isAdmin} style={{ padding: '8px 10px', fontSize: '0.85rem' }} />
                </label>
                <label style={FIELD_LABEL}>
                    Per month (USD)
                    <input className="modern-input" data-testid="budget-monthly" type="number" min="0" step="0.5" value={monthly}
                        onChange={(e) => setMonthly(e.target.value)} disabled={!isAdmin} style={{ padding: '8px 10px', fontSize: '0.85rem' }} />
                </label>
            </div>
            {saved && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 6, maxWidth: 520 }} data-testid="budget-meter">
                    {meter && (
                        <div role="meter" aria-label="Spend this month" aria-valuemin={0} aria-valuemax={100} aria-valuenow={meter.pct}
                            style={{ height: 6, borderRadius: 3, background: 'var(--bg-tertiary)', overflow: 'hidden' }}>
                            <div style={{ width: `${meter.pct}%`, height: '100%', borderRadius: 3, background: METER_FILL[meter.tone] }} />
                        </div>
                    )}
                    <span style={{ fontSize: '0.78rem', color: 'var(--text-secondary)' }}>
                        ${saved.spentUsd.toFixed(2)} estimated spend this month{meter ? ` of $${saved.monthlyUsd.toFixed(2)}` : ''} · resets on the 1st (UTC)
                    </span>
                </div>
            )}
        </section>
    );
}
