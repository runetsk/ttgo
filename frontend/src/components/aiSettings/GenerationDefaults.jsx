import React, { useState, useEffect, useCallback } from 'react';
import { aiGeneration } from '../../api';
import { saveError } from '../../utils/saveBar';
import { useSaveSection } from './saveBarContext';
import { s } from './styles';

const LEVELS = [
    { key: 'essential_max_tokens', label: 'Essential', defaultVal: 4096 },
    { key: 'thorough_max_tokens', label: 'Thorough', defaultVal: 8192 },
    { key: 'comprehensive_max_tokens', label: 'Comprehensive', defaultVal: 16384 },
];
const NO_ERRORS = [];
const pickCoverage = (cfg) => ({
    essential_max_tokens: cfg.essential_max_tokens,
    thorough_max_tokens: cfg.thorough_max_tokens,
    comprehensive_max_tokens: cfg.comprehensive_max_tokens,
});

/* ── Coverage Token Limits Section (saves through the Settings → AI save bar) ── */
export default function GenerationDefaults({ isAdmin }) {
    const [coverageCfg, setCoverageCfg] = useState(null);
    const [coverageForm, setCoverageForm] = useState({ essential_max_tokens: 4096, thorough_max_tokens: 8192, comprehensive_max_tokens: 16384 });

    const loadCoverageConfig = useCallback(() => {
        aiGeneration.getCoverageConfig()
            .then(cfg => { setCoverageCfg(cfg); setCoverageForm(pickCoverage(cfg)); })
            .catch(() => {});
    }, []);

    useEffect(() => {
        loadCoverageConfig();
    }, [loadCoverageConfig]);

    const coverageModified = !!coverageCfg && LEVELS.some(({ key }) => coverageForm[key] !== coverageCfg[key]);

    const saveCoverage = async () => {
        let cfg;
        try {
            cfg = await aiGeneration.updateCoverageConfig(coverageForm);
        } catch (err) {
            throw saveError(err, 'Failed to save the token limits');
        }
        setCoverageCfg(cfg);
        setCoverageForm(pickCoverage(cfg));
    };
    const saving = useSaveSection('coverage', {
        label: 'Output tokens per coverage level', tab: 'limits', dirty: coverageModified, errors: NO_ERRORS,
        save: saveCoverage,
        discard: () => { if (coverageCfg) setCoverageForm(pickCoverage(coverageCfg)); },
    });

    return (
        <section style={s.section}>
            <div style={s.sectionHead}>
                <div style={s.sectionHeadLeft}>
                    <span style={s.sectionDot} />
                    <h4 style={s.sectionTitle}>Output tokens per coverage level</h4>
                    {coverageModified && <span style={s.modifiedBadge}>Unsaved changes</span>}
                </div>
            </div>
            <p style={s.templateDesc}>
                The most the model may write for one generation. Higher limits allow more test cases and cost more.
            </p>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 12, marginTop: 8 }}>
                {LEVELS.map(({ key, label, defaultVal }) => (
                    <div key={key} style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                        <label htmlFor={`coverage-${key}`} style={{ fontSize: '0.78rem', color: 'var(--text-secondary)', fontWeight: 500 }}>
                            {label}
                        </label>
                        <input
                            id={`coverage-${key}`}
                            data-testid={`coverage-${key}`}
                            className="modern-input"
                            type="number"
                            min={1024}
                            step={1024}
                            value={coverageForm[key]}
                            onChange={e => setCoverageForm(prev => ({ ...prev, [key]: parseInt(e.target.value, 10) || defaultVal }))}
                            disabled={!isAdmin || saving}
                            style={{ padding: '8px 10px', fontSize: '0.85rem' }}
                        />
                    </div>
                ))}
            </div>
        </section>
    );
}
