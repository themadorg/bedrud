package cli

import (
	"bedrud/config"
	"bedrud/internal/database"
	"fmt"

	"github.com/spf13/cobra"
)

func newDBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Database utilities",
	}
	cmd.AddCommand(newDBMigrateCmd(), newDBStatusCmd())
	return cmd
}

func newDBMigrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Run pending database migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(resolveConfigPath(defaultEtcConfig))
			if err != nil {
				return err
			}
			if err := database.Initialize(&cfg.Database); err != nil {
				return err
			}
			defer database.Close()
			if err := database.RunMigrations(); err != nil {
				return fmt.Errorf("migrate: %w", err)
			}
			fmt.Println("✓ Database migrations applied")
			return nil
		},
	}
}

func newDBStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check database connectivity",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(resolveConfigPath(defaultEtcConfig))
			if err != nil {
				return err
			}
			if err := database.Initialize(&cfg.Database); err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer database.Close()

			db := database.GetDB()
			sqlDB, err := db.DB()
			if err != nil {
				return fmt.Errorf("get sql.DB: %w", err)
			}
			if err := sqlDB.Ping(); err != nil {
				return fmt.Errorf("ping: %w", err)
			}
			fmt.Printf("✓ Database OK (%s)\n", cfg.Database.Type)
			if cfg.Database.Type == "sqlite" {
				fmt.Printf("  Path: %s\n", cfg.Database.Path)
			} else {
				fmt.Printf("  Host: %s:%s/%s\n", cfg.Database.Host, cfg.Database.Port, cfg.Database.DBName)
			}
			return nil
		},
	}
}
