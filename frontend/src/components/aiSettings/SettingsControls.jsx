import React, { useState } from 'react';
import { cs } from './settingsControlStyles';

// Shared building blocks for the AI settings cards (AI Failure Analysis, TypeSafe.ai), so the
// cards on the AI Generation tab look and behave alike: toggle tiles with a switch, rows with the
// label and hint on the left and a compact control on the right, and the unsaved-changes badge.

// ToggleCard is a whole-tile switch. The real checkbox covers the tile, transparent, so a click
// anywhere lands on it, the keyboard can reach it, and tests can check or uncheck it by testId.
export function ToggleCard({ icon, iconColor = '#818cf8', label, desc, checked, disabled, onChange, testId, note }) {
    const [focused, setFocused] = useState(false);
    return (
        <label
            style={{
                ...cs.toggleCard,
                borderColor: checked ? 'rgba(99,102,241,0.35)' : 'var(--border-color)',
                background: checked ? 'rgba(99,102,241,0.04)' : 'var(--bg-tertiary)',
                cursor: disabled ? 'not-allowed' : 'pointer',
                opacity: disabled ? 0.6 : 1,
                boxShadow: focused ? '0 0 0 2px rgba(99,102,241,0.35)' : 'none',
            }}
        >
            {icon && (
                <div style={{ ...cs.toggleIcon, color: iconColor, borderColor: `${iconColor}33`, background: `${iconColor}14` }}>
                    {icon}
                </div>
            )}
            <div style={{ flex: 1, minWidth: 0 }}>
                <div style={cs.toggleLabel}>{label}</div>
                {desc && <div style={cs.toggleDesc}>{desc}</div>}
                {note && <div style={cs.toggleNote}>{note}</div>}
            </div>
            <Switch checked={checked} />
            <input
                type="checkbox"
                checked={!!checked}
                disabled={disabled}
                data-testid={testId}
                aria-label={label}
                onChange={(e) => onChange(e.target.checked)}
                onFocus={() => setFocused(true)}
                onBlur={() => setFocused(false)}
                style={cs.overlayInput}
            />
        </label>
    );
}

// InlineSwitch is a small labelled switch for a secondary option inside a row.
export function InlineSwitch({ label, checked, disabled, onChange, testId }) {
    return (
        <label style={{ ...cs.inlineSwitch, cursor: disabled ? 'not-allowed' : 'pointer', opacity: disabled ? 0.6 : 1 }}>
            <Switch checked={checked} small />
            <span>{label}</span>
            <input
                type="checkbox"
                checked={!!checked}
                disabled={disabled}
                data-testid={testId}
                aria-label={label}
                onChange={(e) => onChange(e.target.checked)}
                style={cs.overlayInput}
            />
        </label>
    );
}

function Switch({ checked, small }) {
    const w = small ? 28 : 34;
    const knob = small ? 12 : 14;
    return (
        <span style={{ ...cs.switch, width: w, height: knob + 4, marginTop: small ? 0 : 4, background: checked ? 'var(--accent-indigo)' : 'rgba(148,163,184,0.35)' }}>
            <span style={{ ...cs.switchKnob, width: knob, height: knob, transform: checked ? `translateX(${w - knob - 4}px)` : 'translateX(0)' }} />
        </span>
    );
}

// FieldRow puts a label and hint on the left and the control on the right.
export function FieldRow({ label, htmlFor, hint, children, testId, disabled }) {
    return (
        <div style={{ ...cs.fieldRow, opacity: disabled ? 0.6 : 1 }} data-testid={testId}>
            <div style={cs.fieldLabelCol}>
                <label style={cs.fieldLabel} htmlFor={htmlFor}>{label}</label>
                {hint && <div style={cs.fieldHint}>{hint}</div>}
            </div>
            <div style={cs.fieldControl}>{children}</div>
        </div>
    );
}

// SuffixInput is a compact number input with a unit after it (s, %).
export function SuffixInput({ suffix, width = 110, ...inputProps }) {
    return (
        <span style={cs.suffixWrap}>
            <input className="modern-input" type="number" {...inputProps} style={{ width, padding: '8px 10px', fontSize: '0.85rem' }} />
            <span style={cs.suffix}>{suffix}</span>
        </span>
    );
}

export function UnsavedBadge() {
    return <span style={cs.modifiedBadge}>Unsaved changes</span>;
}
