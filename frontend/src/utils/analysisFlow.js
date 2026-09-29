// Pure model of the failure-analysis process drawn on Settings → AI Generation. No React, no network.
// Consumers: components/aiSettings/FailureAnalysisSection.jsx and AnalysisFlowDiagram.jsx.
//
// It ports the server's routing and must change with it:
//   - newAnalyzeDepsResolver / resolveTypeSafe (backend/internal/api/failure_analysis_worker.go):
//     who may group, decide and explain for a trigger;
//   - Analyze (backend/pkg/tracker/failureanalysis/analyzer.go): what each TypeSafe outcome does
//     (kept, taken over, fallback, failed) and when an explanation is written;
//   - the worker (backend/pkg/tracker/failureanalysis/worker/worker.go): grouping, merge, cap.
// Where the page cannot know something (a provider's own settings, a key never tried) the model
// says so rather than guessing.
import { defaultProvider } from './typesafeSettings.js';
import { FA_DEFAULTS } from './failureAnalysisSettings.js';

const KEY_USABLE = { stored: true, new: true, none: false, removing: false, unreadable: false };
const KEY_LABEL = { stored: 'Stored', new: 'New, not yet checked', none: 'None', removing: 'Being removed', unreadable: 'Unreadable' };
const KEY_REASON = {
    none: 'no API key is stored',
    removing: 'the stored API key is being removed',
    unreadable: 'the stored API key cannot be decrypted',
};
const LLM_CHECKED_NOTE = "The default LLM's own settings (key, model) are checked when the analysis runs.";
// Wave 3 notes. The guard and the companion questions ride on TypeSafe's verdict request; the
// transfer check follows an explanation when semantic grouping runs; auto-apply writes labels.
const COMPANION_NOTE = 'In the same request TypeSafe also answers companion questions: a flaky pattern in the recent outcomes, the same failure seen before, an error from outside the app, and which linked defect matches. Each answer of 80% or more shows as a chip on the result.';
const GUARD_NOTE = 'Injection guard: TypeSafe checks the failure text and the related failures of its group for instructions aimed at an AI. At 80% or more its decision is kept, but nothing from that group is sent to the LLM. Nothing is checked when TypeSafe is unavailable and the LLM steps in.';
const NO_GUARD_NOTE = 'With the LLM deciding, nothing checks the failure text for prompt injection before it is sent.';
const GUARD_EXPLAIN_NOTE = 'A group flagged for possible prompt injection gets no explanation; Explain on it asks before sending the failure to the LLM.';
// Wave 4: the semantic pass remembers its answers, and a person's split keeps a pair apart.
const MEMORY_NOTE = "TypeSafe's same-cause answers are remembered for 30 days, so a pair of errors seen again is not asked again; \"Not the same failure — split\" on a grouped result keeps the two apart for good and analyzes the split-off results on their own.";
const TRANSFER_NOTE = 'After a group is explained, TypeSafe checks that the explanation fits each semantically grouped result (up to 40 per group). A result below 50% reads "This explanation may not apply to this result" and offers Explain this result.';
const RETRY_NOTE = 'Retry failed groups is a manual action using the current settings, even for a run first analyzed automatically, so TypeSafe.ai is used whenever it is enabled.';
const IDLE_STEPS = [
    ['group', 'Group the failures'],
    ['evidence', 'Prepare the evidence'],
    ['decide', 'Decide the verdict'],
    ['explain', 'Explain'],
    ['store', 'Store and show'],
];
const LOAD_LABELS = {
    aiFeatures: 'the AI features switch',
    providers: 'the LLM providers',
    typesafe: 'the TypeSafe.ai settings',
    failureAnalysis: 'the failure-analysis settings',
};

const onOff = (v) => (v ? 'On' : 'Off');
const skipped = (id, title, reason) => ({ id, title, status: 'skip', reason, detail: '', sends: [] });

