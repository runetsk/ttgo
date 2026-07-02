import { useEffect } from 'react';
import { AIC, pageStyles } from './constants';

export function ImportModal({ onClose, hasDrafts, children }) {
    useEffect(() => {
        const onKey = (e) => { if (e.key === 'Escape') onClose(); };
        window.addEventListener('keydown', onKey);
        document.body.style.overflow = 'hidden';
        return () => {
            window.removeEventListener('keydown', onKey);
            document.body.style.overflow = '';
        };
    }, [onClose]);
    return (
        <div style={pageStyles.importModalBackdrop} onClick={onClose}>
            <div style={pageStyles.importModalCard} onClick={e => e.stopPropagation()}>
                <div style={pageStyles.importModalHead}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 0 }}>
                        <div style={{ ...pageStyles.aiIcon, width: 32, height: 32, background: 'linear-gradient(135deg, rgba(20,184,166,0.2), rgba(56,189,248,0.15))', borderColor: 'rgba(20,184,166,0.25)', color: '#14b8a6' }}>
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/>
                            </svg>
                        </div>
                        <div>
                            <h3 style={pageStyles.importModalTitle}>{hasDrafts ? 'Review imported drafts' : 'Import existing test cases'}</h3>
                            <p style={pageStyles.importModalSub}>Paste, upload, or drag AI-generated content — JSON, CSV, Markdown, or numbered lists.</p>
                        </div>
                    </div>
                    <button onClick={onClose} style={pageStyles.importModalClose} aria-label="Close import">
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
                        </svg>
                    </button>
                </div>
                <div style={pageStyles.importModalBody}>{children}</div>
            </div>
        </div>
    );
}

export function StudioBanner({ tone = 'amber', icon, children, action }) {
    const toneMap = {
        amber: { bg: 'rgba(234,179,8,0.08)', bd: 'rgba(234,179,8,0.3)', fg: '#fde047' },
        red: { bg: 'rgba(239,68,68,0.08)', bd: 'rgba(239,68,68,0.3)', fg: '#fca5a5' },
    };
    const t = toneMap[tone] || toneMap.amber;
    return (
        <div style={{
            margin: '0 0 10px', padding: '8px 14px',
            background: t.bg, border: `1px solid ${t.bd}`, borderRadius: 8,
            display: 'flex', alignItems: 'center', gap: 10,
            fontSize: 12.5, color: t.fg,
        }}>
            <span style={{ flexShrink: 0, display: 'inline-flex' }}>{icon}</span>
            <span style={{ flex: 1 }}>{children}</span>
            {action}
        </div>
    );
}

