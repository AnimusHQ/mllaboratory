package redaction

import (
	"encoding/json"
	"regexp"
	"strings"
)

var (
	keyPattern    = regexp.MustCompile(`(?i)(secret|token|password|api[_-]?key|credential)`)
	valuePatterns = []*regexp.Regexp{
		regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9\-_.=]+`),
		regexp.MustCompile(`(?i)api[_-]?key\s*[:=]\s*[^\s]+`),
		regexp.MustCompile(`(?i)secret[_-]?key\s*[:=]\s*[^\s]+`),
		regexp.MustCompile(`(?i)token\s*[:=]\s*[^\s]+`),
		regexp.MustCompile(`(?i)password\s*[:=]\s*[^\s]+`),
	}
)

const redactedValue = "[REDACTED]"

func RedactString(input string) string {
	out := input
	for _, pattern := range valuePatterns {
		out = pattern.ReplaceAllString(out, redactedValue)
	}
	return out
}

func RedactMetadata(meta map[string]any) map[string]any {
	if meta == nil {
		return map[string]any{}
	}
	return redactValue(meta).(map[string]any)
}

func RedactJSON(raw []byte) []byte {
	if len(raw) == 0 {
		return raw
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return raw
	}
	redacted := redactValue(payload)
	out, err := json.Marshal(redacted)
	if err != nil {
		return raw
	}
	return out
}

func redactValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for k, v := range typed {
			if keyPattern.MatchString(k) {
				out[k] = redactedValue
				continue
			}
			out[k] = redactValue(v)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, v := range typed {
			out = append(out, redactValue(v))
		}
		return out
	case string:
		return RedactString(typed)
	default:
		return value
	}
}

func RedactMapString(input map[string]string) map[string]string {
	if input == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(input))
	for k, v := range input {
		key := strings.TrimSpace(k)
		val := strings.TrimSpace(v)
		if key == "" {
			continue
		}
		if keyPattern.MatchString(key) {
			out[key] = redactedValue
			continue
		}
		out[key] = RedactString(val)
	}
	return out
}
