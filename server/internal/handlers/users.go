package handlers

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/database"
	"bedrud/internal/models"
	"bedrud/internal/queue"
	"bedrud/internal/repository"
	"bedrud/internal/services"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

type UsersHandler struct {
	userRepo         *repository.UserRepository
	roomRepo         *repository.RoomRepository
	passkeyRepo      *repository.PasskeyRepository
	prefsRepo        *repository.UserPreferencesRepository
	cleanupSvc       *services.RoomCleanupService
	verifEventRepo   *repository.VerificationEventRepository
	deletionInFlight sync.Map
}

type UserListResponse struct {
	Users []UserDetails `json:"users"`
}

type UserDetails struct {
	ID              string   `json:"id" example:"123e4567-e89b-12d3-a456-426614174000"`
	Email           string   `json:"email" example:"user@example.com"`
	Name            string   `json:"name" example:"John Doe"`
	Provider        string   `json:"provider" example:"local"`
	IsActive        bool     `json:"isActive" example:"true"`
	IsAdmin         bool     `json:"isAdmin" example:"false"`
	Accesses        []string `json:"accesses" example:"user,admin"`
	EmailVerifiedAt *string  `json:"emailVerifiedAt,omitempty" example:"2025-01-01 12:00:00"`
	CreatedAt       string   `json:"createdAt" example:"2025-01-01 12:00:00"`
}

type UserStatusUpdateRequest struct {
	Active bool `json:"active" example:"true"`
}

type UserStatusUpdateResponse struct {
	Message string `json:"message" example:"User status updated successfully"`
}

func NewUsersHandler(
	userRepo *repository.UserRepository,
	roomRepo *repository.RoomRepository,
	passkeyRepo *repository.PasskeyRepository,
	prefsRepo *repository.UserPreferencesRepository,
	cleanupSvc *services.RoomCleanupService,
	verifEventRepo *repository.VerificationEventRepository,
) *UsersHandler {
	return &UsersHandler{
		userRepo:       userRepo,
		roomRepo:       roomRepo,
		passkeyRepo:    passkeyRepo,
		prefsRepo:      prefsRepo,
		cleanupSvc:     cleanupSvc,
		verifEventRepo: verifEventRepo,
	}
}

