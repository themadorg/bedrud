package repository

import (
	"bedrud/internal/models"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// hashToken returns the SHA-256 hex digest of a token string.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func (r *UserRepository) CreateOrUpdateUser(user *models.User) error {
	now := time.Now()
	user.UpdatedAt = now

	result := r.db.Where("email = ? AND provider = ?", user.Email, user.Provider).
		Assign(user).
		FirstOrCreate(user)

	if result.Error != nil {
		log.Error().Err(result.Error).Msg("Failed to create or update user")
		return result.Error
	}

	return nil
}

func (r *UserRepository) GetUserByEmailAndProvider(email, provider string) (*models.User, error) {
	var user models.User
	result := r.db.Where("email = ? AND provider = ?", email, provider).First(&user)

	if result.Error == gorm.ErrRecordNotFound {
		return nil, nil
	}

	if result.Error != nil {
		log.Error().Err(result.Error).Msg("Failed to get user")
		return nil, result.Error
	}

	return &user, nil
}

func (r *UserRepository) GetUserByEmail(email string) (*models.User, error) {
	var user models.User
	result := r.db.Where("email = ?", email).First(&user)

	if result.Error == gorm.ErrRecordNotFound {
		return nil, nil
	}

	if result.Error != nil {
		log.Error().Err(result.Error).Msg("Failed to get user by email")
		return nil, result.Error
	}

	return &user, nil
}

func (r *UserRepository) CreateUser(user *models.User) error {
	result := r.db.Create(user)
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("Failed to create user")
		return result.Error
	}
	return nil
}

// CreateUserWithPasskey creates a user and passkey in a single transaction.
// Prevents orphaned users if passkey creation fails.
func (r *UserRepository) CreateUserWithPasskey(user *models.User, passkey *models.Passkey) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		if err := tx.Create(passkey).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *UserRepository) UpdateRefreshToken(userID, refreshToken string) error {
	result := r.db.Model(&models.User{}).
		Where("id = ?", userID).
		Update("refresh_token", hashToken(refreshToken))

	if result.Error != nil {
		log.Error().Err(result.Error).Msg("Failed to update refresh token")
		return result.Error
	}
	return nil
}

