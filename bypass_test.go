package main

import (
	"encoding/json"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

func TestBypassExecutorRequest(t *testing.T) {
	headers := hostModelHeaders(pluginapi.ExecutorRequest{})
	if headers.Get(bypassHeaderName) != "1" {
		t.Fatalf("bypass header = %q, want 1", headers.Get(bypassHeaderName))
	}
	if !bypassExecutorRequest(pluginapi.ExecutorRequest{Headers: headers}) {
		t.Fatal("bypassExecutorRequest() = false, want true")
	}
}

func TestExecuteStreamBypassReturnsErrorEnvelope(t *testing.T) {
	headers := hostModelHeaders(pluginapi.ExecutorRequest{})
	rawReq, errMarshal := json.Marshal(rpcExecutorRequest{
		ExecutorRequest: pluginapi.ExecutorRequest{Headers: headers},
		StreamID:        "stream-1",
	})
	if errMarshal != nil {
		t.Fatalf("marshal request: %v", errMarshal)
	}
	rawResp, errExecute := executeStream(rawReq)
	if errExecute != nil {
		t.Fatalf("executeStream() error = %v", errExecute)
	}
	var env envelope
	if errDecode := json.Unmarshal(rawResp, &env); errDecode != nil {
		t.Fatalf("decode envelope: %v", errDecode)
	}
	if env.OK {
		t.Fatal("envelope OK = true, want false")
	}
	if env.Error == nil || env.Error.Code != "executor_bypass" {
		t.Fatalf("envelope error = %#v, want executor_bypass", env.Error)
	}
}
