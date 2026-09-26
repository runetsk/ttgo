// How a stored secret's status reads in the settings UI. The server reports it as
// api_key_status (LLM providers) or api_token_status (Jira, Confluence): missing | ok |
// undecryptable. keyStatusLabel in typesafeSettings.js is the TypeSafe card's counterpart.

const cap = (s) => s.charAt(0).toUpperCase() + s.slice(1);

// secretNote returns what to show beside a secret input, or null when nothing is stored.
export function secretNote(status, masked, noun = 'key') {
    switch (status) {
        case 'undecryptable': return { tone: 'red', text: `Stored ${noun} cannot be decrypted — enter it again` };
        case 'ok': return { tone: 'muted', text: masked ? `Current: ${masked} — leave blank to keep` : `${cap(noun)} stored — leave blank to keep` };
        default: return null;
    }
}

// secretPlaceholder: blank keeps a usable secret; otherwise the input asks for one.
export function secretPlaceholder(status, fallback, noun = 'key') {
    return status === 'ok' ? `Leave blank to keep the current ${noun}` : fallback;
}

// canClearSecret: any stored secret, readable or not, can be removed.
export function canClearSecret(status) {
    return status === 'ok' || status === 'undecryptable';
}

// secretPayload is the secret part of a save: a typed value is sent, a blank one is left out
// (the server keeps what it has), and clear wins so the server never sees both.
export function secretPayload(field, clearField, value, clear) {
    if (clear) return { [clearField]: true };
    const v = typeof value === 'string' ? value.trim() : '';
    return v ? { [field]: v } : {};
}
