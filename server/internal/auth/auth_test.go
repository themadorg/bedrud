package auth

import (
	"bedrud/config"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"
	"errors"
	"testing"
	"time"
)

// testAuthConfig returns a config suitable for auth service tests
func testAuthConfig() *config.Config {
	return &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:     "auth-service-test-secret-key-32b",
			TokenDuration: 1,
		},
	}
}

func setupAuthService(t *testing.T) (*AuthService, *config.Config) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	svc := NewAuthService(userRepo, passkeyRepo)
	cfg := testAuthConfig()
	// We need to set the global config for functions that use config.Get()
	// Since config.Load uses sync.Once, we bypass it by setting the package-level var
	return svc, cfg
}

const testEmail = "test@example.com"

func TestAuthService_Register_Success(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	svc := NewAuthService(userRepo, passkeyRepo)

	user, err := svc.Register(testEmail, "password123", "Test User")
	if err != nil {
		t.Fatalf("failed to register: %v", err)
	}
	if user == nil {
		t.Fatal("expected non-nil user")
	}
	if user.Email != testEmail {
		t.Fatalf("expected email '%s', got '%s'", testEmail, user.Email)
	}
	if user.Name != "Test User" {
		t.Fatalf("expected name 'Test User', got '%s'", user.Name)
	}
	if user.Provider != "local" {
		t.Fatal("expected provider 'local'")
	}
	if !user.IsActive {
		t.Fatal("expected IsActive to be true")
	}
	if len(user.Accesses) != 1 || user.Accesses[0] != "user" {
		t.Fatalf("expected accesses [user], got %v", user.Accesses)
	}
	// Password should be hashed, not plain
	if user.Password == "password123" {
		t.Fatal("password should be hashed")
	}
}

func TestAuthService_Register_DuplicateEmail(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	svc := NewAuthService(userRepo, passkeyRepo)

	_, _ = svc.Register("dup@example.com", "password123", "First")
	_, err := svc.Register("dup@example.com", "password456", "Second")
	if err == nil {
		t.Fatal("expected error for duplicate registration")
	}
	if err.Error() != "user already exists" {
		t.Fatalf("expected 'user already exists', got '%s'", err.Error())
	}
}

func TestAuthService_GetUserByID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	svc := NewAuthService(userRepo, passkeyRepo)

	registered, _ := svc.Register("get@example.com", "pass", "Get User")

	found, err := svc.GetUserByID(registered.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find user")
	}
	if found.Email != "get@example.com" {
		t.Fatalf("unexpected email: %s", found.Email)
	}
}

func TestAuthService_GetUserByEmail(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	svc := NewAuthService(userRepo, passkeyRepo)

	_, _ = svc.Register("email@example.com", "pass", "Email User")

	found, err := svc.GetUserByEmail("email@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find user")
	}
}

func TestAuthService_UpdateRefreshToken(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	svc := NewAuthService(userRepo, passkeyRepo)

	user, _ := svc.Register("refresh@example.com", "pass", "Refresh User")

	err := svc.UpdateRefreshToken(user.ID, "new-refresh-token")
	if err != nil {
		t.Fatalf("failed to update refresh token: %v", err)
	}

	foundUser, _ := svc.GetUserByID(user.ID)
	// Token is stored as SHA-256 hash, not plaintext
	if foundUser.RefreshToken == "new-refresh-token" {
		t.Fatal("refresh token should be hashed, not stored in plaintext")
	}
}

func TestAuthService_UpdateUserAccesses(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	svc := NewAuthService(userRepo, passkeyRepo)

	user, _ := svc.Register("access@example.com", "pass", "Access User")

	err := svc.UpdateUserAccesses(user.ID, []string{"admin", "user"})
	if err != nil {
		t.Fatalf("failed to update accesses: %v", err)
	}

	found, _ := svc.GetUserByID(user.ID)
	if len(found.Accesses) != 2 {
		t.Fatalf("expected 2 accesses, got %d", len(found.Accesses))
	}
}

