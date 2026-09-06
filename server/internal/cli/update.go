package cli

import (
	"fmt"
	"strings"

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
		self         bool
		skipBinary   bool
		skipMigrate  bool
		skipRestart  bool
		skipChecksum bool
		nightly      bool
		yes          bool
	)

	cmd := &cobra.Command{
		Use:   use + " [SOURCE|--self]",
		Short: short,
		Long: `Update an existing Bedrud installation in place.

A SOURCE (or --self / --skip-binary / --nightly) is required. Running without
arguments prints this help and does not change the system.

SOURCE may be:
  BIN_PATH     Local bare binary (e.g. ./bedrud or /tmp/bedrud)
  ARCHIVE      Local .tar.xz / .tar.gz release archive containing bedrud
  latest       Download the latest *stable* GitHub release (HTTPS + SHA256)
  HTTPS_URL    Download a release asset or binary over HTTPS

"latest" never installs prereleases or nightlies. GitHub /releases/latest
is used, and the payload is rejected if it is marked prerelease.

GitHub "latest" / "--nightly" asks for confirmation unless you pass -y
(required when stdin is not a TTY).

  sudo bedrud update latest
  sudo bedrud update latest -y
  sudo bedrud update --nightly
  sudo bedrud update --nightly -y

Preserves configuration, secrets, certificates, and the database.
Runs versioned install migrations and database schema migrations, refreshes
init service units, restarts services, rewrites
/usr/share/doc/bedrud/examples/, and refreshes the man page plus
bash/zsh/fish shell completions under /usr/share.

Examples:
  sudo bedrud update /tmp/bedrud
  sudo bedrud update /tmp/bedrud_linux_amd64.tar.xz
  sudo bedrud update latest
  sudo bedrud update https://github.com/themadorg/bedrud/releases/download/v1.2.3/bedrud_linux_amd64.tar.xz
  sudo bedrud update --self
  sudo bedrud update --skip-binary   # package already replaced binary (apt/dnf)

update and upgrade are identical.

Package installs (apt/dnf): after the package manager replaces the binary,
run "sudo bedrud update --skip-binary" to apply migrations and restart.
`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("expected at most one source argument")
			}
			if len(args) == 0 && !self && !skipBinary && !nightly {
				_ = cmd.Help()
				return fmt.Errorf("missing source: path, URL, \"latest\", --nightly, or --self (or --skip-binary for migrations only)")
			}
			if self && len(args) > 0 {
				return fmt.Errorf("--self cannot be combined with a source argument")
			}
			if skipBinary && (self || len(args) > 0) {
				return fmt.Errorf("--skip-binary cannot be combined with a source or --self")
			}
			if nightly && (self || skipBinary) {
				return fmt.Errorf("--nightly cannot be combined with --self or --skip-binary")
			}
			if nightly && len(args) == 1 && !strings.EqualFold(args[0], "latest") {
				return fmt.Errorf("--nightly cannot be combined with a local path or URL")
			}
			if skipChecksum && ((len(args) == 1 && strings.EqualFold(args[0], "latest")) || nightly) {
				return fmt.Errorf("--skip-checksum is not allowed with \"latest\" or --nightly")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			source := ""
			if len(args) == 1 {
				source = args[0]
			}
			opts := install.UpdateOptions{
				Version:      Version,
				ConfigPath:   resolveConfigPath(defaultEtcConfig),
				Source:       source,
				Self:         self,
				SkipBinary:   skipBinary,
				SkipMigrate:  skipMigrate,
				SkipRestart:  skipRestart,
				SkipChecksum: skipChecksum,
				Nightly:      nightly,
				Yes:          yes,
			}
			if opts.ConfigPath == "" || opts.ConfigPath == defaultConfigPath {
				opts.ConfigPath = defaultEtcConfig
			}

			if err := install.LinuxUpdate(opts); err != nil {
				return fmt.Errorf("%s: %w", use, err)
			}
			return clioutput.Success("✓ Bedrud "+use+"d successfully", map[string]any{
				"version":      Version,
				"configPath":   opts.ConfigPath,
				"source":       source,
				"self":         self,
				"skipBinary":   skipBinary,
				"skipMigrate":  skipMigrate,
				"skipRestart":  skipRestart,
				"skipChecksum": skipChecksum,
				"nightly":      nightly,
				"yes":          yes,
			})
		},
	}

	f := cmd.Flags()
	f.BoolVar(&self, "self", false, "Install from this running executable")
	f.BoolVar(&skipBinary, "skip-binary", false, "Do not replace the installed binary (migrations + restart only)")
	f.BoolVar(&skipMigrate, "skip-migrate", false, "Skip database migrations")
	f.BoolVar(&skipRestart, "skip-restart", false, "Do not stop/start init services")
	f.BoolVar(&skipChecksum, "skip-checksum", false, "Skip SHA256 verification (local/trusted sources only; not with latest or --nightly)")
	f.BoolVar(&nightly, "nightly", false, "Install the latest GitHub prerelease/nightly (not stable)")
	f.BoolVarP(&yes, "yes", "y", false, "Do not prompt; confirm GitHub latest/nightly update")

	return cmd
}
