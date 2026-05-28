package utils

import (
	"testing"
)

func TestGenerateLiveKitKeypair(t *testing.T) {
	key, secret, err := GenerateLiveKitKeypair()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key == "" {
		t.Fatal("expected non-empty key")
	}
	if secret == "" {
		t.Fatal("expected non-empty secret")
	}
	if len(key) != 36 { // "gen-" + 32 hex chars = 36
		t.Fatalf("expected key length 36 (gen- + 32 hex), got %d: %q", len(key), key)
	}
	if len(secret) != 64 { // 32 bytes = 64 hex chars
		t.Fatalf("expected secret length 64, got %d", len(secret))
	}

	// Verify uniqueness
	key2, secret2, err := GenerateLiveKitKeypair()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key == key2 {
		t.Fatal("expected different keys on second call")
	}
	if secret == secret2 {
		t.Fatal("expected different secrets on second call")
	}
}

func TestGenerateLiveKitKeypair_Format(t *testing.T) {
	key, secret, err := GenerateLiveKitKeypair()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Key should start with "gen-" prefix
	if key[:4] != "gen-" {
		t.Fatalf("expected key to start with 'gen-', got %q", key[:4])
	}

	// Secret should be valid hex
	if len(secret) != 64 {
		t.Fatalf("expected 64-char hex secret, got %d", len(secret))
	}

	// Both should only contain hex chars (after prefix)
	for _, c := range key[4:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Fatalf("key contains non-hex char %c", c)
		}
	}
	for _, c := range secret {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Fatalf("secret contains non-hex char %c", c)
		}
	}
}
