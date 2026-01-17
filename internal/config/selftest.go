package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// SyntheticEvmKey returns a synthetic 0x-prefixed 64-hex key.
func SyntheticEvmKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate synthetic key: %w", err)
	}
	return "0x" + hex.EncodeToString(buf), nil
}
