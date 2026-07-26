package install

import (
	"fmt"
	"os"
	"runtime"

	"bedrud/config"
	"bedrud/internal/database"

	"gopkg.in/yaml.v3"
)

// yamlUnmarshal is a thin alias so services.go / update.go share one import path.
func yamlUnmarshal(data []byte, v any) error {
	return yaml.Unmarshal(data, v)
}

// UpdateOptions controls LinuxUpdate behaviour.
type UpdateOptions struct {
	// Version is the new binary version (injected via ldflags / CLI).
	Version string
	// ConfigPath overrides the default /etc/bedrud/config.yaml.
	ConfigPath string
	// Source is a local binary path, archive path, HTTPS URL, or "latest".
	// Empty when Self or SkipBinary is set.
	Source string
	// Self installs from the currently running executable (--self).
	Self bool
	// SkipBinary skips replacing the installed binary (migrations + restart only).
	SkipBinary bool
	// SkipMigrate skips database AutoMigrate.
	SkipMigrate bool
	// SkipRestart skips stopping/starting init services.
	SkipRestart bool
	// SkipChecksum allows local operator-provided files without SHA256SUMS.
	// Never used for "latest" (always verified).
	SkipChecksum bool
}

// LinuxUpdate upgrades an existing Bedrud installation in place:
//  1. Verify prior install
//  2. Resolve binary source (BIN_PATH, archive, URL, latest, or --self)
//  3. Stop services
//  4. Replace binary (unless SkipBinary)
//  5. Run versioned install migrations + database migrations
//  6. Refresh service units and restart; write doc examples
//  7. Record installed version
//
// Config, secrets, database, and certificates are preserved.
func LinuxUpdate(opts UpdateOptions) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("only linux is supported")
	}

	if err := validateUpdateOptions(opts); err != nil {
		return err
	}

	cfgPath := opts.ConfigPath
	if cfgPath == "" {
		cfgPath = etcConfigPath
	}

	if _, err := os.Stat(cfgPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(
				"no existing installation found at %s\n\n"+
					"Run a full install first:\n"+
					"  sudo bedrud install\n"+
					"Or specify the config path:\n"+
					"  sudo bedrud update --config /path/to/config.yaml",
				cfgPath,
			)
		}
		return fmt.Errorf("stat config: %w", err)
	}

	newVersion := opts.Version
	if newVersion == "" {
		newVersion = "dev"
	}
	previousVersion := readInstalledVersion()
	if previousVersion == "" {
		previousVersion = "unknown"
	}

	fmt.Println("➜ Bedrud update")
	fmt.Println("  Previous version:", previousVersion)
	fmt.Println("  New version:     ", newVersion)
	fmt.Println("  Config:          ", cfgPath)

	// Resolve binary source before stopping services so network/checksum
	// failures leave the running install untouched.
	var (
		srcBinary string
		cleanup   func()
		srcMeta   resolvedSource
	)
	if !opts.SkipBinary {
		var err error
		srcMeta, err = resolveUpdateSource(opts)
		if err != nil {
			return err
		}
		srcBinary = srcMeta.BinaryPath
		cleanup = srcMeta.Cleanup
		if cleanup != nil {
			defer cleanup()
		}
		if srcMeta.Version != "" {
			newVersion = srcMeta.Version
			fmt.Println("  Source version: ", newVersion)
		}
		fmt.Println("  Source:         ", srcMeta.Description)
	}

	// Ensure runtime layout still exists (partial upgrades / moved data).
	for _, dir := range []string{etcDir, varLibDir, varLibDir + "/certs", varLogDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	if err := createBedrudUser(); err != nil {
		fmt.Printf("⚠ Warning: could not ensure 'bedrud' user: %v\n", err)
	}

	targetBin := resolveInstalledBinary()
	packageManaged := isPackageManaged(targetBin)

	// Stop services before replacing binary (ETXTBSY + clean restart).
	if !opts.SkipRestart {
		fmt.Println("➜ Stopping services...")
		stopAllInitSystems([]string{"bedrud", "livekit"})
	}

	// Replace binary
	binaryUpdated := false
	if opts.SkipBinary {
		fmt.Println("➜ Skipping binary replacement (--skip-binary)")
	} else {
		installTarget := targetBin
		if packageManaged {
			// Package managers own /usr/bin/bedrud — install to /usr/local/bin instead.
			fmt.Printf("➜ Package-managed binary at %s — installing to %s\n", targetBin, binaryLocalPath)
			installTarget = binaryLocalPath
			fmt.Println("  Note: ensure PATH prefers /usr/local/bin over /usr/bin, or update service ExecStart.")
		}
		fmt.Println("➜ Replacing binary at", installTarget)
		if err := installBinaryFile(installTarget, srcBinary); err != nil {
			return err
		}
		targetBin = installTarget
		binaryUpdated = true
		fmt.Println("➜ Binary updated:", targetBin)
	}

	// Ownership for data dirs (config stays root/bedrud 0600).
	for _, dir := range []string{etcDir, varLibDir, varLogDir} {
		if err := runChownR("bedrud:bedrud", dir); err != nil {
			fmt.Printf("⚠ Warning: chown %s: %v\n", dir, err)
		}
	}

	// Versioned install-state migrations (config/data layout).
	if err := runVersionMigrations(previousVersion, newVersion); err != nil {
		return err
	}

	// Database migrations
	if opts.SkipMigrate {
		fmt.Println("➜ Skipping database migrations (--skip-migrate)")
	} else {
		fmt.Println("➜ Running database migrations...")
		if err := runDBMigrations(cfgPath); err != nil {
			return fmt.Errorf("database migrations: %w", err)
		}
		fmt.Println("➜ Database migrations complete")
	}

	// Refresh units so ExecStart points at the correct binary, then start.
	if !opts.SkipRestart {
		isExternal, err := isExternalLiveKitFromConfig(cfgPath)
		if err != nil {
			fmt.Printf("⚠ Warning: could not read LiveKit topology from config: %v — assuming embedded\n", err)
			isExternal = false
		}
		if err := refreshServices(targetBin, isExternal); err != nil {
			return err
		}
	} else {
		fmt.Println("➜ Skipping service restart (--skip-restart)")
	}

	if err := installDocExamples(); err != nil {
		fmt.Printf("⚠ Warning: could not write doc examples: %v\n", err)
	} else {
		fmt.Println("➜ Doc examples:", docExamplesDir)
	}

	// Refresh man page + shell completions so tab-completion matches this binary.
	if err := installCLIDocs(); err != nil {
		fmt.Printf("⚠ Warning: could not refresh man page / shell completions: %v\n", err)
	} else {
		fmt.Println("➜ Man page and shell completions updated (bash, zsh, fish)")
	}

	if err := writeInstalledVersion(newVersion); err != nil {
		fmt.Printf("⚠ Warning: could not write version file: %v\n", err)
	}

	fmt.Println("\n✓ Update complete!")
	fmt.Println("--------------------------------------------------")
	fmt.Printf("  Version:  %s → %s\n", previousVersion, newVersion)
	fmt.Printf("  Binary:   %s", targetBin)
	if binaryUpdated {
		fmt.Print(" (replaced)")
	}
	fmt.Println()
	fmt.Println("  Config:   preserved")
	fmt.Println("  Database: migrated")
	fmt.Println("  Examples: ", docExamplesDir)
	fmt.Println("  Docs:     man page + bash/zsh/fish completions refreshed")
	fmt.Println("--------------------------------------------------")
	fmt.Println("  Status:   systemctl status bedrud livekit")
	fmt.Println("  Logs:     journalctl -u bedrud -f")
	return nil
}

func validateUpdateOptions(opts UpdateOptions) error {
	if opts.SkipBinary {
		if opts.Source != "" || opts.Self {
			return fmt.Errorf("--skip-binary cannot be combined with a source or --self")
		}
		return nil
	}
	if opts.Self && opts.Source != "" {
		return fmt.Errorf("--self cannot be combined with a source argument")
	}
	if !opts.Self && opts.Source == "" {
		return fmt.Errorf("missing source: path, URL, \"latest\", or --self (or use --skip-binary for migrations only)")
	}
	return nil
}

func runDBMigrations(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	if err := database.Initialize(&cfg.Database); err != nil {
		return err
	}
	defer func() { _ = database.Close() }()
	return database.RunMigrations()
}
