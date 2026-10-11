package auth

import (
	"strings"
	"testing"
)

// urlSafeAlphabet is what base64.RawURLEncoding emits. A password that strays outside it would
// need escaping wherever it is carried, which is exactly the trouble a generated one should avoid.
const urlSafeAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

func TestGenerateRandomPassword_HonoursLength(t *testing.T) {
	for _, n := range []int{12, 20, 33, 64} {
		got, err := GenerateRandomPassword(n)
		if err != nil {
			t.Fatalf("n=%d: %v", n, err)
		}
		if len(got) != n {
			t.Fatalf("n=%d: expected %d characters, got %d (%q)", n, n, len(got), got)
		}
	}
}

func TestGenerateRandomPassword_RaisesShortRequestsToTheFloor(t *testing.T) {
	// The floor is what keeps a caller from minting a password the login path would reject.
	for _, n := range []int{-1, 0, 1, 11} {
		got, err := GenerateRandomPassword(n)
		if err != nil {
			t.Fatalf("n=%d: %v", n, err)
		}
		if len(got) != MinGeneratedPasswordLength {
			t.Fatalf("n=%d: expected the %d character floor, got %d", n, MinGeneratedPasswordLength, len(got))
		}
	}
}

func TestGenerateRandomPassword_IsURLSafe(t *testing.T) {
	got, err := GenerateRandomPassword(64)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range got {
		if !strings.ContainsRune(urlSafeAlphabet, r) {
			t.Fatalf("password %q contains %q, which is outside the URL-safe alphabet", got, r)
		}
	}
}

func TestGenerateRandomPassword_DoesNotRepeat(t *testing.T) {
	seen := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		got, err := GenerateRandomPassword(20)
		if err != nil {
			t.Fatal(err)
		}
		if _, dup := seen[got]; dup {
			t.Fatalf("generated the same password twice: %q", got)
		}
		seen[got] = struct{}{}
	}
}
