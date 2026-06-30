# CPA Reasoning 516 Retry Plugin

CPA native plugin that makes Codex retry a request when the upstream response reports exactly `reasoning_tokens == 516`.

When detected, the plugin discards the upstream response and returns:

```text
[RETRY_REQUIRED_REASONING_516] reasoning_tokens=516; 降智请求自动重试中.
```

The plugin routes matching requests through a CPA executor wrapper that streams chunks through immediately. It does not buffer the model stream, call providers directly, or change the requested model. Response interceptors are also registered as a fallback for traffic that does not route through the executor.

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
```

Fields:

- `enabled`: enables or disables routing and response interception.
- `source_formats`: optional inbound protocol allowlist. Empty means all formats.
- `models`: optional model patterns with `*` wildcard. Empty means all models.

## Build

```bash
go test ./...
go build -buildmode=c-shared -o dist/reasoning-516-retry.so .
```