// UpdateRefreshTokenAtomic atomically updates the refresh token only if the stored hash
// matches the old (raw) token's hash. Returns true if the update succeeded, false if
// another request already rotated the token. Prevents refresh token rotation races.
func (r *UserRepository) UpdateRefreshTokenAtomic(userID, oldRawToken, newRawToken string) (bool, error) {
	result := r.db.Model(&models.User{}).
		Where("id = ? AND refresh_token = ?", userID, hashToken(oldRawToken)).
		Update("refresh_token", hashToken(newRawToken))
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// MatchRefreshToken checks whether the provided raw token matches the stored hash for the user.
func (r *UserRepository) MatchRefreshToken(userID, rawToken string) (bool, error) {
	var stored string
	err := r.db.Model(&models.User{}).
		Where("id = ?", userID).
		Select("refresh_token").
		Row().
		Scan(&stored)
	if err != nil {
		return false, err
	}
	return stored == hashToken(rawToken), nil
}

func (r *UserRepository) GetUserByID(id string) (*models.User, error) {
	var user models.User
	result := r.db.Where("id = ?", id).First(&user)

	if result.Error == gorm.ErrRecordNotFound {
		return nil, nil
	}

	if result.Error != nil {
		log.Error().Err(result.Error).Msg("Failed to get user by ID")
		return nil, result.Error
	}

	return &user, nil
}

func (r *UserRepository) BlockRefreshToken(userID, token string, expiresAt time.Time) error {
	blocked := &models.BlockedRefreshToken{
		ID:        uuid.New().String(),
		Token:     hashToken(token),
		UserID:    userID,
		ExpiresAt: expiresAt,
	}

	result := r.db.Create(blocked)
	return result.Error
}

func (r *UserRepository) IsRefreshTokenBlocked(token string) (bool, error) {
	var count int64
	err := r.db.Model(&models.BlockedRefreshToken{}).
		Where("token = ? AND expires_at > ?", hashToken(token), time.Now()).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *UserRepository) CleanupBlockedTokens() error {
	result := r.db.Where("expires_at < ?", time.Now()).
		Delete(&models.BlockedRefreshToken{})
	return result.Error
}

func (r *UserRepository) UpdateUserAccesses(userID string, accesses []string) error {
	result := r.db.Model(&models.User{}).
		Where("id = ?", userID).
		Update("accesses", models.StringArray(accesses))

	return result.Error
}

// UpdateAccessesAndClearToken atomically updates accesses and clears the refresh token.
// Used when changing user roles to force re-login with correct accesses in the JWT.
func (r *UserRepository) UpdateAccessesAndClearToken(userID string, accesses []string) error {
	result := r.db.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]any{
		"accesses":      models.StringArray(accesses),
		"refresh_token": "",
		"updated_at":    time.Now(),
	})
	if result.Error != nil {
		log.Error().Err(result.Error).Str("userID", userID).Msg("Failed to update accesses and clear token")
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ClearRefreshToken removes the stored refresh token for a user, invalidating all sessions.
func (r *UserRepository) ClearRefreshToken(userID string) error {
	result := r.db.Model(&models.User{}).
		Where("id = ?", userID).
		Update("refresh_token", "")
	if result.Error != nil {
		log.Error().Err(result.Error).Str("userID", userID).Msg("Failed to clear refresh token")
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateUserStatusAndClearToken atomically updates is_active and clears the refresh token.
// Used when banning a user to immediately invalidate all sessions.
func (r *UserRepository) UpdateUserStatusAndClearToken(userID string, active bool) error {
	result := r.db.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]any{
		"is_active":     active,
		"refresh_token": "",
		"updated_at":    time.Now(),
	})
	if result.Error != nil {
		log.Error().Err(result.Error).Str("userID", userID).Msg("Failed to update status and clear token")
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ActivateUser sets is_active=true without clearing the refresh token.
// Used when unbanning a user — they keep their existing sessions.
func (r *UserRepository) ActivateUser(userID string) error {
	result := r.db.Model(&models.User{}).Where("id = ?", userID).
		Updates(map[string]interface{}{"is_active": true, "updated_at": time.Now()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *UserRepository) UpdatePassword(userID, hashedPassword string) error {
	now := time.Now()
	result := r.db.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]any{
		"password":          hashedPassword,
		"refresh_token":     "",
		"password_changed_at": now,
		"updated_at":        now,
	})
	if result.Error != nil {
		log.Error().Err(result.Error).Str("userID", userID).Msg("Failed to update password")
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *UserRepository) GetUsersByAccess(access models.AccessLevel) ([]models.User, error) {
	var users []models.User
	accessStr := string(access)
	if r.db.Dialector.Name() == "postgres" {
		// PostgreSQL: use ANY() for array contains
		err := r.db.Where("? = ANY(accesses)", accessStr).Find(&users).Error
		return users, err
	}
	// SQLite: accesses stored as {val1,val2} format — use LIKE patterns
	err := r.db.Where(
		"accesses LIKE ? OR accesses LIKE ? OR accesses LIKE ? OR accesses = ?",
		"%{"+accessStr+",%",
		"%,"+accessStr+",%",
		"%,"+accessStr+"}%",
		"{"+accessStr+"}",
	).Find(&users).Error
	return users, err
}

// UpdateUser updates an existing user
func (r *UserRepository) UpdateUser(user *models.User) error {
	user.UpdatedAt = time.Now()
	result := r.db.Save(user)
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("Failed to update user")
		return result.Error
	}
	return nil
}

// DeleteUser deletes a user by ID
func (r *UserRepository) DeleteUser(userID string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&models.Passkey{}, "user_id = ?", userID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&models.UserPreferences{}, "user_id = ?", userID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&models.RoomParticipant{}, "user_id = ?", userID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&models.RoomPermissions{}, "user_id = ?", userID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&models.BlockedRefreshToken{}, "user_id = ?", userID).Error; err != nil {
			return err
		}
		return tx.Delete(&models.User{}, "id = ?", userID).Error
	})
}

// DeleteUnverifiedUsers hard-deletes local/passkey users who registered but never verified their email,
// older than the specified cutoff. Uses existing DeleteUser cascade for related records.
func (r *UserRepository) DeleteUnverifiedUsers(cutoff time.Time) (int64, error) {
	var users []models.User
	if err := r.db.Where("email_verified_at IS NULL AND provider IN ? AND updated_at < ?",
		[]string{"local", "passkey"}, cutoff).
		Find(&users).Error; err != nil {
		return 0, err
	}

	count := int64(len(users))
	for _, u := range users {
		if err := r.DeleteUser(u.ID); err != nil {
			log.Warn().Err(err).Str("userID", u.ID).Msg("Failed to delete unverified user")
			count--
		}
	}
	return count, nil
}

