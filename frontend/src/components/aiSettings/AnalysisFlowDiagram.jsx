import React, { useId, useState } from 'react';
import { SETTING_HELP, STEP_HELP } from '../../utils/analysisSettingsHelp';
import { fs } from './analysisFlowStyles';

// AnalysisFlowDiagram draws the failure-analysis process that diagramModel / buildAnalysisFlow
// (utils/analysisFlow.js) derive from the settings on screen. It only places what the model says;
// every rule lives in the model.

const STATUS = {
    run: { icon: '✓', label: 'Runs' },
    skip: { icon: '–', label: 'Skipped' },
    warn: { icon: '!', label: 'Check this' },
    blocked: { icon: '✕', label: 'Blocked' },
    idle: { icon: '·', label: 'Not reached' },
};
const SENDS = { typesafe: '→ TypeSafe.ai', llm: '→ LLM' };
const TRIGGERS = [['manual', 'Started by hand'], ['auto', 'Started by run completion']];

// jumpToSetting scrolls the setting a chip names into view, flashes it and focuses its control,
// or the setting's tile when the control is disabled (non-admins, greyed-out dependents).
function jumpToSetting(key) {
    const el = document.querySelector(`[data-setting="${key}"]`);
    if (!el) return;
    el.scrollIntoView({ behavior: 'smooth', block: 'center' });
    el.animate?.(
        [{ boxShadow: '0 0 0 3px rgba(99,102,241,0.75)' }, { boxShadow: '0 0 0 3px rgba(99,102,241,0)' }],
        { duration: 1600, easing: 'ease-out' },
    );
    const control = el.querySelector('input:not([disabled]), textarea:not([disabled]), select:not([disabled]), button[role="switch"]:not([disabled])');
    (control || el).focus({ preventScroll: true });
}

export default function AnalysisFlowDiagram({ model, trigger, onTriggerChange }) {
    return (
        <div style={fs.panel} data-testid="analysis-flow">
            <div style={fs.head}>
                <div style={{ minWidth: 0 }}>
                    <div style={fs.title}>How a run&apos;s failures are analyzed</div>
                    {model.state === 'ready' && model.dirty && (
                        <div style={fs.unsaved} data-testid="analysis-flow-unsaved">Showing unsaved changes</div>
                    )}
                </div>
                <div style={fs.segmented} role="group" aria-label="Show the process for an analysis">
                    {TRIGGERS.map(([value, label]) => (
                        <button key={value} type="button" aria-pressed={trigger === value} data-testid={`analysis-flow-trigger-${value}`}
                            onClick={() => onTriggerChange(value)} style={{ ...fs.segment, ...(trigger === value ? fs.segmentOn : null) }}>
                            {label}
                        </button>
                    ))}
                </div>
            </div>
            <Legend />
            {model.state === 'loading' && <Skeleton />}
            {model.state === 'error' && (
                <div role="alert" style={fs.error} data-testid="analysis-flow-error">
                    Couldn&apos;t load {model.failed.join(' and ')}, so the process can&apos;t be shown. Reload the page to try again.
                </div>
            )}
            {model.state === 'ready' && (
                <>
                    {model.stale && <div style={fs.stale} data-testid="analysis-flow-stale">The LLM provider list may be out of date.</div>}
                    <ol style={fs.steps}>
                        {model.flow.steps.map((step, i) => (
                            <Step key={step.id} step={step} number={i + 1} last={i === model.flow.steps.length - 1} unredacted={model.flow.unredacted} />
                        ))}
                    </ol>
                </>
            )}
        </div>
    );
}

function Legend() {
    return (
        <div style={fs.legend} aria-hidden="true">
            {['run', 'skip', 'warn', 'blocked'].map((k) => (
                <span key={k} style={{ ...fs.status, ...fs.statusTone[k] }}>{STATUS[k].icon} {STATUS[k].label}</span>
            ))}
            <span style={fs.tag}>{SENDS.typesafe}</span>
            <span style={fs.tag}>{SENDS.llm}</span>
            <span>failure text is sent there</span>
        </div>
    );
}

