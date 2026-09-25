import React, { useContext, useId, useState } from 'react';
import { SETTING_HELP, STEP_HELP } from '../../utils/analysisSettingsHelp';
import { selectedStepId } from '../../utils/analysisFlowView';
import { RevealSettingContext } from './jumpToSetting';
import { fs } from './analysisFlowStyles';

// AnalysisFlowDiagram draws the failure-analysis process that diagramModel / buildAnalysisFlow
// (utils/analysisFlow.js) derive from the settings on screen: the six steps in a row, and the
// selected step's detail underneath. It only places what the model says; every rule lives in the
// model, and which step is shown lives in utils/analysisFlowView.js.

const STATUS = {
    run: { icon: '✓', label: 'Runs' },
    skip: { icon: '–', label: 'Skipped' },
    warn: { icon: '!', label: 'Check this' },
    blocked: { icon: '✕', label: 'Blocked' },
    idle: { icon: '·', label: 'Not reached' },
};
const SENDS = { typesafe: '→ TypeSafe.ai', llm: '→ LLM' };
const TRIGGERS = [['manual', 'Started by hand'], ['auto', 'Started by run completion']];

export default function AnalysisFlowDiagram({ model, trigger, onTriggerChange }) {
    const [picked, setPicked] = useState(null);
    const detailId = useId();
    const flow = model.state === 'ready' ? model.flow : null;
    const selected = selectedStepId(flow, picked);
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
            {flow && (
                <>
                    {model.stale && <div style={fs.stale} data-testid="analysis-flow-stale">The LLM provider list may be out of date.</div>}
                    <div style={fs.scroller}>
                        <ol style={{ ...fs.row, gridTemplateColumns: `repeat(${flow.steps.length}, minmax(0, 1fr))` }}>
                            {flow.steps.map((step, i) => (
                                <StepNode key={step.id} step={step} number={i + 1} last={i === flow.steps.length - 1}
                                    selected={step.id === selected} detailId={detailId} unredacted={flow.unredacted}
                                    onSelect={() => setPicked(step.id)} />
                            ))}
                        </ol>
                        {selected && (
                            <StepDetail id={detailId} steps={flow.steps} selected={selected} unredacted={flow.unredacted} />
                        )}
                    </div>
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
            <span>failure text is sent there · click a step for its detail</span>
        </div>
    );
}

function Skeleton() {
    return (
        <div aria-busy="true" aria-label="Loading the process" data-testid="analysis-flow-loading" style={fs.skeletonRow}>
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

function StepNode({ step, number, last, selected, detailId, unredacted, onSelect }) {
    const idle = step.status === 'idle';
    return (
        <li style={fs.nodeItem} data-testid={`analysis-flow-step-${step.id}`} data-status={step.status}>
            <button type="button" onClick={onSelect} disabled={idle} aria-expanded={selected} aria-controls={detailId}
                style={{ ...fs.node, ...fs.nodeTone[step.status], ...(selected ? fs.nodeSelected : null), ...(idle ? fs.nodeIdle : null) }}>
                <span style={fs.nodeHead}>
                    <span style={{ ...fs.dot, ...fs.dotTone[step.status] }} aria-hidden="true">{number}</span>
                    <span style={fs.nodeTitle}>{step.title}</span>
                </span>
                <StatusBadge status={step.status} />
                {step.sends.length > 0 && (
                    <span style={fs.nodeTags}>{step.sends.map((to) => <SendTag key={to} to={to} unredacted={unredacted} />)}</span>
                )}
            </button>
            {!last && (
                <svg style={fs.arrow} width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2"
                    strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                    <line x1="4" y1="12" x2="19" y2="12" /><polyline points="13 6 19 12 13 18" />
                </svg>
            )}
        </li>
    );
}

function StepDetail({ id, steps, selected, unredacted }) {
    const [openFor, setOpenFor] = useState(null);
    const howId = useId();
    const index = steps.findIndex((s) => s.id === selected);
    const step = steps[index];
    const help = STEP_HELP[step.id];
    const open = openFor === step.id;
    const caretLeft = `calc(${((index + 0.5) / steps.length) * 100}% - 7px)`;
    return (
        <div id={id} role="region" aria-label={`Step ${index + 1}: ${step.title}`} style={fs.detail}
            data-testid="analysis-flow-detail" data-step={step.id}>
            <span style={{ ...fs.caret, left: caretLeft }} aria-hidden="true" />
            <div style={fs.stepHead}>
                <h5 style={fs.stepTitle}>{index + 1}. {step.title}</h5>
                <StatusBadge status={step.status} />
                {step.sends.map((to) => <SendTag key={to} to={to} unredacted={unredacted} />)}
            </div>
            {step.reason && <p style={fs.reason}>{step.reason}</p>}
            {step.detail && <p style={fs.detailText}>{step.detail}</p>}
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
            {help && (
                <>
                    <button type="button" style={fs.howBtn} aria-expanded={open} aria-controls={howId}
                        data-testid={`analysis-flow-how-${step.id}`} onClick={() => setOpenFor(open ? null : step.id)}>
                        {open ? 'Hide how this step works' : 'How this step works'}
                    </button>
                    <div id={howId} style={{ ...fs.how, display: open ? 'flex' : 'none' }}>
                        <p style={fs.howPara}>{help.what}</p>
                        {help.example && <p style={fs.howPara}><strong>Example: </strong>{help.example}</p>}
                    </div>
                </>
            )}
        </div>
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
            {part.detail && <p style={fs.detailText}>{part.detail}</p>}
        </div>
    );
}

function Chip({ chip }) {
    const reveal = useContext(RevealSettingContext);
    const what = SETTING_HELP[chip.key]?.what;
    return (
        <button type="button" style={{ ...fs.chip, ...(chip.invalid ? fs.chipInvalid : null) }}
            title={what} aria-description={what} aria-label={`${chip.label}: ${chip.value}, go to setting`}
            data-testid={`analysis-flow-chip-${chip.key}`} onClick={() => reveal(chip.key)}>
            <span style={fs.chipLabel}>{chip.label}:</span> {chip.value}
        </button>
    );
}