// keyState reads the TypeSafe key from the card's draft: an unsaved removal or a newly typed key
// wins over what is stored.
export function keyState(ts) {
    if (ts?.clear_api_key) return 'removing';
    if (typeof ts?.api_key === 'string' && ts.api_key.trim() !== '') return 'new';
    if (ts?.api_key_status === 'ok') return 'stored';
    if (ts?.api_key_status === 'undecryptable') return 'unreadable';
    return 'none';
}

// llmName names the default provider once when its label is its model.
export function llmName(provider) {
    if (!provider) return null;
    if (!provider.model_name || provider.label === provider.model_name) return provider.label;
    return `${provider.label} (${provider.model_name})`;
}

// Server: llmKeyUnreadableReason (internal/api/failure_analysis_worker.go).
const LLM_KEY_UNREADABLE = "the default LLM provider's stored key can't be decrypted — re-enter it";

// resolveRoute ports newAnalyzeDepsResolver for one trigger ('manual' | 'auto').
export function resolveRoute({ aiEnabled, typesafe: ts, provider, trigger }) {
    const byHand = trigger !== 'auto';
    const key = keyState(ts);
    const keyUsable = KEY_USABLE[key];
    const allowed = !!ts?.enabled && (byHand || !!ts?.allow_auto_failure_analysis);
    const active = allowed && (!!ts?.verdict_engine_enabled || !!ts?.semantic_dedup_enabled);
    let decider = 'none';
    let semantic = false;
    if (active && keyUsable) {
        semantic = !!ts.semantic_dedup_enabled;
        if (ts.verdict_engine_enabled) decider = 'typesafe';
    } else if (active && ts.verdict_engine_enabled) {
        decider = 'unavailable'; // TypeSafe is the configured decider but cannot run
    }
    const tsDecides = decider !== 'none';
    const pct = tsDecides ? (ts.escalate_below_pct ?? 0) : 0;
    const needLLM = !tsDecides || !!ts.narrative_enabled || pct > 0 || !!ts.llm_fallback_enabled;
    let llm = null;
    let llmReason = null;
    if (needLLM) {
        if (!provider) llmReason = 'no default LLM provider is configured';
        else if (!byHand && !provider.allow_auto_failure_analysis) llmReason = 'the default LLM provider is not approved for automatic analysis';
        // The worker still records a manual attempt the LLM must decide as failed (category
        // configuration); either way no analysis comes out of the LLM, so show the reason.
        else if (provider.api_key_status === 'undecryptable') llmReason = LLM_KEY_UNREADABLE;
        else llm = llmName(provider);
    }
    return {
        byHand, key, keyUsable, allowed, decider, semantic, tsDecides, pct, llm, llmReason,
        takeover: tsDecides && !!llm && pct > 0,
        canAnalyze: !!aiEnabled && (!!llm || tsDecides),
    };
}

// typesafeVerdictOff says why TypeSafe.ai is not the one deciding, or null when it is.
function typesafeVerdictOff(route, ts) {
    if (!ts?.enabled) return 'TypeSafe.ai is off';
    if (!route.allowed) return 'TypeSafe.ai is not allowed on automatic analysis';
    if (!ts.verdict_engine_enabled) return 'TypeSafe.ai is not deciding verdicts';
    return null;
}

// semanticSkipReason gives the first missing condition for the semantic merge, or null when it
// runs (the worker merges only after identical grouping and only with a TypeSafe client attached).
function semanticSkipReason(route, ts, fa) {
    if (!fa?.dedup_enabled) return 'Needs "Deduplicate similar failures"';
    if (!ts?.semantic_dedup_enabled) return 'Semantic failure grouping is off';
    if (!ts.enabled) return 'TypeSafe.ai is off';
    if (!route.allowed) return 'TypeSafe.ai is not allowed on automatic analysis';
    if (!route.keyUsable) return `No usable API key: ${KEY_REASON[route.key]}`;
    return null;
}

