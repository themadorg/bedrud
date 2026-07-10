package auth

import (
	"testing"
	"time"

	"bedrud/config"

	"github.com/golang-jwt/jwt/v5"
)

const testUserID = "user-123"

func testConfig() *config.Config {
	return &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:     "test-secret-key-for-testing-only",
			TokenDuration: 1, // 1 hour
		},
	}
}

// --- GenerateToken Tests ---

func TestGenerateToken_Success(t *testing.T) {
	cfg := testConfig()
	token, err := GenerateToken(testUserID, testEmail, "Test User", "local", []string{"user"}, cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestGenerateToken_WithMultipleAccesses(t *testing.T) {
	cfg := testConfig()
	token, err := GenerateToken(testUserID, "admin@example.com", "Admin", "local", []string{"admin", "user"}, cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestGenerateToken_EmptySecret(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:     "",
			TokenDuration: 1,
		},
	}
	// Should still generate (empty secret is valid for HS256, just insecure)
	token, err := GenerateToken(testUserID, testEmail, "Test", "local", []string{"user"}, cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

// --- ValidateToken Tests ---

func TestValidateToken_Valid(t *testing.T) {
	cfg := testConfig()
	token, _ := GenerateToken(testUserID, testEmail, "Test User", "local", []string{"user", "admin"}, cfg, nil)

	claims, err := ValidateToken(token, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.UserID != testUserID {
		t.Fatalf("expected UserID 'user-123', got '%s'", claims.UserID)
	}
	if claims.Email != testEmail {
		t.Fatalf("expected Email 'test@example.com', got '%s'", claims.Email)
	}
	if claims.Name != "Test User" {
		t.Fatalf("expected Name 'Test User', got '%s'", claims.Name)
	}
	if claims.Provider != "local" {
		t.Fatalf("expected Provider 'local', got '%s'", claims.Provider)
	}
	if len(claims.Accesses) != 2 || claims.Accesses[0] != "user" || claims.Accesses[1] != "admin" {
		t.Fatalf("unexpected accesses: %v", claims.Accesses)
	}
}

func TestValidateToken_InvalidSignature(t *testing.T) {
	cfg := testConfig()
	token, _ := GenerateToken(testUserID, testEmail, "Test User", "local", []string{"user"}, cfg, nil)

	// Use a different secret for validation
	wrongCfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret: "wrong-secret",
		},
	}

	_, err := ValidateToken(token, wrongCfg)
	if err == nil {
		t.Fatal("expected error for invalid signature")
	}
}

func TestValidateToken_ExpiredToken(t *testing.T) {
	cfg := testConfig()

	// Create a token that expired 1 hour ago
	claims := &Claims{
		UserID:   testUserID,
		Email:    testEmail,
		Name:     "Test",
		Provider: "local",
		Accesses: []string{"user"},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(cfg.Auth.JWTSecret))

	_, err := ValidateToken(tokenString, cfg)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidateToken_MalformedToken(t *testing.T) {
	cfg := testConfig()
	_, err := ValidateToken("not.a.valid.token", cfg)
	if err == nil {
		t.Fatal("expected error for malformed token")
	}
}

func TestValidateToken_EmptyToken(t *testing.T) {
	cfg := testConfig()
	_, err := ValidateToken("", cfg)
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestValidateToken_WrongSigningMethod(t *testing.T) {
	cfg := testConfig()

	// Create a token with "none" signing method (unsigned)
	claims := &Claims{
		UserID: testUserID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenString, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)

	_, err := ValidateToken(tokenString, cfg)
	if err == nil {
		t.Fatal("expected error for wrong signing method")
	}
}

// --- GenerateTokenPair Tests ---

func TestGenerateTokenPair_Success(t *testing.T) {
	cfg := testConfig()
	accessToken, refreshToken, err := GenerateTokenPair(testUserID, testEmail, "Test", "local", []string{"user"}, cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if accessToken == "" {
		t.Fatal("expected non-empty access token")
	}
	if refreshToken == "" {
		t.Fatal("expected non-empty refresh token")
	}
	if accessToken == refreshToken {
		t.Fatal("access and refresh tokens should be different")
	}
}

func TestGenerateTokenPair_RefreshTokenHasJTI(t *testing.T) {
	cfg := testConfig()
	_, refreshToken, _ := GenerateTokenPair(testUserID, testEmail, "Test", "local", []string{"user"}, cfg, nil)

	// Validate the refresh token and check it has a JTI
	claims, err := ValidateToken(refreshToken, cfg)
	if err != nil {
		t.Fatalf("failed to validate refresh token: %v", err)
	}
	if claims.RegisteredClaims.ID == "" {
		t.Fatal("expected refresh token to have a JTI (ID)")
	}
}

func TestGenerateTokenPair_RefreshTokenLongerExpiration(t *testing.T) {
	cfg := testConfig()
	accessToken, refreshToken, _ := GenerateTokenPair(testUserID, testEmail, "Test", "local", []string{"user"}, cfg, nil)

	accessClaims, _ := ValidateToken(accessToken, cfg)
	refreshClaims, _ := ValidateToken(refreshToken, cfg)

	// Refresh token should expire later than access token
	if !refreshClaims.ExpiresAt.Time.After(accessClaims.ExpiresAt.Time) {
		t.Fatal("refresh token should expire after access token")
	}
}

func TestGenerateTokenPair_AccessTokenContainsAccesses(t *testing.T) {
	cfg := testConfig()
	accessToken, _, _ := GenerateTokenPair(testUserID, testEmail, "Test", "local", []string{"user", "admin"}, cfg, nil)

	claims, err := ValidateToken(accessToken, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(claims.Accesses) != 2 {
		t.Fatalf("expected 2 accesses, got %d", len(claims.Accesses))
	}
}

// --- Revocation Tests ---

func TestAccessTokenRevocation(t *testing.T) {
	cfg := testConfig()

	token, err := GenerateToken(testUserID, testEmail, "Test User", "local", []string{"user"}, cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error generating token: %v", err)
	}

	// Token should be valid before revocation
	claims, err := ValidateToken(token, cfg)
	if err != nil {
		t.Fatalf("expected valid token before revocation, got error: %v", err)
	}
	if claims.UserID != testUserID {
		t.Fatalf("expected UserID 'user-123', got '%s'", claims.UserID)
	}

	// Revoke it
	RevokeAccessToken(token, cfg)

	// Token should be invalid after revocation
	_, err = ValidateToken(token, cfg)
	if err == nil {
		t.Fatal("expected error after revocation, got nil")
	}
	if err != ErrTokenRevoked {
		t.Fatalf("expected ErrTokenRevoked, got: %v", err)
	}
}

// --- Claims Tests ---

// --- GenerateResetToken Tests ---

func TestGenerateResetToken_Success(t *testing.T) {
	cfg := testConfig()
	token, err := GenerateResetToken(testUserID, testEmail, nil, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestGenerateResetToken_PurposeClaim(t *testing.T) {
	cfg := testConfig()
	token, _ := GenerateResetToken(testUserID, testEmail, nil, cfg)

	claims := &Claims{}
	if _, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (any, error) {
		return []byte(cfg.Auth.JWTSecret), nil
	}); err != nil {
		t.Fatal(err)
	}
	if claims.Purpose != "password_reset" {
		t.Fatalf("expected purpose 'password_reset', got '%s'", claims.Purpose)
	}
}

func TestValidateToken_RejectsPurposeTokens(t *testing.T) {
	cfg := testConfig()
	verifyTok, err := GenerateVerificationToken(testUserID, testEmail, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateToken(verifyTok, cfg); err == nil {
		t.Fatal("expected verification token rejected as access")
	}
	resetTok, err := GenerateResetToken(testUserID, testEmail, nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateToken(resetTok, cfg); err == nil {
		t.Fatal("expected reset token rejected as access")
	}
	// Dedicated validators still accept purpose tokens
	if _, _, err := ValidateVerificationToken(verifyTok, cfg); err != nil {
		t.Fatalf("ValidateVerificationToken: %v", err)
	}
	if _, _, _, err := ValidateResetToken(resetTok, cfg); err != nil {
		t.Fatalf("ValidateResetToken: %v", err)
	}
}

func TestGenerateResetToken_CustomTTL(t *testing.T) {
	cfg := testConfig()
	cfg.Auth.ResetTokenTTLHours = 2
	token, err := GenerateResetToken(testUserID, testEmail, nil, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

// --- ValidateResetToken Tests ---

func TestValidateResetToken_Valid(t *testing.T) {
	cfg := testConfig()
	token, _ := GenerateResetToken(testUserID, testEmail, nil, cfg)

	userID, email, pca, err := ValidateResetToken(token, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID != testUserID {
		t.Fatalf("expected userID 'user-123', got '%s'", userID)
	}
	if email != testEmail {
		t.Fatalf("expected email 'test@example.com', got '%s'", email)
	}
	if pca != nil {
		t.Fatal("expected nil passwordChangedAt for user with no changes")
	}
}

func TestValidateResetToken_WithPasswordChanged(t *testing.T) {
	cfg := testConfig()
	now := time.Now()
	token, _ := GenerateResetToken(testUserID, testEmail, &now, cfg)

	userID, email, pca, err := ValidateResetToken(token, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID != testUserID {
		t.Fatalf("expected userID 'user-123', got '%s'", userID)
	}
	if email != testEmail {
		t.Fatalf("expected email 'test@example.com', got '%s'", email)
	}
	if pca == nil {
		t.Fatal("expected non-nil passwordChangedAt")
	}
	if *pca != now.Unix() {
		t.Fatalf("expected passwordChangedAt %d, got %d", now.Unix(), *pca)
	}
}

func TestValidateResetToken_InvalidSignature(t *testing.T) {
	cfg := testConfig()
	token, _ := GenerateResetToken(testUserID, testEmail, nil, cfg)

	wrongCfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret: "wrong-secret",
		},
	}

	_, _, _, err := ValidateResetToken(token, wrongCfg)
	if err == nil {
		t.Fatal("expected error for invalid signature")
	}
}

func TestValidateResetToken_Expired(t *testing.T) {
	cfg := testConfig()
	claims := &Claims{
		UserID:  testUserID,
		Email:   testEmail,
		Purpose: "password_reset",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(cfg.Auth.JWTSecret))

	_, _, _, err := ValidateResetToken(tokenString, cfg)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidateResetToken_WrongPurpose(t *testing.T) {
	cfg := testConfig()
	// Generate a verification token (purpose=email_verify), not a reset token
	token, _ := GenerateVerificationToken(testUserID, testEmail, cfg)

	_, _, _, err := ValidateResetToken(token, cfg)
	if err == nil {
		t.Fatal("expected error for wrong purpose")
	}
}

func TestValidateResetToken_Malformed(t *testing.T) {
	cfg := testConfig()
	_, _, _, err := ValidateResetToken("not.a.valid.token", cfg)
	if err == nil {
		t.Fatal("expected error for malformed token")
	}
}

func TestValidateResetToken_EmptyToken(t *testing.T) {
	cfg := testConfig()
	_, _, _, err := ValidateResetToken("", cfg)
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestClaims_Structure(t *testing.T) {
	c := Claims{
		UserID:   "id-1",
		Email:    "e@e.com",
		Name:     "Name",
		Provider: "google",
		Accesses: []string{"user"},
	}
	if c.UserID != "id-1" {
		t.Fatalf("unexpected UserID: %s", c.UserID)
	}
	if c.Provider != "google" {
		t.Fatalf("unexpected Provider: %s", c.Provider)
	}
}