// DeleteGuestUsers removes guest users older than the specified cutoff who have no active room participations.
func (r *UserRepository) DeleteGuestUsers(cutoff time.Time) (int64, error) {
	subQuery := r.db.Table("room_participants").
		Select("user_id").
		Where("is_active = ?", true)

	result := r.db.Where("provider = ? AND created_at < ? AND id NOT IN (?)", "guest", cutoff, subQuery).
		Delete(&models.User{})
	return result.RowsAffected, result.Error
}

// PaginationParams holds page and limit for paginated queries.
type PaginationParams struct {
	Page  int
	Limit int
}

// UserFilterParams holds all filtering, sorting, and pagination params for admin user listing.
type UserFilterParams struct {
	Page     int
	Limit    int
	Search   string   // q — name OR email LIKE search
	Provider []string // "local", "google", "github", "guest"
	Role     []string // "superadmin", "admin", "moderator", "user", "guest"
	Status   []string // "active", "banned"
	Verified *bool    // true = only verified, false = only unverified, nil = all
	Created  string   // "today", "7d", "30d"
	Sort     string   // "name", "email", "provider", "createdAt"
	Order    string   // "asc", "desc"
}

// GetAllUsersFiltered returns a filtered, sorted, paginated list of users and the total count.
func (r *UserRepository) GetAllUsersFiltered(p UserFilterParams) ([]models.User, int64, error) {
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 50
	}
	if p.Page <= 0 {
		p.Page = 1
	}
	offset := (p.Page - 1) * p.Limit
	if offset > 100000 {
		offset = 100000
	}

	query := r.db.Model(&models.User{})

	// Search
	if p.Search != "" {
		query = query.Where("name LIKE ? OR email LIKE ?", "%"+p.Search+"%", "%"+p.Search+"%")
	}

	// Provider filter
	if len(p.Provider) > 0 {
		query = query.Where("provider IN ?", p.Provider)
	}

	// Role filter (accesses stored as JSON array or {val1,val2} format)
	if len(p.Role) > 0 {
		if r.db.Dialector.Name() == "postgres" {
			conditions := make([]string, len(p.Role))
			args := make([]interface{}, len(p.Role))
			for i, role := range p.Role {
				conditions[i] = "? = ANY(accesses)"
				args[i] = role
			}
			query = query.Where(strings.Join(conditions, " OR "), args...)
		} else {
			// SQLite: accesses stored as {val1,val2} format
			conditions := make([]string, len(p.Role))
			args := make([]interface{}, len(p.Role)*3)
			argIndex := 0
			for i, role := range p.Role {
				conditions[i] = "(accesses LIKE ? OR accesses LIKE ? OR accesses LIKE ? OR accesses = ?)"
				args[argIndex] = "%{" + role + ",%"
				args[argIndex+1] = "%," + role + ",%"
				args[argIndex+2] = "%," + role + "}%"
				args[argIndex+3] = "{" + role + "}"
				argIndex += 4
			}
			query = query.Where(strings.Join(conditions, " OR "), args...)
		}
	}

	// Status
	if len(p.Status) > 0 {
		hasActive := contains(p.Status, "active")
		hasBanned := contains(p.Status, "banned")
		if hasActive && !hasBanned {
			query = query.Where("is_active = ?", true)
		} else if hasBanned && !hasActive {
			query = query.Where("is_active = ?", false)
		}
	}

	// Verified filter
	if p.Verified != nil {
		if *p.Verified {
			query = query.Where("email_verified_at IS NOT NULL")
		} else {
			query = query.Where("email_verified_at IS NULL")
		}
	}

	// Created date
	switch p.Created {
	case "today":
		query = query.Where("created_at >= ?", startOfDay(time.Now()))
	case "7d":
		query = query.Where("created_at >= ?", time.Now().AddDate(0, 0, -7))
	case "30d":
		query = query.Where("created_at >= ?", time.Now().AddDate(0, 0, -30))
	}

	// Count before sorting/pagination
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Sort
	orderClause := "created_at DESC"
	switch p.Sort {
	case "name":
		orderClause = "name " + p.Order
	case "email":
		orderClause = "email " + p.Order
	case "provider":
		orderClause = "provider " + p.Order
	case "createdAt":
		orderClause = "created_at " + p.Order
	}
	query = query.Order(orderClause)

	var users []models.User
	if err := query.Limit(p.Limit).Offset(offset).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// GetAllUsers returns a paginated list of users and the total count.
