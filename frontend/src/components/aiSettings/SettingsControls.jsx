import React, { useId, useState } from 'react';
import { cs } from './settingsControlStyles';

// Shared building blocks for the AI settings cards (AI Failure Analysis, TypeSafe.ai), so the
// cards on the AI Generation tab look and behave alike: toggle tiles with a switch, rows with the
// label and hint on the left and a compact control on the right, and the unsaved-changes badge.
// Every control can carry `setting`, a data-setting anchor the process diagram's chips jump to,
// and `help`, an entry from utils/analysisSettingsHelp.js shown by a "What this does" toggle.

// ToggleCard is a switch tile. The real checkbox covers the tile's label area, transparent, so a
// click there lands on it, the keyboard can reach it, and tests can check or uncheck it by testId.
// The help toggle sits below that area, outside the <label>, so opening it never flips the switch.
export function ToggleCard({ icon, iconColor = '#818cf8', label, desc, checked, disabled, onChange, testId, note, help, setting }) {
    const [focused, setFocused] = useState(false);
    return (
        <div
            data-setting={setting}
            tabIndex={setting ? -1 : undefined}
            style={{
                ...cs.toggleCard,
                borderColor: checked ? 'rgba(99,102,241,0.35)' : 'var(--border-color)',
                background: checked ? 'rgba(99,102,241,0.04)' : 'var(--bg-tertiary)',
                opacity: disabled ? 0.6 : 1,
                boxShadow: focused ? '0 0 0 2px rgba(99,102,241,0.35)' : 'none',
            }}
        >
            <label style={{ ...cs.toggleMain, cursor: disabled ? 'not-allowed' : 'pointer' }}>
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
            <HelpToggle help={help} setting={setting} />
        </div>
    );
}

// HelpToggle is the "What this does" link under a setting and the explanation it expands.
export function HelpToggle({ help, setting }) {
    const [open, setOpen] = useState(false);
    const id = useId();
    if (!help) return null;
    return (
        <div style={cs.help}>
            <button type="button" style={cs.helpBtn} aria-expanded={open} aria-controls={id}
                data-testid={setting ? `help-${setting}` : undefined} onClick={() => setOpen((v) => !v)}>
                <span aria-hidden="true" style={cs.helpIcon}>i</span>
                {open ? 'Hide' : 'What this does'}
            </button>
            <div id={id} style={{ ...cs.helpText, display: open ? 'flex' : 'none' }}>
                <p style={cs.helpPara}>{help.what}</p>
                {help.details && <p style={cs.helpPara}>{help.details}</p>}
                {help.example && <p style={cs.helpPara}><strong>Example: </strong>{help.example}</p>}
                {help.off && <p style={cs.helpPara}><strong>Off: </strong>{help.off}</p>}
            </div>
        </div>
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

// FieldRow puts a label, hint and help toggle on the left and the control on the right.
export function FieldRow({ label, htmlFor, hint, children, testId, disabled, help, setting }) {
    return (
        <div style={{ ...cs.fieldRow, opacity: disabled ? 0.6 : 1 }} data-testid={testId} data-setting={setting} tabIndex={setting ? -1 : undefined}>
            <div style={cs.fieldLabelCol}>
                <label style={cs.fieldLabel} htmlFor={htmlFor}>{label}</label>
                {hint && <div style={cs.fieldHint}>{hint}</div>}
                <HelpToggle help={help} setting={setting} />
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
