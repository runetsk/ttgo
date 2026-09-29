// Plain-language explanations of the failure-analysis settings and steps, written from the server
// code. One source for the cards' "What this does" toggles and the process diagram (a chip's
// tooltip is `what`; "How this step works" uses STEP_HELP). Keys are the data-setting anchors:
// ai.* (master switch), fa.* (AI Failure Analysis card), ts.* (TypeSafe.ai card), provider.default.
// When server behaviour changes (dedup normalization, redaction patterns, thresholds), update here.

export const SETTING_HELP = {
    'ai.enabled': {
        title: 'AI features',
        what: 'Master switch for every AI feature, including failure analysis.',
        off: 'AI actions are hidden, new analyses are refused and completed runs are not queued. An analysis already running finishes.',
    },
    'fa.enabled_on_completion': {
        title: 'Auto-analyze on run completion',
        what: 'When a run is completed with failing results, its failures are queued as if someone clicked Analyze failures.',
        details: 'It needs something that may analyze automatically: TypeSafe.ai with "Allow on automatic analysis", or a default LLM with "Auto failure analysis" ticked in its provider settings. The LLM needs that tick for any part it plays in an automatic analysis. Runs completed before you switch this on are not queued.',
        off: 'Failures are analyzed only when someone asks.',
    },
    'fa.dedup_enabled': {
        title: 'Deduplicate similar failures',
        what: 'Failures with the same failure type and the same error text form one group; one result is analyzed and its answer is copied to the rest.',
        details: 'Before comparing, standalone numbers of four or more digits (order and request IDs), timestamps and hex addresses (0x7ff…) are replaced, and folder paths written with / are cut to the file name. Numbers joined to letters, such as 30000ms, and Windows \\ paths still count.',
        example: '"Timeout 30000ms waiting for #checkout (order 55121)" and the same message with order 55904 → one group, because only the order number differs.',
        off: 'Every failing result is analyzed on its own, which is slower and costs a call each.',
    },
    'ts.semantic_dedup_enabled': {
        title: 'Semantic failure grouping',
        what: 'After identical grouping, TypeSafe.ai merges groups of the same failure type that it is at least 80% sure describe the same failure.',
        details: 'Only groups whose wording overlaps are compared. TypeSafe.ai receives short excerpts: the test name, failure type, error and the start of the stack. These calls happen before the cap, so Max analyses per run does not limit them. Needs Deduplicate similar failures and a usable TypeSafe.ai key. After a group is explained, TypeSafe.ai also checks, in one request, that the explanation fits each semantically grouped result (up to 40 per group). A result rated below 50% shows "This explanation may not apply to this result" and an Explain this result button.',
        example: '"POST /api/orders failed: upstream payment service returned 503" and "POST /api/orders failed: payment gateway unavailable (503)" → one group.',
        off: 'Only identical failures are grouped.',
    },
    'fa.redaction_enabled': {
        title: 'Redact secrets',
        what: 'Before failure text leaves the server, recognized secrets are replaced in every field sent to TypeSafe.ai and the LLM.',
        details: 'Bearer tokens; OpenAI, AWS, GitHub, Stripe, Slack and Google keys; credentials in URLs; private key blocks; JWTs; password=, token= and api_key= style values; email addresses. It is best effort: values in other shapes, or too short to look like a secret (password=abc), are sent as recorded, and the prompt template is sent as written.',
        example: 'password=hunter2 → password=<REDACTED>; Authorization: Bearer eyJ… → Authorization: Bearer <REDACTED_TOKEN>; qa@example.com → <REDACTED_EMAIL>.',
        off: 'Failure text is sent as recorded.',
    },
    'fa.max_analyses_per_run': {
        title: 'Max analyses per run',
        what: 'The most failure groups one analysis covers, largest groups first (1 to 500).',
        example: '300 failing results in 40 groups with a cap of 20 → the 20 largest groups are analyzed; the other 20 stay without an analysis. Analyze them one result at a time, or raise the cap.',
    },
    'fa.parallel_groups': {
        title: 'Groups analyzed at once',
        what: 'How many groups are analyzed side by side (1 to 8). Nearly all the time is spent waiting on TypeSafe.ai and the LLM.',
        details: 'Lower it if the LLM provider answers with rate-limit errors.',
        example: '24 groups that wait about 6 s each on the LLM take about 2½ minutes one at a time and roughly half that four at a time; the slowest calls set the pace.',
    },
    'fa.llm_call_timeout_seconds': {
        title: 'LLM call timeout',
        what: 'How long one request to the default LLM may take during failure analysis (10 to 120 s). A request cut off at this limit counts as a call timeout and is sent again once.',
        details: 'It applies to every failure-analysis LLM call: deciding when the LLM decides, taking over, stepping in when TypeSafe is unavailable, explanations and Explain. Test generation keeps its own timeouts. The time one group may take grows with it: 6 × the TypeSafe timeout (its decision, and the check that the group\'s explanation fits each semantically grouped result) + 4 × this timeout + pauses, and never less than 5 minutes (9½ minutes with the defaults).',
        example: 'At 45 s, a call that would hang for two minutes is cut at 45 s and sent again, instead of holding its group for the whole two minutes.',
    },
    'fa.hedge_after_seconds': {
        title: 'Hedge slow LLM calls after',
        what: 'When an LLM request has not answered after this many seconds, an identical second request is sent; the first answer is used and the other is cancelled. 0 turns it off; otherwise from 3 s up to one second below the LLM call timeout.',
        details: 'It trims the slow tail of LLM calls at the price of extra requests. Each hedge that fires is recorded as a "hedge" cost event priced at the winning request\'s prompt, an estimate because the cancelled request\'s usage is not reported, and budget estimates count the LLM part twice while it is on. The run banner shows how many hedges fired and won.',
        example: 'At 10 s: answers that take the usual 6 s are untouched; a call still waiting at 10 s gets a twin, and whichever answers first is used.',
    },
    'fa.few_shot_examples': {
        title: 'Past triage examples',
        what: 'How many past failures that people triaged (0 to 8) go with each group, so the LLM and TypeSafe.ai see how this team labels failures. 0 turns it off.',
        details: 'Examples come from other runs, from the 90 days before the failure being analyzed: failures where the AI suggested a defect type for that result itself and a person set product bug, automation bug or system issue. The same test comes first, then the same failure type, then the newest, with agreements and corrections balanced. Each example\'s error is cut to 300 characters and redacted when Redact secrets is on; examples are dropped first when the evidence is too long (in the LLM prompt, right after the group\'s related failures). Decisions made with at least one example are stamped fa-verdict-v8, others fa-verdict-v7 (fa-verdict-v6 and fa-verdict-v5 before the companion questions), so the accuracy panel can tell them apart.',
        example: 'A timeout on #checkout that people twice marked as an automation bug, although the AI said product bug, is shown to the model with that correction.',
    },
    'fa.auto_apply_defect_type': {
        title: 'Set the defect type automatically',
        what: 'When TypeSafe.ai answers the defect-type question at or above the minimum confidence, its suggestion is written to failures nobody has triaged yet (To Investigate). The run grid marks these labels "AI" with a Confirm button. Off by default.',
        details: 'It can be switched on only while its accuracy gate is open. The gate needs at least 50 failures, from the last 90 days, where people triaged TypeSafe\'s own defect-type suggestion made at this minimum confidence (fa-verdict-v7 and fa-verdict-v8 decisions only), and people must have agreed with at least 95 % of them. Confirming a label auto-apply set does not count: only independent triage does. Each analysis checks the gate once. While the gate is closed, nothing is applied and the run banner says "Auto-apply paused". Only TypeSafe decisions answered by the defect-type question qualify. LLM decisions, takeovers, suggestions derived from the verdict and decisions flagged for possible prompt injection never do. A person\'s label is never overwritten, and a newer analysis moves or resets only labels auto-apply wrote. AI labels do not count as decisions: the accuracy panel, the past triage examples and the history the AI sees all leave them out.',
        example: 'At 95 %: "Automation bug, 97 % sure" on an untriaged failure → the row reads Automation bug with an AI badge; Confirm records it as your decision.',
        off: 'Suggestions stay suggestions: Accept on each row writes them.',
    },
    'fa.auto_apply_min_confidence': {
        title: 'Minimum confidence for automatic labels',
        what: "TypeSafe.ai's defect-type confidence needed before its suggestion is written (80 to 99 %, default 95 %). The accuracy gate is measured at this threshold.",
        example: 'Raising it from 95 to 98 applies fewer labels, and the gate then counts only suggestions made with 98 % confidence or more.',
    },
    'fa.prompt_template': {
        title: 'Prompt template',
        what: 'The instructions and layout sent to the LLM with the failure evidence.',
        details: 'It is sent as written, without redaction, and must ask for JSON with verdict, confidence, summary, next_action and rationale.',
    },
    'ts.enabled': {
        title: 'Enable TypeSafe.ai',
        what: 'Turns TypeSafe.ai on for the features switched on in its card.',
        off: 'Nothing is sent to TypeSafe.ai; its settings are kept.',
    },
    'ts.api_key': {
        title: 'API key',
        what: 'Stored encrypted and shown masked. Test connection checks the stored key, so save a new key first.',
        details: 'A key that can no longer be decrypted must be entered again. A new key is checked when it is first used.',
    },
    'ts.model': {
        title: 'Model',
        what: 'The TypeSafe.ai model that answers. Pinned to jev-1.13.0 by default because confidence thresholds are tuned per version.',
    },
    'ts.price_per_mtok': {
        title: 'Price',
        what: 'What TypeSafe.ai charges per million input tokens, in USD. Failure analysis records what each decision and each semantic grouping pass cost at this price, and holds new analyses against the AI budgets with it.',
        details: 'The default is 0.042. Set it to match your plan; 0 records TypeSafe calls as free. Calls already made keep the price that applied when they ran.',
    },
    'ts.timeout_seconds': {
        title: 'Timeout',
        what: 'How long one request to TypeSafe.ai may take (5 to 300 s). A request that times out is not repeated and the decision counts as unavailable.',
        details: 'Rate limits, overload, network and server errors are retried up to three times with a pause, so one decision can take longer than this.',
    },
    'ts.verdict_engine_enabled': {
        title: 'Use for failure verdicts',
        what: 'TypeSafe.ai answers two multiple-choice questions, the verdict and the defect type, with a probability for every option and a confidence.',
        details: 'In the same request TypeSafe.ai answers companion questions. They ask whether the failure text contains instructions aimed at an AI (possible prompt injection), whether the recent outcomes alternate between pass and fail, whether the same failure was seen before, whether the error comes from outside the app, and which linked defect matches. Answers of 80% or more show as chips on the result. At 80% or more on prompt injection the decision is kept, but no explanation is written for the group, and Explain asks before sending the failure to the LLM. The guard runs only when TypeSafe decides: with the LLM deciding, or stepping in because TypeSafe is unavailable, nothing checks the failure text. Decisions are stamped fa-verdict-v7, or fa-verdict-v8 with past triage examples.',
        off: 'The default LLM decides.',
    },
    'ts.allow_auto_failure_analysis': {
        title: 'Allow on automatic analysis',
        what: "TypeSafe.ai's own consent for runs analyzed on completion.",
        details: 'Analyses started by hand, including Retry failed groups, use TypeSafe.ai whenever it is enabled.',
        off: 'Automatic analyses do not contact TypeSafe.ai; the LLM decides for them.',
    },
    'ts.narrative_enabled': {
        title: 'Write an explanation with the default LLM',
        what: 'After TypeSafe.ai decides, the default LLM writes a summary, a next action and the reasoning.',
        details: 'When TypeSafe is unsure or unavailable and the LLM decides instead, its answer includes an explanation either way.',
        off: "TypeSafe's decisions are stored without that extra call, so a group takes about as long as TypeSafe's answer. Explain on a result writes one on demand.",
    },
    'ts.llm_fallback_enabled': {
        title: 'Use the default LLM when TypeSafe is unavailable',
        what: 'What happens when TypeSafe.ai cannot answer: an error, a timeout or no usable key.',
        example: 'TypeSafe times out after 30 s. On: the LLM decides and explains, and the analysis says TypeSafe was unavailable.',
        off: 'The attempt is recorded as failed and can be retried; failure data never goes to the LLM in its place.',
    },
    'ts.escalate_below_pct': {
        title: "Ask the LLM when TypeSafe's confidence is below",
        what: 'Below this confidence the default LLM decides instead of TypeSafe.ai, and the analysis notes what TypeSafe said. 0 never hands over.',
        example: 'At 90%: "flaky test, 72% sure" goes to the LLM, which decides and explains; "product bug, 96% sure" stays with TypeSafe. If the LLM cannot answer, TypeSafe\'s decision is kept.',
    },
    'provider.default': {
        title: 'Default LLM',
        what: 'The enabled default provider in LLM Providers; failure analysis always uses this one.',
        details: 'Automatic analyses use it only when "Auto failure analysis" is ticked in its provider settings.',
    },
};

