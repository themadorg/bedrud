package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateLiveKitKeypair generates a random API key and secret for LiveKit.
// Used when the user hasn't configured apiKey/apiSecret in config.
func GenerateLiveKitKeypair() (key, secret string, err error) {
	k := make([]byte, 16)
	if _, err := rand.Read(k); err != nil {
		return "", "", fmt.Errorf("failed to generate LiveKit API key: %w", err)
	}
	s := make([]byte, 32)
	if _, err := rand.Read(s); err != nil {
		return "", "", fmt.Errorf("failed to generate LiveKit API secret: %w", err)
	}
	return "gen-" + hex.EncodeToString(k), hex.EncodeToString(s), nil
}
