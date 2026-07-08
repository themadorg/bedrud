package auth

import (
	"bedrud/config"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ErrTokenRevoked is returned when a token has been explicitly revoked (e.g. on logout).
var ErrTokenRevoked = errors.New("token has been revoked")

// tokenHash returns the hex-encoded SHA-256 hash of a token string.
// Storing hashes instead of full JWTs reduces memory usage in the revocation set.
func tokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// revokedSet holds revoked access tokens keyed by their SHA-256 hash, with their expiry time.
//
// TRADE-OFF: The revocation list is kept in-memory and is lost on server restart.
// After a restart, revoked tokens remain valid until their natural expiry (up to TokenDuration).
// This mainly affects tokens explicitly revoked on logout. To close the window:
//   - Reduce tokenDuration in config.yaml (default 24h)
//   - Or persist revoked hashes to a DB table (check BlockedRefreshToken model for pattern)
type revokedSet struct {
	mu sync.RWMutex
	m  map[string]time.Time
}

var revokedTokens = &revokedSet{m: make(map[string]time.Time)}

// RevokeAccessToken marks a JWT as invalid until its natural expiry.
func RevokeAccessToken(tokenStr string, cfg *config.Config) {
	claims, err := parseTokenUnchecked(tokenStr, cfg)
	if err != nil {
		return
	}
	exp := time.Unix(claims.ExpiresAt.Unix(), 0)
	h := tokenHash(tokenStr)
	revokedTokens.mu.Lock()
	revokedTokens.m[h] = exp
	revokedTokens.mu.Unlock()
}

// PruneRevokedTokens removes expired entries from the revocation set. Call periodically (e.g. hourly).
func PruneRevokedTokens() {
	now := time.Now()
	revokedTokens.mu.Lock()
	defer revokedTokens.mu.Unlock()
	for h, exp := range revokedTokens.m {
		if now.After(exp) {
			delete(revokedTokens.m, h)
		}
	}
}

func isRevoked(tokenStr string) bool {
	h := tokenHash(tokenStr)
	revokedTokens.mu.RLock()
	defer revokedTokens.mu.RUnlock()
	exp, exists := revokedTokens.m[h]
	return exists && time.Now().Before(exp)
}

// bannedUsers holds IDs of deactivated users for fast middleware checks.
// Populated when an admin bans a user; removed when unbanned.
var bannedUsers = &struct {
	mu sync.RWMutex
	m  map[string]struct{}
}{m: make(map[string]struct{})}

// BanUser adds a user ID to the in-memory banned set.
func BanUser(userID string) {
	bannedUsers.mu.Lock()
	bannedUsers.m[userID] = struct{}{}
	bannedUsers.mu.Unlock()
}

// UnbanUser removes a user ID from the in-memory banned set.
func UnbanUser(userID string) {
	bannedUsers.mu.Lock()
	delete(bannedUsers.m, userID)
	bannedUsers.mu.Unlock()
}

// LoadBannedUsersFromDB populates the in-memory banned set from a list of inactive user IDs.
// Call once on server startup after DB initialization.
func LoadBannedUsersFromDB(userIDs []string) {
	bannedUsers.mu.Lock()
	defer bannedUsers.mu.Unlock()
	for _, id := range userIDs {
		bannedUsers.m[id] = struct{}{}
	}
}

// IsUserBanned checks if a user ID is in the banned set.
func IsUserBanned(userID string) bool {
	bannedUsers.mu.RLock()
	defer bannedUsers.mu.RUnlock()
	_, exists := bannedUsers.m[userID]
	return exists
}

type Claims struct {
	UserID   string   `json:"userId"`
	Email    string   `json:"email"`
	Name     string   `json:"name"`
	Provider string   `json:"provider"`
	Accesses []string `json:"accesses"`
	Purpose  string   `json:"purpose,omitempty"`
	// EmailVerifiedAt is the time the user's email was verified, if at all.
	// Populated in GenerateTokenPair for use by RequireEmailVerified middleware
	// to avoid a DB lookup on every request. Nil means unverified (or legacy token).
	EmailVerifiedAt *time.Time `json:"emailVerifiedAt,omitempty"`
	// PasswordChangedAt is the unix timestamp of the user's last password change.
	// Used in password reset tokens to invalidate tokens issued before a password change.
	// Nil means the password has never been changed since account creation.
	PasswordChangedAt *int64 `json:"passwordChangedAt,omitempty"`
	jwt.RegisteredClaims
}

func GenerateToken(userID, email, name, provider string, accesses []string, cfg *config.Config, emailVerifiedAt *time.Time) (string, error) {
	expirationTime := time.Now().Add(time.Duration(cfg.Auth.TokenDuration.Int()) * time.Hour)

	claims := &Claims{
		UserID:          userID,
		Email:           email,
		Name:            name,
		Provider:        provider,
		Accesses:        accesses,
		EmailVerifiedAt: emailVerifiedAt,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "bedrud",
			Subject:   userID,
			Audience:  []string{"bedrud"},
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(cfg.Auth.JWTSecret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// parseTokenUnchecked parses and verifies the JWT signature/expiry WITHOUT checking revocation.
// Used internally so RevokeAccessToken can read expiry without triggering an infinite loop.
func parseTokenUnchecked(tokenString string, cfg *config.Config) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.Auth.JWTSecret), nil
	},
		jwt.WithIssuer("bedrud"),
		jwt.WithAudience("bedrud"),
	)
	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

func ValidateToken(tokenString string, cfg *config.Config) (*Claims, error) {
	claims, err := parseTokenUnchecked(tokenString, cfg)
	if err != nil {
		return nil, err
	}

	if isRevoked(tokenString) {
		return nil, ErrTokenRevoked
	}

	return claims, nil
}

// GenerateVerificationToken creates a short-lived JWT with purpose="email_verify" containing the userID and email.
// It uses the same HMAC-SHA256 signing key as access tokens, but the "purpose" claim
// prevents misuse as an access token. TTL defaults to 24 hours, configurable.
func GenerateVerificationToken(userID, email string, cfg *config.Config) (string, error) {
	ttl := 24 * time.Hour
	if cfg.Auth.VerificationTokenTTLHours > 0 {
		ttl = time.Duration(cfg.Auth.VerificationTokenTTLHours) * time.Hour
	}
	claims := &Claims{
		UserID:  userID,
		Email:   email,
		Purpose: "email_verify",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "bedrud",
			Subject:   userID,
			Audience:  []string{"bedrud"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Auth.JWTSecret))
}

// parseClaims parses and validates a JWT using the bedrud secret, issuer, and audience.
// Returns the raw Claims without purpose checking so callers can validate purpose themselves.
func parseClaims(tokenString string, cfg *config.Config) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.Auth.JWTSecret), nil
	},
		jwt.WithIssuer("bedrud"),
		jwt.WithAudience("bedrud"),
	)
	if err != nil || !token.Valid {
		return nil, err
	}
	return claims, nil
}