func (r *UserRepository) GetAllUsers(p PaginationParams) ([]models.User, int64, error) {
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 50
	}
	if p.Page <= 0 {
		p.Page = 1
	}
	offset := (p.Page - 1) * p.Limit
	if offset > 100000 {
		offset = 100000
	}
	var total int64
	var users []models.User
	if err := r.db.Model(&models.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := r.db.Limit(p.Limit).Offset(offset).Find(&users).Error
	return users, total, err
}

// GetUsersByIDs fetches multiple users by their IDs.
func (r *UserRepository) GetUsersByIDs(ids []string) ([]models.User, error) {
	var users []models.User
	err := r.db.Where("id IN ?", ids).Find(&users).Error
	return users, err
}

// GetInactiveUserIDs returns IDs of all deactivated users for populating the in-memory ban set on startup.
func (r *UserRepository) GetInactiveUserIDs() ([]string, error) {
	var ids []string
	err := r.db.Model(&models.User{}).Where("is_active = ?", false).Pluck("id", &ids).Error
	return ids, err
}

// batchChunk divides a slice into chunks of at most chunkSize.
func batchChunk[T any](items []T, chunkSize int) [][]T {
	if chunkSize <= 0 {
		chunkSize = 100
	}
	var chunks [][]T
	for i := 0; i < len(items); i += chunkSize {
		end := i + chunkSize
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[i:end])
	}
	return chunks
}

// BatchBan deactivates multiple users atomically and clears their refresh tokens.
// Returns per-ID errors. Non-existent IDs are silently skipped.
func (r *UserRepository) BatchBan(ids []string) map[string]error {
	errors := make(map[string]error)
	for _, chunk := range batchChunk(ids, 100) {
		result := r.db.Model(&models.User{}).
			Where("id IN ?", chunk).
			Where("is_active = ?", true).
			Updates(map[string]any{
				"is_active":     false,
				"refresh_token": "",
				"updated_at":    time.Now(),
			})
		if result.Error != nil {
			log.Error().Err(result.Error).Msg("BatchBan failed")
			for _, id := range chunk {
				errors[id] = result.Error
			}
		}
	}
	if len(errors) == 0 {
		return nil
	}
	return errors
}

// BatchPromote adds superadmin access to multiple users and clears their refresh tokens.
// Returns per-ID errors. Non-existent IDs are silently skipped.
func (r *UserRepository) BatchPromote(ids []string) map[string]error {
	errors := make(map[string]error)
	for _, chunk := range batchChunk(ids, 100) {
		var users []models.User
		if err := r.db.Where("id IN ?", chunk).Find(&users).Error; err != nil {
			log.Error().Err(err).Msg("BatchPromote: failed to fetch users")
			for _, id := range chunk {
				errors[id] = err
			}
			continue
		}
		for _, u := range users {
			if !slices.Contains(u.Accesses, "superadmin") {
				newAccesses := append(u.Accesses, "superadmin")
				if err := r.db.Model(&u).Updates(map[string]any{
					"accesses":      models.StringArray(newAccesses),
					"refresh_token": "",
					"updated_at":    time.Now(),
				}).Error; err != nil {
					errors[u.ID] = err
				}
			}
		}
	}
	if len(errors) == 0 {
		return nil
	}
	return errors
}

// CountUsers returns total number of users.
func (r *UserRepository) CountUsers() (int64, error) {
	var count int64
	err := r.db.Model(&models.User{}).Count(&count).Error
	return count, err
}

// CountUsersFiltered returns total user count excluding specified provider types.
// Pass provider constants like ProviderGuest to exclude guest accounts.
func (r *UserRepository) CountUsersFiltered(excludeProviders []string) (int64, error) {
	var count int64
	query := r.db.Model(&models.User{})
	for _, p := range excludeProviders {
		query = query.Where("provider != ?", p)
	}
	err := query.Count(&count).Error
	return count, err
}

// CountUsersSince returns users created since the given time.
func (r *UserRepository) CountUsersSince(t time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&models.User{}).Where("created_at >= ?", t).Count(&count).Error
	return count, err
}

// CountUsersSinceFiltered returns users created since the given time, excluding specified provider types.
func (r *UserRepository) CountUsersSinceFiltered(t time.Time, excludeProviders []string) (int64, error) {
	var count int64
	query := r.db.Model(&models.User{}).Where("created_at >= ?", t)
	for _, p := range excludeProviders {
		query = query.Where("provider != ?", p)
	}
	err := query.Count(&count).Error
	return count, err
}

