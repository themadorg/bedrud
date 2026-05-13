package lkutil

import (
	"context"
	"testing"
)

func TestAuthContext_ReturnsContext(t *testing.T) {
	ctx, err := AuthContext(context.Background(), "test-key", "test-secret")
	if err != nil {
		t.Fatalf("AuthContext should not return error: %v", err)
	}
	if ctx == nil {
		t.Fatal("AuthContext should return non-nil context")
	}
}
