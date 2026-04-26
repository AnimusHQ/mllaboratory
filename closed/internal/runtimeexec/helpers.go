package runtimeexec

import (
	"strconv"
	"strings"
)

func parseIntResource(resources map[string]any, key string) int {
	if len(resources) == 0 {
		return 0
	}
	v, ok := resources[key]
	if !ok {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(t))
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func isReservedJobEnvKey(key string) bool {
	switch strings.ToUpper(strings.TrimSpace(key)) {
	case "RUN_ID", "DATASET_VERSION_ID", "DATAPILOT_URL", "TOKEN", "ANIMUS_JOB_KIND":
		return true
	default:
		return false
	}
}