// BatchDeleteSoft marks users as inactive and clears tokens. The actual cascade
// delete is handled by the caller (since it involves async LK room cleanup).
func (r *UserRepository) BatchDeleteSoft(ids []string) map[string]error {
	errors := make(map[string]error)
	for _, chunk := range batchChunk(ids, 100) {
		result := r.db.Model(&models.User{}).
			Where("id IN ?", chunk).
			Updates(map[string]any{
				"is_active":     false,
				"refresh_token": "",
				"updated_at":    time.Now(),
			})
		if result.Error != nil {
			log.Error().Err(result.Error).Msg("BatchDeleteSoft failed")
			for _, id := range chunk {
				errors[id] = result.Error
			}
		}
	}
	if len(errors) == 0 {
		return nil
	}
	return errors
}

// RecentSignupsFilterParams holds filtering/pagination params for recent signups.
type RecentSignupsFilterParams struct {
	Page         int
	Limit        int
	Search       string   // q — name OR email LIKE search
	Provider     []string // "local", "google", "github", "guest"
	ExcludeGuest bool     // exclude guest users (default false)
	DateFrom     string   // ISO date or relative ("today", "7d", "30d")
	DateTo       string
	Sort         string // "createdAt"
	Order        string // "asc", "desc"
}

// GetRecentSignupsFiltered returns paginated recent signups with optional filters.
func (r *UserRepository) GetRecentSignupsFiltered(p RecentSignupsFilterParams) ([]models.RecentUser, int64, error) {
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 50
	}
	if p.Page <= 0 {
		p.Page = 1
	}
	offset := (p.Page - 1) * p.Limit

	query := r.db.Model(&models.User{})

	// Search
	if p.Search != "" {
		query = query.Where("name LIKE ? OR email LIKE ?", "%"+p.Search+"%", "%"+p.Search+"%")
	}

	// Provider filter
	if len(p.Provider) > 0 {
		query = query.Where("provider IN ?", p.Provider)
	} else if p.ExcludeGuest {
		// Default: exclude guest users from sign-ups list
		query = query.Where("provider != ?", models.ProviderGuest)
	}

	// Date range
	if p.DateFrom != "" {
		if t, err := time.Parse("2006-01-02", p.DateFrom); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if p.DateTo != "" {
		if t, err := time.Parse("2006-01-02", p.DateTo); err == nil {
			query = query.Where("created_at <= ?", t.Add(24*time.Hour))
		}
	}

	// Sort
	sortField := "created_at"
	if p.Sort == "name" {
		sortField = "name"
	}
	order := "DESC"
	if p.Order == "asc" {
		order = "ASC"
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []models.User
	if err := query.Order(sortField+" "+order).Offset(offset).Limit(p.Limit).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	result := make([]models.RecentUser, 0, len(users))
	for _, u := range users {
		result = append(result, models.RecentUser{
			ID:        u.ID,
			Name:      u.Name,
			Email:     u.Email,
			Provider:  u.Provider,
			CreatedAt: u.CreatedAt.Format(time.RFC3339),
		})
	}
	return result, total, nil
}

// GetRecentUsers returns the most recently created users, limited by count.
func (r *UserRepository) GetRecentUsers(limit int) ([]models.User, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	var users []models.User
	err := r.db.Order("created_at DESC").Limit(limit).Find(&users).Error
	return users, err
}

// CountUsersByDay returns user signup counts grouped by day for the last N days.
func (r *UserRepository) CountUsersByDay(days int) ([]models.DayCount, error) {
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	type dateRow struct {
		Date  string
		Count int
	}
	var rows []dateRow
	err := r.db.Model(&models.User{}).
		Select("DATE(created_at) as date, COUNT(*) as count").
		Where("created_at >= ?", cutoff).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	results := make([]models.DayCount, len(rows))
	for i, r := range rows {
		t, _ := time.Parse("2006-01-02", r.Date)
		results[i] = models.DayCount{Date: t, Count: r.Count}
	}
	// Zero-fill gaps
	found := make(map[string]int)
	for _, r := range results {
		key := r.Date.Format("2006-01-02")
		found[key] = r.Count
	}
	var filled []models.DayCount
	for i := 0; i < days; i++ {
		day := cutoff.Add(time.Duration(i) * 24 * time.Hour)
		key := day.Format("2006-01-02")
		filled = append(filled, models.DayCount{
			Date:  day,
			Count: found[key],
		})
	}
	return filled, nil
}
