import React from 'react';
import { presetFromConfig } from './constants';
import { s } from './styles';

/* ── Provider Card ─────────────────────────────────── */
export default function ProviderCard({ provider, isAdmin, testingId, testResult, onTest, onSetDefault, onEdit, onDelete }) {
    const meta = presetFromConfig(provider);
    const isTesting = testingId === provider.id;

    return (
        <div style={{
            ...s.providerCard,
            borderLeftColor: provider.enabled ? meta.color : 'var(--border-color)',
        }}>
            <div style={s.cardMain}>
                {/* Left: icon + name + badges */}
                <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12, flex: 1, minWidth: 0 }}>
                    <div style={{ ...s.providerAvatar, color: meta.color, background: meta.bg }}>
                        {meta.initial}
                    </div>
                    <div style={{ flex: 1, minWidth: 0 }}>
                        <div style={s.providerNameRow}>
                            <span style={s.providerName}>{provider.label}</span>
                            {provider.is_default && <span style={s.defaultBadge}>★ Default</span>}
                            {!provider.enabled && <span style={s.disabledBadge}>Disabled</span>}
                        </div>
                        <div style={s.providerMeta}>
                            <span style={s.metaChip}>{meta.label}</span>
                            <span style={s.metaDot}>·</span>
                            <span style={{ ...s.metaChip, fontFamily: 'monospace' }}>{provider.model_name}</span>
                            {provider.timeout_seconds && (
                                <>
                                    <span style={s.metaDot}>·</span>
                                    <span style={s.metaChip}>{provider.timeout_seconds}s</span>
                                </>
                            )}
                            {provider.api_key_masked && (
                                <>
                                    <span style={s.metaDot}>·</span>
                                    <span style={{ ...s.metaChip, fontFamily: 'monospace', opacity: 0.7 }}>key: {provider.api_key_masked}</span>
                                </>
                            )}
                        </div>

                        {/* Test result */}
                        {testResult && (
                            <div style={{
                                ...s.testResult,
                                background: testResult.success ? 'rgba(34,197,94,0.07)' : 'rgba(239,68,68,0.07)',
                                borderColor: testResult.success ? 'rgba(34,197,94,0.25)' : 'rgba(239,68,68,0.25)',
                                color: testResult.success ? '#4ade80' : '#f87171',
                            }}>
                                {testResult.success ? '✓ Connection OK' : `✗ ${testResult.error}`}
                            </div>
                        )}
                    </div>
                </div>

                {/* Right: actions */}
                {isAdmin && (
                    <div style={s.cardActions}>
                        <button
                            style={{ ...s.iconBtn, ...(isTesting ? s.iconBtnActive : {}) }}
                            onClick={() => onTest(provider)}
                            disabled={isTesting}
                            title="Test connection"
                        >
                            {isTesting ? (
                                <span style={s.miniSpinner} />
                            ) : (
                                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                    <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/>
                                </svg>
                            )}
                            <span style={s.iconBtnLabel}>{isTesting ? 'Testing…' : 'Test'}</span>
                        </button>

                        {!provider.is_default && (
                            <button style={s.iconBtn} onClick={() => onSetDefault(provider)} title="Set as default">
                                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                    <polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/>
                                </svg>
                                <span style={s.iconBtnLabel}>Default</span>
                            </button>
                        )}

                        <button style={s.iconBtn} onClick={() => onEdit(provider)} title="Edit provider">
                            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                            </svg>
                            <span style={s.iconBtnLabel}>Edit</span>
                        </button>

                        <button style={{ ...s.iconBtn, ...s.iconBtnDanger }} onClick={() => onDelete(provider)} title="Delete provider">
                            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                <polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/>
                            </svg>
                            <span style={s.iconBtnLabel}>Delete</span>
                        </button>
                    </div>
                )}
            </div>
        </div>
    );
}
