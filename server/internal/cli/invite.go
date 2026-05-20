package cli

import (
	"bedrud/config"
	"bedrud/internal/database"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func newInviteTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "invite-token",
		Aliases: []string{"invite"},
		Short:   "Manage registration invite tokens",
	}
	cmd.AddCommand(newInviteListCmd(), newInviteCreateCmd(), newInviteDeleteCmd())
	return cmd
}

func newInviteListCmd() *cobra.Command {
	var page, pageSize int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List invite tokens",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withInviteRepo(func(repo *repository.InviteTokenRepository) error {
				if page < 1 {
					page = 1
				}
				if pageSize <= 0 {
					pageSize = 50
				}
				tokens, total, err := repo.List(repository.PaginationParams{Page: page, Limit: pageSize})
				if err != nil {
					return err
				}
				fmt.Printf("%-36s  %-64s  %-32s  %-25s  %-25s\n", "ID", "TOKEN", "EMAIL", "EXPIRES_AT", "USED_AT")
				for _, t := range tokens {
					used := "-"
					if t.UsedAt != nil {
						used = t.UsedAt.Format(time.RFC3339)
					}
					fmt.Printf("%-36s  %-64s  %-32s  %-25s  %-25s\n",
						t.ID, t.Token, t.Email, t.ExpiresAt.Format(time.RFC3339), used)
				}
				fmt.Printf("\n%d total invite token(s)\n", total)
				return nil
			})
		},
	}
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&pageSize, "page-size", 50, "Tokens per page")
	return cmd
}

func newInviteCreateCmd() *cobra.Command {
	var email, createdBy string
	var ttlHours int
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new invite token",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withInviteRepo(func(repo *repository.InviteTokenRepository) error {
				tok, err := generateInviteToken()
				if err != nil {
					return err
				}
				if ttlHours <= 0 {
					ttlHours = 168 // 7 days
				}
				if createdBy == "" {
					createdBy = "cli"
				}
				it := &models.InviteToken{
					ID:        uuid.NewString(),
					Token:     tok,
					Email:     email,
					CreatedBy: createdBy,
					ExpiresAt: time.Now().Add(time.Duration(ttlHours) * time.Hour),
					CreatedAt: time.Now(),
				}
				if err := repo.Create(it); err != nil {
					return err
				}
				fmt.Println("✓ Created invite token:")
				fmt.Printf("  ID:        %s\n", it.ID)
				fmt.Printf("  Token:     %s\n", it.Token)
				fmt.Printf("  Email:     %s\n", it.Email)
				fmt.Printf("  ExpiresAt: %s\n", it.ExpiresAt.Format(time.RFC3339))
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "Bind token to a specific email (optional)")
	cmd.Flags().StringVar(&createdBy, "created-by", "", "User ID to attribute as creator (default: cli)")
	cmd.Flags().IntVar(&ttlHours, "ttl-hours", 168, "Lifetime in hours (default 168 = 7 days)")
	return cmd
}

func newInviteDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an invite token by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withInviteRepo(func(repo *repository.InviteTokenRepository) error {
				if err := repo.Delete(args[0]); err != nil {
					return err
				}
				fmt.Printf("✓ Deleted invite token %s\n", args[0])
				return nil
			})
		},
	}
	return cmd
}

func withInviteRepo(fn func(*repository.InviteTokenRepository) error) error {
	cfg, err := config.Load(resolveConfigPath(defaultEtcConfig))
	if err != nil {
		return err
	}
	if err := database.Initialize(&cfg.Database); err != nil {
		return err
	}
	defer database.Close()
	if err := database.RunMigrations(); err != nil {
		return err
	}
	return fn(repository.NewInviteTokenRepository(database.GetDB()))
}

func generateInviteToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
