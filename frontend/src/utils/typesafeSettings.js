// Pure helpers for the TypeSafe settings card. No React, no network.
// Consumer: frontend/src/components/aiSettings/TypeSafeSettingsCard.jsx

export const TIMEOUT_MIN = 5;
export const TIMEOUT_MAX = 300;

const FIELDS = ['enabled', 'model', 'price_per_mtok', 'timeout_seconds', 'verdict_engine_enabled', 'narrative_enabled', 'escalate_below_pct', 'llm_fallback_enabled', 'semantic_dedup_enabled', 'allow_auto_failure_analysis',
    'import_structure_enabled', 'draft_review_enabled', 'defect_assist_enabled', 'search_rerank_enabled'];

// escalationValid: the "ask the LLM below N%" threshold is a whole percentage, 0 (never) to 100.
export function escalationValid(pct) {
    return Number.isInteger(pct) && pct >= 0 && pct <= 100;
}

// escalationNote explains the threshold in words under the input.
export function escalationNote(pct) {
    if (!escalationValid(pct)) return 'Enter a whole number from 0 to 100.';
    if (pct === 0) return 'Off: TypeSafe decides every verdict it can.';
    if (pct === 100) return 'Every verdict below 100% goes to the LLM, which in practice is almost all of them.';
    return `Verdicts TypeSafe gives less than ${pct}% confidence are decided by your default LLM instead; the analysis notes what TypeSafe said.`;
}

// priceValid: the USD price per million TypeSafe input tokens; 0 means free.
export function priceValid(v) {
    return typeof v === 'number' && Number.isFinite(v) && v >= 0;
}

// validateTypeSafeDraft returns a message per field the server would reject. Shared by the card
// (inline errors, Save) and the process diagram (an invalid field shows as "invalid").
export function validateTypeSafeDraft(form) {
    const errors = {};
    const t = form?.timeout_seconds;
    if (!Number.isInteger(t) || t < TIMEOUT_MIN || t > TIMEOUT_MAX) {
        errors.timeout_seconds = `Enter a whole number of seconds from ${TIMEOUT_MIN} to ${TIMEOUT_MAX}.`;
    }
    if (typeof form?.model !== 'string' || form.model.trim() === '') {
        errors.model = 'Enter a model name.';
    }
    if (!escalationValid(form?.escalate_below_pct ?? 0)) {
        errors.escalate_below_pct = 'Enter a whole number from 0 to 100.';
    }
    if (form?.price_per_mtok !== undefined && !priceValid(form.price_per_mtok)) {
        errors.price_per_mtok = 'Enter a price of 0 or more (USD per million input tokens).';
    }
    return errors;
}

// defaultProvider picks the LLM provider failure analysis uses: the enabled default.
export function defaultProvider(providers) {
    return (providers || []).find((p) => p.is_default && p.enabled) || null;
}

// typesafeStatus summarises the saved settings for the card's header pill.
export function typesafeStatus(settings) {
    if (!settings?.enabled) return { label: 'Off', tone: 'muted' };
    switch (settings.api_key_status) {
        case 'ok': return { label: 'On', tone: 'green' };
        case 'undecryptable': return { label: 'Key unreadable', tone: 'red' };
        default: return { label: 'No API key', tone: 'amber' };
    }
}

// verdictDependencyNote explains why the explanation, fallback and takeover settings are
// greyed out: they only change anything while TypeSafe decides the verdict.
export function verdictDependencyNote(form) {
    return form?.verdict_engine_enabled ? null : 'Only used while "Use for failure verdicts" is on.';
}

// formFromSettings turns the masked server response into editable form state. The key input
// starts empty: blank means "keep the stored key" on save.
export function formFromSettings(settings) {
    const form = { api_key: '', clear_api_key: false };
    for (const f of FIELDS) form[f] = settings?.[f];
    return form;
}

// buildTypeSafePatch returns only what changed, or null. A typed key is sent; a blank key is
// omitted (preserve); clear_api_key wins over any typed key so the server never sees both.
export function buildTypeSafePatch(form, original) {
    const patch = {};
    for (const f of FIELDS) {
        if (form[f] !== original?.[f]) patch[f] = form[f];
    }
    if (form.clear_api_key) {
        patch.clear_api_key = true;
    } else if (typeof form.api_key === 'string' && form.api_key.trim() !== '') {
        patch.api_key = form.api_key.trim();
    }
    return Object.keys(patch).length === 0 ? null : patch;
}

// canTestConnection reports whether the "Test connection" action should be enabled. It only
// ever exercises the STORED key, so it must be blocked whenever the form holds an unsaved
// change to that key — a typed-but-unsaved value, or a pending clear — otherwise the button
// would silently test the old key while the screen implies it's testing what was just typed.
// It is also blocked when there is no stored key to test at all.
export function canTestConnection(form, settings) {
    if (settings?.api_key_status === 'missing') return false;
    if (form?.clear_api_key) return false;
    if (typeof form?.api_key === 'string' && form.api_key.trim() !== '') return false;
    return true;
}

export function keyStatusLabel(settings) {
    switch (settings?.api_key_status) {
        case 'ok': return `Key stored (${settings.api_key_masked})`;
        case 'undecryptable': return 'Stored key cannot be decrypted — enter it again';
        default: return 'No API key stored';
    }
}
