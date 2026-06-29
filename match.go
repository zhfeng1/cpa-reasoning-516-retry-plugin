package main

import "strings"

func shouldHandle(cfg pluginConfig, sourceFormat, model string) bool {
	if !cfg.Enabled {
		return false
	}
	if len(cfg.SourceFormats) > 0 && !stringInList(strings.ToLower(strings.TrimSpace(sourceFormat)), cfg.SourceFormats) {
		return false
	}
	if len(cfg.Models) == 0 {
		return true
	}
	for _, pattern := range cfg.Models {
		if wildcardMatch(pattern, model) {
			return true
		}
	}
	return false
}

func stringInList(value string, list []string) bool {
	for _, item := range list {
		if strings.EqualFold(value, item) {
			return true
		}
	}
	return false
}

func wildcardMatch(pattern, value string) bool {
	pattern = strings.TrimSpace(pattern)
	value = strings.TrimSpace(value)
	if pattern == "" {
		return false
	}
	if pattern == "*" {
		return true
	}
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return strings.EqualFold(pattern, value)
	}
	position := 0
	for index, part := range parts {
		if part == "" {
			continue
		}
		found := strings.Index(strings.ToLower(value[position:]), strings.ToLower(part))
		if found < 0 {
			return false
		}
		if index == 0 && !strings.HasPrefix(strings.ToLower(value), strings.ToLower(part)) {
			return false
		}
		position += found + len(part)
	}
	last := parts[len(parts)-1]
	if last != "" && !strings.HasSuffix(strings.ToLower(value), strings.ToLower(last)) {
		return false
	}
	return true
}
