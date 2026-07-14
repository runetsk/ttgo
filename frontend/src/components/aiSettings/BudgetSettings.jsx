import React, { useState, useEffect, useCallback } from 'react';
import { aiGeneration } from '../../api';
import { toast } from '../../toast';
import { s } from './styles';

/* ── Soft Cost Budgets Section ── */
export default function BudgetSettings({ isAdmin }) {
    const [perRequest, setPerRequest] = useState('');
    const [monthly, setMonthly] = useState('');
    const [saving, setSaving] = useState(false);

    const loadBudgetSettings = useCallback(() => {
        aiGeneration.getBudgetSettings()
            .then(cfg => {
                setPerRequest(cfg.per_request_usd > 0 ? String(cfg.per_request_usd) : '');
                setMonthly(cfg.monthly_usd > 0 ? String(cfg.monthly_usd) : '');
            })
            .catch(() => {});
    }, []);

    useEffect(() => {
        loadBudgetSettings();
    }, [loadBudgetSettings]);

    const save = async () => {
        setSaving(true);
        try {
            const cfg = await aiGeneration.updateBudgetSettings({
                per_request_usd: perRequest === '' ? 0 : parseFloat(perRequest),
                monthly_usd: monthly === '' ? 0 : parseFloat(monthly),
            });
            setPerRequest(cfg.per_request_usd > 0 ? String(cfg.per_request_usd) : '');
            setMonthly(cfg.monthly_usd > 0 ? String(cfg.monthly_usd) : '');
            toast.success('Budgets saved');
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Failed to save budgets');
        } finally {
            setSaving(false);
        }
    };

    return (
        <section style={s.section}>
            <div style={s.sectionHead}>
                <div style={s.sectionHeadLeft}>
                    <span style={s.sectionDot} />
                    <h4 style={s.sectionTitle}>Soft Cost Budgets</h4>
                </div>
                {isAdmin && (
                    <button
                        className="primary-btn"
                        onClick={save}
                        disabled={saving}
                        style={{ fontSize: '0.82rem' }}
                    >
                        {saving ? 'Saving…' : 'Save'}
                    </button>
                )}
            </div>
            <p style={s.templateDesc}>
                Warnings only — generation asks for confirmation instead of degrading the request.
                Requires provider pricing to be configured. Empty or zero means off.
            </p>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12, marginTop: 8, maxWidth: 420 }}>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                    <label style={{ fontSize: '0.78rem', color: 'var(--text-secondary)', fontWeight: 500 }}>
                        Per request (USD)
                    </label>
                    <input
                        className="modern-input"
                        type="number"
                        min="0"
                        step="0.01"
                        value={perRequest}
                        onChange={e => setPerRequest(e.target.value)}
                        disabled={!isAdmin}
                        style={{ padding: '8px 10px', fontSize: '0.85rem' }}
                    />
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                    <label style={{ fontSize: '0.78rem', color: 'var(--text-secondary)', fontWeight: 500 }}>
                        Per month (USD)
                    </label>
                    <input
                        className="modern-input"
                        type="number"
                        min="0"
                        step="0.5"
                        value={monthly}
                        onChange={e => setMonthly(e.target.value)}
                        disabled={!isAdmin}
                        style={{ padding: '8px 10px', fontSize: '0.85rem' }}
                    />
                </div>
            </div>
        </section>
    );
}
