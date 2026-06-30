package main

import (
	"bytes"
	"testing"
)

func TestStreamForwardStateEmitsNormalChunksImmediately(t *testing.T) {
	state := streamForwardState{}
	payload, emit, terminal := state.nextPayload([]byte(`data: {"type":"response.output_text.delta","delta":"hi"}`), "gpt-5.5")
	if !emit {
		t.Fatal("emit = false, want true")
	}
	if terminal {
		t.Fatal("terminal = true, want false")
	}
	if string(payload) != `data: {"type":"response.output_text.delta","delta":"hi"}` {
		t.Fatalf("payload = %q", payload)
	}
	if pending := state.flush(); len(pending) != 0 {
		t.Fatalf("pending = %q, want empty", pending)
	}
}

func TestStreamForwardStateEmitsSingleFailedForSplitReasoning516Completed(t *testing.T) {
	state := streamForwardState{}
	payload, emit, terminal := state.nextPayload([]byte("event: response.completed"), "gpt-5.5")
	if emit || terminal || len(payload) != 0 {
		t.Fatalf("first chunk payload=%q emit=%v terminal=%v, want buffered", payload, emit, terminal)
	}

	payload, emit, terminal = state.nextPayload([]byte(`data: {"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.5","usage":{"output_tokens_details":{"reasoning_tokens":516}}}}`), "gpt-5.5")
	if !emit {
		t.Fatal("emit = false, want true")
	}
	if !terminal {
		t.Fatal("terminal = false, want true")
	}
	if bytes.Contains(payload, []byte("event: response.completed")) {
		t.Fatalf("payload should not contain response.completed: %s", payload)
	}
	if !bytes.Contains(payload, []byte("event: response.failed")) {
		t.Fatalf("payload missing response.failed: %s", payload)
	}
	if bytes.Contains(payload, []byte("Upstream request failed")) {
		t.Fatalf("payload contains upstream failure: %s", payload)
	}
}
