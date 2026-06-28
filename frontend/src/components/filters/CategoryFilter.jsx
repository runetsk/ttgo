import React, { useState, useRef, useEffect } from 'react';

/**
 * CategoryFilter — multi-select popover (match-ANY). Emits selected category IDs.
 * Props: categories [{id,name}], value string[], onChange(ids), testId
 */
export default function CategoryFilter({ categories = [], value = [], onChange, testId }) {
    const [open, setOpen] = useState(false);
    const [search, setSearch] = useState('');
    const triggerRef = useRef(null);
    const popoverRef = useRef(null);
    const selected = new Set(value);

    useEffect(() => {
        if (!open) return;
        const handler = (e) => {
            if (!popoverRef.current?.contains(e.target) && !triggerRef.current?.contains(e.target)) setOpen(false);
        };
        const keyHandler = (e) => { if (e.key === 'Escape') setOpen(false); };
        document.addEventListener('mousedown', handler);
        document.addEventListener('keydown', keyHandler);
        return () => {
            document.removeEventListener('mousedown', handler);
            document.removeEventListener('keydown', keyHandler);
        };
    }, [open]);

    const toggle = (id) => {
        const next = new Set(selected);
        if (next.has(id)) {
            next.delete(id);
        } else {
            next.add(id);
        }
        onChange([...next]);
    };

    const label = value.length === 0 ? 'All categories' : `${value.length} selected`;
    const shown = categories.filter(c => c.name.toLowerCase().includes(search.toLowerCase()));

    return (
        <div style={{ position: 'relative' }}>
            <button
                ref={triggerRef}
                type="button"
                data-testid={testId}
                onClick={() => setOpen(o => !o)}
                className="modern-input"
                style={{
                    width: '100%', fontSize: '0.75rem', padding: '4px 8px', textAlign: 'left', cursor: 'pointer',
                    color: value.length ? 'var(--text-primary)' : 'var(--text-secondary)',
                    borderColor: value.length ? 'var(--accent-indigo)' : undefined,
                }}
            >{label}</button>
            {open && (
                <div
                    ref={popoverRef}
                    role="dialog"
                    aria-label="Category filter"
                    data-testid={testId ? `${testId}-popover` : undefined}
                    style={{
                        position: 'absolute', top: 'calc(100% + 4px)', left: 0, zIndex: 300, width: 220,
                        background: 'var(--bg-secondary)', border: '1px solid var(--border-color)',
                        borderRadius: 10, boxShadow: '0 16px 40px rgba(0,0,0,0.28)', overflow: 'hidden',
                    }}
                >
                    <div style={{ padding: 8, borderBottom: '1px solid var(--border-color)' }}>
                        <input
                            className="modern-input"
                            placeholder="Search…"
                            value={search}
                            onChange={(e) => setSearch(e.target.value)}
                            style={{ width: '100%', fontSize: '0.75rem', padding: '4px 8px' }}
                        />
                    </div>
                    <div style={{ maxHeight: 240, overflowY: 'auto', padding: '4px 0' }}>
                        {shown.length === 0 && (
                            <div style={{ padding: '8px 12px', fontSize: '0.75rem', color: 'var(--text-secondary)' }}>No categories</div>
                        )}
                        {shown.map(c => {
                            const checked = selected.has(c.id);
                            return (
                                <button
                                    key={c.id}
                                    type="button"
                                    role="checkbox"
                                    aria-checked={checked}
                                    data-testid={testId ? `${testId}-option-${c.id}` : undefined}
                                    onClick={() => toggle(c.id)}
                                    style={{
                                        display: 'flex', alignItems: 'center', gap: 8, width: '100%',
                                        padding: '6px 12px', background: 'transparent', border: 'none',
                                        cursor: 'pointer', textAlign: 'left',
                                    }}
                                >
                                    <span style={{
                                        width: 15, height: 15, borderRadius: 4, flexShrink: 0,
                                        border: checked ? '2px solid var(--accent-indigo)' : '2px solid var(--border-color)',
                                        background: checked ? 'var(--accent-indigo)' : 'transparent',
                                    }} />
                                    <span style={{ fontSize: '0.8rem', color: checked ? 'var(--text-primary)' : 'var(--text-secondary)' }}>{c.name}</span>
                                </button>
                            );
                        })}
                    </div>
                    {value.length > 0 && (
                        <div style={{ padding: 8, borderTop: '1px solid var(--border-color)' }}>
                            <button
                                type="button"
                                data-testid={testId ? `${testId}-clear` : undefined}
                                onClick={() => onChange([])}
                                style={{
                                    width: '100%', fontSize: '0.75rem', padding: '5px', borderRadius: 6, cursor: 'pointer',
                                    border: '1px solid var(--border-color)', background: 'transparent', color: 'var(--text-secondary)',
                                }}
                            >Clear</button>
                        </div>
                    )}
                </div>
            )}
        </div>
    );
}
