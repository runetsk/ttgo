import React, { useState, useEffect, useRef } from 'react';

/**
 * ColumnPicker — popover for toggling grid column visibility.
 *
 * Props:
 *   columnDefs  {Array}     Array of { key, label, mandatory, defaultVisible }
 *   visibleKeys {Set}       Currently visible column keys
 *   onToggle    {Function}  Called with (key) when a column is toggled
 *   onReset     {Function}  Optional. Called when "Reset to defaults" is clicked.
 */
export default function ColumnPicker({ columnDefs, visibleKeys, onToggle, onReset }) {
    const [open, setOpen] = useState(false);
    const triggerRef = useRef(null);
    const popoverRef = useRef(null);

    const mandatory = columnDefs.filter(c => c.mandatory);
    const optional  = columnDefs.filter(c => !c.mandatory);
    const visibleOptionalCount = optional.filter(c => visibleKeys.has(c.key)).length;

    // ── Close on outside click ────────────────────────────────────────────────
    useEffect(() => {
        if (!open) return;
        const handler = (e) => {
            if (
                !popoverRef.current?.contains(e.target) &&
                !triggerRef.current?.contains(e.target)
            ) {
                setOpen(false);
            }
        };
        const keyHandler = (e) => {
            if (e.key === 'Escape') setOpen(false);
        };
        document.addEventListener('mousedown', handler);
        document.addEventListener('keydown', keyHandler);
        return () => {
            document.removeEventListener('mousedown', handler);
            document.removeEventListener('keydown', keyHandler);
        };
    }, [open]);

    return (
        <div style={{ position: 'relative', display: 'inline-block' }}>

            {/* ── Trigger button ──────────────────────────────────────────── */}
            <button
                ref={triggerRef}
                type="button"
                className="action-btn"
                onClick={() => setOpen(o => !o)}
                aria-label="Columns"
                aria-expanded={open}
                style={{
                    padding: '6px 12px',
                    display: 'flex',
                    alignItems: 'center',
                    gap: 6,
                    background: open ? 'rgba(99,102,241,0.08)' : undefined,
                    borderColor: open ? 'rgba(99,102,241,0.4)' : undefined,
                }}
            >
                <svg width="14" height="14" viewBox="0 0 14 14" fill="none" style={{ opacity: 0.7 }}>
                    <rect x="1" y="2" width="12" height="1.5" rx="0.75" fill="currentColor"/>
                    <rect x="1" y="6.25" width="12" height="1.5" rx="0.75" fill="currentColor"/>
                    <rect x="1" y="10.5" width="12" height="1.5" rx="0.75" fill="currentColor"/>
                    <circle cx="4.5" cy="2.75" r="1.5" fill="var(--bg-primary, #0f172a)" stroke="currentColor" strokeWidth="1"/>
                    <circle cx="9.5" cy="7" r="1.5" fill="var(--bg-primary, #0f172a)" stroke="currentColor" strokeWidth="1"/>
                    <circle cx="5.5" cy="11.25" r="1.5" fill="var(--bg-primary, #0f172a)" stroke="currentColor" strokeWidth="1"/>
                </svg>
                <span>Columns</span>
                <span style={{
                    fontSize: '0.7rem',
                    fontWeight: 700,
                    padding: '1px 6px',
                    borderRadius: 99,
                    background: 'rgba(99,102,241,0.15)',
                    color: 'var(--accent-indigo)',
                    minWidth: 22,
                    textAlign: 'center',
                }}>
                    {visibleOptionalCount}/{optional.length}
                </span>
            </button>

            {/* ── Popover ──────────────────────────────────────────────────── */}
            {open && (
                <div
                    ref={popoverRef}
                    role="dialog"
                    aria-label="Column visibility"
                    style={{
                        position: 'absolute',
                        top: 'calc(100% + 6px)',
                        right: 0,
                        zIndex: 300,
                        width: 230,
                        background: 'var(--bg-secondary)',
                        border: '1px solid var(--border-color)',
                        borderRadius: 10,
                        boxShadow: '0 16px 40px rgba(0,0,0,0.28), 0 2px 8px rgba(0,0,0,0.15)',
                        overflow: 'hidden',
                    }}
                >
                    {/* Header */}
                    <div style={{
                        padding: '10px 14px',
                        borderBottom: '1px solid var(--border-color)',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                    }}>
                        <span style={{
                            fontSize: '0.7rem',
                            fontWeight: 700,
                            letterSpacing: '0.06em',
                            color: 'var(--text-secondary)',
                            textTransform: 'uppercase',
                        }}>
                            Column Visibility
                        </span>
                        <span style={{
                            fontSize: '0.72rem',
                            color: 'var(--text-secondary)',
                        }}>
                            {visibleOptionalCount} of {optional.length} shown
                        </span>
                    </div>

                    {/* Scrollable list */}
                    <div style={{ maxHeight: 380, overflowY: 'auto' }}>

                        {/* Always-visible (mandatory) section */}
                        {mandatory.length > 0 && (
                            <div style={{ padding: '8px 0 4px' }}>
                                <div style={sectionLabel}>Always visible</div>
                                {mandatory.map(col => (
                                    <div key={col.key} style={mandatoryRow} aria-disabled="true">
                                        <span style={lockIcon}>
                                            <svg width="10" height="12" viewBox="0 0 10 12" fill="none">
                                                <rect x="1.5" y="5" width="7" height="6" rx="1.5" stroke="currentColor" strokeWidth="1.2"/>
                                                <path d="M3 5V3.5a2 2 0 1 1 4 0V5" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round"/>
                                            </svg>
                                        </span>
                                        <span style={{ flex: 1, fontSize: '0.84rem', color: 'var(--text-secondary)' }}>
                                            {col.label}
                                        </span>
                                    </div>
                                ))}
                            </div>
                        )}

                        {/* Divider */}
                        {mandatory.length > 0 && optional.length > 0 && (
                            <div style={{ height: 1, background: 'var(--border-color)', margin: '2px 0' }} />
                        )}

                        {/* Optional columns section */}
                        {optional.length > 0 && (
                            <div style={{ padding: '8px 0 4px' }}>
                                <div style={sectionLabel}>Optional</div>
                                {optional.map(col => {
                                    const checked = visibleKeys.has(col.key);
                                    return (
                                        <ColumnRow
                                            key={col.key}
                                            label={col.label}
                                            checked={checked}
                                            onToggle={() => onToggle(col.key)}
                                        />
                                    );
                                })}
                            </div>
                        )}
                    </div>

                    {/* Footer */}
                    {onReset && (
                        <div style={{
                            padding: '8px',
                            borderTop: '1px solid var(--border-color)',
                        }}>
                            <ResetButton onClick={() => { onReset(); setOpen(false); }} />
                        </div>
                    )}
                </div>
            )}
        </div>
    );
}

