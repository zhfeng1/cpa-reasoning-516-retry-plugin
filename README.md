# CPA Reasoning 516 Retry Plugin

CPA native plugin that makes Codex retry a request when the upstream response reports exactly `reasoning_tokens == 516`.

When detected, the plugin discards the upstream response and returns:

```text
stream disconnected before completion: [RETRY_REQUIRED_REASONING_516] reasoning_tokens=516; discard this response and resend the request.
```

The plugin delegates model execution back to CPA through the host model callback. It does not call providers directly and does not change the requested model.

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
      models:
        - "*"
```

Fields:

- `enabled`: enables or disables the router.
- `source_formats`: optional inbound protocol allowlist. Empty means all formats.
- `models`: optional model patterns with `*` wildcard. Empty means all models.

## Build

```bash
go test ./...
go build -buildmode=c-shared -o dist/reasoning-516-retry.so .
```