// @Summary List all users
// @Description Get a list of all users in the system (requires superadmin access)
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} UserListResponse "List of users"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /admin/users [get]
func (h *UsersHandler) ListUsers(c *fiber.Ctx) error {
	p := repository.UserFilterParams{
		Page:   c.QueryInt("page", 1),
		Limit:  c.QueryInt("limit", 50),
		Search: c.Query("q"),
	}

	// Parse provider
	if prov := c.Query("provider"); prov != "" {
		p.Provider = strings.Split(prov, ",")
		validProviders := map[string]bool{"local": true, "google": true, "github": true, "guest": true}
		for _, v := range p.Provider {
			if !validProviders[v] {
				return c.Status(400).JSON(fiber.Map{"error": "Invalid provider: " + v})
			}
		}
	}

	// Parse role
	if role := c.Query("role"); role != "" {
		p.Role = strings.Split(role, ",")
		validRoles := map[string]bool{"superadmin": true, "admin": true, "moderator": true, "user": true, "guest": true}
		for _, v := range p.Role {
			if !validRoles[v] {
				return c.Status(400).JSON(fiber.Map{"error": "Invalid role: " + v})
			}
		}
	}

	// Parse status
	if st := c.Query("status"); st != "" {
		p.Status = strings.Split(st, ",")
		for _, v := range p.Status {
			if v != "active" && v != "banned" {
				return c.Status(400).JSON(fiber.Map{"error": "Invalid status: " + v})
			}
		}
	}

	// Parse verified filter
	if v := c.Query("verified"); v != "" {
		switch v {
		case queryTrue:
			t := true
			p.Verified = &t
		case "false":
			t := false
			p.Verified = &t
		default:
			return c.Status(400).JSON(fiber.Map{"error": "Invalid verified filter, use true/false"})
		}
	}

	// Parse created
	p.Created = c.Query("created")
	if p.Created != "" {
		validDurations := map[string]bool{"today": true, "7d": true, "30d": true}
		if !validDurations[p.Created] {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid created filter"})
		}
	}

	// Parse sort/order
	p.Sort = c.Query("sort", "createdAt")
	p.Order = c.Query("order", orderDesc)
	validSorts := map[string]bool{"name": true, "email": true, "provider": true, "createdAt": true}
	if !validSorts[p.Sort] {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid sort field"})
	}
	if p.Order != orderAsc && p.Order != orderDesc {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid order, must be asc or desc"})
	}

	// Clamp limit
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 50
	}

	// Clamp page
	if p.Page <= 0 {
		p.Page = 1
	}

	users, total, err := h.userRepo.GetAllUsersFiltered(&p)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch users",
		})
	}

	response := make([]UserDetails, 0, len(users))
	for i := range users {
		user := &users[i]
		var eva *string
		if user.EmailVerifiedAt != nil {
			s := user.EmailVerifiedAt.Format("2006-01-02 15:04:05")
			eva = &s
		}
		response = append(response, UserDetails{
			ID:              user.ID,
			Email:           user.Email,
			Name:            user.Name,
			Provider:        user.Provider,
			IsActive:        user.IsActive,
			IsAdmin:         containsAccess(user.Accesses, "admin"),
			Accesses:        user.Accesses,
			EmailVerifiedAt: eva,
			CreatedAt:       user.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return c.JSON(fiber.Map{
		"users": response,
		"total": total,
		"page":  p.Page,
		"limit": p.Limit,
	})
}

// @Summary Update user accesses
// @Description Update user access levels (requires superadmin access). Invalidates all existing sessions by clearing their refresh token.
// @Tags admin
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param request body object{accesses=[]string} true "Access levels to assign"
// @Security BearerAuth
// @Success 200 {object} map[string]interface{} "Accesses updated"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 404 {object} ErrorResponse "User not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /admin/users/{id}/accesses [put]
func (h *UsersHandler) UpdateUserAccesses(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*auth.Claims)
	if !ok || claims == nil || !containsAccess(claims.Accesses, "superadmin") {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Insufficient permissions"})
	}

	userID := c.Params("id")
	var input struct {
		Accesses []string `json:"accesses"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	// Prevent self-modification
	if userID == claims.UserID {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot modify your own access levels"})
	}

	// Validate role values
	validRoles := map[string]bool{
		string(models.AccessSuperAdmin): true,
		string(models.AccessAdmin):      true,
		string(models.AccessMod):        true,
		string(models.AccessUser):       true,
		string(models.AccessGuest):      true,
	}
	for _, r := range input.Accesses {
		if !validRoles[r] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("Invalid access role: %s", r)})
		}
	}

	// Check last-superadmin guard — prevent demoting the only superadmin
	if !containsAccess(input.Accesses, string(models.AccessSuperAdmin)) {
		targetUser, err := h.userRepo.GetUserByID(userID)
		if err == nil && targetUser != nil && containsAccess(targetUser.Accesses, string(models.AccessSuperAdmin)) {
			superadmins, _ := h.userRepo.GetUsersByAccess(models.AccessSuperAdmin)
			if len(superadmins) <= 1 {
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Cannot demote the last superadmin"})
			}
		}
	}

	// Atomically update accesses and clear the refresh token so the user gets a
	// fresh JWT with correct roles on their next login.
	if err := h.userRepo.UpdateAccessesAndClearToken(userID, input.Accesses); err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update user"})
	}

	return c.JSON(fiber.Map{"message": "User accesses updated"})
}

// UpdateUserStatus activates or deactivates a user account.
// PUT /api/admin/users/:id/status
//
// @Summary Update user status
// @Description Activate or deactivate a user account. Superadmin access required. Cannot change own status.
// @Tags admin
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param request body object true "{isActive: bool, reason?: string}"
// @Success 200 {object} map[string]string "{message: status updated}"
// @Failure 400 {object} ErrorResponse "Cannot change own status or invalid input"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 404 {object} ErrorResponse "User not found"
// @Failure 500 {object} ErrorResponse "Failed to update status"
// @Router /admin/users/{id}/status [put]
func (h *UsersHandler) UpdateUserStatus(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*auth.Claims)
	if !ok || claims == nil || !containsAccess(claims.Accesses, "superadmin") {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Insufficient permissions"})
	}

	userID := c.Params("id")
	var input UserStatusUpdateRequest

	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid input",
		})
	}

	if userID == claims.UserID {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot change your own status"})
	}

	_, err := h.userRepo.GetUserByID(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to lookup user",
		})
	}

	// Check last-superadmin guard before deactivating
	if !input.Active {
		targetUser, err := h.userRepo.GetUserByID(userID)
		if err == nil && targetUser != nil && containsAccess(targetUser.Accesses, string(models.AccessSuperAdmin)) {
			superadmins, _ := h.userRepo.GetUsersByAccess(models.AccessSuperAdmin)
			if len(superadmins) <= 1 {
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Cannot deactivate the last superadmin"})
			}
		}

		// When banning, atomically set is_active=false and clear the refresh token
		// to immediately invalidate all sessions. Uses a single DB call to avoid
		// the security gap where ban succeeds but token-clear fails independently.

		if err := h.userRepo.UpdateUserStatusAndClearToken(userID, false); err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update user status",
			})
		}
		// Add to in-memory banned set for fast middleware checks
		auth.BanUser(userID)
		return c.JSON(UserStatusUpdateResponse{
			Message: "User status updated successfully",
		})
	}

	// Unbanning: only set is_active=true, no token manipulation needed.
	if err := h.userRepo.ActivateUser(userID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update user status",
		})
	}
	// Remove from in-memory banned set
	auth.UnbanUser(userID)

	return c.JSON(UserStatusUpdateResponse{
		Message: "User status updated successfully",
	})
}

// DeleteUser permanently deletes a user and all associated data asynchronously.
// @Summary Delete a user
// @Description Permanently delete a user and all associated data (requires superadmin access). Cannot delete your own account. Returns 202 Accepted — deletion runs in background.
// @Tags admin
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Security BearerAuth
// @Success 202 {object} map[string]interface{} "Deletion queued"
// @Failure 400 {object} ErrorResponse "Cannot delete own account"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 404 {object} ErrorResponse "User not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /admin/users/{id} [delete]
func (h *UsersHandler) DeleteUser(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*auth.Claims)
	if !ok || claims == nil || !containsAccess(claims.Accesses, "superadmin") {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Insufficient permissions"})
	}

	userID := c.Params("id")

	if userID == claims.UserID {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot delete your own account. Use the CLI or have another admin perform this action.",
		})
	}

	// Idempotency guard — prevent concurrent deletions of the same user
	if _, loaded := h.deletionInFlight.LoadOrStore(userID, true); loaded {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Deletion already in progress"})
	}

	// Immediately deactivate the user to prevent further actions
	if err := h.userRepo.UpdateUserStatusAndClearToken(userID, false); err != nil {
		h.deletionInFlight.Delete(userID)
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to deactivate user"})
	}
	auth.BanUser(userID)

	user, err := h.userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		h.deletionInFlight.Delete(userID)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	// Prevent deleting the last superadmin
	if containsAccess(user.Accesses, string(models.AccessSuperAdmin)) {
		superadmins, err := h.userRepo.GetUsersByAccess(models.AccessSuperAdmin)
		if err != nil {
			h.deletionInFlight.Delete(userID)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to verify superadmin count"})
		}
		if len(superadmins) <= 1 {
			h.deletionInFlight.Delete(userID)
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Cannot delete the last superadmin"})
		}
	}

	rooms, err := h.roomRepo.GetRoomsCreatedByUser(userID)
	if err != nil {
		h.deletionInFlight.Delete(userID)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch user rooms"})
	}
	if rooms == nil {
		rooms = []models.Room{}
	}

	payload := queue.UserDeletePayload{
		UserID:  userID,
		Email:   user.Email,
		RoomIDs: roomIDsToStrings(rooms),
	}
	if err := queue.Enqueue(context.Background(), database.GetDB(), "user_delete", payload,
		queue.WithPriority(1), queue.WithMaxAttempts(3)); err != nil {
		h.deletionInFlight.Delete(userID)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to queue deletion"})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message": "User deletion queued",
		"rooms":   len(rooms),
	})
}

func roomIDsToStrings(rooms []models.Room) []string {
	ids := make([]string, len(rooms))
	for i := range rooms {
		ids[i] = rooms[i].ID
	}
	return ids
}

// @Summary Set user password (admin)
// @Description Force-set a user's password. Superadmin only. Invalidates all existing sessions by clearing their refresh token.
// @Tags admin
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param request body object{password=string} true "New password (12-128 characters)"
// @Security BearerAuth
// @Success 200 {object} map[string]interface{} "Password updated"
// @Failure 400 {object} ErrorResponse "Invalid password"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 404 {object} ErrorResponse "User not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /admin/users/{id}/password [put]
func (h *UsersHandler) SetUserPassword(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*auth.Claims)
	if !ok || claims == nil || !containsAccess(claims.Accesses, "superadmin") {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Insufficient permissions"})
	}

	userID := c.Params("id")
	var input struct {
		Password string `json:"password"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}
	if len(input.Password) < MinPasswordLength {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("Password must be at least %d characters", MinPasswordLength)})
	}
	if len(input.Password) > MaxPasswordLength {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("Password must be at most %d characters", MaxPasswordLength)})
	}

	hashed, err := auth.HashPassword(input.Password)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to hash password"})
	}

	if err := h.userRepo.UpdatePassword(userID, hashed); err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update password"})
	}

	log.Info().Str("adminID", claims.UserID).Str("targetUserID", userID).Msg("Admin reset user password")
	return c.JSON(fiber.Map{"message": "Password updated successfully"})
}