// buildAnalysisFlow describes the six steps an analysis takes with the given settings.
// `invalid` lists chip keys whose draft value fails validation (the caller has already put the
// saved value back in those fields).
export function buildAnalysisFlow({ aiEnabled, typesafe: ts, failureAnalysis: fa, provider, trigger, invalid = [] }) {
    const bad = new Set(invalid);
    const chip = (key, label, value) => (bad.has(key) ? { key, label, value: 'invalid', invalid: true } : { key, label, value });
    const route = resolveRoute({ aiEnabled, typesafe: ts, provider, trigger });
    const mode = route.byHand ? 'manual' : 'auto';
    const name = llmName(provider);
    const providerChip = chip('provider.default', 'Default LLM', !provider ? 'None'
        : route.byHand ? name
            : `${name}, ${provider.allow_auto_failure_analysis ? 'approved' : 'not approved'} for automatic analysis`);
    const aiChip = chip('ai.enabled', 'AI features', onOff(aiEnabled));
    const tsOff = typesafeVerdictOff(route, ts);
    // Every failure-analysis LLM call runs under the call timeout, hedged when that is on.
    const llmTimeout = fa?.llm_call_timeout_seconds ?? FA_DEFAULTS.llm_call_timeout_seconds;
    const hedge = fa?.hedge_after_seconds ?? FA_DEFAULTS.hedge_after_seconds;
    const llmChips = route.llm ? [
        chip('fa.llm_call_timeout_seconds', 'LLM call timeout', `${llmTimeout} s`),
        chip('fa.hedge_after_seconds', 'Hedge slow LLM calls', hedge > 0 ? `After ${hedge} s` : 'Off'),
    ] : [];

    // 1. Start
    const blockReason = !aiEnabled ? 'AI features are off: nothing is analyzed.'
        : !route.byHand && !fa?.enabled_on_completion ? 'Runs are not analyzed automatically: "Auto-analyze on run completion" is off.'
            : !route.canAnalyze ? `Nothing can analyze: ${route.llmReason}${tsOff ? `, and ${tsOff}` : ''}.`
                : null;
    const start = {
        id: 'start', title: 'Start',
        detail: route.byHand ? 'You click Analyze failures on a run.' : 'A run is completed with failing results.',
        status: blockReason ? 'blocked' : 'run', reason: blockReason,
        notes: [
            route.byHand
                ? 'Re-analyzing a single result skips step 2 and stores one analysis; Explain on a result always uses the default LLM.'
                : 'Only runs completed after this is switched on, and only when they have failing results.',
            RETRY_NOTE,
        ],
        chips: route.byHand ? [aiChip] : [
            aiChip,
            chip('fa.enabled_on_completion', 'Auto-analyze on run completion', onOff(fa?.enabled_on_completion)),
            chip('ts.allow_auto_failure_analysis', 'TypeSafe on automatic analysis',
                !ts?.enabled ? 'TypeSafe.ai off' : ts.allow_auto_failure_analysis ? 'Allowed' : 'Not allowed'),
            providerChip,
        ],
        sends: [], parts: [],
    };
    if (blockReason) {
        return {
            trigger: mode, blocked: true, canAnalyze: route.canAnalyze, unredacted: false,
            steps: [start, ...IDLE_STEPS.map(([id, title]) => ({ id, title, status: 'idle', detail: '', notes: [], chips: [], sends: [], parts: [] }))],
        };
    }

    // 2. Group
    const dedup = !!fa.dedup_enabled;
    const semanticSkip = semanticSkipReason(route, ts, fa);
    const cap = fa.max_analyses_per_run;
    const parallel = fa.parallel_groups;
    const group = {
        id: 'group', title: 'Group the failures', status: 'run',
        detail: dedup
            ? 'Failures with the same failure type and error text form one group; one result per group is analyzed and its answer is copied to the rest.'
            : 'Each failing result is analyzed on its own.',
        notes: [
            'Retry failed groups regroups all failures first, semantic calls included, then analyzes only the groups with a failed attempt, up to the cap.',
            ...(semanticSkip ? [] : [MEMORY_NOTE]),
        ],
        chips: [
            chip('fa.dedup_enabled', 'Deduplicate similar failures', onOff(dedup)),
            chip('ts.semantic_dedup_enabled', 'Semantic failure grouping', onOff(ts?.semantic_dedup_enabled)),
            chip('fa.max_analyses_per_run', 'Max analyses per run', String(cap)),
            chip('fa.parallel_groups', 'Groups analyzed at once', String(parallel)),
        ],
        sends: semanticSkip ? [] : ['typesafe'],
        parts: [
            semanticSkip
                ? skipped('semantic', 'Semantic merge by TypeSafe.ai', `${semanticSkip}.`)
                : {
                    id: 'semantic', title: 'Semantic merge by TypeSafe.ai', status: 'run', sends: ['typesafe'],
                    detail: 'TypeSafe compares groups of the same failure type whose wording overlaps and merges those it is at least 80% sure are the same failure. These calls happen before the cap.',
                },
            {
                id: 'cap', title: dedup ? `Up to ${cap} groups, largest first` : `Up to ${cap} results`, status: 'run', sends: [],
                detail: 'The rest are left without an analysis. They did not fail, so Retry failed groups does not pick them up.',
            },
            { id: 'parallel', title: `${parallel} at a time`, status: 'run', sends: [], detail: 'Nearly all the time is spent waiting on TypeSafe.ai and the LLM.' },
        ],
    };

    // 3. Evidence
    const redact = !!fa.redaction_enabled;
    const examples = fa.few_shot_examples ?? FA_DEFAULTS.few_shot_examples;
    const evidence = {
        id: 'evidence', title: 'Prepare the evidence',
        detail: "The error, stack, log and steps of one result per group, with this test's failures from the 30 days before it.",
        status: redact ? 'run' : 'warn',
        reason: redact ? null : 'Redact secrets is off: failure text is sent as recorded.',
        notes: [
            ...(redact ? ['Recognized secrets are replaced before anything is sent in steps 2, 4 and 5.'] : []),
            ...(examples > 0 ? [`Up to ${examples} past failures people triaged in other runs go with each group as examples.`] : []),
        ],
        chips: [
            chip('fa.redaction_enabled', 'Redact secrets', onOff(redact)),
            chip('fa.few_shot_examples', 'Past triage examples', examples > 0 ? `Up to ${examples}` : 'Off'),
        ],
        sends: [], parts: [],
    };

    // 4. Decide
    const noKey = route.decider === 'unavailable';
    const llmNotes = route.llm ? [LLM_CHECKED_NOTE] : [];
    let decide;
    if (!route.tsDecides) {
        decide = {
            id: 'decide', title: 'Decide the verdict', status: 'run',
            detail: `${route.llm} decides the verdict and explains it in one answer.`,
            notes: [`The LLM decides because ${tsOff}.`, ...llmNotes, NO_GUARD_NOTE],
            chips: [
                chip('ts.enabled', 'TypeSafe.ai', onOff(ts?.enabled)),
                ...(ts?.enabled ? [chip('ts.verdict_engine_enabled', 'Use for failure verdicts', onOff(ts.verdict_engine_enabled))] : []),
                providerChip,
                ...llmChips,
            ],
            sends: ['llm'], parts: [],
        };
    } else {
        const when = noKey ? 'Without a usable key' : 'If TypeSafe errors or times out';
        const unavailable = !ts.llm_fallback_enabled
            ? { status: noKey ? 'warn' : 'run', sends: [], detail: `${when}, the attempt is recorded as failed and can be retried; failure data never goes to the LLM in its place.` }
            : route.llm
                ? { status: noKey ? 'warn' : 'run', sends: ['llm'], detail: `${when}, ${route.llm} decides and explains, and the analysis says TypeSafe was unavailable.` }
                : { status: 'warn', sends: [], detail: `${when}, the attempt is recorded as failed: ${route.llmReason}.` };
        decide = {
            id: 'decide', title: 'Decide the verdict',
            status: noKey ? 'warn' : 'run',
            reason: noKey ? `No usable API key (${KEY_REASON[route.key]}): every decision takes the "TypeSafe unavailable" path.` : null,
            detail: `TypeSafe.ai (${ts.model}) answers the verdict and defect-type questions.`,
            notes: [...(route.key === 'new' ? ['New API key, checked on first use.'] : []), ...(noKey ? [] : [COMPANION_NOTE, GUARD_NOTE]), ...llmNotes],
            chips: [
                chip('ts.verdict_engine_enabled', 'Use for failure verdicts', 'On'),
                chip('ts.model', 'Model', ts.model),
                chip('ts.api_key', 'API key', KEY_LABEL[route.key]),
                chip('ts.timeout_seconds', 'Timeout', `${ts.timeout_seconds} s`),
                chip('ts.escalate_below_pct', 'Ask the LLM below', route.pct > 0 ? `${route.pct}%` : 'Never'),
                chip('ts.llm_fallback_enabled', 'Use the default LLM when unavailable', onOff(ts.llm_fallback_enabled)),
                providerChip,
            ],
            sends: noKey ? [] : ['typesafe'],
            partsLabel: 'Then, depending on the answer:',
            parts: [
                noKey ? skipped('answers', 'TypeSafe answers', 'No usable API key.')
                    : {
                        id: 'answers', title: 'TypeSafe answers', status: 'run', sends: [],
                        detail: route.takeover ? `Confidence ${route.pct}% or more: TypeSafe's decision is kept.` : "TypeSafe's decision is kept.",
                    },
                noKey ? skipped('unsure', 'TypeSafe unsure', 'No usable API key.')
                    : route.pct === 0 ? skipped('unsure', 'TypeSafe unsure', 'Never handed over: "Ask the LLM below" is 0%.')
                        : !route.llm ? skipped('unsure', 'TypeSafe unsure', `No LLM to hand over to: ${route.llmReason}.`)
                            : {
                                id: 'unsure', title: `TypeSafe below ${route.pct}% sure`, status: 'run', sends: ['llm'],
                                detail: `${route.llm} decides instead; the analysis notes what TypeSafe said. If that call fails or cannot be read, TypeSafe's decision is kept.`,
                            },
                { id: 'unavailable', title: 'TypeSafe unavailable', ...unavailable },
            ],
        };
    }

    // 5. Explain
    let explain;
    if (!route.tsDecides) {
        explain = {
            id: 'explain', title: 'Explain', status: 'run', detail: 'Written in the same answer as the verdict (step 4).',
            notes: [], chips: [providerChip], sends: [], parts: [],
        };
    } else {
        const parts = [];
        if (!noKey) {
            parts.push(!ts.narrative_enabled
                ? { id: 'answers', title: 'After TypeSafe answers', status: 'skip', sends: [], detail: 'No explanation; Explain on a result writes one on demand.' }
                : route.llm
                    ? { id: 'answers', title: 'After TypeSafe answers', status: 'run', sends: ['llm'], detail: `${route.llm} writes the explanation.` }
                    : { id: 'answers', title: 'After TypeSafe answers', status: 'warn', sends: [], detail: `No explanation: ${route.llmReason}.` });
        }
        if (route.takeover && !noKey) {
            parts.push({
                id: 'unsure', title: 'After the LLM takes over', status: 'run', sends: [],
                detail: 'The LLM explains in the same answer, whatever "Write an explanation" says. If it cannot answer, the kept TypeSafe decision has no explanation.',
            });
        }
        if (ts.llm_fallback_enabled && route.llm) {
            parts.push({ id: 'unavailable', title: 'After the LLM steps in', status: 'run', sends: [], detail: 'The LLM explains in the same answer as its verdict.' });
        }
        const status = parts.some((p) => p.status === 'run') ? 'run' : parts.some((p) => p.status === 'warn') ? 'warn' : 'skip';
        explain = {
            id: 'explain', title: 'Explain', status,
            reason: status === 'skip' ? 'No explanation is written; Explain on a result writes one on demand.' : null,
            detail: '',
            notes: ts.narrative_enabled && route.llm && !noKey
                ? [
                    'The decision is shown as soon as TypeSafe answers; the result reads "Explanation being written…" until the explanation arrives. One explanation is written per group and copied to every result in it.',
                    GUARD_EXPLAIN_NOTE,
                    ...(semanticSkip ? [] : [TRANSFER_NOTE]),
                ]
                : [],
            chips: [chip('ts.narrative_enabled', 'Write an explanation', onOff(ts.narrative_enabled)), providerChip, ...llmChips],
            sends: parts.some((p) => p.sends.length > 0) ? ['llm'] : [],
            partsLabel: 'Depends on the path in step 4:', parts,
        };
    }

    // 6. Store (and auto-apply)
    const autoApply = fa.auto_apply_defect_type ?? FA_DEFAULTS.auto_apply_defect_type;
    const minConfidence = fa.auto_apply_min_confidence ?? FA_DEFAULTS.auto_apply_min_confidence;
    const autoApplyNotes = !autoApply ? []
        : route.tsDecides && !noKey
            ? [`Defect types TypeSafe.ai suggests with ${minConfidence}% confidence or more are written to untriaged failures while the accuracy gate is open; the run grid marks them "AI". A person's label is never overwritten, and flagged or taken-over decisions are never applied.`]
            : ['Auto-apply needs TypeSafe.ai deciding; suggestions from the LLM are never applied.'];
    const store = {
        id: 'store', title: 'Store and show', status: 'run',
        detail: dedup ? 'The answer is saved on every result in its group and shown on the run.' : 'Each analysis is saved on its result and shown on the run.',
        notes: ['A failed attempt is kept as a failed analysis; Retry failed groups on the run tries those groups again.', ...autoApplyNotes],
        chips: [chip('fa.auto_apply_defect_type', 'Set the defect type automatically', autoApply ? `At ≥ ${minConfidence}%` : 'Off')],
        sends: [], parts: [],
    };

    return { trigger: mode, blocked: false, canAnalyze: true, unredacted: !redact, steps: [start, group, evidence, decide, explain, store] };
}

