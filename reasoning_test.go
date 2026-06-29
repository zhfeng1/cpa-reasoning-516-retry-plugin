package main

import "testing"

func TestRetryRequiredForReasoningTokensResponsesPayload(t *testing.T) {
	payload := []byte(`{"response":{"usage":{"output_tokens_details":{"reasoning_tokens":516}}}}`)
	if !retryRequiredForReasoningTokens(payload) {
		t.Fatal("retryRequiredForReasoningTokens() = false, want true")
	}
}

func TestRetryRequiredForReasoningTokensChatCompletionsPayload(t *testing.T) {
	payload := []byte(`{"usage":{"completion_tokens_details":{"reasoning_tokens":516}}}`)
	if !retryRequiredForReasoningTokens(payload) {
		t.Fatal("retryRequiredForReasoningTokens() = false, want true")
	}
}

func TestRetryRequiredForReasoningTokensResponsesCompletedTopLevelUsage(t *testing.T) {
	payload := []byte(`{"type":"response.completed","response":{"id":"resp_1"},"usage":{"output_tokens_details":{"reasoning_tokens":516}}}`)
	if !retryRequiredForReasoningTokens(payload) {
		t.Fatal("retryRequiredForReasoningTokens(top-level response usage) = false, want true")
	}
}

func TestRetryRequiredForReasoningTokensStreamingPayload(t *testing.T) {
	payload := []byte("event: response.completed\n" +
		`data: {"type":"response.completed","response":{"usage":{"output_tokens_details":{"reasoning_tokens":516}}}}` + "\n\n")
	if !retryRequiredForReasoningTokens(payload) {
		t.Fatal("retryRequiredForReasoningTokens(stream) = false, want true")
	}
}

func TestRetryRequiredForReasoningTokensIgnoresOtherValues(t *testing.T) {
	for _, payload := range [][]byte{
		[]byte(`{"usage":{"output_tokens_details":{"reasoning_tokens":515}}}`),
		[]byte(`{"usage":{"completion_tokens_details":{"reasoning_tokens":517}}}`),
		[]byte("data: [DONE]\n\n"),
	} {
		if retryRequiredForReasoningTokens(payload) {
			t.Fatalf("retryRequiredForReasoningTokens(%s) = true, want false", payload)
		}
	}
}