// ForceLogout revokes all sessions by clearing the stored refresh token and blocking
// existing access tokens at the shared middleware ban set (process-local).
// @Summary Force logout a user
// @Description Revoke all active sessions for a user by clearing their refresh token (superadmin only)
// @Tags admin
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Security BearerAuth
// @Success 200 {object} map[string]interface{} "Sessions revoked"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 404 {object} ErrorResponse "User not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /admin/users/{id}/force-logout [post]
func (h *UsersHandler) ForceLogout(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*auth.Claims)
	if !ok || claims == nil || !containsAccess(claims.Accesses, "superadmin") {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Insufficient permissions"})
	}

	userID := c.Params("id")

	user, err := h.userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if err := h.userRepo.ClearRefreshToken(userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to revoke sessions"})
	}
	auth.BanUser(userID)

	log.Info().Str("adminID", claims.UserID).Str("targetUserID", userID).Msg("Admin force-logged out user")
	return c.JSON(fiber.Map{"message": "All sessions revoked"})
}

// ListUserSessions returns paginated room participation sessions for a user.
// @Summary List user room sessions
// @Description Get paginated room participation history for a user (superadmin only)
// @Tags admin
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(50)
// @Security BearerAuth
// @Success 200 {object} map[string]interface{} "{sessions, total, page, limit}"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 404 {object} ErrorResponse "User not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /admin/users/{id}/sessions [get]
func (h *UsersHandler) ListUserSessions(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*auth.Claims)
	if !ok || claims == nil || !containsAccess(claims.Accesses, "superadmin") {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Insufficient permissions"})
	}

	userID := c.Params("id")

	user, err := h.userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	p := repository.UserParticipationsParams{
		Page:  c.QueryInt("page", 1),
		Limit: c.QueryInt("limit", 50),
	}

	participants, total, err := h.roomRepo.GetUserParticipationsPaginated(userID, p)
	if err != nil {
		log.Error().Err(err).Str("userID", userID).Msg("Failed to fetch user sessions")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch user sessions"})
	}

	sessions := make([]fiber.Map, 0, len(participants))
	for _, part := range participants {
		roomName := ""
		if part.Room != nil {
			roomName = part.Room.Name
		}

		var durationSeconds int64
		if part.LeftAt != nil {
			durationSeconds = int64(part.LeftAt.Sub(part.JoinedAt).Seconds())
		} else if !part.JoinedAt.IsZero() {
			durationSeconds = int64(time.Since(part.JoinedAt).Seconds())
		}

		sessions = append(sessions, fiber.Map{
			"id":              part.ID,
			"roomId":          part.RoomID,
			"roomName":        roomName,
			"joinedAt":        part.JoinedAt.Format(time.RFC3339),
			"leftAt":          leftAtOrNil(part.LeftAt),
			"isActive":        part.IsActive,
			"durationSeconds": durationSeconds,
		})
	}

	return c.JSON(fiber.Map{
		"sessions": sessions,
		"total":    total,
		"page":     p.Page,
		"limit":    p.Limit,
	})
}