func TestAuthService_BeginRegisterPasskey(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	svc := NewAuthService(userRepo, passkeyRepo)

	challenge, err := svc.BeginRegisterPasskey("user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if challenge == "" {
		t.Fatal("expected non-empty challenge")
	}
}

func TestAuthService_BeginLoginPasskey(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	svc := NewAuthService(userRepo, passkeyRepo)

	challenge, err := svc.BeginLoginPasskey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if challenge == "" {
		t.Fatal("expected non-empty challenge")
	}
}

func TestNewAuthService(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)

	svc := NewAuthService(userRepo, passkeyRepo)
	if svc == nil {
		t.Fatal("expected non-nil auth service")
	}
}

// --- Request/Response struct tests ---

func TestRegisterRequest_Fields(t *testing.T) {
	r := RegisterRequest{
		Email:    testEmail,
		Password: "pass123",
		Name:     "Test",
	}
	if r.Email != testEmail {
		t.Fatal("unexpected email")
	}
}

func TestLoginRequest_Fields(t *testing.T) {
	r := LoginRequest{
		Email:    testEmail,
		Password: "pass123",
	}
	if r.Email != testEmail {
		t.Fatal("unexpected email")
	}
}

func TestTokenResponse_Fields(t *testing.T) {
	r := TokenResponse{
		AccessToken:  "access",
		RefreshToken: "refresh",
	}
	if r.AccessToken != "access" || r.RefreshToken != "refresh" {
		t.Fatal("unexpected tokens")
	}
}

func TestLoginResponse_Fields(t *testing.T) {
	r := LoginResponse{
		User: &models.User{ID: "u1", Email: "e@e.com"},
		Token: TokenPair{
			AccessToken:  "at",
			RefreshToken: "rt",
		},
	}
	if r.User.ID != "u1" {
		t.Fatal("unexpected user ID")
	}
	if r.Token.AccessToken != "at" {
		t.Fatal("unexpected access token")
	}
}

func TestGuestLoginRequest_Fields(t *testing.T) {
	r := GuestLoginRequest{Name: "Guest"}
	if r.Name != "Guest" {
		t.Fatal("unexpected name")
	}
}

func TestErrorResponse_Fields(t *testing.T) {
	r := ErrorResponse{Error: "something went wrong"}
	if r.Error != "something went wrong" {
		t.Fatal("unexpected error message")
	}
}

func TestLogoutRequest_Fields(t *testing.T) {
	r := LogoutRequest{RefreshToken: "some-token"}
	if r.RefreshToken != "some-token" {
		t.Fatal("unexpected token")
	}
}

// --- Login tests ---