// effectiveDraft is the card's draft with the saved value back in every field that fails
// validation, so the diagram draws what the valid changes would do once saved.
export function effectiveDraft(card) {
    if (!card?.draft) return null;
    const out = { ...card.draft };
    for (const field of Object.keys(card.errors || {})) out[field] = card.saved?.[field];
    return out;
}

// diagramModel turns the section's inputs into what the diagram shows: a skeleton while any input
// loads, an error naming what failed to load, or the flow for the chosen trigger.
export function diagramModel({ aiEnabled, aiFeaturesStatus, providers, providersStatus, typesafeCard, faCard, trigger }) {
    const statuses = {
        aiFeatures: aiFeaturesStatus,
        providers: providersStatus === 'stale' ? 'ready' : providersStatus,
        typesafe: typesafeCard?.status,
        failureAnalysis: faCard?.status,
    };
    const failed = Object.keys(statuses).filter((k) => statuses[k] === 'error');
    if (failed.length > 0) return { state: 'error', failed: failed.map((k) => LOAD_LABELS[k]) };
    if (Object.values(statuses).some((s) => s !== 'ready')) return { state: 'loading' };
    const invalid = [
        ...Object.keys(typesafeCard.errors || {}).map((f) => `ts.${f}`),
        ...Object.keys(faCard.errors || {}).map((f) => `fa.${f}`),
    ];
    return {
        state: 'ready',
        stale: providersStatus === 'stale',
        dirty: !!typesafeCard.dirty || !!faCard.dirty,
        flow: buildAnalysisFlow({
            aiEnabled,
            typesafe: { ...effectiveDraft(typesafeCard), api_key_status: typesafeCard.saved?.api_key_status },
            failureAnalysis: effectiveDraft(faCard),
            provider: defaultProvider(providers),
            trigger,
            invalid,
        }),
    };
}