// BulkBanUsers deactivates multiple users and clears their sessions.
// Reports per-ID errors: "already inactive", "user not found", or DB errors.
// Self-ban is blocked. Last superadmin guard applies.
// @Summary Bulk ban users
// @Tags admin
// @Accept json
// @Produce json
// @Param request body BulkIDsRequest true "User IDs to ban"
// @Success 200 {object} BulkResult
// @Router /admin/users/bulk/ban [post]
func (h *UsersHandler) BulkBanUsers(c *fiber.Ctx) error {
	var req BulkIDsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}
	if len(req.IDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No user IDs provided"})
	}
	if len(req.IDs) > 500 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Maximum 500 users per request"})
	}

	claims := c.Locals("user").(*auth.Claims)
	users, err := h.userRepo.GetUsersByIDs(req.IDs)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch users"})
	}

	userByID := make(map[string]*models.User, len(users))
	for i := range users {
		userByID[users[i].ID] = &users[i]
	}

	results := make(map[string]BulkItemResult, len(req.IDs))
	processed := 0
	failed := 0

	// Build list of IDs that actually need banning
	var toBan []string
	for _, id := range req.IDs {
		u, found := userByID[id]
		if !found {
			results[id] = BulkItemResult{Success: false, Error: "user not found"}
			failed++
			continue
		}

		// Self-ban guard
		if id == claims.UserID {
			results[id] = BulkItemResult{Success: false, Name: u.Name, Error: "cannot ban yourself"}
			failed++
			continue
		}

		// Last superadmin guard
		if containsAccess(u.Accesses, "superadmin") && u.IsActive {
			superadmins, _ := h.userRepo.GetUsersByAccess(models.AccessSuperAdmin)
			if len(superadmins) <= 1 {
				results[id] = BulkItemResult{Success: false, Name: u.Name, Error: "cannot ban the last superadmin"}
				failed++
				continue
			}
		}

		if !u.IsActive {
			results[id] = BulkItemResult{Success: true, Name: u.Name}
			processed++
			continue
		}

		toBan = append(toBan, id)
	}

	// Batch-ban remaining
	if len(toBan) > 0 {
		dbErrors := h.userRepo.BatchBan(toBan)
		for _, id := range toBan {
			if dbErr, ok := dbErrors[id]; ok {
				results[id] = BulkItemResult{Success: false, Name: userByID[id].Name, Error: dbErr.Error()}
				failed++
			} else {
				auth.BanUser(id)
				results[id] = BulkItemResult{Success: true, Name: userByID[id].Name}
				processed++
			}
		}
	}

	// Fix processed/failed counts: "already inactive" users counted as success
	return c.JSON(BulkResult{
		Results:        results,
		TotalProcessed: processed,
		TotalFailed:    failed,
	})
}

