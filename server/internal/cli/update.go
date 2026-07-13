package cli

import (
	"fmt"

	"bedrud/internal/clioutput"
	"bedrud/internal/install"

	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	return newUpdateLikeCmd("update", "Update Bedrud in place (binary, migrations, restart)")
}

func newUpgradeCmd() *cobra.Command {
	return newUpdateLikeCmd("upgrade", "Alias for update — upgrade Bedrud in place")
}

func newUpdateLikeCmd(use, short string) *cobra.Command {
	var (
		skipBinary  bool
		skipMigrate bool
		skipRestart bool
	)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long: `Update an existing Bedrud installation in place.

Preserves configuration, secrets, certificates, and the database.
Replaces the installed binary with this executable, runs any versioned
install migrations and database schema migrations, refreshes init service
units, and restarts services.

update and upgrade are identical.

Typical workflow after downloading a new release:

  curl -fsSL https://bedrud.org/install.sh | bash -s -- --no-setup
  sudo bedrud update

Or run the new binary directly:

  sudo ./bedrud-linux-amd64 update

Package installs (apt/dnf): after the package manager replaces the binary,
run "sudo bedrud update" to apply migrations and restart. The package
postinst may also restart services automatically.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := install.UpdateOptions{
				Version:     Version,
				ConfigPath:  resolveConfigPath(defaultEtcConfig),
				SkipBinary:  skipBinary,
				SkipMigrate: skipMigrate,
				SkipRestart: skipRestart,
			}
			// Only use defaultEtcConfig when --config was not set and env empty —
			// resolveConfigPath already handles that. If user passes a non-etc path
			// via --config, honor it. When config path is still the empty-resolved
			// default "config.yaml" for some contexts, prefer /etc for update.
			if opts.ConfigPath == "" || opts.ConfigPath == defaultConfigPath {
				opts.ConfigPath = defaultEtcConfig
			}

			if err := install.LinuxUpdate(opts); err != nil {
				return fmt.Errorf("%s: %w", use, err)
			}
			return clioutput.Success("✓ Bedrud "+use+"d successfully", map[string]any{
				"version":     Version,
				"configPath":  opts.ConfigPath,
				"skipBinary":  skipBinary,
				"skipMigrate": skipMigrate,
				"skipRestart": skipRestart,
			})
		},
	}

	f := cmd.Flags()
	f.BoolVar(&skipBinary, "skip-binary", false, "Do not replace the installed binary (migrations + restart only)")
	f.BoolVar(&skipMigrate, "skip-migrate", false, "Skip database migrations")
	f.BoolVar(&skipRestart, "skip-restart", false, "Do not stop/start init services")

	return cmd
}
