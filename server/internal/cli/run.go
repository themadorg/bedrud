package cli

import (
	"bedrud/internal/server"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var skipMigrate bool

	cmd := &cobra.Command{
		Use:     "run",
		Aliases: []string{"server"},
		Short:   "Start the meeting server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if skipMigrate {
				os.Setenv("BEDRUD_SKIP_MIGRATE", "1")
			}
			path := resolveConfigPath(defaultConfigPath)
			if err := server.Run(path, Version); err != nil {
				return fmt.Errorf("server: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&skipMigrate, "skip-migrate", false, "Skip database migrations on startup")
	return cmd
}