// BulkPromoteUsers grants superadmin access to multiple users.
// Reports per-ID errors: "already admin", "user not found", or DB errors.
// @Summary Bulk promote users to admin
// @Tags admin
// @Accept json
// @Produce json
// @Param request body BulkIDsRequest true "User IDs to promote"
// @Success 200 {object} BulkResult
// @Router /admin/users/bulk/promote [post]
func (h *UsersHandler) BulkPromoteUsers(c *fiber.Ctx) error {
	var req BulkIDsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}
	if len(req.IDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No user IDs provided"})
	}
	if len(req.IDs) > 500 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Maximum 500 users per request"})
	}

	claims := c.Locals("user").(*auth.Claims)
	users, err := h.userRepo.GetUsersByIDs(req.IDs)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch users"})
	}

	userByID := make(map[string]*models.User, len(users))
	for i := range users {
		userByID[users[i].ID] = &users[i]
	}

	results := make(map[string]BulkItemResult, len(req.IDs))
	processed := 0
	failed := 0

	var toPromote []string
	for _, id := range req.IDs {
		u, found := userByID[id]
		if !found {
			results[id] = BulkItemResult{Success: false, Error: "user not found"}
			failed++
			continue
		}

		// Self-promote guard
		if id == claims.UserID {
			results[id] = BulkItemResult{Success: false, Name: u.Name, Error: "cannot promote yourself"}
			failed++
			continue
		}

		if containsAccess(u.Accesses, "superadmin") {
			results[id] = BulkItemResult{Success: true, Name: u.Name}
			processed++
			continue
		}

		toPromote = append(toPromote, id)
	}

	if len(toPromote) > 0 {
		dbErrors := h.userRepo.BatchPromote(toPromote)
		for _, id := range toPromote {
			if dbErr, ok := dbErrors[id]; ok {
				results[id] = BulkItemResult{Success: false, Name: userByID[id].Name, Error: dbErr.Error()}
				failed++
			} else {
				results[id] = BulkItemResult{Success: true, Name: userByID[id].Name}
				processed++
			}
		}
	}

	return c.JSON(BulkResult{
		Results:        results,
		TotalProcessed: processed,
		TotalFailed:    failed,
	})
}

