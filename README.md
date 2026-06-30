# CPA Reasoning 516 Retry Plugin

CPA native plugin that makes Codex retry a request when the upstream response reports exactly `reasoning_tokens == 516`.

When detected, the plugin discards the upstream response and returns:

```text
[RETRY_REQUIRED_REASONING_516] reasoning_tokens=516; 降智请求自动重试中.
```

The plugin routes matching requests through a CPA executor wrapper that streams chunks through immediately. It does not buffer the model stream, call providers directly, or change the requested model. On CPA builds without model-router support, it creates a local virtual auth and registers a lightweight wrapper provider for configured model IDs. The wrapper uses an internal bypass marker to delegate the actual upstream call back to CPA.

## Configuration

```yaml
plugins:
  enabled: true
  dir: "/app/plugins"
  configs:
    reasoning-516-retry:
      enabled: true
      priority: 100
      source_formats:
        - codex
        - openai-response
        - openai
        - chat-completions
      models:
        - "*"
      provider_wrapper: true
      ensure_auth: true
      registered_models:
        - gpt-5.5
      provider_weight: 512
      stream_interceptor_fallback: false
```

Fields:

- `enabled`: enables or disables routing and response interception.
- `source_formats`: optional inbound protocol allowlist. Empty means all formats.
- `models`: optional model patterns with `*` wildcard. Empty means all models.
- `provider_wrapper`: registers a wrapper provider so the plugin can close streams cleanly after rewriting `reasoning_tokens == 516`.
- `ensure_auth`: creates a local virtual auth entry for the wrapper provider.
- `registered_models`: concrete model IDs exposed by the wrapper provider. Add other model IDs here if needed.
- `provider_weight`: duplicate model registration weight used to prefer the wrapper provider over native providers for those model IDs.
- `stream_interceptor_fallback`: legacy fallback path. It is disabled by default because older CPA versions can append a second upstream terminal error after interceptor output.

## Build

```bash
go test ./...
go build -buildmode=c-shared -o dist/reasoning-516-retry.so .
```
