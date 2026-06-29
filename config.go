package main

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type pluginConfig struct {
	Enabled       bool     `yaml:"enabled"`
	SourceFormats []string `yaml:"source_formats"`
	Models        []string `yaml:"models"`
}

func defaultPluginConfig() pluginConfig {
	return pluginConfig{
		Enabled:       false,
		SourceFormats: []string{"codex", "openai-response", "openai"},
		Models:        []string{"*"},
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