export const STEP_HELP = {
    start: {
        what: 'An analysis covers every failing result of a run. It starts when you click Analyze failures, when a run is completed (with Auto-analyze on), or from Retry failed groups.',
        example: 'Retry failed groups is a manual action, so it follows the "Started by hand" route even for a run first analyzed automatically.',
    },
    group: {
        what: 'Failing results that look alike are grouped so each group is analyzed once and the answer is copied to the rest.',
        example: '120 failing results that come from 9 distinct errors need 9 analyses, not 120.',
    },
    evidence: {
        what: 'For each group, one result is sent with its context: the error, stack, log and steps, the environment, linked defects and requirements, how this test failed in the 30 days before, and, with Past triage examples on, a few past failures people triaged.',
        example: 'A test that failed with the same timeout three times last week points the model toward a flaky test.',
    },
    decide: {
        what: 'Someone chooses the verdict (product bug, flaky test, environment issue…) and suggests a defect type: TypeSafe.ai when it is set up to decide, otherwise the default LLM. In the same request TypeSafe answers companion questions and checks the failure for possible prompt injection.',
        example: 'TypeSafe answers "flaky test, 93% sure"; with "Ask the LLM below" at 90% that answer is kept.',
    },
    explain: {
        what: 'An explanation is a summary, a suggested next action and the reasoning, written by the default LLM after the decision is shown: one per group, copied to every result in it. A group flagged for possible prompt injection gets none, and a grouped result the explanation may not fit is marked.',
        example: 'With explanations off, a TypeSafe decision shows "No explanation was written" and an Explain button.',
    },
    store: {
        what: 'Every analysis is kept as a version on its result and shown on the run page and in the run grid. With "Set the defect type automatically" on and its accuracy gate open, confident TypeSafe defect types are also written to untriaged failures.',
        example: 'Re-analyze adds a new version; a failed attempt shows as "Analysis failed" and can be retried.',
    },
};
