package handlers

import (
	"bedrud/internal/auth"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/services"
	"context"
	"fmt"
	"sync"
	"time"

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
	wg               sync.WaitGroup
	deletionInFlight sync.Map
}

type UserListResponse struct {
	Users []UserDetails `json:"users"`
}

type UserDetails struct {
	ID        string   `json:"id" example:"123e4567-e89b-12d3-a456-426614174000"`
	Email     string   `json:"email" example:"user@example.com"`
	Name      string   `json:"name" example:"John Doe"`
	Provider  string   `json:"provider" example:"local"`
	IsActive  bool     `json:"isActive" example:"true"`
	IsAdmin   bool     `json:"isAdmin" example:"false"`
	Accesses  []string `json:"accesses" example:"user,admin"`
	CreatedAt string   `json:"createdAt" example:"2025-01-01 12:00:00"`
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
) *UsersHandler {
	return &UsersHandler{
		userRepo:    userRepo,
		roomRepo:    roomRepo,
		passkeyRepo: passkeyRepo,
		prefsRepo:   prefsRepo,
		cleanupSvc:  cleanupSvc,
	}
}

func (h *UsersHandler) Shutdown() {
	h.wg.Wait()
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
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)
	users, total, err := h.userRepo.GetAllUsers(repository.PaginationParams{Page: page, Limit: limit})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch users",
		})
	}

	response := make([]UserDetails, 0, len(users))
	for i := range users {
		user := &users[i]
		response = append(response, UserDetails{
			ID:        user.ID,
			Email:     user.Email,
			Name:      user.Name,
			Provider:  user.Provider,
			IsActive:  user.IsActive,
			IsAdmin:   containsAccess(user.Accesses, "admin"),
			Accesses:  user.Accesses,
			CreatedAt: user.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return c.JSON(fiber.Map{
		"users": response,
		"total": total,
		"page":  page,
		"limit": limit,
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

	// When banning, atomically set is_active=false and clear the refresh token
	// to immediately invalidate all sessions. Uses a single DB call to avoid
	// the security gap where ban succeeds but token-clear fails independently.
	if !input.Active {
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	h.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Str("userID", userID).Msg("delete goroutine panicked")
			}
		}()
		defer cancel()
		defer h.wg.Done()
		defer h.deletionInFlight.Delete(userID)
		h.runHardDeleteJob(ctx, user.Email, userID, rooms)
	}()

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message": "User deletion queued",
		"rooms":   len(rooms),
	})
}

func (h *UsersHandler) runHardDeleteJob(ctx context.Context, userEmail, userID string, rooms []models.Room) {
	if err := h.cleanupSvc.DeleteUserRooms(ctx, rooms, userID); err != nil {
		log.Warn().Err(err).Str("userID", userID).
			Int("total", len(rooms)).
			Msg("room cleanup had errors, proceeding with user deletion")
	}

	if err := h.userRepo.DeleteUser(userID); err != nil {
		log.Error().Err(err).Str("userID", userID).Msg("failed to delete user")
		return
	}

	log.Info().Str("userID", userID).Str("email", userEmail).Int("rooms", len(rooms)).Msg("user deleted with room cleanup")
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

// ForceLogout revokes all sessions for a user by clearing their stored refresh token.
// The user's access token will remain valid until it naturally expires.
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

	log.Info().Str("adminID", claims.UserID).Str("targetUserID", userID).Msg("Admin force-logged out user")
	return c.JSON(fiber.Map{"message": "All sessions revoked"})
}

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

	return c.JSON(fiber.Map{
		"user": UserDetails{
			ID:        user.ID,
			Email:     user.Email,
			Name:      user.Name,
			Provider:  user.Provider,
			IsActive:  user.IsActive,
			IsAdmin:   containsAccess(user.Accesses, "admin"),
			Accesses:  user.Accesses,
			CreatedAt: user.CreatedAt.Format("2006-01-02 15:04:05"),
		},
		"rooms": rooms,
	})
}
