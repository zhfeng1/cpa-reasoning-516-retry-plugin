package main

import (
	"encoding/json"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

func TestRegisteredModelInfosUsesWeight(t *testing.T) {
	cfg := defaultPluginConfig()
	cfg.RegisteredModels = []string{"gpt-5.5"}
	cfg.ProviderWeight = 3
	models := registeredModelInfos(cfg)
	if len(models) != 3 {
		t.Fatalf("registered models = %d, want 3", len(models))
	}
	for _, model := range models {
		if model.ID != "gpt-5.5" {
			t.Fatalf("model id = %q, want gpt-5.5", model.ID)
		}
	}
}

func TestRegisterModelsReturnsNoStaticModels(t *testing.T) {
	cfg := defaultPluginConfig()
	cfg.RegisteredModels = []string{"gpt-5.5"}
	cfg.ProviderWeight = 2
	currentConfig.Store(cfg)
	t.Cleanup(func() { currentConfig.Store(defaultPluginConfig()) })

	rawResp, errRegister := registerModels(nil)
	if errRegister != nil {
		t.Fatalf("registerModels() error = %v", errRegister)
	}
	var env envelope
	if errDecode := json.Unmarshal(rawResp, &env); errDecode != nil {
		t.Fatalf("decode envelope: %v", errDecode)
	}
	if !env.OK {
		t.Fatalf("envelope error = %#v", env.Error)
	}
	var resp pluginapi.ModelRegistrationResponse
	if errDecode := json.Unmarshal(env.Result, &resp); errDecode != nil {
		t.Fatalf("decode model registration: %v", errDecode)
	}
	if resp.Provider != pluginIdentifier {
		t.Fatalf("provider = %q, want %q", resp.Provider, pluginIdentifier)
	}
	if len(resp.Models) != 0 {
		t.Fatalf("models = %d, want 0 static models", len(resp.Models))
	}
}

func TestModelsForAuthReturnsWeightedWrapperModels(t *testing.T) {
	cfg := defaultPluginConfig()
	cfg.RegisteredModels = []string{"gpt-5.5"}
	cfg.ProviderWeight = 2
	currentConfig.Store(cfg)
	t.Cleanup(func() { currentConfig.Store(defaultPluginConfig()) })

	rawReq, errMarshal := json.Marshal(rpcAuthModelRequest{
		AuthModelRequest: pluginapi.AuthModelRequest{AuthProvider: pluginIdentifier},
	})
	if errMarshal != nil {
		t.Fatalf("marshal request: %v", errMarshal)
	}
	rawResp, errModels := modelsForAuth(rawReq)
	if errModels != nil {
		t.Fatalf("modelsForAuth() error = %v", errModels)
	}
	var env envelope
	if errDecode := json.Unmarshal(rawResp, &env); errDecode != nil {
		t.Fatalf("decode envelope: %v", errDecode)
	}
	if !env.OK {
		t.Fatalf("envelope error = %#v", env.Error)
	}
	var resp pluginapi.ModelResponse
	if errDecode := json.Unmarshal(env.Result, &resp); errDecode != nil {
		t.Fatalf("decode model response: %v", errDecode)
	}
	if resp.Provider != pluginIdentifier {
		t.Fatalf("provider = %q, want %q", resp.Provider, pluginIdentifier)
	}
	if len(resp.Models) != 2 {
		t.Fatalf("models = %d, want 2", len(resp.Models))
	}
}

func TestParseSyntheticAuth(t *testing.T) {
	rawReq, errMarshal := json.Marshal(pluginapi.AuthParseRequest{
		FileName: syntheticAuthFileName,
		RawJSON:  syntheticAuthJSON(),
	})
	if errMarshal != nil {
		t.Fatalf("marshal request: %v", errMarshal)
	}
	rawResp, errParse := parseAuth(rawReq)
	if errParse != nil {
		t.Fatalf("parseAuth() error = %v", errParse)
	}
	var env envelope
	if errDecode := json.Unmarshal(rawResp, &env); errDecode != nil {
		t.Fatalf("decode envelope: %v", errDecode)
	}
	if !env.OK {
		t.Fatalf("envelope error = %#v", env.Error)
	}
	var resp pluginapi.AuthParseResponse
	if errDecode := json.Unmarshal(env.Result, &resp); errDecode != nil {
		t.Fatalf("decode auth response: %v", errDecode)
	}
	if !resp.Handled {
		t.Fatal("handled = false, want true")
	}
	if resp.Auth.Provider != pluginIdentifier {
		t.Fatalf("provider = %q, want %q", resp.Auth.Provider, pluginIdentifier)
	}
	if resp.Auth.ID != syntheticAuthID {
		t.Fatalf("id = %q, want %q", resp.Auth.ID, syntheticAuthID)
	}
}