// ValidateVerificationToken validates a verification JWT and returns the userID and email.
// It checks that the token's purpose is "email_verify" and that it hasn't expired.
func ValidateVerificationToken(tokenString string, cfg *config.Config) (string, string, error) {
	claims, err := parseClaims(tokenString, cfg)
	if err != nil {
		return "", "", fmt.Errorf("invalid or expired verification token")
	}
	if claims.Purpose != "email_verify" {
		return "", "", fmt.Errorf("invalid token purpose")
	}
	return claims.UserID, claims.Email, nil
}

// GenerateResetToken creates a short-lived JWT with purpose="password_reset" containing the userID and email.
// Uses same HMAC-SHA256 signing key as access tokens. TTL defaults to 1 hour, configurable.
// passwordChangedAt is embedded in the token so ValidateResetToken can detect reuse after a password change.
func GenerateResetToken(userID, email string, passwordChangedAt *time.Time, cfg *config.Config) (string, error) {
	ttl := 1 * time.Hour
	if cfg.Auth.ResetTokenTTLHours > 0 {
		ttl = time.Duration(cfg.Auth.ResetTokenTTLHours) * time.Hour
	}
	var pca *int64
	if passwordChangedAt != nil {
		u := passwordChangedAt.Unix()
		pca = &u
	}
	claims := &Claims{
		UserID:            userID,
		Email:             email,
		Purpose:           "password_reset",
		PasswordChangedAt: pca,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "bedrud",
			Subject:   userID,
			Audience:  []string{"bedrud"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Auth.JWTSecret))
}

// ValidateResetToken validates a password reset JWT and returns the userID, email,
// and the passwordChangedAt timestamp embedded in the token (nil if never changed).
// Checks that the token's purpose is "password_reset" and that it hasn't expired.
func ValidateResetToken(tokenString string, cfg *config.Config) (string, string, *int64, error) {
	claims, err := parseClaims(tokenString, cfg)
	if err != nil {
		return "", "", nil, fmt.Errorf("invalid or expired reset token")
	}
	if claims.Purpose != "password_reset" {
		return "", "", nil, fmt.Errorf("invalid token purpose")
	}
	return claims.UserID, claims.Email, claims.PasswordChangedAt, nil
}

func GenerateTokenPair(userID, email, name, provider string, accesses []string, cfg *config.Config, emailVerifiedAt *time.Time) (accessToken, refreshToken string, err error) {
	// Generate access token
	accessToken, err = GenerateToken(userID, email, name, provider, accesses, cfg, emailVerifiedAt)
	if err != nil {
		return "", "", err
	}

	// Generate refresh token
	refreshClaims := &Claims{
		UserID:          userID,
		Email:           email,
		Name:            name,
		Provider:        provider,
		Accesses:        accesses,
		EmailVerifiedAt: emailVerifiedAt,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "bedrud",
			Subject:   userID,
			Audience:  []string{"bedrud"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24 * 7)), // 7 days
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        uuid.New().String(),
		},
	}

	rt := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err = rt.SignedString([]byte(cfg.Auth.JWTSecret))
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}