function Skeleton() {
    return (
        <div aria-busy="true" aria-label="Loading the process" data-testid="analysis-flow-loading" style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            {[0, 1, 2, 3, 4, 5].map((i) => <div key={i} style={fs.skeleton} />)}
        </div>
    );
}

function StatusBadge({ status }) {
    const st = STATUS[status];
    return <span style={{ ...fs.status, ...fs.statusTone[status] }}><span aria-hidden="true">{st.icon}</span> {st.label}</span>;
}

function SendTag({ to, unredacted }) {
    return (
        <span style={{ ...fs.tag, ...(unredacted ? fs.tagUnredacted : null) }} data-testid={`analysis-flow-sends-${to}`}>
            {SENDS[to]}{unredacted ? ', sent without redaction' : ''}
        </span>
    );
}

function Step({ step, number, last, unredacted }) {
    const [open, setOpen] = useState(false);
    const howId = useId();
    const help = STEP_HELP[step.id];
    const idle = step.status === 'idle';
    return (
        <li style={{ ...fs.step, ...(idle ? fs.stepIdle : null) }} data-testid={`analysis-flow-step-${step.id}`} data-status={step.status}>
            <div style={fs.rail}>
                <span style={{ ...fs.dot, ...fs.dotTone[step.status] }} aria-hidden="true">{number}</span>
                {!last && <span style={fs.line} />}
            </div>
            <div style={fs.body}>
                <div style={fs.stepHead}>
                    <h5 style={fs.stepTitle}>{step.title}</h5>
                    <StatusBadge status={step.status} />
                    {step.sends.map((to) => <SendTag key={to} to={to} unredacted={unredacted} />)}
                </div>
                {step.reason && <p style={fs.reason}>{step.reason}</p>}
                {step.detail && <p style={fs.detail}>{step.detail}</p>}
                {step.parts.length > 0 && (
                    <div>
                        {step.partsLabel && <div style={fs.partsLabel}>{step.partsLabel}</div>}
                        <div style={fs.parts}>
                            {step.parts.map((p) => <Part key={p.id} part={p} stepId={step.id} unredacted={unredacted} />)}
                        </div>
                    </div>
                )}
                {step.notes.map((n) => <p key={n} style={fs.note}>{n}</p>)}
                {step.chips.length > 0 && (
                    <div style={fs.chips}>{step.chips.map((c) => <Chip key={c.key} chip={c} />)}</div>
                )}
                {help && !idle && (
                    <>
                        <button type="button" style={fs.howBtn} aria-expanded={open} aria-controls={howId}
                            data-testid={`analysis-flow-how-${step.id}`} onClick={() => setOpen((v) => !v)}>
                            {open ? 'Hide how this step works' : 'How this step works'}
                        </button>
                        <div id={howId} style={{ ...fs.how, display: open ? 'flex' : 'none' }}>
                            <p style={fs.howPara}>{help.what}</p>
                            {help.example && <p style={fs.howPara}><strong>Example: </strong>{help.example}</p>}
                        </div>
                    </>
                )}
            </div>
        </li>
    );
}

function Part({ part, stepId, unredacted }) {
    return (
        <div style={{ ...fs.part, ...fs.partTone[part.status] }} data-testid={`analysis-flow-part-${stepId}-${part.id}`} data-status={part.status}>
            <div style={fs.partHead}>
                <span style={fs.partTitle}>{part.title}</span>
                <StatusBadge status={part.status} />
                {part.sends.map((to) => <SendTag key={to} to={to} unredacted={unredacted} />)}
            </div>
            {part.reason && <p style={fs.reason}>{part.reason}</p>}
            {part.detail && <p style={fs.detail}>{part.detail}</p>}
        </div>
    );
}

function Chip({ chip }) {
    const what = SETTING_HELP[chip.key]?.what;
    return (
        <button type="button" style={{ ...fs.chip, ...(chip.invalid ? fs.chipInvalid : null) }}
            title={what} aria-description={what} aria-label={`${chip.label}: ${chip.value}, go to setting`}
            data-testid={`analysis-flow-chip-${chip.key}`} onClick={() => jumpToSetting(chip.key)}>
            <span style={fs.chipLabel}>{chip.label}:</span> {chip.value}
        </button>
    );
}
