package main

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

const syntheticAuthFileName = pluginIdentifier + "-plugin.json"
const syntheticAuthID = pluginIdentifier + "-plugin"

func registerModels(_ []byte) ([]byte, error) {
	return okEnvelope(pluginapi.ModelRegistrationResponse{Provider: pluginIdentifier})
}

func staticModels(_ []byte) ([]byte, error) {
	return okEnvelope(pluginapi.ModelResponse{Provider: pluginIdentifier})
}

func modelsForAuth(raw []byte) ([]byte, error) {
	var req rpcAuthModelRequest
	if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
		return nil, errUnmarshal
	}
	cfg := loadedConfig()
	if !cfg.Enabled || !cfg.ProviderWrapper {
		return okEnvelope(pluginapi.ModelResponse{Provider: pluginIdentifier})
	}
	if !strings.EqualFold(strings.TrimSpace(req.AuthProvider), pluginIdentifier) {
		return okEnvelope(pluginapi.ModelResponse{})
	}
	models := registeredModelInfos(cfg)
	return okEnvelope(pluginapi.ModelResponse{
		Provider: pluginIdentifier,
		Models:   models,
	})
}

func registeredModelInfos(cfg pluginConfig) []pluginapi.ModelInfo {
	names := registeredModelNames(cfg)
	if len(names) == 0 {
		return nil
	}
	weight := cfg.ProviderWeight
	if weight <= 0 {
		weight = defaultPluginConfig().ProviderWeight
	}
	out := make([]pluginapi.ModelInfo, 0, len(names)*weight)
	for _, name := range names {
		info := pluginapi.ModelInfo{
			ID:          name,
			Object:      "model",
			OwnedBy:     pluginIdentifier,
			Type:        "chat",
			DisplayName: name,
			Name:        name,
		}
		for i := 0; i < weight; i++ {
			out = append(out, info)
		}
	}
	return out
}

func registeredModelNames(cfg pluginConfig) []string {
	candidates := cfg.RegisteredModels
	if len(candidates) == 0 {
		candidates = concreteModelPatterns(cfg.Models)
	}
	if len(candidates) == 0 {
		candidates = defaultPluginConfig().RegisteredModels
	}
	out := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, model := range candidates {
		model = strings.TrimSpace(model)
		if model == "" || strings.Contains(model, "*") {
			continue
		}
		key := strings.ToLower(model)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, model)
	}
	return out
}

func concreteModelPatterns(patterns []string) []string {
	out := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" || strings.Contains(pattern, "*") {
			continue
		}
		out = append(out, pattern)
	}
	return out
}

func ensureSyntheticAuth() {
	rawAuth, errMarshal := json.Marshal(map[string]any{
		"id":        syntheticAuthID,
		"type":      pluginIdentifier,
		"email":     pluginIdentifier + "@plugin.local",
		"note":      "Synthetic auth used by the reasoning-516-retry CPA plugin wrapper provider.",
		"priority":  10000,
		"generated": true,
	})
	if errMarshal != nil {
		return
	}
	_, errSave := callHost(pluginabi.MethodHostAuthSave, pluginapi.HostAuthSaveRequest{
		Name: syntheticAuthFileName,
		JSON: rawAuth,
	})
	if errSave != nil {
		hostLog("", "debug", "reasoning-516-retry synthetic auth save skipped", map[string]any{
			"error": errSave.Error(),
		})
	}
}

func parseAuth(raw []byte) ([]byte, error) {
	var req pluginapi.AuthParseRequest
	if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
		return nil, errUnmarshal
	}
	if !isSyntheticAuthJSON(req.RawJSON) {
		return okEnvelope(pluginapi.AuthParseResponse{Handled: false})
	}
	return okEnvelope(pluginapi.AuthParseResponse{
		Handled: true,
		Auth:    syntheticAuthData(req.RawJSON, req.FileName),
	})
}

func refreshAuth(raw []byte) ([]byte, error) {
	var req rpcAuthRefreshRequest
	if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
		return nil, errUnmarshal
	}
	authJSON := req.StorageJSON
	if !isSyntheticAuthJSON(authJSON) {
		authJSON = syntheticAuthJSON()
	}
	return okEnvelope(pluginapi.AuthRefreshResponse{
		Auth:             syntheticAuthData(authJSON, syntheticAuthFileName),
		NextRefreshAfter: time.Now().UTC().Add(24 * time.Hour),
	})
}

func syntheticAuthData(rawJSON []byte, fileName string) pluginapi.AuthData {
	if len(strings.TrimSpace(fileName)) == 0 {
		fileName = syntheticAuthFileName
	}
	if len(strings.TrimSpace(string(rawJSON))) == 0 {
		rawJSON = syntheticAuthJSON()
	}
	return pluginapi.AuthData{
		Provider:    pluginIdentifier,
		ID:          syntheticAuthID,
		FileName:    fileName,
		Label:       "Reasoning 516 Retry Wrapper",
		StorageJSON: append([]byte(nil), rawJSON...),
		Attributes: map[string]string{
			"auth_kind":             "plugin",
			"priority":              "10000",
			"reasoning_516_retry":   "true",
			"reasoning_516_wrapper": "true",
		},
		NextRefreshAfter: time.Now().UTC().Add(24 * time.Hour),
	}
}

func syntheticAuthJSON() []byte {
	raw, _ := json.Marshal(map[string]any{
		"id":        syntheticAuthID,
		"type":      pluginIdentifier,
		"email":     pluginIdentifier + "@plugin.local",
		"note":      "Synthetic auth used by the reasoning-516-retry CPA plugin wrapper provider.",
		"priority":  10000,
		"generated": true,
	})
	return raw
}

func isSyntheticAuthJSON(raw []byte) bool {
	var obj map[string]any
	if errUnmarshal := json.Unmarshal(raw, &obj); errUnmarshal != nil {
		return false
	}
	for _, key := range []string{"type", "provider"} {
		if strings.EqualFold(strings.TrimSpace(stringValue(obj[key])), pluginIdentifier) {
			return true
		}
	}
	return false
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}