// ── Injected styles for Studio-specific rules ───────────────────────────────
export function StudioStyles() {
    return (
        <style>{`
            .aig-studio-grid {
                position: relative;
                display: grid;
                grid-template-columns: var(--aig-left, 300px) minmax(0, 1fr) var(--aig-right, 380px);
                transition: grid-template-columns .18s ease;
                border-top: 1px solid var(--border-color);
                background: var(--bg-primary);
            }
            .aig-studio-grid.is-dragging {
                transition: none;
                user-select: none;
            }
            .aig-studio-grid.collapse-left  { grid-template-columns: 40px minmax(0, 1fr) var(--aig-right, 380px) !important; }
            .aig-studio-grid.collapse-right { grid-template-columns: var(--aig-left, 300px) minmax(0, 1fr) 40px !important; }
            .aig-studio-grid.collapse-left.collapse-right { grid-template-columns: 40px minmax(0, 1fr) 40px !important; }
            @media (max-width: 960px) {
                .aig-studio-grid { grid-template-columns: minmax(0, 1fr) !important; }
                .aig-studio-grid > aside { height: auto !important; border-bottom: 1px solid ${AIC.border}; }
                .aig-resizer { display: none !important; }
            }

            .aig-resizer {
                position: absolute;
                top: 0; bottom: 0;
                width: 6px;
                cursor: col-resize;
                z-index: 5;
                background: transparent;
                touch-action: none;
            }
            .aig-resizer::after {
                content: '';
                position: absolute;
                top: 0; bottom: 0;
                left: 50%; width: 1px; margin-left: -0.5px;
                background: transparent;
                transition: background .14s ease;
            }
            .aig-resizer:hover::after,
            .aig-resizer.is-active::after {
                background: linear-gradient(180deg, transparent 0%, ${AIC.indigo} 25%, ${AIC.teal} 75%, transparent 100%);
                box-shadow: 0 0 8px rgba(99,102,241,0.5);
            }
            .aig-resizer-left  { left: calc(var(--aig-left, 300px) - 3px); }
            .aig-resizer-right { right: calc(var(--aig-right, 380px) - 3px); }
            .aig-studio-grid.collapse-left  .aig-resizer-left  { display: none; }
            .aig-studio-grid.collapse-right .aig-resizer-right { display: none; }

            .aig-pane-rail {
                width: 40px; height: var(--aig-studio-h, calc(100vh - 180px));
                display: flex; flex-direction: column; align-items: center;
                padding: 10px 0; gap: 14px;
                background: var(--aig-surface-tint);
            }
            .aig-rail-btn {
                width: 26px; height: 26px; border-radius: 6px;
                background: var(--aig-surface-tint);
                border: 1px solid var(--border-color);
                color: var(--text-secondary);
                display: inline-flex; align-items: center; justify-content: center;
                cursor: pointer;
            }
            .aig-rail-btn:hover { background: var(--aig-surface-tint-strong); color: var(--text-primary); }
            .aig-rail-label {
                writing-mode: vertical-rl; transform: rotate(180deg);
                font-size: 10.5px; color: var(--sidebar-muted); letter-spacing: 0.1em;
                text-transform: uppercase; font-weight: 600; user-select: none;
            }
            .aig-pane-collapse-btn {
                width: 22px; height: 22px; border-radius: 5px;
                background: transparent; border: 1px solid transparent;
                color: var(--sidebar-muted);
                display: inline-flex; align-items: center; justify-content: center;
                cursor: pointer; transition: all .12s;
            }
            .aig-pane-collapse-btn:hover {
                background: var(--aig-surface-hover); color: var(--text-primary);
                border-color: var(--border-color);
            }

            .aig-gradient-text {
                background: linear-gradient(100deg, #a5b4fc, #5eead4 45%, #a5b4fc);
                background-size: 200% 100%;
                -webkit-background-clip: text; background-clip: text; color: transparent;
                animation: aigShimmer 3.5s linear infinite;
            }
            .aig-progress-shuttle { animation: aigShuttle 1.4s cubic-bezier(0.4, 0, 0.6, 1) infinite; }

            @keyframes aigShimmer { to { background-position: -200% 0; } }
            @keyframes aigShuttle {
                0% { left: -40%; }
                100% { left: 100%; }
            }
            @keyframes aigAiBlip {
                0%, 80%, 100% { opacity: .35; transform: translateY(0); }
                40% { opacity: 1; transform: translateY(-2px); }
            }
            @keyframes aigSlideUp {
                from { opacity: 0; transform: translateY(8px); }
                to { opacity: 1; transform: none; }
            }
        `}</style>
    );
}

// ── Page shell styles (formerly lived in AIGeneratePage) ────────────────────
export function PageShellStyles() {
    return (
        <style>{`
            @keyframes spin { to { transform: rotate(360deg); } }
            .aigen-back-link:hover { color: var(--text-primary) !important; }
            .aigen-req-switcher-btn:not(:disabled):hover { opacity: 0.85; }
            .aigen-switcher-row:hover { background: rgba(99,102,241,0.07) !important; }
            .aigen-switcher-create:hover { background: rgba(99,102,241,0.1) !important; }
            .aigen-empty-req-row:hover { background: rgba(99,102,241,0.07) !important; }
            .aigen-empty-req-row:last-child { border-bottom: none !important; }
            .aigen-manage-reqs-link:hover { color: var(--text-primary) !important; }
            .aigen-path-card:hover {
                border-color: rgba(99,102,241,0.3) !important;
                box-shadow: 0 4px 20px rgba(99,102,241,0.08) !important;
            }
            .aigen-import-cta:hover {
                background: rgba(20,184,166,0.15) !important;
                border-color: rgba(20,184,166,0.5) !important;
                transform: translateY(-1px);
                box-shadow: 0 4px 16px rgba(20,184,166,0.15);
            }
            .aigen-create-req-btn:hover {
                filter: brightness(1.1);
                transform: translateY(-1px);
            }
            .aigen-create-submit-btn:not(:disabled):hover { filter: brightness(1.1); }
            .aigen-modal-close:hover {
                color: var(--text-primary) !important;
                background: var(--aig-surface-hover) !important;
            }
        `}</style>
    );
}