// BulkDeleteUsers queues deletion of multiple users asynchronously.
// Filters out self and last superadmin before queuing.
// @Summary Bulk delete users
// @Tags admin
// @Accept json
// @Produce json
// @Param request body BulkIDsRequest true "User IDs to delete"
// @Success 202 {object} map[string]interface{}
// @Router /admin/users/bulk/delete [post]
func (h *UsersHandler) BulkDeleteUsers(c *fiber.Ctx) error {
	var req BulkIDsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}
	if len(req.IDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No user IDs provided"})
	}
	if len(req.IDs) > 500 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Maximum 500 users per request"})
	}

	claims := c.Locals("user").(*auth.Claims)
	users, err := h.userRepo.GetUsersByIDs(req.IDs)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch users"})
	}

	userByID := make(map[string]*models.User, len(users))
	for i := range users {
		userByID[users[i].ID] = &users[i]
	}

	var toDelete []string
	var skipped []string
	for _, id := range req.IDs {
		u, found := userByID[id]
		if !found {
			skipped = append(skipped, id)
			continue
		}

		// Self-delete guard
		if id == claims.UserID {
			skipped = append(skipped, id+"(self)")
			continue
		}

		// Last superadmin guard
		if containsAccess(u.Accesses, "superadmin") {
			superadmins, _ := h.userRepo.GetUsersByAccess(models.AccessSuperAdmin)
			if len(superadmins) <= 1 {
				skipped = append(skipped, id+"(last-superadmin)")
				continue
			}
		}

		toDelete = append(toDelete, id)
	}

	if len(toDelete) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No users eligible for deletion"})
	}

	// Soft-deactivate
	h.userRepo.BatchBan(toDelete)
	for _, id := range toDelete {
		auth.BanUser(id)
	}

	for _, id := range toDelete {
		user, err := h.userRepo.GetUserByID(id)
		if err != nil || user == nil {
			continue
		}
		rooms, err := h.roomRepo.GetRoomsCreatedByUser(id)
		if err != nil {
			log.Warn().Err(err).Str("userID", id).Msg("BulkDelete: failed to fetch rooms")
			rooms = []models.Room{}
		}
		if rooms == nil {
			rooms = []models.Room{}
		}
		payload := queue.UserDeletePayload{
			UserID:  id,
			Email:   user.Email,
			RoomIDs: roomIDsToStrings(rooms),
		}
		if err := queue.Enqueue(context.Background(), database.GetDB(), "user_delete", payload,
			queue.WithPriority(1), queue.WithMaxAttempts(3)); err != nil {
			log.Error().Err(err).Str("userID", id).Msg("BulkDelete: failed to enqueue deletion")
		}
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message": fmt.Sprintf("Deletion queued for %d users", len(toDelete)),
		"count":   len(toDelete),
		"skipped": skipped,
	})
}

