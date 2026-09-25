package handlers

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const (
	passkeyTestRPID   = "localhost"
	passkeyTestOrigin = "http://localhost"
)

// noneAttestation returns the clientDataJSON and "none"-format attestation object an
// authenticator hands back from navigator.credentials.create(), for a fresh P-256 key,
// both base64url-encoded as the web client posts them.
func noneAttestation(t *testing.T, challenge string) (clientDataJSON, attestationObject string) {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	point := key.PublicKey().Bytes() // 0x04 || X || Y

	// COSE_Key {1: 2 (EC2), 3: -7 (ES256), -1: 1 (P-256), -2: X, -3: Y}
	cose := []byte{0xa5, 0x01, 0x02, 0x03, 0x26, 0x20, 0x01, 0x21, 0x58, 0x20}
	cose = append(cose, point[1:33]...)
	cose = append(cose, 0x22, 0x58, 0x20)
	cose = append(cose, point[33:]...)

	credID := make([]byte, 16)
	if _, err := rand.Read(credID); err != nil {
		t.Fatal(err)
	}
	rpIDHash := sha256.Sum256([]byte(passkeyTestRPID))
	authData := append([]byte{}, rpIDHash[:]...)
	authData = append(authData, 0x45)                // flags: UP | UV | AT
	authData = append(authData, 0, 0, 0, 0)          // sign count
	authData = append(authData, make([]byte, 16)...) // AAGUID
	authData = binary.BigEndian.AppendUint16(authData, uint16(len(credID)))
	authData = append(authData, credID...)
	authData = append(authData, cose...)
	if len(authData) > 0xff {
		t.Fatalf("authData is %d bytes, too long for a one-byte CBOR length", len(authData))
	}

	// {"fmt": "none", "attStmt": {}, "authData": <bytes>}
	att := []byte{0xa3, 0x63}
	att = append(att, "fmt"...)
	att = append(att, 0x64)
	att = append(att, "none"...)
	att = append(att, 0x67)
	att = append(att, "attStmt"...)
	att = append(att, 0xa0, 0x68)
	att = append(att, "authData"...)
	att = append(att, 0x58, byte(len(authData)))
	att = append(att, authData...)

	clientData, err := json.Marshal(map[string]string{
		"type":      "webauthn.create",
		"challenge": challenge,
		"origin":    passkeyTestOrigin,
	})
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(clientData), base64.RawURLEncoding.EncodeToString(att)
}

type inviteOnlyPasskeySignup struct {
	app        *fiber.App
	userRepo   *repository.UserRepository
	inviteRepo *repository.InviteTokenRepository
	invite     *models.InviteToken
}

func setupInviteOnlyPasskeySignup(t *testing.T, requireVerification bool) *inviteOnlyPasskeySignup {
	t.Helper()
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	inviteRepo := repository.NewInviteTokenRepository(db)
	authSvc := auth.NewAuthService(userRepo, repository.NewPasskeyRepository(db))
	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:                "pk-signup-invite-secret-key-32b!",
			TokenDuration:            1,
			SessionSecret:            "session-secret-for-testing",
			FrontendURL:              passkeyTestOrigin,
			RequireEmailVerification: requireVerification,
		},
		Server: config.ServerConfig{Domain: passkeyTestRPID},
	}
	config.SetForTest(cfg)
	auth.InitializeSessionStore(cfg.Auth.SessionSecret, false)

	s, _ := settingsRepo.GetSettings()
	s.RegistrationEnabled = true
	s.TokenRegistrationOnly = true
	if err := settingsRepo.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	invite := &models.InviteToken{
		ID: uuid.New().String(), Token: "invite-" + uuid.New().String()[:8], CreatedBy: "admin",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if err := inviteRepo.Create(invite); err != nil {
		t.Fatal(err)
	}

	h := NewAuthHandler(authSvc, cfg, settingsRepo, inviteRepo, auth.NewChallengeStore(5), NewCooldownCache(0), nil)
	app := fiber.New()
	app.Post("/api/auth/passkey/signup/begin", h.PasskeySignupBegin)
	app.Post("/api/auth/passkey/signup/finish", h.PasskeySignupFinish)
	return &inviteOnlyPasskeySignup{app: app, userRepo: userRepo, inviteRepo: inviteRepo, invite: invite}
}

