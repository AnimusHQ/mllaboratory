package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func integritySHA256(v any) (string, error) {
	blob, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal integrity input: %w", err)
	}
	sum := sha256.Sum256(blob)
	return hex.EncodeToString(sum[:]), nil
}
