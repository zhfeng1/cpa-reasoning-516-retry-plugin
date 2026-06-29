package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

const retryRequiredReasoningTokens = 516

const retryRequiredReasoning516Message = "[RETRY_REQUIRED_REASONING_516] reasoning_tokens=516; 降智请求自动重试中."

func retryRequiredForReasoningTokens(payload []byte) bool {
	if !bytes.Contains(payload, []byte("reasoning_tokens")) {
		return false
	}
	for _, raw := range responseJSONPayloads(payload) {
		if reasoningTokensFromJSON(raw) == retryRequiredReasoningTokens {
			return true
		}
	}
	return false
}

func responseJSONPayloads(payload []byte) [][]byte {
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 {
		return nil
	}
	if payload[0] == '{' {
		return [][]byte{payload}
	}
	var out [][]byte
	for _, line := range bytes.Split(payload, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		data := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if len(data) == 0 || bytes.Equal(data, []byte("[DONE]")) || data[0] != '{' {
			continue
		}
		out = append(out, data)
	}
	return out
}

func reasoningTokensFromJSON(raw []byte) int {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return 0
	}
	for _, path := range [][]string{
		{"usage", "output_tokens_details", "reasoning_tokens"},
		{"usage", "completion_tokens_details", "reasoning_tokens"},
		{"response", "usage", "output_tokens_details", "reasoning_tokens"},
		{"response", "usage", "completion_tokens_details", "reasoning_tokens"},
	} {
		if tokens, ok := intAtPath(value, path); ok {
			return tokens
		}
	}
	return 0
}

func intAtPath(value any, path []string) (int, bool) {
	current := value
	for _, key := range path {
		obj, ok := current.(map[string]any)
		if !ok {
			return 0, false
		}
		current, ok = obj[key]
		if !ok {
			return 0, false
		}
	}
	switch typed := current.(type) {
	case json.Number:
		var out int
		if _, err := fmt.Sscan(string(typed), &out); err == nil {
			return out, true
		}
	case float64:
		if typed == float64(int(typed)) {
			return int(typed), true
		}
	case string:
		var out int
		if _, err := fmt.Sscan(strings.TrimSpace(typed), &out); err == nil {
			return out, true
		}
	}
	return 0, false
}
