package env

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

func String(key string, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func Duration(key string, def time.Duration) (time.Duration, error) {
	if v, ok := os.LookupEnv(key); ok {
		d, err := time.ParseDuration(v)
		if err != nil {
			return 0, fmt.Errorf("parse %s: %w", key, err)
		}
		return d, nil
	}
	return def, nil
}

func Bool(key string, def bool) (bool, error) {
	if v, ok := os.LookupEnv(key); ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return false, fmt.Errorf("parse %s: %w", key, err)
		}
		return b, nil
	}
	return def, nil
}

func Int(key string, def int) (int, error) {
	if v, ok := os.LookupEnv(key); ok {
		i, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("parse %s: %w", key, err)
		}
		return i, nil
	}
	return def, nil
}
