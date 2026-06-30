package main

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type pluginConfig struct {
	Enabled                   bool     `yaml:"enabled"`
	SourceFormats             []string `yaml:"source_formats"`
	Models                    []string `yaml:"models"`
	ProviderWrapper           bool     `yaml:"provider_wrapper"`
	EnsureAuth                bool     `yaml:"ensure_auth"`
	RegisteredModels          []string `yaml:"registered_models"`
	ProviderWeight            int      `yaml:"provider_weight"`
	StreamInterceptorFallback bool     `yaml:"stream_interceptor_fallback"`
}

func defaultPluginConfig() pluginConfig {
	return pluginConfig{
		Enabled:                   true,
		SourceFormats:             []string{"codex", "openai-response", "openai", "chat-completions"},
		Models:                    []string{"*"},
		ProviderWrapper:           true,
		EnsureAuth:                true,
		RegisteredModels:          []string{"gpt-5.5"},
		ProviderWeight:            512,
		StreamInterceptorFallback: false,
	}
}

func decodeConfig(raw []byte) (pluginConfig, error) {
	cfg := defaultPluginConfig()
	if strings.TrimSpace(string(raw)) != "" {
		if err := yaml.Unmarshal(raw, &cfg); err != nil {
			return pluginConfig{}, fmt.Errorf("invalid %s config: %w", pluginIdentifier, err)
		}
	}
	cfg.SourceFormats = normalizeStringList(cfg.SourceFormats, true)
	cfg.Models = normalizeStringList(cfg.Models, false)
	cfg.RegisteredModels = normalizeStringList(cfg.RegisteredModels, false)
	if cfg.ProviderWeight <= 0 {
		cfg.ProviderWeight = defaultPluginConfig().ProviderWeight
	}
	if cfg.ProviderWeight > 4096 {
		cfg.ProviderWeight = 4096
	}
	return cfg, nil
}

func normalizeStringList(input []string, lower bool) []string {
	out := make([]string, 0, len(input))
	for _, item := range input {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if lower {
			item = strings.ToLower(item)
		}
		out = append(out, item)
	}
	return out
}
