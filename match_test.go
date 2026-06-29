package main

import "testing"

func TestShouldHandleMatchesConfig(t *testing.T) {
	cfg := pluginConfig{
		Enabled:       true,
		SourceFormats: []string{"codex"},
		Models:        []string{"gpt-*"},
	}
	if !shouldHandle(cfg, "codex", "gpt-5.4") {
		t.Fatal("shouldHandle() = false, want true")
	}
	if shouldHandle(cfg, "openai", "gpt-5.4") {
		t.Fatal("shouldHandle(openai) = true, want false")
	}
	if shouldHandle(cfg, "codex", "claude-sonnet") {
		t.Fatal("shouldHandle(claude) = true, want false")
	}
}
