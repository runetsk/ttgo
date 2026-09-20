// Pure helpers for the TypeSafe settings card. No React, no network.
// Consumer: frontend/src/components/aiSettings/TypeSafeSettingsCard.jsx

export const TIMEOUT_MIN = 5;
export const TIMEOUT_MAX = 300;

const FIELDS = ['enabled', 'model', 'timeout_seconds', 'verdict_engine_enabled', 'semantic_dedup_enabled', 'allow_auto_failure_analysis'];

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

export function keyStatusLabel(settings) {
    switch (settings?.api_key_status) {
        case 'ok': return `Key stored (${settings.api_key_masked})`;
        case 'undecryptable': return 'Stored key cannot be decrypted — enter it again';
        default: return 'No API key stored';
    }
}
