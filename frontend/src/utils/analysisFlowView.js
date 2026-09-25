// View state for AnalysisFlowDiagram (components/aiSettings). No React, no network.

// selectedStepId picks the step whose detail the diagram shows: the one the user picked while it
// is still reachable, else the step that explains the current state: Start when the flow is
// blocked (it carries the reason), otherwise Decide the verdict.
export function selectedStepId(flow, picked) {
    const steps = flow?.steps || [];
    if (steps.length === 0) return null;
    if (steps.some((s) => s.id === picked && s.status !== 'idle')) return picked;
    if (flow.blocked) return 'start';
    return steps.some((s) => s.id === 'decide') ? 'decide' : steps[0].id;
}
