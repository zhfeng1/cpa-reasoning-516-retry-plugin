package main

import (
	"net/http"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

const bypassHeaderName = "X-CPA-Reasoning-516-Retry-Bypass"

func bypassExecutorRequest(exec pluginapi.ExecutorRequest) bool {
	return strings.EqualFold(strings.TrimSpace(exec.Headers.Get(bypassHeaderName)), "1")
}

func hostModelHeaders(exec pluginapi.ExecutorRequest) http.Header {
	headers := cloneHeader(exec.Headers)
	if headers == nil {
		headers = make(http.Header)
	}
	headers.Set(bypassHeaderName, "1")
	return headers
}