// GetUserDetail returns detailed information about a specific user.
// GET /api/admin/users/:id
//
// @Summary Get user detail
// @Description Get detailed user information including room count and metadata. Superadmin access required.
// @Tags admin
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} map[string]interface{} "User details with room count"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 404 {object} ErrorResponse "User not found"
// @Failure 500 {object} ErrorResponse "Failed to fetch user"
// @Router /admin/users/{id} [get]
func (h *UsersHandler) GetUserDetail(c *fiber.Ctx) error {
	userID := c.Params("id")
	user, err := h.userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	rooms, err := h.roomRepo.GetRoomsCreatedByUser(userID)
	if err != nil {
		log.Error().Err(err).Str("userID", userID).Msg("Failed to fetch user rooms")
		rooms = []models.Room{}
	} else if rooms == nil {
		rooms = []models.Room{}
	}

	var eva *string
	if user.EmailVerifiedAt != nil {
		s := user.EmailVerifiedAt.Format("2006-01-02 15:04:05")
		eva = &s
	}

	return c.JSON(fiber.Map{
		"user": UserDetails{
			ID:              user.ID,
			Email:           user.Email,
			Name:            user.Name,
			Provider:        user.Provider,
			IsActive:        user.IsActive,
			IsAdmin:         containsAccess(user.Accesses, "admin"),
			Accesses:        user.Accesses,
			EmailVerifiedAt: eva,
			CreatedAt:       user.CreatedAt.Format("2006-01-02 15:04:05"),
		},
		"rooms": rooms,
	})
}

// AdminVerifyEmail force-verifies a user's email (bypasses verification token).
// AdminVerifyEmail force-verifies a user's email.
// POST /api/admin/users/:id/verify
//
// @Summary Admin verify email
// @Description Force-verify a user's email address. Superadmin access required.
// @Tags admin
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} map[string]string "{message: email verified}"
// @Failure 400 {object} ErrorResponse "Email already verified"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 404 {object} ErrorResponse "User not found"
// @Failure 500 {object} ErrorResponse "Failed to verify"
// @Router /admin/users/{id}/verify [post]
func (h *UsersHandler) AdminVerifyEmail(c *fiber.Ctx) error {
	userID := c.Params("id")
	user, err := h.userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if user.EmailVerifiedAt != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "User email is already verified"})
	}

	now := time.Now()
	user.EmailVerifiedAt = &now
	if err := h.userRepo.UpdateUser(user); err != nil {
		log.Error().Err(err).Str("userID", userID).Msg("AdminVerifyEmail: failed to update user")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to verify email"})
	}

	// Audit log
	if h.verifEventRepo != nil {
		if claims, ok := c.Locals("user").(*auth.Claims); ok && claims != nil {
			metadata := "admin_id: " + claims.UserID
			if err := h.verifEventRepo.RecordEvent(userID, user.Email, models.VerificationAdminForce, c.IP(), metadata); err != nil {
				log.Warn().Err(err).Str("userID", userID).Msg("AdminVerifyEmail: failed to record verification event")
			}
		}
	}

	return c.JSON(fiber.Map{"message": "Email verified successfully"})
}