// begin starts a passkey signup with the invite token and returns the challenge together
// with the session cookie that ties the finish call to it.
func (e *inviteOnlyPasskeySignup) begin(t *testing.T, email string) (string, []*http.Cookie) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "name": "Invitee", "inviteToken": e.invite.Token})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/signup/begin", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("begin %s: status %d: %s", email, resp.StatusCode, b)
	}
	var out struct {
		Challenge string `json:"challenge"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out.Challenge, resp.Cookies()
}

func (e *inviteOnlyPasskeySignup) finish(t *testing.T, challenge string, cookies []*http.Cookie) (int, string) {
	t.Helper()
	clientData, att := noneAttestation(t, challenge)
	body, _ := json.Marshal(map[string]string{"clientDataJSON": clientData, "attestationObject": att})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/signup/finish", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := e.app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func (e *inviteOnlyPasskeySignup) storedInvite(t *testing.T) *models.InviteToken {
	t.Helper()
	tok, err := e.inviteRepo.GetByToken(e.invite.Token)
	if err != nil || tok == nil {
		t.Fatalf("invite lookup: %v", err)
	}
	return tok
}

func TestPasskeySignupFinish_InviteOnly_SpendsTokenOnAccountCreation(t *testing.T) {
	e := setupInviteOnlyPasskeySignup(t, false)
	challenge, cookies := e.begin(t, "invitee@ex.com")

	status, body := e.finish(t, challenge, cookies)
	if status != http.StatusOK {
		t.Fatalf("finish: status %d: %s", status, body)
	}

	user, _ := e.userRepo.GetUserByEmail("invitee@ex.com")
	if user == nil {
		t.Fatal("expected the account to be created")
	}
	tok := e.storedInvite(t)
	if tok.UsedAt == nil || tok.UsedBy != user.ID {
		t.Fatalf("expected invite spent by %s, got usedAt=%v usedBy=%q", user.ID, tok.UsedAt, tok.UsedBy)
	}
}

// With email verification on, the handler returns before issuing tokens. The invite must
// be spent on that path too, or it can be redeemed again for another account.
func TestPasskeySignupFinish_InviteOnly_VerificationRequired_SpendsToken(t *testing.T) {
	e := setupInviteOnlyPasskeySignup(t, true)
	challenge, cookies := e.begin(t, "verify@ex.com")

	status, body := e.finish(t, challenge, cookies)
	if status != http.StatusOK {
		t.Fatalf("finish: status %d: %s", status, body)
	}
	var out map[string]any
	_ = json.Unmarshal([]byte(body), &out)
	if out["requiresVerification"] != true {
		t.Fatalf("expected requiresVerification, got %s", body)
	}
	if tok := e.storedInvite(t); tok.UsedAt == nil {
		t.Fatal("expected invite to be spent when the account waits for email verification")
	}

	// The same token no longer opens a new signup.
	req := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/signup/begin", bytes.NewReader(mustJSON(t, map[string]string{
		"email": "second@ex.com", "name": "Second", "inviteToken": e.invite.Token,
	})))
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("reusing a spent invite: want 403, got %d", resp.StatusCode)
	}
}

// Two signups begun with one token both pass the begin check. Only the first to finish
// may get an account; the second is refused before its user is written.
func TestPasskeySignupFinish_InviteOnly_SecondSignupOnSameTokenGetsNoAccount(t *testing.T) {
	e := setupInviteOnlyPasskeySignup(t, false)
	firstChallenge, firstCookies := e.begin(t, "first@ex.com")
	secondChallenge, secondCookies := e.begin(t, "second@ex.com")

	if status, body := e.finish(t, firstChallenge, firstCookies); status != http.StatusOK {
		t.Fatalf("first finish: status %d: %s", status, body)
	}
	if status, body := e.finish(t, secondChallenge, secondCookies); status != http.StatusConflict {
		t.Fatalf("second finish: want 409, got %d: %s", status, body)
	}

	if user, _ := e.userRepo.GetUserByEmail("second@ex.com"); user != nil {
		t.Fatal("second signup must not create an account once the invite is spent")
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
