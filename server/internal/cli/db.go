package cli

import (
	"bedrud/config"
	"bedrud/internal/clioutput"
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
			defer func() { _ = database.Close() }()
			if err := database.RunMigrations(); err != nil {
				return fmt.Errorf("migrate: %w", err)
			}
			return clioutput.Success("✓ Database migrations applied", map[string]string{
				"databaseType": cfg.Database.Type,
			})
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
			defer func() { _ = database.Close() }()

			db := database.GetDB()
			sqlDB, err := db.DB()
			if err != nil {
				return fmt.Errorf("get sql.DB: %w", err)
			}
			if err := sqlDB.Ping(); err != nil {
				return fmt.Errorf("ping: %w", err)
			}
			data := map[string]string{
				"type":   cfg.Database.Type,
				"status": "ok",
			}
			if cfg.Database.Type == "sqlite" {
				data["path"] = cfg.Database.Path
				if !clioutput.JSON() {
					fmt.Printf("✓ Database OK (%s)\n", cfg.Database.Type)
					fmt.Printf("  Path: %s\n", cfg.Database.Path)
				}
			} else {
				data["host"] = cfg.Database.Host
				data["port"] = cfg.Database.Port
				data["dbname"] = cfg.Database.DBName
				if !clioutput.JSON() {
					fmt.Printf("✓ Database OK (%s)\n", cfg.Database.Type)
					fmt.Printf("  Host: %s:%s/%s\n", cfg.Database.Host, cfg.Database.Port, cfg.Database.DBName)
				}
			}
			return clioutput.Success("✓ Database OK", data)
		},
	}
}