// AdminResendVerification re-sends the verification email for a user.
// AdminResendVerification resends the verification email for a user.
// POST /api/admin/users/:id/verify/resend
//
// @Summary Admin resend verification
// @Description Resend the email verification link for a user. Superadmin access required.
// @Tags admin
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} map[string]string "{message: verification email sent}"
// @Failure 400 {object} ErrorResponse "Email already verified"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 404 {object} ErrorResponse "User not found"
// @Failure 500 {object} ErrorResponse "Failed to send"
// @Router /admin/users/{id}/verify/resend [post]
func (h *UsersHandler) AdminResendVerification(c *fiber.Ctx) error {
	userID := c.Params("id")
	user, err := h.userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if user.EmailVerifiedAt != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "User email is already verified"})
	}

	// Build context and enqueue
	ctx := context.Background()
	token, err := auth.GenerateVerificationToken(userID, user.Email, config.Get())
	if err != nil {
		log.Error().Err(err).Str("userID", userID).Msg("AdminResendVerification: failed to generate token")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate verification token"})
	}

	frontendURL := frontendBaseURL(config.Get())
	if frontendURL == "" && config.Get().Server.Domain != "" {
		frontendURL = fmt.Sprintf("https://%s", strings.TrimRight(config.Get().Server.Domain, "/"))
	}
	verifyURL := frontendURL + "/auth/verify?token=" + token

	if err := queue.Enqueue(ctx, database.GetDB(), "send_email",
		queue.SendEmailPayload{
			To:           user.Email,
			Subject:      "Verify your Bedrud email (admin resend)",
			TemplateName: "verify_email",
			TemplateData: map[string]any{
				"Name":      user.Name,
				"VerifyURL": verifyURL,
			},
		},
	); err != nil {
		log.Error().Err(err).Str("userID", userID).Msg("AdminResendVerification: failed to enqueue email")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to send verification email"})
	}

	// Audit log
	if h.verifEventRepo != nil {
		if claims, ok := c.Locals("user").(*auth.Claims); ok && claims != nil {
			metadata := "admin_id: " + claims.UserID
			if err := h.verifEventRepo.RecordEvent(userID, user.Email, models.VerificationResent, c.IP(), metadata); err != nil {
				log.Warn().Err(err).Str("userID", userID).Msg("AdminResendVerification: failed to record verification event")
			}
		}
	}

	return c.JSON(fiber.Map{"message": "Verification email sent"})
}

// ListRecentSignups returns paginated recent user signups.
// @Summary List recent signups
// @Description Get a paginated list of recently registered users (requires superadmin access)
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(50)
// @Param q query string false "Search by name or email"
// @Param provider query string false "Comma-separated providers: local,google,github,guest"
// @Param dateFrom query string false "Start date (YYYY-MM-DD)"
// @Param dateTo query string false "End date (YYYY-MM-DD)"
// @Param sort query string false "Sort field: createdAt, name" default(createdAt)
// @Param order query string false "Sort direction: asc, desc" default(desc)
// @Success 200 {object} map[string]interface{} "{users, total, page, limit}"
// @Failure 400 {object} ErrorResponse "Invalid parameters"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /admin/users/recent [get]
func (h *UsersHandler) ListRecentSignups(c *fiber.Ctx) error {
	p := repository.RecentSignupsFilterParams{
		Page:   c.QueryInt("page", 1),
		Limit:  c.QueryInt("limit", 50),
		Search: c.Query("q"),
	}

	// Parse provider
	if prov := c.Query("provider"); prov != "" {
		p.Provider = strings.Split(prov, ",")
		validProviders := map[string]bool{"local": true, "google": true, "github": true, "guest": true}
		for _, v := range p.Provider {
			if !validProviders[v] {
				return c.Status(400).JSON(fiber.Map{"error": "Invalid provider: " + v})
			}
		}
	} else {
		// Default: exclude guest users from sign-ups
		p.ExcludeGuest = true
	}

	// Date range — validate format
	p.DateFrom = c.Query("dateFrom")
	p.DateTo = c.Query("dateTo")
	if p.DateFrom != "" {
		if _, err := time.Parse("2006-01-02", p.DateFrom); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid dateFrom format, expected YYYY-MM-DD"})
		}
	}
	if p.DateTo != "" {
		if _, err := time.Parse("2006-01-02", p.DateTo); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid dateTo format, expected YYYY-MM-DD"})
		}
	}

	// Sort
	p.Sort = c.Query("sort", "createdAt")
	p.Order = c.Query("order", orderDesc)
	if p.Sort != "createdAt" && p.Sort != "name" {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid sort field"})
	}
	if p.Order != orderAsc && p.Order != orderDesc {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid order, must be asc or desc"})
	}

	// Clamp limit
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 50
	}

	// Clamp page
	if p.Page <= 0 {
		p.Page = 1
	}

	users, total, err := h.userRepo.GetRecentSignupsFiltered(&p)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch recent signups"})
	}

	return c.JSON(fiber.Map{
		"users": users,
		"total": total,
		"page":  p.Page,
		"limit": p.Limit,
	})
}

func leftAtOrNil(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}
