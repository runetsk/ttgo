import React from 'react';
import { useAuth } from '../contexts/AuthContext';
import { s } from './aiSettings/styles';
import ProviderManager from './aiSettings/ProviderManager';
import GenerationDefaults from './aiSettings/GenerationDefaults';
import TemplateEditor from './aiSettings/TemplateEditor';

/* ── Main Component ────────────────────────────────── */
export default function AIGenSettings() {
    const { user: currentUser } = useAuth();
    const isAdmin = currentUser?.role === 'admin';

    return (
        <div style={s.page}>
            {/* ── Page Header ── */}
            <div style={s.pageHeader}>
                <div style={s.pageHeaderIcon}>
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <path d="M12 2L2 7l10 5 10-5-10-5z"/><path d="M2 17l10 5 10-5"/><path d="M2 12l10 5 10-5"/>
                    </svg>
                </div>
                <div>
                    <h3 style={s.pageTitle}>AI Test Generation</h3>
                    <p style={s.pageDesc}>
                        Configure LLM providers and the prompt template for AI-powered test case generation.
                        API keys are stored server-side and masked in responses.
                    </p>
                </div>
            </div>

            {/* ── LLM Providers Section (+ its add/edit/delete modals) ── */}
            <ProviderManager isAdmin={isAdmin} />

            {/* ── Standard + Parent Prompt Template Sections ── */}
            <TemplateEditor isAdmin={isAdmin} />

            {/* ── Coverage Token Limits Section ── */}
            <GenerationDefaults isAdmin={isAdmin} />

            <style>{`
                @keyframes spin { to { transform: rotate(360deg); } }
                @keyframes aigenSettingsFadeIn {
                    from { opacity: 0; transform: translateY(8px); }
                    to   { opacity: 1; transform: translateY(0); }
                }
                .aigen-icon-btn:hover:not(:disabled) {
                    background: rgba(255,255,255,0.08) !important;
                    color: var(--text-primary) !important;
                }
                .aigen-icon-btn-danger:hover:not(:disabled) {
                    background: rgba(239,68,68,0.1) !important;
                    color: #f87171 !important;
                    border-color: rgba(239,68,68,0.25) !important;
                }
                .aigen-provider-card:hover {
                    border-color: rgba(99,102,241,0.3) !important;
                    background: rgba(99,102,241,0.03) !important;
                }
                .aigen-type-card:hover {
                    border-color: var(--accent-indigo) !important;
                    opacity: 0.9;
                }
            `}</style>
        </div>
    );
}