func TestAuthService_Login_Success(t *testing.T) {
	svc, cfg := setupAuthService(t)
	config.SetForTest(cfg)

	_, _ = svc.Register("loginok@example.com", "correctpass", "Login OK")

	resp, err := svc.Login("loginok@example.com", "correctpass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.User == nil {
		t.Fatal("expected non-nil user in response")
	}
	if resp.Token.AccessToken == "" {
		t.Fatal("expected non-empty access token")
	}
	if resp.Token.RefreshToken == "" {
		t.Fatal("expected non-empty refresh token")
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	svc, cfg := setupAuthService(t)
	config.SetForTest(cfg)

	_, _ = svc.Register("wrongpass@example.com", "realpass", "User")

	_, err := svc.Login("wrongpass@example.com", "wrongpass")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
	if err.Error() != "invalid credentials" {
		t.Fatalf("expected 'invalid credentials', got '%s'", err.Error())
	}
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
	svc, cfg := setupAuthService(t)
	config.SetForTest(cfg)

	_, err := svc.Login("nobody@example.com", "anypass")
	if err == nil {
		t.Fatal("expected error for missing user")
	}
	if err.Error() != "invalid credentials" {
		t.Fatalf("expected 'invalid credentials', got '%s'", err.Error())
	}
}

// --- GuestLogin tests ---

func TestAuthService_GuestLogin_Success(t *testing.T) {
	svc, cfg := setupAuthService(t)
	config.SetForTest(cfg)

	resp, err := svc.GuestLogin("Guest Player")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.User == nil {
		t.Fatal("expected non-nil user")
	}
	if resp.User.Provider != "guest" {
		t.Fatalf("expected provider 'guest', got '%s'", resp.User.Provider)
	}
	if resp.User.Name != "Guest Player" {
		t.Fatalf("expected name 'Guest Player', got '%s'", resp.User.Name)
	}
	if resp.Token.AccessToken == "" {
		t.Fatal("expected non-empty access token")
	}
}

// --- UpdateProfile tests ---

func TestAuthService_UpdateProfile_Success(t *testing.T) {
	svc, _ := setupAuthService(t)

	user, _ := svc.Register("profile@example.com", "pass", "Old Name")

	updated, err := svc.UpdateProfile(user.ID, "New Name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "New Name" {
		t.Fatalf("expected 'New Name', got '%s'", updated.Name)
	}
}

func TestAuthService_UpdateProfile_NotFound(t *testing.T) {
	svc, _ := setupAuthService(t)

	_, err := svc.UpdateProfile("nonexistent-id", "Some Name")
	if err == nil {
		t.Fatal("expected error for missing user")
	}
}

// --- ChangeEmail tests ---

func TestAuthService_ChangeEmail_Success(t *testing.T) {
	svc, cfg := setupAuthService(t)
	config.SetForTest(cfg)

	user, _ := svc.Register("old@example.com", "pass123", "Old Email")

	// Simulate a session by logging in (stores refresh token in DB)
	loginResp, _ := svc.Login("old@example.com", "pass123")
	if loginResp.Token.RefreshToken == "" {
		t.Fatal("expected refresh token after login")
	}

	// Change email
	err := svc.ChangeEmail(user.ID, "new@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify DB updated
	updated, _ := svc.GetUserByID(user.ID)
	if updated.Email != "new@example.com" {
		t.Fatalf("expected email 'new@example.com', got '%s'", updated.Email)
	}

	// EmailVerifiedAt reset
	if updated.EmailVerifiedAt != nil {
		t.Fatal("expected EmailVerifiedAt to be nil after email change")
	}

	// Refresh token cleared (forces re-login)
	if updated.RefreshToken != "" {
		t.Fatal("expected refresh token to be cleared after email change")
	}
}

func TestAuthService_ChangeEmail_UserNotFound(t *testing.T) {
	svc, _ := setupAuthService(t)

	err := svc.ChangeEmail("nonexistent-id", "any@example.com")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
	if err.Error() != "user not found" {
		t.Fatalf("expected 'user not found', got '%s'", err.Error())
	}
}

func TestAuthService_ChangeEmail_InvalidEmail(t *testing.T) {
	svc, _ := setupAuthService(t)

	user, _ := svc.Register("valid@example.com", "pass123", "Valid")

	err := svc.ChangeEmail(user.ID, "not-an-email")
	if err == nil {
		t.Fatal("expected error for invalid email")
	}
	// Could be either "invalid email format" or a mail.ParseAddress error
	if err.Error() != "invalid email format" {
		t.Fatalf("expected 'invalid email format', got '%s'", err.Error())
	}
}

func TestAuthService_ChangeEmail_EmailAlreadyInUse(t *testing.T) {
	svc, _ := setupAuthService(t)

	user1, _ := svc.Register("first@example.com", "pass123", "First")
	svc.Register("second@example.com", "pass456", "Second")

	// user1 tries to take second@example.com
	err := svc.ChangeEmail(user1.ID, "second@example.com")
	if err == nil {
		t.Fatal("expected error for email already in use")
	}
	if err.Error() != "email already in use" {
		t.Fatalf("expected 'email already in use', got '%s'", err.Error())
	}
}

func TestAuthService_ChangeEmail_SameEmailIdempotent(t *testing.T) {
	svc, _ := setupAuthService(t)

	user, _ := svc.Register("same@example.com", "pass123", "Same")

	// Change to same email — should succeed (no-op without error)
	err := svc.ChangeEmail(user.ID, "same@example.com")
	if err != nil {
		t.Fatalf("expected no error when changing to same email, got %v", err)
	}
}

// --- ChangePassword tests ---

func TestAuthService_ChangePassword_Success(t *testing.T) {
	svc, cfg := setupAuthService(t)
	config.SetForTest(cfg)

	user, _ := svc.Register("chpass@example.com", "oldpass123", "Change Pass")

	// Simulate an existing session by logging in (stores refresh token in DB)
	loginResp, _ := svc.Login("chpass@example.com", "oldpass123")

	// Verify the refresh token was stored
	beforeUser, _ := svc.userRepo.GetUserByID(user.ID)
	if beforeUser.RefreshToken == "" {
		t.Fatal("expected refresh token to be stored after login")
	}

	err := svc.ChangePassword(user.ID, "oldpass123", "newpass456", loginResp.Token.AccessToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify refresh token was cleared (all sessions invalidated) BEFORE re-login
	updatedUser, _ := svc.userRepo.GetUserByID(user.ID)
	if updatedUser.RefreshToken != "" {
		t.Fatalf("expected refresh token to be cleared after password change, got: %q", updatedUser.RefreshToken)
	}

	// Verify old refresh token is blocked (cannot use old session)
	_, err = svc.ValidateRefreshToken(loginResp.Token.RefreshToken)
	if err == nil {
		t.Fatal("expected old refresh token to be invalid after password change")
	}

	// Verify new password works
	_, loginErr := svc.Login("chpass@example.com", "newpass456")
	if loginErr != nil {
		t.Fatalf("login with new password failed: %v", loginErr)
	}
}

func TestAuthService_ChangePassword_WrongCurrent(t *testing.T) {
	svc, cfg := setupAuthService(t)

	user, _ := svc.Register("wrongcur@example.com", "realpass", "User")
	token, _ := GenerateToken(user.ID, user.Email, user.Name, "local", user.Accesses, cfg, nil)

	err := svc.ChangePassword(user.ID, "wrongcurrent", "newpass456", token)
	if err == nil {
		t.Fatal("expected error for wrong current password")
	}
}

func TestAuthService_ChangePassword_NotFound(t *testing.T) {
	svc, _ := setupAuthService(t)

	err := svc.ChangePassword("nonexistent", "any", "newpass", "dummy-token")
	if err == nil {
		t.Fatal("expected error for missing user")
	}
}

func TestAuthService_ChangePassword_NonLocalProvider(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	svc := NewAuthService(userRepo, passkeyRepo)

	// Create a user with google provider directly
	googleUser := &models.User{
		ID: "google-user-1", Email: "google@example.com", Name: "Google User",
		Provider: "google", IsActive: true, Accesses: models.StringArray{"user"},
	}
	_ = userRepo.CreateUser(googleUser)

	err := svc.ChangePassword("google-user-1", "any", "newpass", "dummy-token")
	if err == nil {
		t.Fatal("expected error for non-local provider")
	}
	if err.Error() != "password change is only available for local accounts" {
		t.Fatalf("unexpected error: %s", err.Error())
	}
}

// --- ResetPassword tests ---

func TestAuthService_ResetPassword_Success(t *testing.T) {
	svc, cfg := setupAuthService(t)
	config.SetForTest(cfg)

	user, _ := svc.Register("resetpass@example.com", "oldpass123", "Reset Pass")

	// Simulate an existing session
	loginResp, _ := svc.Login("resetpass@example.com", "oldpass123")

	// Verify refresh token was stored
	beforeUser, _ := svc.userRepo.GetUserByID(user.ID)
	if beforeUser.RefreshToken == "" {
		t.Fatal("expected refresh token to be stored after login")
	}

	err := svc.ResetPassword(user.ID, "newSecurePass456!", loginResp.Token.AccessToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify refresh token was cleared (all sessions invalidated)
	updatedUser, _ := svc.userRepo.GetUserByID(user.ID)
	if updatedUser.RefreshToken != "" {
		t.Fatalf("expected refresh token to be cleared after password reset, got: %q", updatedUser.RefreshToken)
	}

	// Verify old password no longer works
	_, loginErr := svc.Login("resetpass@example.com", "oldpass123")
	if loginErr == nil {
		t.Fatal("expected login with old password to fail after reset")
	}

	// Verify new password works
	_, loginErr = svc.Login("resetpass@example.com", "newSecurePass456!")
	if loginErr != nil {
		t.Fatalf("login with new password failed: %v", loginErr)
	}
}

func TestAuthService_ResetPassword_UserNotFound(t *testing.T) {
	svc, _ := setupAuthService(t)

	err := svc.ResetPassword("nonexistent-user-id", "newpass123")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
	if err.Error() != "user not found" {
		t.Fatalf("expected 'user not found', got: %v", err)
	}
}

func TestAuthService_ResetPassword_ClearsRefreshToken(t *testing.T) {
	svc, cfg := setupAuthService(t)
	config.SetForTest(cfg)

	user, _ := svc.Register("resettoken@example.com", "oldpass123", "Reset Token")

	// Login to create a session
	loginResp, _ := svc.Login("resettoken@example.com", "oldpass123")

	// Verify refresh token is stored
	beforeUser, _ := svc.userRepo.GetUserByID(user.ID)
	if beforeUser.RefreshToken == "" {
		t.Fatal("expected refresh token to be stored after login")
	}

	// Reset password without passing access tokens
	err := svc.ResetPassword(user.ID, "newpass456!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify refresh token was cleared
	updatedUser, _ := svc.userRepo.GetUserByID(user.ID)
	if updatedUser.RefreshToken != "" {
		t.Fatalf("expected refresh token to be cleared after password reset, got: %q", updatedUser.RefreshToken)
	}

	// Verify old refresh token is blocked
	_, err = svc.ValidateRefreshToken(loginResp.Token.RefreshToken)
	if err == nil {
		t.Fatal("expected old refresh token to be invalid after password reset")
	}
}

// --- Logout / BlockRefreshToken / ValidateRefreshToken tests ---

func TestAuthService_Logout_Success(t *testing.T) {
	svc, cfg := setupAuthService(t)
	config.SetForTest(cfg)

	user, _ := svc.Register("logout@example.com", "pass", "Logout User")
	loginResp, _ := svc.Login("logout@example.com", "pass")

	err := svc.Logout(user.ID, loginResp.Token.RefreshToken, loginResp.Token.AccessToken)
	if err != nil {
		t.Fatalf("unexpected error during logout: %v", err)
	}
}

func TestAuthService_Logout_InvalidToken(t *testing.T) {
	svc, cfg := setupAuthService(t)
	config.SetForTest(cfg)

	user, _ := svc.Register("logout2@example.com", "pass", "Logout User 2")

	err := svc.Logout(user.ID, "not-a-real-jwt", "")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestAuthService_ValidateRefreshToken_Success(t *testing.T) {
	svc, cfg := setupAuthService(t)
	config.SetForTest(cfg)

	user, _ := svc.Register("valrt@example.com", "pass", "Val RT")
	loginResp, _ := svc.Login("valrt@example.com", "pass")

	claims, err := svc.ValidateRefreshToken(loginResp.Token.RefreshToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.UserID != user.ID {
		t.Fatalf("expected userID '%s', got '%s'", user.ID, claims.UserID)
	}
}

func TestAuthService_ValidateRefreshToken_Blocked(t *testing.T) {
	svc, cfg := setupAuthService(t)
	config.SetForTest(cfg)

	_, _ = svc.Register("blockedrt@example.com", "pass", "Blocked RT")
	loginResp, _ := svc.Login("blockedrt@example.com", "pass")

	// Block the token via logout
	_ = svc.BlockRefreshToken("blockedrt@example.com", loginResp.Token.RefreshToken)

	_, err := svc.ValidateRefreshToken(loginResp.Token.RefreshToken)
	if err == nil {
		t.Fatal("expected error for blocked token")
	}
}

func TestAuthService_ValidateRefreshToken_Invalid(t *testing.T) {
	svc, cfg := setupAuthService(t)
	config.SetForTest(cfg)

	_, err := svc.ValidateRefreshToken("totally-invalid-token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestRefreshTokenBoundToUser(t *testing.T) {
	svc, cfg := setupAuthService(t)
	config.SetForTest(cfg)

	// Register and login to get a valid refresh token
	_, _ = svc.Register("bound@example.com", "pass", "Bound User")
	loginResp, err := svc.Login("bound@example.com", "pass")
	if err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}
	oldRefreshToken := loginResp.Token.RefreshToken

	// Simulate token rotation on another device: update stored token to a new value
	if err := svc.UpdateRefreshToken(loginResp.User.ID, "rotated-token-from-other-device"); err != nil {
		t.Fatalf("failed to rotate token: %v", err)
	}

	// The old refresh token should now be rejected with ErrRefreshTokenMismatch
	_, err = svc.ValidateRefreshToken(oldRefreshToken)
	if err == nil {
		t.Fatal("expected error: old refresh token should be rejected after rotation")
	}
	if err != ErrRefreshTokenMismatch {
		t.Fatalf("expected ErrRefreshTokenMismatch, got: %v", err)
	}
}

// --- Email Verification Gate Tests ---

func TestAuthService_Login_UnverifiedReturnsErrEmailNotVerified(t *testing.T) {
	svc, cfg := setupAuthService(t)
	cfg.Auth.RequireEmailVerification = true
	config.SetForTest(cfg)

	_, _ = svc.Register("unver@example.com", "mypassword", "Unverified")

	_, err := svc.Login("unver@example.com", "mypassword")
	if err == nil {
		t.Fatal("expected ErrEmailNotVerified for unverified user")
	}

	var emailNotVerified *ErrEmailNotVerified
	if !errors.As(err, &emailNotVerified) {
		t.Fatalf("expected ErrEmailNotVerified, got %T: %v", err, err)
	}
	if emailNotVerified.Email != "unver@example.com" {
		t.Fatalf("expected email 'unver@example.com' in error, got '%s'", emailNotVerified.Email)
	}

	// Verify no refresh token was stored (no tokens generated)
	user, _ := svc.GetUserByEmail("unver@example.com")
	if user.RefreshToken != "" {
		t.Fatal("expected no refresh token stored for unverified user after failed login")
	}
}

func TestAuthService_Login_VerifiedOkWithEmailVerificationOn(t *testing.T) {
	svc, cfg := setupAuthService(t)
	cfg.Auth.RequireEmailVerification = true
	config.SetForTest(cfg)

	user, _ := svc.Register("ver@example.com", "mypassword", "Verified")

	// Manually verify the user
	now := time.Now()
	user.EmailVerifiedAt = &now
	_ = svc.UpdateUser(user)

	resp, err := svc.Login("ver@example.com", "mypassword")
	if err != nil {
		t.Fatalf("unexpected error for verified user: %v", err)
	}
	if resp.Token.AccessToken == "" {
		t.Fatal("expected access token for verified user")
	}
	if resp.Token.RefreshToken == "" {
		t.Fatal("expected refresh token for verified user")
	}
}

func TestAuthService_GuestLogin_SkipsEmailVerification(t *testing.T) {
	svc, cfg := setupAuthService(t)
	cfg.Auth.RequireEmailVerification = true
	config.SetForTest(cfg)

	resp, err := svc.GuestLogin("Guest Player")
	if err != nil {
		t.Fatalf("unexpected error for guest login: %v", err)
	}
	if resp.Token.AccessToken == "" {
		t.Fatal("expected access token for guest user")
	}
}

func TestCanonicalizeEmail(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{" Test@Example.com ", "test@example.com"},
		{"Straße@ExAmPlE.cOm", "straße@example.com"},
		{"test@münchen.de", "test@xn--mnchen-3ya.de"},
		{"  USER@example.com  ", "user@example.com"},
		{"\uFEFFtest@example.com", "test@example.com"},
		{"TEST@[127.0.0.1]", "test@[127.0.0.1]"},
		{"user+tag@GMAIL.com", "user+tag@gmail.com"},
		{"  ", ""},
	}
	for _, tc := range tests {
		got := CanonicalizeEmail(tc.input)
		if got != tc.expected {
			t.Errorf("CanonicalizeEmail(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
