import { useState, useEffect, useRef } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { auth } from '../api';
import { useAuth } from '../contexts/AuthContext';
import { useAIGeneration } from '../contexts/AIGenerationContext';

export default function AppNav({ theme, toggleTheme }) {
  const navigate = useNavigate();
  const location = useLocation();
  const { user, refetchUser } = useAuth();
  const aiGen = useAIGeneration();

  // Notification center
  const [notifOpen, setNotifOpen] = useState(false);
  const notifRef = useRef(null);

  // Close notification dropdown on outside click
  useEffect(() => {
    if (!notifOpen) return;
    const handler = (e) => {
      if (notifRef.current && !notifRef.current.contains(e.target)) setNotifOpen(false);
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [notifOpen]);

  return (
    <header className="app-header tt-header-live" style={{ display: location.pathname === '/login' ? 'none' : undefined, padding: '0 20px', position: 'relative' }}>
      {/* ── Left spacer (grid column 1) ── */}
      <div />
      {/* ── Nav (grid column 2 — always centered) ── */}
      <nav style={{ display: 'flex', alignItems: 'center', gap: 2, padding: '0 10px' }}>
        {[
          {
            path: '/library', label: 'Tests',
            isActive: location.pathname === '/library' || location.pathname.startsWith('/library/folders/') || location.pathname.startsWith('/library/tests/'),
            icon: <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polyline points="9 11 12 14 22 4"/><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/></svg>,
          },
          {
            path: '/runs', label: 'Runs',
            isActive: location.pathname.startsWith('/runs'),
            icon: <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polygon points="5 3 19 12 5 21 5 3"/></svg>,
          },
          {
            path: '/requirements', label: 'Quality',
            isActive: location.pathname.startsWith('/requirements') || location.pathname === '/defects' || location.pathname === '/categories' || location.pathname === '/traceability' || location.pathname === '/analytics',
            icon: <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><polyline points="9 12 11 14 15 10"/></svg>,
          },
          {
            path: '/ai-generate', label: 'AI Gen',
            isActive: location.pathname === '/ai-generate',
            icon: <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>,
            badge: aiGen.generating || aiGen.pendingCount > 0,
          },
          {
            path: '/settings', label: 'Settings',
            isActive: location.pathname === '/settings',
            icon: <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>,
          },
          {
            path: '/help', label: 'Help',
            isActive: location.pathname === '/help',
            icon: <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="10"/><path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>,
          },
        ].filter(item => item.path !== '/ai-generate' || aiGen.aiFeaturesEnabled)
         .map(({ path, label, isActive, icon, badge }) => (
          <button
            key={path}
            onClick={() => navigate(path)}
            className={isActive ? 'tt-nav-active' : 'tt-nav-btn'}
            style={{
              display: 'flex', alignItems: 'center', gap: 5,
              padding: '5px 9px',
              borderRadius: 7,
              border: 'none',
              position: 'relative',
              background: isActive
                ? 'linear-gradient(135deg, #6366f1 0%, #4f46e5 100%)'
                : 'transparent',
              color: isActive ? '#fff' : 'var(--text-secondary)',
              fontSize: '0.82rem',
              fontWeight: isActive ? 600 : 400,
              cursor: 'pointer',
              transition: 'all 0.18s cubic-bezier(0.16, 1, 0.3, 1)',
              fontFamily: 'inherit',
              whiteSpace: 'nowrap',
              boxShadow: isActive
                ? '0 2px 14px rgba(99,102,241,0.45), inset 0 1px 0 rgba(255,255,255,0.18)'
                : 'none',
              transform: isActive ? 'translateY(-1px)' : 'none',
            }}
          >
            {icon}
            {label}
            {badge && !isActive && (
              <span style={{
                position: 'absolute', top: 3, right: 3,
                width: 7, height: 7, borderRadius: '50%',
                background: aiGen.generating ? '#818cf8' : '#4ade80',
                boxShadow: `0 0 6px ${aiGen.generating ? 'rgba(129,140,248,0.6)' : 'rgba(74,222,128,0.6)'}`,
                animation: aiGen.generating ? 'aiDotPulse 1.5s ease-in-out infinite' : 'none',
              }} />
            )}
          </button>
        ))}
      </nav>

      {/* ── Utility (grid column 3 — right-aligned) ── */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexShrink: 0, justifySelf: 'end' }}>
        {/* ── Notification center ── */}
        {(() => {
          if (!aiGen.aiFeaturesEnabled) return null;
          const hasGen = aiGen.generating || aiGen.pendingCount > 0;
          const importVisible = aiGen.importReviewDrafts.filter(d => !aiGen.importDiscardedIds.has(d.temp_id));
          const importPending = importVisible.filter(d => !aiGen.importAcceptedIds.has(d.temp_id)).length;
          const importReady = aiGen.importAcceptedIds.size;
          const hasImport = importVisible.length > 0;
          const totalCount = (hasGen ? aiGen.pendingCount : 0) + (hasImport ? importVisible.length : 0);
          if (!hasGen && !hasImport) return null;
          const hasAction = aiGen.pendingCount > 0 || importPending > 0;
          return (
            <div ref={notifRef} style={{ position: 'relative', flexShrink: 0 }}>
              <button
                onClick={() => setNotifOpen(o => !o)}
                className={`tt-util-btn${aiGen.generating ? ' tt-ai-indicator-pulse' : ''}`}
                style={{
                  position: 'relative',
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                  width: 34, height: 34,
                  borderRadius: 9,
                  border: '1px solid',
                  borderColor: hasAction ? 'rgba(251,146,60,0.35)' : 'rgba(255,255,255,0.08)',
                  background: hasAction ? 'rgba(251,146,60,0.06)' : 'rgba(255,255,255,0.04)',
                  color: aiGen.generating ? '#818cf8' : hasAction ? '#fb923c' : 'var(--text-secondary)',
                  cursor: 'pointer',
                  transition: 'all 0.18s',
                  padding: 0,
                }}
              >
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/>
                </svg>
                {totalCount > 0 && (
                  <span style={{
                    position: 'absolute', top: -4, right: -4,
                    minWidth: 16, height: 16, lineHeight: '16px',
                    borderRadius: 8, padding: '0 4px',
                    fontSize: '0.62rem', fontWeight: 700,
                    textAlign: 'center',
                    background: aiGen.generating ? '#6366f1' : hasAction ? '#fb923c' : '#22c55e',
                    color: '#fff',
                    boxShadow: '0 1px 4px rgba(0,0,0,0.3)',
                  }}>
                    {totalCount}
                  </span>
                )}
              </button>

              {notifOpen && (
                <div className="tt-notif-dropdown" style={{
                  position: 'absolute', top: 'calc(100% + 8px)', right: 0,
                  width: 280, borderRadius: 12, overflow: 'hidden',
                  background: 'var(--bg-primary)',
                  border: '1px solid var(--border-color)',
                  boxShadow: '0 12px 40px rgba(0,0,0,0.4)',
                  zIndex: 1000,
                }}>
                  <div style={{
                    padding: '10px 14px 8px', fontSize: '0.72rem', fontWeight: 700,
                    color: 'var(--text-secondary)', textTransform: 'uppercase',
                    letterSpacing: '0.06em', borderBottom: '1px solid var(--border-color)',
                  }}>
                    Draft Test Cases
                  </div>

                  {hasGen && (
                    <button
                      onClick={() => { setNotifOpen(false); navigate('/ai-generate'); }}
                      className="tt-notif-item"
                      style={{
                        display: 'flex', alignItems: 'center', gap: 10,
                        width: '100%', padding: '11px 14px',
                        background: 'none', border: 'none', borderBottom: '1px solid var(--border-color)',
                        cursor: 'pointer', textAlign: 'left', fontFamily: 'inherit',
                        transition: 'background 0.12s',
                      }}
                    >
                      <div style={{
                        width: 30, height: 30, borderRadius: 8,
                        display: 'flex', alignItems: 'center', justifyContent: 'center',
                        background: aiGen.generating ? 'rgba(99,102,241,0.12)' : 'rgba(34,197,94,0.1)',
                        color: aiGen.generating ? '#818cf8' : '#4ade80',
                        flexShrink: 0,
                      }}>
                        {aiGen.generating ? (
                          <span style={{
                            display: 'inline-block', width: 14, height: 14,
                            border: '2px solid rgba(129,140,248,0.3)', borderTopColor: '#818cf8',
                            borderRadius: '50%', animation: 'spin 0.7s linear infinite',
                          }} />
                        ) : (
                          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M12 2L2 7l10 5 10-5-10-5z"/><path d="M2 17l10 5 10-5"/><path d="M2 12l10 5 10-5"/>
                          </svg>
                        )}
                      </div>
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <div style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-primary)' }}>
                          {aiGen.generating ? 'Generating…' : 'AI Generated'}
                        </div>
                        <div style={{ fontSize: '0.74rem', color: 'var(--text-secondary)', marginTop: 1 }}>
                          {aiGen.generating
                            ? 'AI generation in progress'
                            : `${aiGen.pendingCount} draft${aiGen.pendingCount !== 1 ? 's' : ''} ready for review`}
                        </div>
                      </div>
                      <span style={{
                        minWidth: 22, height: 22, lineHeight: '22px',
                        borderRadius: 6, textAlign: 'center',
                        fontSize: '0.74rem', fontWeight: 700,
                        background: aiGen.generating ? 'rgba(99,102,241,0.15)' : 'rgba(34,197,94,0.12)',
                        color: aiGen.generating ? '#818cf8' : '#4ade80',
                      }}>
                        {aiGen.pendingCount}
                      </span>
                    </button>
                  )}

                  {hasImport && (
                    <button
                      onClick={() => { setNotifOpen(false); navigate('/ai-generate'); }}
                      className="tt-notif-item"
                      style={{
                        display: 'flex', alignItems: 'center', gap: 10,
                        width: '100%', padding: '11px 14px',
                        background: 'none', border: 'none', borderBottom: '1px solid var(--border-color)',
                        cursor: 'pointer', textAlign: 'left', fontFamily: 'inherit',
                        transition: 'background 0.12s',
                      }}
                    >
                      <div style={{
                        width: 30, height: 30, borderRadius: 8,
                        display: 'flex', alignItems: 'center', justifyContent: 'center',
                        background: importPending > 0 ? 'rgba(251,146,60,0.1)' : 'rgba(34,197,94,0.1)',
                        color: importPending > 0 ? '#fb923c' : '#4ade80',
                        flexShrink: 0,
                      }}>
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                          <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/>
                        </svg>
                      </div>
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <div style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-primary)' }}>
                          Imported
                        </div>
                        <div style={{ fontSize: '0.74rem', color: 'var(--text-secondary)', marginTop: 1 }}>
                          {importPending > 0
                            ? `${importPending} draft${importPending !== 1 ? 's' : ''} to review`
                            : `${importReady} ready to import`}
                        </div>
                      </div>
                      <span style={{
                        minWidth: 22, height: 22, lineHeight: '22px',
                        borderRadius: 6, textAlign: 'center',
                        fontSize: '0.74rem', fontWeight: 700,
                        background: importPending > 0 ? 'rgba(251,146,60,0.12)' : 'rgba(34,197,94,0.12)',
                        color: importPending > 0 ? '#fb923c' : '#4ade80',
                      }}>
                        {importVisible.length}
                      </span>
                    </button>
                  )}

                  <div style={{ padding: '8px 14px', textAlign: 'center' }}>
                    <button
                      onClick={() => { setNotifOpen(false); navigate('/ai-generate'); }}
                      style={{
                        background: 'none', border: 'none', color: '#818cf8',
                        fontSize: '0.78rem', fontWeight: 600, cursor: 'pointer',
                        fontFamily: 'inherit', padding: '2px 0',
                      }}
                    >
                      Open AI Test Cases →
                    </button>
                  </div>
                </div>
              )}
            </div>
          );
        })()}

        {/* Theme toggle */}
        <button
          onClick={toggleTheme}
          title={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`}
          className="tt-util-btn"
          style={{
            width: 34, height: 34,
            borderRadius: 9,
            border: '1px solid rgba(255,255,255,0.08)',
            background: 'rgba(255,255,255,0.04)',
            color: 'var(--text-secondary)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            cursor: 'pointer',
            transition: 'all 0.18s cubic-bezier(0.16, 1, 0.3, 1)',
            flexShrink: 0,
          }}
        >
          {theme === 'dark' ? (
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/>
            </svg>
          ) : (
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
            </svg>
          )}
        </button>

        {/* Divider */}
        <div style={{ width: 1, height: 22, background: 'rgba(255,255,255,0.08)', margin: '0 6px', flexShrink: 0 }} />

        {/* Auth section */}
        {user ? (
          <div style={{ display: 'flex', alignItems: 'center', gap: 9 }}>
            {/* Avatar */}
            <div style={{
              width: 32, height: 32, borderRadius: '50%',
              background: 'linear-gradient(135deg, #6366f1, #14b8a6)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontSize: '0.72rem', fontWeight: 700, color: '#fff',
              flexShrink: 0, userSelect: 'none',
              boxShadow: '0 0 0 2px var(--bg-primary), 0 0 0 3.5px rgba(99,102,241,0.55)',
            }}>
              {(user.display_name || user.email || '?')
                .split(/[\s@.]+/).slice(0, 2)
                .map(w => w[0]?.toUpperCase()).join('')}
            </div>
            {/* Name */}
            <span style={{
              fontSize: '0.82rem', fontWeight: 500,
              color: 'var(--text-primary)',
              maxWidth: 90, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
            }}>
              {user.display_name || user.email?.split('@')[0]}
            </span>
            {user.role === 'admin' && (
              <span style={{
                fontSize: '0.63rem', fontWeight: 700,
                color: '#a5b4fc',
                background: 'rgba(99,102,241,0.18)',
                border: '1px solid rgba(99,102,241,0.35)',
                padding: '2px 7px', borderRadius: 5,
                textTransform: 'uppercase', letterSpacing: '0.06em',
                flexShrink: 0,
              }}>
                Admin
              </span>
            )}
            {/* Sign out */}
            <button
              onClick={async () => { await auth.logout(); await refetchUser(); navigate('/login'); }}
              title="Sign out"
              className="tt-util-btn"
              style={{
                width: 32, height: 32,
                borderRadius: 8,
                border: '1px solid rgba(255,255,255,0.08)',
                background: 'rgba(255,255,255,0.04)',
                color: 'var(--text-secondary)',
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                cursor: 'pointer', transition: 'all 0.18s cubic-bezier(0.16, 1, 0.3, 1)', flexShrink: 0,
              }}
            >
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/>
              </svg>
            </button>
          </div>
        ) : (
          <button
            className="primary-btn"
            style={{ padding: '5px 14px', fontSize: '0.82rem' }}
            onClick={() => navigate('/login')}
          >
            Sign In
          </button>
        )}
      </div>

      <style>{`
        .tt-header-live::after {
          content: '';
          position: absolute;
          bottom: 0; left: 0; right: 0;
          height: 1px;
          background: linear-gradient(90deg,
            transparent 0%,
            rgba(99,102,241,0.35) 15%,
            #6366f1 35%,
            #818cf8 48%,
            #14b8a6 62%,
            rgba(20,184,166,0.35) 85%,
            transparent 100%
          );
          pointer-events: none;
        }
        .tt-nav-btn:hover {
          background: rgba(255,255,255,0.07) !important;
          color: var(--text-primary) !important;
          transform: translateY(-1px) !important;
        }
        .tt-nav-active:hover {
          filter: brightness(1.12);
          transform: translateY(-2px) !important;
          box-shadow: 0 5px 20px rgba(99,102,241,0.55), inset 0 1px 0 rgba(255,255,255,0.2) !important;
        }
        .tt-util-btn:hover {
          background: rgba(255,255,255,0.09) !important;
          color: var(--text-primary) !important;
          border-color: rgba(255,255,255,0.14) !important;
          transform: translateY(-1px) !important;
        }
        @keyframes spin { to { transform: rotate(360deg); } }
        @keyframes aiPulse {
          0%, 100% { box-shadow: 0 0 0 0 rgba(99,102,241,0.3); }
          50% { box-shadow: 0 0 0 6px rgba(99,102,241,0); }
        }
        .tt-ai-indicator-pulse {
          animation: aiPulse 2s ease-in-out infinite;
        }
        .tt-notif-item:hover {
          background: rgba(255,255,255,0.04) !important;
        }
        @keyframes aiDotPulse {
          0%, 100% { opacity: 1; transform: scale(1); }
          50% { opacity: 0.5; transform: scale(0.7); }
        }
      `}</style>
    </header>
  );
}
