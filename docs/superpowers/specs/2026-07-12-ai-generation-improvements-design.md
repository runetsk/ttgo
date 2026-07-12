# AI Generation Improvements Design

**Status:** Approved for implementation planning

**Date:** 2026-07-12

**Scope:** AI test generation reliability and cost, generation quality, and reviewer workflow

## Summary

TTGO's AI generation feature already supports multiple LLM providers, requirement and child-requirement context, configurable coverage and detail, exact prompt preview, tolerant response parsing, retry on malformed output, token feedback, and draft acceptance. The next stage is to turn that transient prompt-and-response flow into a durable, measurable generation lifecycle.

The design has three coordinated workstreams:

1. **Reliability and cost** provide structured output, durable records, safe acceptance, and observable usage.
2. **Generation quality** measures requirement coverage, draft quality, and duplication, then supports targeted repair.
3. **Reviewer workflow** exposes those capabilities through editable, resumable, efficient review sessions.

The work is delivered foundation-first because quality analytics require durable generation records, and the reviewer workflow needs stable quality and lifecycle APIs.

## Goals

- Reduce structured-response failures to below 1% for supported cloud providers.
- Make generation and acceptance idempotent and prevent partially accepted batches.
- Preserve unfinished generation sessions across refreshes and sign-ins.
- Explain quality findings instead of presenting an opaque aggregate score.
- Map every generated draft to one or more requirement coverage targets.
- Warn about likely duplicates within a batch and against existing tests.
- Let reviewers edit, regenerate, compare, accept, and reject individual drafts.
- Measure latency, token usage, configured cost, acceptance, editing, rejection, and duplication.
- Support provider and prompt comparisons without sending TTGO data outside the configured provider.

## Non-goals

- Training or fine-tuning external models with customer data.
- Automatically accepting generated tests without human review.
- Replacing deterministic validation with an LLM evaluator.
- Adding an embedding database in the first release.
- Maintaining a centrally hosted model-price catalog. Administrators configure prices because models and prices change.
- Making the whole generation flow asynchronous before measurements show that synchronous requests are inadequate.

## Current-State Findings

- `GeneratedTestCase` is explicitly transient and lives only in the HTTP response and frontend state.
- The frontend context has an `editDraft` state operation, but the draft detail interface is read-only.
- The OpenAI-compatible client requests `json_object`, while the generation prompt asks for a top-level JSON array.
- Malformed output receives one lower-temperature retry, but transient rate-limit and server errors do not have a bounded retry policy.
- Batch acceptance creates and links test cases one at a time. A later failure can leave earlier tests committed, and link failures are logged without rolling back the created test.
- Token use and latency are returned for the most recent request but are not retained for historical or cost analysis.
- The feature does not capture structured rejection reasons, edit deltas, prompt versions, or acceptance-without-edit outcomes.

## Architecture

### Generation lifecycle

The lifecycle is:

1. Resolve requirement context, coverage targets, template version, provider, model, and generation settings.
2. Create an idempotent generation run.
3. Request provider-native structured output where supported.
4. Normalize and validate the response into persisted drafts.
5. Evaluate deterministic quality and duplicate candidates.
6. Optionally critique and repair selected weak drafts.
7. Let a reviewer edit, regenerate, reject, or accept drafts.
8. Atomically materialize accepted drafts as test cases and requirement links.
9. Aggregate lifecycle events into quality, latency, and cost reporting.

### Persistent models

#### `AIGenerationRun`

Stores one generation attempt:

- ID and idempotency key.
- Requirement ID, requesting user ID, provider ID, and model name.
- Prompt-template type, version, and content hash.
- Coverage level, detail level, additional instructions, and resolved token budget.
- Status: `pending`, `running`, `completed`, `failed`, or `cancelled`.
- Start/completion timestamps and duration.
- Prompt, completion, and total token usage.
- Configured estimated cost.
- Retry count, finish reason, and normalized error category/message.
- Parent run ID when a run is cloned or regenerated.

The exact rendered prompt may be retained because TTGO already exposes it in the UI, but raw provider responses remain disabled by default. Neither record stores credentials.

#### `AIGeneratedDraft`

Stores one draft in a run:

- ID, run ID, stable position, and current version.
- Original and current name, category, description, steps, and source references.
- Status: `pending`, `accepted`, `rejected`, or `superseded`.
- Validation findings, quality dimensions, and duplicate candidates.
- Edited flag and accepted test-case ID.
- Created and updated timestamps.

#### `AIGenerationEvent`

Append-only events record lifecycle actions:

- `generated`, `validated`, `edited`, `regenerated`, `rejected`, `accepted`, and `restored`.
- Run and draft IDs, actor ID, timestamp, structured reason, and safe metadata.

Events provide an audit trail and feed aggregate reporting without requiring prompt inspection.

### Canonical provider response

All providers normalize to one envelope:

```json
{
  "test_cases": [
    {
      "name": "[Functional] Sign in with valid credentials",
      "category": "Functional",
      "description": "Verifies successful authentication.",
      "source_refs": ["AC-1"],
      "steps": [
        {
          "action": "Enter user@example.com in the Email field.",
          "expected_result": "The Email field contains user@example.com."
        }
      ]
    }
  ]
}
```

