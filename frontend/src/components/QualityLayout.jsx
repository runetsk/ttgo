import { useState } from 'react';
import { useNavigate, useLocation, Outlet } from 'react-router-dom';

const ITEMS = [
    {
        path: '/requirements', label: 'Requirements',
        match: (p) => p === '/requirements' || p.startsWith('/requirements/'),
        icon: <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/></svg>,
    },
    {
        path: '/traceability', label: 'Traceability',
        match: (p) => p === '/traceability',
        icon: <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/><line x1="8.59" y1="13.51" x2="15.42" y2="17.49"/><line x1="15.41" y1="6.51" x2="8.59" y2="10.49"/></svg>,
    },
    {
        path: '/defects', label: 'Defects',
        match: (p) => p === '/defects',
        icon: <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><rect x="8" y="6" width="8" height="14" rx="4"/><path d="M12 2v4M5 9H2M5 15H2M22 9h-3M22 15h-3M7 4 5 2M17 4l2-2"/></svg>,
    },
    {
        path: '/categories', label: 'Categories',
        match: (p) => p === '/categories',
        icon: <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><rect x="2" y="3" width="20" height="4" rx="1"/><rect x="2" y="10" width="20" height="4" rx="1"/><rect x="2" y="17" width="20" height="4" rx="1"/></svg>,
    },
    {
        path: '/analytics', label: 'Analytics',
        match: (p) => p === '/analytics',
        icon: <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg>,
    },
];

const STORAGE_KEY = 'qnav-collapsed';

function readCollapsed() {
    try {
        return localStorage.getItem(STORAGE_KEY) === '1';
    } catch {
        return false;
    }
}

export default function QualityLayout() {
    const navigate = useNavigate();
    const location = useLocation();
    const [collapsed, setCollapsed] = useState(readCollapsed);

    const toggle = () => setCollapsed((c) => {
        const next = !c;
        try { localStorage.setItem(STORAGE_KEY, next ? '1' : '0'); } catch { /* ignore */ }
        return next;
    });

    return (
        <div style={{ display: 'flex', minHeight: '100%' }}>
            <nav style={{
                width: collapsed ? 52 : 196, flexShrink: 0, padding: collapsed ? '14px 6px' : '14px 10px',
                borderRight: '1px solid var(--border-color)',
                display: 'flex', flexDirection: 'column', gap: 3,
                transition: 'width 0.18s cubic-bezier(0.16,1,0.3,1), padding 0.18s cubic-bezier(0.16,1,0.3,1)',
            }}>
                <button
                    onClick={toggle}
                    className="tt-qnav-btn"
                    aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
                    title={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
                    style={{
                        display: 'flex', alignItems: 'center', justifyContent: collapsed ? 'center' : 'flex-end',
                        padding: '6px 10px', marginBottom: 4, borderRadius: 8, border: 'none',
                        width: '100%', background: 'transparent', color: 'var(--text-secondary)',
                        cursor: 'pointer', fontFamily: 'inherit',
                    }}
                >
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"
                        style={{ transform: collapsed ? 'rotate(180deg)' : 'none', transition: 'transform 0.18s cubic-bezier(0.16,1,0.3,1)' }}>
                        <polyline points="15 18 9 12 15 6"/>
                    </svg>
                </button>

                {ITEMS.map(({ path, label, match, icon }) => {
                    const active = match(location.pathname);
                    return (
                        <button
                            key={path}
                            onClick={() => navigate(path)}
                            className={active ? 'tt-qnav-active' : 'tt-qnav-btn'}
                            title={label}
                            style={{
                                display: 'flex', alignItems: 'center',
                                justifyContent: collapsed ? 'center' : 'flex-start',
                                gap: collapsed ? 0 : 9,
                                padding: '8px 12px', borderRadius: 8, border: 'none',
                                width: '100%', textAlign: 'left',
                                background: active ? 'linear-gradient(135deg, #6366f1 0%, #4f46e5 100%)' : 'transparent',
                                color: active ? '#fff' : 'var(--text-secondary)',
                                fontSize: '0.86rem', fontWeight: active ? 600 : 400,
                                cursor: 'pointer', transition: 'all 0.15s cubic-bezier(0.16,1,0.3,1)',
                                fontFamily: 'inherit', whiteSpace: 'nowrap', overflow: 'hidden',
                                boxShadow: active ? '0 2px 12px rgba(99,102,241,0.4)' : 'none',
                            }}
                        >
                            {icon}
                            {!collapsed && label}
                        </button>
                    );
                })}
            </nav>
            <div style={{ flex: 1, minWidth: 0 }}>
                <Outlet />
            </div>
            <style>{`
                .tt-qnav-btn:hover {
                    background: rgba(255,255,255,0.07) !important;
                    color: var(--text-primary) !important;
                }
                .tt-qnav-active:hover { filter: brightness(1.1); }
            `}</style>
        </div>
    );
}
