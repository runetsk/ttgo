import http from 'node:http';

// Starts a self-hosted server speaking the OpenAI-compatible chat-completions
// wire format the backend calls (POST {endpoint_url}/v1/chat/completions), and
// registers it as the default `local` LLM provider. Loopback passes the
// integration SSRF guard (safehttp.ValidateIntegrationURL); is_default wins the
// server's single-default invariant so the studio resolves to this provider.
//
// `api` is an ApiClient (registers/deletes the provider). `envelope` is the JSON
// the model "returns" as the assistant message content — a string (or object,
// which is stringified), or a `(callCount) => string` function to vary the
// response across calls (regenerate flows).
//
// Returns { url, providerId, dispose() }; `dispose` deletes the provider and
// closes the server. Usable from the `fakeLLM` fixture and directly in a
// describe-scoped beforeAll/afterAll.
export async function startFakeLLM(api, envelope) {
    let calls = 0;
    const server = http.createServer((req, res) => {
        let content = typeof envelope === 'function' ? envelope(calls++) : envelope;
        if (typeof content !== 'string') content = JSON.stringify(content);
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({
            model: 'fake-model',
            choices: [{ finish_reason: 'stop', message: { content } }],
            usage: { prompt_tokens: 50, completion_tokens: 150, total_tokens: 200 },
        }));
    });
    await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
    const url = `http://127.0.0.1:${server.address().port}`;

    const provider = await api.createLlmProvider({
        label: `e2e-fake-llm-${Date.now()}`,
        provider_type: 'local',
        endpoint_url: url,
        model_name: 'fake-model',
        enabled: true,
        is_default: true,
    });

    return {
        url,
        providerId: provider.id,
        async dispose() {
            await api.deleteLlmProvider(provider.id).catch(() => {});
            await new Promise((resolve) => server.close(resolve));
        },
    };
}