The LLM abstraction gains optional structured-output capabilities. Cloud providers use a JSON schema when their configured endpoint supports it. Compatible providers fall back to JSON-object mode, and local providers may use prompt-only output. The existing tolerant parser remains as a compatibility fallback.

## Workstream 1: Reliability and Cost

### Structured output and validation

- Align every prompt, provider request, parser, preview, and test fixture with the canonical object envelope.
- Define one reusable generation schema and deterministic validator.
- Validate required strings, maximum lengths, allowed categories, non-empty steps, and step field completeness.
- Preserve valid drafts when some drafts fail validation.
- Return field-level findings and support repairing only invalid drafts.

### Idempotency and atomic acceptance

- Accept an idempotency key on generation and acceptance mutations.
- Repeating a completed request returns its existing result.
- Reject concurrent requests using the same key while one is running.
- Move batch materialization into one store transaction.
- Create category folders, test cases, steps, and requirement links within that transaction.
- Validate all drafts and target folders before writes begin.
- Roll back the entire batch on any creation or link failure.
- Record the accepted test-case ID on each draft in the same transaction.

### Retry and error handling

- Retry rate limits and transient 5xx/network failures with bounded exponential backoff and jitter.
- Respect request cancellation, provider timeout, and any provider `Retry-After` value.
- Keep one targeted repair attempt for malformed structured output.
- Normalize errors into authentication, authorization, rate limit, timeout, provider, schema, parse, validation, cancellation, and internal categories.
- Show actionable messages while retaining detailed server logs.

### Usage and cost

- Persist token usage and latency per run and attempt.
- Add optional prompt/completion prices per million tokens to provider configuration.
- Calculate estimated run cost, cost per generated draft, and cost per accepted test.
- Add soft per-request and monthly budgets with warnings before expensive operations.
- Do not silently lower coverage, change providers, or truncate a request to meet a budget.

## Workstream 2: Generation Quality

### Coverage targets

- Derive stable coverage targets from acceptance criteria, child requirements, business rules, boundaries, and explicit error behavior.
- Prefer deterministic extraction from structured requirement fields and child identifiers.
- Use an optional LLM extraction pass only for unstructured descriptions.
- Require generated drafts to include `source_refs` pointing at target IDs.
- Calculate uncovered, covered, and over-represented targets.
- Support generating additional drafts only for selected uncovered targets.

### Explainable quality rubric

Evaluate separate dimensions:

- Requirement relevance.
- Scenario uniqueness.
- Preconditions and test-data specificity.
- Action clarity.
- Expected-result observability.
- Negative-path completeness where relevant.
- Traceability to coverage targets.
- Structural validity.

Deterministic checks run first and produce findings such as `expected result is vague` or `no source criterion linked`. An optional LLM critic can add semantic findings, but it cannot override structural validation. The UI displays dimensions and findings, not only a composite number.

### Duplicate detection

- Normalize draft names, descriptions, and steps.
- Detect exact and near-exact duplicates within the current generation run.
- Use existing SQLite FTS5 search to retrieve candidate matches from the target folder and tests linked to the requirement.
- Rank candidates using lexical and structural similarity.
- Optionally send only ambiguous top candidates to an LLM judge.
- Show the matching test and reason; never automatically delete a draft.

### Critique and targeted repair

- Make the critic optional for `thorough` and `comprehensive` generation.
- Send only drafts with selected findings to the repair pass.
- Retain original and repaired versions and their separate token usage.
- Allow reviewers to compare and choose either version.
- Avoid regenerating an entire otherwise valid batch.

### Feedback and evaluation

Capture rejection reasons:

- Duplicate.
- Irrelevant.
- Incorrect.
- Too vague.
- Incomplete coverage.
- Poor steps.
- Other, with an optional note.

Track accepted unchanged, accepted after editing, rejected, regenerated, and restored outcomes. Build a versioned local benchmark set spanning happy paths, validation-heavy forms, APIs, security, parent requirements, ambiguity, and boundary-heavy rules. Deterministic evaluation runs in CI; calls to real providers are opt-in.

## Workstream 3: Reviewer Workflow

### Editable drafts

- Connect the existing frontend `editDraft` operation to the draft detail pane.
- Edit name, category, description, and individual steps.
- Add, remove, and reorder steps.
- Validate fields inline and mark edited drafts.
- Support undoing local changes or restoring the generated version.
- Autosave edits through the draft API.

### Draft-level regeneration

- Regenerate one draft with an optional instruction.
- Offer targeted actions such as `make more specific`, `add a negative case`, and `repair selected findings`.
- Generate alternatives without replacing the original.
- Show a concise field and step diff before choosing a replacement.
- Mark replaced versions as superseded rather than deleting them.

### Resumable sessions and history

- Resume an unfinished run after refresh or sign-in.
- List generation history for a requirement.
- Show provider, model, prompt version, user, settings, tokens, cost, and outcome.
- Reopen prior runs read-only and clone one with current context or settings.
- Warn when the requirement changed after a run was generated.

### Quality-aware batch review

