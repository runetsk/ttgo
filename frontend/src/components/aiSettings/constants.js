export const PROVIDER_TYPES = [
    { value: 'openai',    label: 'OpenAI',           color: '#10a37f', bg: 'rgba(16,163,127,0.12)',  initial: 'O' },
    { value: 'gemini',    label: 'Google Gemini',    color: '#4285f4', bg: 'rgba(66,133,244,0.12)',  initial: 'G' },
    { value: 'anthropic', label: 'Anthropic Claude', color: '#d97706', bg: 'rgba(217,119,6,0.12)',   initial: 'A' },
    { value: 'local',     label: 'Local / Ollama',   color: '#8b5cf6', bg: 'rgba(139,92,246,0.12)',  initial: '⚙' },
];

export const DEFAULT_ENDPOINTS = {
    openai:    'https://api.openai.com',
    gemini:    'https://generativelanguage.googleapis.com/v1beta/openai',
    anthropic: 'https://api.anthropic.com',
    local:     'http://localhost:11434',
};

export const TEMPLATE_VARS = ['{{COVERAGE}}', '{{TITLE}}', '{{DESCRIPTION}}', '{{CHILDREN}}', '{{DETAIL_LEVEL}}', '{{ADDITIONAL_INSTRUCTIONS}}'];
export const REQUIRED_TEMPLATE_VARS = ['{{COVERAGE}}', '{{TITLE}}', '{{DESCRIPTION}}'];
export const PARENT_REQUIRED_VARS = ['{{COVERAGE}}', '{{TITLE}}', '{{CHILDREN}}'];

export function providerMeta(type) {
    return PROVIDER_TYPES.find(t => t.value === type) || PROVIDER_TYPES[0];
}
