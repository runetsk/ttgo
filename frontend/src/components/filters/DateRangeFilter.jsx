import React, { useState, useRef, useEffect } from 'react';
import { presetRange, hasDateRange } from '../../utils/dateFilter';

const PRESETS = [
    { key: 'today', label: 'Today' },
    { key: 'last7', label: 'Last 7 days' },
    { key: 'last30', label: 'Last 30 days' },
    { key: 'thisMonth', label: 'This month' },
];

function triggerLabel(value) {
    if (!hasDateRange(value)) return 'Any date';
    if (value.from && value.to) return value.from === value.to ? value.from : `${value.from} → ${value.to}`;
    if (value.from) return `≥ ${value.from}`;
    return `≤ ${value.to}`;
}

/**
 * DateRangeFilter — popover with presets + From/To native date inputs.
 * Props: value {from,to}, onChange(next), testId
 */
export default function DateRangeFilter({ value = { from: null, to: null }, onChange, testId }) {
    const [open, setOpen] = useState(false);
    const triggerRef = useRef(null);
    const popoverRef = useRef(null);
    const active = hasDateRange(value);

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

    const set = (next) => onChange({ from: next.from || null, to: next.to || null });

    return (
        <div style={{ position: 'relative' }}>
            <button
                ref={triggerRef}
                type="button"
                data-testid={testId}
                onClick={() => setOpen(o => !o)}
                className="modern-input"
                style={{
                    width: '100%', fontSize: '0.75rem', padding: '4px 8px', textAlign: 'left',
                    cursor: 'pointer', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
                    color: active ? 'var(--text-primary)' : 'var(--text-secondary)',
                    borderColor: active ? 'var(--accent-indigo)' : undefined,
                }}
            >
                {triggerLabel(value)}
            </button>
            {open && (
                <div
                    ref={popoverRef}
                    role="dialog"
                    aria-label="Date range filter"
                    data-testid={testId ? `${testId}-popover` : undefined}
                    style={{
                        position: 'absolute', top: 'calc(100% + 4px)', left: 0, zIndex: 300, width: 240,
                        background: 'var(--bg-secondary)', border: '1px solid var(--border-color)',
                        borderRadius: 10, boxShadow: '0 16px 40px rgba(0,0,0,0.28)', padding: 10,
                    }}
                >
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginBottom: 10 }}>
                        {PRESETS.map(p => (
                            <button
                                key={p.key}
                                type="button"
                                data-testid={testId ? `${testId}-preset-${p.key}` : undefined}
                                onClick={() => set(presetRange(p.key))}
                                style={{
                                    fontSize: '0.72rem', padding: '4px 8px', borderRadius: 6, cursor: 'pointer',
                                    border: '1px solid var(--border-color)', background: 'var(--bg-primary)',
                                    color: 'var(--text-primary)',
                                }}
                            >{p.label}</button>
                        ))}
                    </div>
                    <label style={labelStyle}>From
                        <input
                            type="date"
                            className="modern-input"
                            data-testid={testId ? `${testId}-from` : undefined}
                            value={value.from || ''}
                            onChange={(e) => set({ ...value, from: e.target.value })}
                            style={inputStyle}
                        />
                    </label>
                    <label style={labelStyle}>To
                        <input
                            type="date"
                            className="modern-input"
                            data-testid={testId ? `${testId}-to` : undefined}
                            value={value.to || ''}
                            onChange={(e) => set({ ...value, to: e.target.value })}
                            style={inputStyle}
                        />
                    </label>
                    {active && (
                        <button
                            type="button"
                            data-testid={testId ? `${testId}-clear` : undefined}
                            onClick={() => set({ from: null, to: null })}
                            style={{
                                marginTop: 8, width: '100%', fontSize: '0.75rem', padding: '5px',
                                borderRadius: 6, cursor: 'pointer', border: '1px solid var(--border-color)',
                                background: 'transparent', color: 'var(--text-secondary)',
                            }}
                        >Clear</button>
                    )}
                </div>
            )}
        </div>
    );
}

const labelStyle = { display: 'flex', flexDirection: 'column', gap: 3, fontSize: '0.7rem', color: 'var(--text-secondary)', marginBottom: 8 };
const inputStyle = { fontSize: '0.78rem', padding: '4px 6px', marginTop: 2 };
