// LLM provider presets, grouped by the three behaviors the backend implements:
//   OpenAI-compatible → openAICompatClient (Bearer, /v1/chat/completions, json_schema)
//   Claude            → anthropicClient (x-api-key, /v1/messages, prompt-only)
//   Local             → openAICompatClient with the integration SSRF guard (allows LAN)
// OpenRouter + Custom are OpenAI-compatible, so they store provider_type "openai"
// with their own endpoint. The endpoint carries NO trailing /v1 — the backend
// appends /v1/chat/completions (pkg/tracker/llm/openai_compat.go).
export const PROVIDER_GROUPS = [
    {
        key: 'openai_compatible',
        label: 'OpenAI-compatible',
        presets: [
            { key: 'openai',     label: 'OpenAI',          providerType: 'openai',    endpoint: 'https://api.openai.com',                                  model: 'gpt-4o',           color: '#10a37f', bg: 'rgba(16,163,127,0.12)',  initial: 'O' },
            { key: 'gemini',     label: 'Google Gemini',   providerType: 'gemini',    endpoint: 'https://generativelanguage.googleapis.com/v1beta/openai', model: 'gemini-2.5-flash', color: '#4285f4', bg: 'rgba(66,133,244,0.12)',  initial: 'G' },
            { key: 'openrouter', label: 'OpenRouter',      providerType: 'openai',    endpoint: 'https://openrouter.ai/api',                              model: 'openai/gpt-4o',    color: '#6467f2', bg: 'rgba(100,103,242,0.12)', initial: 'R' },
            { key: 'custom',     label: 'Custom',          providerType: 'openai',    endpoint: '',                                                       model: 'model-name',       color: '#64748b', bg: 'rgba(100,116,139,0.12)', initial: '⚙' },
        ],
    },
    {
        key: 'claude',
        label: 'Claude',
        presets: [
            { key: 'anthropic', label: 'Anthropic Claude', providerType: 'anthropic', endpoint: 'https://api.anthropic.com', model: 'claude-sonnet-4-5', color: '#d97706', bg: 'rgba(217,119,6,0.12)', initial: 'A' },
        ],
    },
    {
        key: 'local',
        label: 'Local',
        presets: [
            { key: 'local', label: 'Local / Ollama', providerType: 'local', endpoint: 'http://localhost:11434', model: 'llama3', color: '#8b5cf6', bg: 'rgba(139,92,246,0.12)', initial: '⚙' },
        ],
    },
];

// Flat list of every preset, for key/type lookups.
const ALL_PRESETS = PROVIDER_GROUPS.flatMap(g => g.presets);

// Single source of truth — derived from the openai preset so the two never drift
// (presetMeta is a hoisted function declaration; ALL_PRESETS is defined above).
const OPENAI_DEFAULT_ENDPOINT = presetMeta('openai').endpoint;

// Look up a preset by its key. Unknown keys fall back to the first preset (OpenAI).
export function presetMeta(key) {
    return ALL_PRESETS.find(p => p.key === key) || ALL_PRESETS[0];
}

// Infer which preset a saved provider config represents. OpenAI, OpenRouter, and
// Custom all share provider_type "openai" and are told apart by their endpoint.
export function presetFromConfig({ provider_type, endpoint_url } = {}) {
    const endpoint = (endpoint_url || '').trim();
    switch (provider_type) {
        case 'anthropic': return presetMeta('anthropic');
        case 'local':     return presetMeta('local');
        case 'gemini':    return presetMeta('gemini');
        case 'openai':
        default:
            if (endpoint.includes('openrouter.ai')) return presetMeta('openrouter');
            if (endpoint === '' || endpoint === OPENAI_DEFAULT_ENDPOINT) return presetMeta('openai');
            return presetMeta('custom');
    }
}

export const TEMPLATE_VARS = ['{{COVERAGE}}', '{{TITLE}}', '{{DESCRIPTION}}', '{{CHILDREN}}', '{{DETAIL_LEVEL}}', '{{ADDITIONAL_INSTRUCTIONS}}'];
export const REQUIRED_TEMPLATE_VARS = ['{{COVERAGE}}', '{{TITLE}}', '{{DESCRIPTION}}'];
export const PARENT_REQUIRED_VARS = ['{{COVERAGE}}', '{{TITLE}}', '{{CHILDREN}}'];
