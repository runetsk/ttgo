import React, { useId, useState } from 'react';
import { cs } from './settingsControlStyles';

// Shared building blocks for the AI settings cards (AI Failure Analysis, TypeSafe.ai), so the
// cards on Settings → AI look and behave alike: toggle tiles with a switch, rows with the label
// and hint on the left and a compact control on the right, compact fields laid out in a grid,
// and the unsaved-changes badge. Every control can carry `setting`, a data-setting anchor the
// process diagram's chips jump to, and `help`, an entry from utils/analysisSettingsHelp.js shown
// by an ⓘ button beside the label.

function useHelp() {
    const [open, setOpen] = useState(false);
    const id = useId();
    return { open, id, toggle: () => setOpen((v) => !v) };
}

// HelpButton is the ⓘ beside a setting's label; it opens the explanation HelpPanel shows.
// It sits above a ToggleCard's transparent checkbox, so a click lands on it, not on the switch.
function HelpButton({ help, setting, label, state }) {
    if (!help) return null;
    return (
        <button type="button" style={{ ...cs.helpBtn, ...(state.open ? cs.helpBtnOn : null) }}
            aria-expanded={state.open} aria-controls={state.id} aria-label={`What this does: ${label}`}
            title={state.open ? 'Hide the explanation' : 'What this does'}
            data-testid={setting ? `help-${setting}` : undefined} onClick={state.toggle}>
            i
        </button>
    );
}

function HelpPanel({ help, state, style }) {
    if (!help) return null;
    return (
        <div id={state.id} style={{ ...cs.helpText, ...style, display: state.open ? 'flex' : 'none' }}>
            <p style={cs.helpPara}>{help.what}</p>
            {help.details && <p style={cs.helpPara}>{help.details}</p>}
            {help.example && <p style={cs.helpPara}><strong>Example: </strong>{help.example}</p>}
            {help.off && <p style={cs.helpPara}><strong>Off: </strong>{help.off}</p>}
        </div>
    );
}

// HelpLabel is a heading with its ⓘ, and the explanation it opens, for a setting that is not
// one of the controls below (the failure-analysis prompt template).
export function HelpLabel({ label, help, setting, labelStyle, children }) {
    const helpState = useHelp();
    return (
        <>
            <div style={cs.labelRow}>
                <span style={labelStyle}>
                    {label}
                    <HelpButton help={help} setting={setting} label={label} state={helpState} />
                </span>
                {children}
            </div>
            <HelpPanel help={help} state={helpState} />
        </>
    );
}

// ToggleCard is a switch tile. The real checkbox covers the tile's main area, transparent, so a
// click there lands on it, the keyboard can reach it, and tests can check or uncheck it by testId.
export function ToggleCard({ icon, iconColor = '#818cf8', label, desc, checked, disabled, onChange, testId, note, help, setting }) {
    const [focused, setFocused] = useState(false);
    const helpState = useHelp();
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
            <div style={{ ...cs.toggleMain, cursor: disabled ? 'not-allowed' : 'pointer' }}>
                {icon && (
                    <div style={{ ...cs.toggleIcon, color: iconColor, borderColor: `${iconColor}33`, background: `${iconColor}14` }}>
                        {icon}
                    </div>
                )}
                <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={cs.toggleLabel}>
                        {label}
                        <HelpButton help={help} setting={setting} label={label} state={helpState} />
                    </div>
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
            </div>
            <HelpPanel help={help} state={helpState} />
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

// FieldRow puts a label and hint on the left and the control on the right: for a setting whose
// control or hint needs the room (the API key with its test button, thresholds with live notes).
export function FieldRow({ label, htmlFor, hint, children, testId, disabled, help, setting }) {
    const helpState = useHelp();
    return (
        <div style={{ ...cs.fieldRow, opacity: disabled ? 0.6 : 1 }} data-testid={testId} data-setting={setting} tabIndex={setting ? -1 : undefined}>
            <div style={cs.fieldLabelCol}>
                <div>
                    <label style={cs.fieldLabel} htmlFor={htmlFor}>{label}</label>
                    <HelpButton help={help} setting={setting} label={label} state={helpState} />
                </div>
                {hint && <div style={cs.fieldHint}>{hint}</div>}
            </div>
            <div style={cs.fieldControl}>{children}</div>
            <HelpPanel help={help} state={helpState} style={cs.helpTextFull} />
        </div>
    );
}

// FieldGrid lays CompactFields out in up to three columns inside one tile. Holding a single
// field, it is a tile that can sit in a row of ToggleCards beside the switch it qualifies.
export function FieldGrid({ children, testId }) {
    return <div style={cs.fieldGrid} data-testid={testId}>{children}</div>;
}

// CompactField stacks a label, a small control and a short hint, for one cell of a FieldGrid.
export function CompactField({ label, htmlFor, hint, children, testId, disabled, help, setting }) {
    const helpState = useHelp();
    return (
        <div style={{ ...cs.compactField, opacity: disabled ? 0.6 : 1 }} data-testid={testId} data-setting={setting} tabIndex={setting ? -1 : undefined}>
            <div>
                <label style={cs.fieldLabel} htmlFor={htmlFor}>{label}</label>
                <HelpButton help={help} setting={setting} label={label} state={helpState} />
            </div>
            <div style={cs.compactControl}>{children}</div>
            {hint && <div style={cs.fieldHint}>{hint}</div>}
            <HelpPanel help={help} state={helpState} />
        </div>
    );
}

// SuffixInput is a compact number input with a unit after it (s, %).
export function SuffixInput({ suffix, width = 110, ...inputProps }) {
    return (
        <span style={cs.suffixWrap}>
            <input className="modern-input" type="number" {...inputProps} style={{ width, padding: '8px 10px', fontSize: '0.85rem' }} />
            {suffix && <span style={cs.suffix}>{suffix}</span>}
        </span>
    );
}

export function UnsavedBadge() {
    return <span style={cs.modifiedBadge}>Unsaved changes</span>;
}