- Filter drafts by pending, edited, invalid, low quality, possible duplicate, uncovered target, accepted, or rejected.
- Show a requirement-to-draft coverage matrix beside the review list.
- Add keyboard navigation and accept/reject shortcuts.
- Support structured rejection reasons and multi-select actions.
- Provide `Accept all clean drafts`, excluding invalid or unresolved high-confidence duplicate drafts.
- Show a final commit summary of tests, folders, links, and overridden warnings.

## API Shape

The existing generation endpoint remains available during migration. New lifecycle endpoints are introduced under `/ai-generations`:

- `POST /ai-generations` creates or returns an idempotent run.
- `GET /ai-generations/{id}` returns the run, drafts, findings, and coverage summary.
- `GET /ai-generations?requirement_id=...` lists history.
- `PATCH /ai-generations/{id}/drafts/{draft_id}` saves draft edits.
- `POST /ai-generations/{id}/drafts/{draft_id}/regenerate` creates an alternative version.
- `POST /ai-generations/{id}/drafts/{draft_id}/reject` records a rejection reason.
- `POST /ai-generations/{id}/accept` atomically materializes selected drafts.
- `POST /ai-generations/{id}/cancel` cancels a running request when possible.

The frontend migrates to these endpoints, after which the old generation and acceptance endpoints can delegate internally and later be deprecated.

## Privacy and Security

- Continue routing all provider calls through the backend.
- Never expose or persist provider credentials in generation records.
- Treat prompts, drafts, edits, and provider responses as potentially sensitive.
- Disable raw-response retention by default and apply a retention period if enabled.
- Apply existing authorization and LLM rate limiting to all lifecycle endpoints.
- Authorize run and draft access using the same authenticated read/write permissions required for their linked requirement.
- Sanitize stored descriptions before rendering and bound all provider and request payload sizes.
- Record user-visible audit events without copying sensitive prompt text into audit action strings.

## Testing Strategy

### Backend

- Unit tests for schema generation, validation, normalization, coverage mapping, quality rules, duplicate ranking, cost calculation, and error classification.
- Provider contract tests for schema-capable, JSON-object, and prompt-only modes.
- Store tests for generation lifecycle transitions and idempotency.
- Transaction tests proving that acceptance rolls back test cases, folders, steps, and links on every injected failure point.
- API tests for permissions, request replay, cancellation, partial validation, regeneration, rejection, and history.
- Migration tests for new tables and existing installations.

All Go tests and builds use the `sqlite_fts5` build tag required by TTGO.

### Frontend

- Component tests for draft editing, validation, diffing, filters, findings, coverage matrix, budgets, and commit summary.
- Context tests for autosave, refresh recovery, stale requirement warnings, and conflict handling.
- Playwright coverage for generate, edit, regenerate, reject, resume, and atomically accept a batch.

### Evaluation

- Version benchmark inputs and expected deterministic findings.
- Keep real-provider evaluation opt-in and exclude it from required CI.
- Store evaluation results by prompt version, provider, and model for manual comparison.

## Observability and Success Measures

Establish the baseline before setting improvement targets. Report:

- Generation success, parse failure, validation failure, retry, timeout, and cancellation rates.
- Median and p95 generation duration.
- Tokens and configured cost per run, generated draft, and accepted test.
- Acceptance without edit, acceptance after edit, rejection, and regeneration rates.
- Rejection reasons and possible-duplicate override rate.
- Coverage-target recall and average uncovered targets per completed run.
- Median reviewer time from completed generation to final decision.

Initial release gates are:

- Less than 1% structured-response failure for schema-capable supported providers in the benchmark.
- Zero partial batches in transactional acceptance tests.
- Idempotency tests demonstrate no duplicate runs or test cases after replay.
- Every accepted test is linked to its requirement or the transaction fails.
- Every quality warning has a human-readable reason and supporting field or match.

## Delivery Sequence

1. **Baseline and contracts:** metrics definitions, canonical envelope, validation rules, lifecycle schema, and compatibility approach.
2. **Correctness foundation:** structured output, persistent runs/drafts/events, idempotency, and transactional acceptance.
3. **Quality engine:** coverage targets, deterministic rubric, FTS5 duplicate candidates, and evaluation harness.
4. **Core reviewer workflow:** editable drafts, autosave, resumable sessions, findings, filters, and commit summary.
5. **Targeted regeneration:** critic, repair, alternatives, version preservation, and comparison.
6. **Learning and cost analytics:** feedback reports, provider/model comparisons, configured cost, and soft budgets.

Each stage is independently releasable. Schema and lifecycle work precede quality reporting; quality contracts precede the final review interface. Basic draft editing may ship early because the state operation already exists, but server persistence remains the source of truth once lifecycle APIs are available.

## Compatibility and Rollout

- Add database models through the existing migration mechanism.
- Preserve existing provider records; new pricing and capability fields are optional.
- Keep current prompt templates working by wrapping their parsed output in the canonical envelope when necessary.
- Keep the existing generation routes as compatibility adapters during frontend migration.
- Gate persisted sessions, critic/repair, and cost controls independently.
- Record notable behavior and API changes under `CHANGELOG.md` as implementation lands.