/* ── Sub-components ─────────────────────────────────────────────────────────── */

function ColumnRow({ label, checked, onToggle }) {
    const [hovered, setHovered] = useState(false);
    return (
        <button
            type="button"
            role="checkbox"
            aria-checked={checked}
            onClick={onToggle}
            onMouseEnter={() => setHovered(true)}
            onMouseLeave={() => setHovered(false)}
            style={{
                display: 'flex',
                alignItems: 'center',
                gap: 9,
                width: '100%',
                padding: '6px 14px',
                background: hovered ? 'rgba(99,102,241,0.06)' : 'transparent',
                border: 'none',
                cursor: 'pointer',
                textAlign: 'left',
                transition: 'background 0.1s',
            }}
        >
            {/* Custom checkbox */}
            <span style={{
                width: 16,
                height: 16,
                borderRadius: 4,
                flexShrink: 0,
                border: checked ? '2px solid var(--accent-indigo)' : '2px solid var(--border-color)',
                background: checked ? 'var(--accent-indigo)' : 'transparent',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                transition: 'all 0.15s',
            }}>
                {checked && (
                    <svg width="10" height="8" viewBox="0 0 10 8" fill="none">
                        <path d="M1 4l3 3 5-6" stroke="white" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round"/>
                    </svg>
                )}
            </span>
            <span style={{
                flex: 1,
                fontSize: '0.84rem',
                color: checked ? 'var(--text-primary)' : 'var(--text-secondary)',
                fontWeight: checked ? 500 : 400,
                transition: 'color 0.1s',
            }}>
                {label}
            </span>
        </button>
    );
}

function ResetButton({ onClick }) {
    const [hovered, setHovered] = useState(false);
    return (
        <button
            type="button"
            onClick={onClick}
            onMouseEnter={() => setHovered(true)}
            onMouseLeave={() => setHovered(false)}
            style={{
                width: '100%',
                padding: '6px 10px',
                background: hovered ? 'var(--bg-tertiary, rgba(100,116,139,0.1))' : 'transparent',
                border: '1px solid var(--border-color)',
                borderRadius: 6,
                cursor: 'pointer',
                fontSize: '0.8rem',
                color: hovered ? 'var(--text-primary)' : 'var(--text-secondary)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                gap: 5,
                transition: 'all 0.1s',
            }}
        >
            <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                <path d="M2 6a4 4 0 1 1 1 2.65" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round"/>
                <path d="M2 9V6h3" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round"/>
            </svg>
            Reset to default
        </button>
    );
}

/* ── Shared styles ─────────────────────────────────────────────────────────── */

const sectionLabel = {
    padding: '0 14px 4px',
    fontSize: '0.68rem',
    fontWeight: 700,
    letterSpacing: '0.06em',
    textTransform: 'uppercase',
    color: 'var(--text-secondary)',
    opacity: 0.65,
};

const mandatoryRow = {
    display: 'flex',
    alignItems: 'center',
    gap: 9,
    padding: '6px 14px',
    opacity: 0.45,
    cursor: 'default',
};

const lockIcon = {
    width: 16,
    height: 16,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    color: 'var(--text-secondary)',
    flexShrink: 0,
};
