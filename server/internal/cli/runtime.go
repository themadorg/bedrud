package cli

import (
	"bedrud/config"
	"bedrud/internal/database"
	"fmt"
)

// withDB loads config (env+yaml), opens the database, runs migrations, then
// invokes fn. The DB is closed when fn returns. Use for any management command
// that needs repository access.
func withDB(fn func(cfg *config.Config) error) error {
	cfg, err := config.Load(resolveConfigPath(defaultEtcConfig))
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := database.Initialize(&cfg.Database); err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	if err := database.RunMigrations(); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return fn(cfg)
}
