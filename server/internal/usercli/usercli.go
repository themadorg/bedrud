package usercli

import (
	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/clioutput"
	"bedrud/internal/database"
	"bedrud/internal/lkutil"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/services"
	"bedrud/internal/storage"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

func PromoteUser(configPath, email, role string) error {
	return withUser(configPath, email, func(repo *repository.UserRepository, user *models.User) error {
		roleAccesses := roleAccessSlice(role)
		if err := repo.UpdateUserAccesses(user.ID, roleAccesses); err != nil {
			return fmt.Errorf("failed to update accesses: %w", err)
		}
		return clioutput.Success(
			fmt.Sprintf("✓ User %q role set to %s (accesses: %v).", email, role, roleAccesses),
			map[string]any{
				"email":    email,
				"role":     role,
				"accesses": roleAccesses,
			},
		)
	})
}

func DemoteUser(configPath, email, role string) error {
	return withUser(configPath, email, func(repo *repository.UserRepository, user *models.User) error {
		if role == "" {
			role = "superadmin"
		}
		filtered := user.Accesses[:0]
		for _, a := range user.Accesses {
			if a != role {
				filtered = append(filtered, a)
			}
		}
		if len(filtered) == len(user.Accesses) {
			msg := fmt.Sprintf("User %q does not have %q access.", email, role)
			return clioutput.Success(msg, map[string]any{
				"email":    email,
				"role":     role,
				"accesses": []string(user.Accesses),
				"changed":  false,
			})
		}
		hasUser := false
		for _, a := range filtered {
			if a == "user" {
				hasUser = true
				break
			}
		}
		if !hasUser && len(filtered) == 0 {
			filtered = append(filtered, "user")
		}
		if err := repo.UpdateUserAccesses(user.ID, []string(filtered)); err != nil {
			return fmt.Errorf("failed to update accesses: %w", err)
		}
		return clioutput.Success(
			fmt.Sprintf("✓ Removed %q from %q. Current accesses: %v", role, email, filtered),
			map[string]any{
				"email":       email,
				"removedRole": role,
				"accesses":    filtered,
				"changed":     true,
			},
		)
	})
}

func roleAccessSlice(role string) []string {
	switch role {
	case "superadmin":
		return []string{"superadmin", "user"}
	case "admin":
		return []string{"admin", "user"}
	case "moderator":
		return []string{"moderator", "user"}
	case "user":
		return []string{"user"}
	case "guest":
		return []string{"guest"}
	default:
		return []string{"user"}
	}
}

func CreateUser(configPath, email, password, name string, admin bool) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if err := database.Initialize(&cfg.Database); err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	if err := database.RunMigrations(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	hashedPassword, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	repo := repository.NewUserRepository(database.GetDB())
	existing, checkErr := repo.GetUserByEmail(email)
	if checkErr != nil {
		return fmt.Errorf("failed to check existing user: %w", checkErr)
	}
	if existing != nil {
		return clioutput.Success(
			fmt.Sprintf("⚠ User %q already exists — skipping creation.", email),
			map[string]any{"email": email, "created": false, "user": userSummary(existing)},
		)
	}

	accesses := models.StringArray{"user"}
	if admin {
		accesses = roleAccessSlice("superadmin")
	}
	user := &models.User{
		ID:        uuid.New().String(),
		Email:     email,
		Password:  hashedPassword,
		Name:      name,
		Provider:  "local",
		Accesses:  accesses,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := repo.CreateUser(user); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return clioutput.Success(
		fmt.Sprintf("✓ Created user: %s", user.Email),
		map[string]any{"created": true, "user": userSummary(user)},
	)
}

func DeleteUser(configPath, email string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if err := database.Initialize(&cfg.Database); err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	if err := database.RunMigrations(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	userRepo := repository.NewUserRepository(database.GetDB())
	user, err := userRepo.GetUserByEmail(email)
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}
	if user == nil {
		return fmt.Errorf("user not found: %s", email)
	}

	roomRepo := repository.NewRoomRepository(database.GetDB())
	rooms, err := roomRepo.GetRoomsCreatedByUser(user.ID)
	if err != nil {
		return fmt.Errorf("failed to fetch user rooms: %w", err)
	}

	if len(rooms) > 0 {
		client := lkutil.NewClient(&cfg.LiveKit)
		uploadDir := cfg.Chat.Uploads.DiskDir
		if uploadDir == "" {
			uploadDir = "./data/uploads/chat"
		}
		var s3Deleter storage.ObjectDeleter
		if cfg.Chat.Uploads.Backend == "s3" &&
			cfg.Chat.Uploads.S3.Endpoint != "" &&
			cfg.Chat.Uploads.S3.Bucket != "" &&
			cfg.Chat.Uploads.S3.AccessKey != "" {
			s3Deleter = storage.NewS3Deleter(cfg.Chat.Uploads.S3)
		}
		uploadTracker := storage.NewChatUploadTracker(database.GetDB(), uploadDir, s3Deleter)
		cleanupSvc := services.NewRoomCleanupService(roomRepo, nil, client, nil, cfg.LiveKit.APIKey, cfg.LiveKit.APISecret, uploadTracker)

		if err := cleanupSvc.DeleteUserRooms(context.Background(), rooms, user.ID); err != nil {
			clioutput.Printf("⚠ Room cleanup had errors (proceeding with user deletion): %v\n", err)
			log.Warn().Err(err).Msg("room cleanup had errors during CLI user deletion")
		}
	}

	passkeyRepo := repository.NewPasskeyRepository(database.GetDB())
	prefsRepo := repository.NewUserPreferencesRepository(database.GetDB())

	if err := passkeyRepo.DeleteByUserID(user.ID); err != nil {
		return fmt.Errorf("failed to delete passkeys: %w", err)
	}
	if err := prefsRepo.DeleteByUserID(user.ID); err != nil {
		return fmt.Errorf("failed to delete preferences: %w", err)
	}
	if err := userRepo.DeleteUser(user.ID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return clioutput.Success(
		fmt.Sprintf("✓ Deleted user: %s (%d room(s) cleaned up)", user.Email, len(rooms)),
		map[string]any{
			"email":        user.Email,
			"userId":       user.ID,
			"roomsCleaned": len(rooms),
		},
	)
}

func ListUsers(configPath string, page, pageSize int) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if err := database.Initialize(&cfg.Database); err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()
	if err := database.RunMigrations(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	repo := repository.NewUserRepository(database.GetDB())
	users, total, err := repo.GetAllUsers(repository.PaginationParams{Page: page, Limit: pageSize})
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	if clioutput.JSON() {
		summaries := make([]map[string]any, 0, len(users))
		for i := range users {
			summaries = append(summaries, userSummary(&users[i]))
		}
		return clioutput.Success("", map[string]any{
			"users":    summaries,
			"page":     page,
			"pageSize": pageSize,
			"total":    total,
		})
	}

	clioutput.Printf("%-36s  %-32s  %-20s  %-10s  %-9s  %s\n", "ID", "EMAIL", "NAME", "PROVIDER", "ACTIVE", "ACCESSES")
	for _, u := range users {
		accesses := strings.Join([]string(u.Accesses), ",")
		active := "no"
		if u.IsActive {
			active = "yes"
		}
		clioutput.Printf("%-36s  %-32s  %-20s  %-10s  %-9s  %s\n",
			u.ID, truncate(u.Email, 32), truncate(u.Name, 20), u.Provider, active, accesses)
	}
	clioutput.Printf("\nshowing page %d (%d per page) of %d total user(s)\n", page, pageSize, total)
	return nil
}

func ShowUser(configPath, email string) error {
	return withUser(configPath, email, func(repo *repository.UserRepository, user *models.User) error {
		if clioutput.JSON() {
			return clioutput.Success("", map[string]any{"user": userDetail(user)})
		}
		accesses := append([]string(nil), []string(user.Accesses)...)
		sort.Strings(accesses)
		clioutput.Println("User:")
		clioutput.Printf("  ID:        %s\n", user.ID)
		clioutput.Printf("  Email:     %s\n", user.Email)
		clioutput.Printf("  Name:      %s\n", user.Name)
		clioutput.Printf("  Provider:  %s\n", user.Provider)
		clioutput.Printf("  Avatar:    %s\n", user.AvatarURL)
		clioutput.Printf("  Active:    %t\n", user.IsActive)
		clioutput.Printf("  Accesses:  %s\n", strings.Join(accesses, ", "))
		clioutput.Printf("  Created:   %s\n", user.CreatedAt.Format(time.RFC3339))
		clioutput.Printf("  Updated:   %s\n", user.UpdatedAt.Format(time.RFC3339))
		return nil
	})
}

func SetUserPassword(configPath, email, newPassword string) error {
	return withUser(configPath, email, func(repo *repository.UserRepository, user *models.User) error {
		generated := false
		if newPassword == "" {
			gen, err := generatePassword(20)
			if err != nil {
				return fmt.Errorf("generate password: %w", err)
			}
			newPassword = gen
			generated = true
		}
		hashed, err := auth.HashPassword(newPassword)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}
		if err := repo.UpdatePassword(user.ID, hashed); err != nil {
			return fmt.Errorf("update password: %w", err)
		}
		if err := repo.ClearRefreshToken(user.ID); err != nil {
			return fmt.Errorf("invalidate sessions: %w", err)
		}
		data := map[string]any{
			"email":     email,
			"generated": generated,
		}
		if generated {
			data["password"] = newPassword
		}
		return clioutput.Success(fmt.Sprintf("✓ Password updated for %s", email), data)
	})
}

func SetUserActive(configPath, email string, active bool) error {
	return withUser(configPath, email, func(repo *repository.UserRepository, user *models.User) error {
		if user.IsActive == active {
			return clioutput.Success(
				fmt.Sprintf("User %q already %s.", email, activeWord(active)),
				map[string]any{"email": email, "active": active, "changed": false},
			)
		}
		if err := repo.UpdateUserStatusAndClearToken(user.ID, active); err != nil {
			return fmt.Errorf("update status: %w", err)
		}
		return clioutput.Success(
			fmt.Sprintf("✓ User %q is now %s.", email, activeWord(active)),
			map[string]any{"email": email, "active": active, "changed": true},
		)
	})
}

func userSummary(u *models.User) map[string]any {
	if u == nil {
		return nil
	}
	return map[string]any{
		"id":       u.ID,
		"email":    u.Email,
		"name":     u.Name,
		"provider": u.Provider,
		"active":   u.IsActive,
		"accesses": []string(u.Accesses),
	}
}

func userDetail(u *models.User) map[string]any {
	if u == nil {
		return nil
	}
	accesses := append([]string(nil), []string(u.Accesses)...)
	sort.Strings(accesses)
	return map[string]any{
		"id":        u.ID,
		"email":     u.Email,
		"name":      u.Name,
		"provider":  u.Provider,
		"avatarUrl": u.AvatarURL,
		"active":    u.IsActive,
		"accesses":  accesses,
		"createdAt": u.CreatedAt.Format(time.RFC3339),
		"updatedAt": u.UpdatedAt.Format(time.RFC3339),
	}
}

func activeWord(active bool) string {
	if active {
		return "active"
	}
	return "disabled"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n < 1 {
		return ""
	}
	return s[:n-1] + "…"
}

func generatePassword(n int) (string, error) {
	if n < 8 {
		n = 8
	}
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw)[:n], nil
}

func withUser(configPath, email string, fn func(*repository.UserRepository, *models.User) error) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if err := database.Initialize(&cfg.Database); err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	if err := database.RunMigrations(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	repo := repository.NewUserRepository(database.GetDB())
	user, err := repo.GetUserByEmail(email)
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}
	if user == nil {
		return fmt.Errorf("no user found with email %q", email)
	}
	return fn(repo, user)
}
