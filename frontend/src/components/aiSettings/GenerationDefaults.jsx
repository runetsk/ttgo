import React, { useState, useEffect, useCallback } from 'react';
import { aiGeneration } from '../../api';
import { toast } from '../../toast';
import { s } from './styles';

/* ── Coverage Token Limits Section ── */
export default function GenerationDefaults({ isAdmin }) {
    const [coverageCfg, setCoverageCfg] = useState(null);
    const [coverageForm, setCoverageForm] = useState({ essential_max_tokens: 4096, thorough_max_tokens: 8192, comprehensive_max_tokens: 16384 });
    const [savingCoverage, setSavingCoverage] = useState(false);

    const loadCoverageConfig = useCallback(() => {
        aiGeneration.getCoverageConfig()
            .then(cfg => { setCoverageCfg(cfg); setCoverageForm({ essential_max_tokens: cfg.essential_max_tokens, thorough_max_tokens: cfg.thorough_max_tokens, comprehensive_max_tokens: cfg.comprehensive_max_tokens }); })
            .catch(() => {});
    }, []);

    useEffect(() => {
        loadCoverageConfig();
    }, [loadCoverageConfig]);

    const coverageModified = coverageCfg && (
        coverageForm.essential_max_tokens !== coverageCfg.essential_max_tokens ||
        coverageForm.thorough_max_tokens !== coverageCfg.thorough_max_tokens ||
        coverageForm.comprehensive_max_tokens !== coverageCfg.comprehensive_max_tokens
    );

    const handleSaveCoverage = async () => {
        setSavingCoverage(true);
        try {
            const cfg = await aiGeneration.updateCoverageConfig(coverageForm);
            setCoverageCfg(cfg);
            setCoverageForm({ essential_max_tokens: cfg.essential_max_tokens, thorough_max_tokens: cfg.thorough_max_tokens, comprehensive_max_tokens: cfg.comprehensive_max_tokens });
            toast.success('Coverage token limits saved');
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Failed to save coverage config');
        } finally {
            setSavingCoverage(false);
        }
    };

    return (
        <section style={s.section}>
            <div style={s.sectionHead}>
                <div style={s.sectionHeadLeft}>
                    <span style={s.sectionDot} />
                    <h4 style={s.sectionTitle}>Coverage Token Limits</h4>
                    {coverageModified && <span style={s.modifiedBadge}>Unsaved changes</span>}
                </div>
                {isAdmin && (
                    <button
                        className="primary-btn"
                        onClick={handleSaveCoverage}
                        disabled={savingCoverage || !coverageModified}
                        style={{ fontSize: '0.82rem' }}
                    >
                        {savingCoverage ? 'Saving…' : 'Save'}
                    </button>
                )}
            </div>
            <p style={s.templateDesc}>
                Maximum tokens the LLM can use per coverage level. Higher values allow more test cases but cost more.
            </p>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 12, marginTop: 8 }}>
                {[
                    { key: 'essential_max_tokens', label: 'Essential', defaultVal: 4096 },
                    { key: 'thorough_max_tokens', label: 'Thorough', defaultVal: 8192 },
                    { key: 'comprehensive_max_tokens', label: 'Comprehensive', defaultVal: 16384 },
                ].map(({ key, label, defaultVal }) => (
                    <div key={key} style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                        <label style={{ fontSize: '0.78rem', color: 'var(--text-secondary)', fontWeight: 500 }}>
                            {label}
                        </label>
                        <input
                            className="modern-input"
                            type="number"
                            min={1024}
                            step={1024}
                            value={coverageForm[key]}
                            onChange={e => setCoverageForm(prev => ({ ...prev, [key]: parseInt(e.target.value, 10) || defaultVal }))}
                            disabled={!isAdmin}
                            style={{ padding: '8px 10px', fontSize: '0.85rem' }}
                        />
                    </div>
                ))}
            </div>
        </section>
    );
}
