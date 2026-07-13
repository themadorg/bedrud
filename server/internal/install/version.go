package install

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/mod/semver"
)

// versionMigration upgrades install state when moving past a release.
// Run is invoked when previousVersion < Version and newVersion >= Version.
// Use semantic versions with a leading "v" (e.g. "v1.2.0").
type versionMigration struct {
	Version string
	Name    string
	Run     func() error
}

// versionMigrations is ordered oldest → newest. Add entries when a release
// needs offline install-state changes beyond GORM AutoMigrate (config tweaks,
// data directory moves, service unit shape changes already handled generically, etc.).
//
// DB schema is always migrated separately via database.RunMigrations.
var versionMigrations = []versionMigration{
	// Example:
	// {
	// 	Version: "v1.2.0",
	// 	Name:    "ensure-webxdc-storage-dir",
	// 	Run: func() error {
	// 		return os.MkdirAll("/var/lib/bedrud/webxdc", 0o755)
	// 	},
	// },
}

func readInstalledVersion() string {
	b, err := os.ReadFile(versionFilePath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func writeInstalledVersion(version string) error {
	if err := os.MkdirAll(varLibDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", varLibDir, err)
	}
	v := strings.TrimSpace(version)
	if v == "" {
		v = "dev"
	}
	if err := os.WriteFile(versionFilePath, []byte(v+"\n"), 0o644); err != nil {
		return fmt.Errorf("write version file: %w", err)
	}
	_ = runChown("bedrud:bedrud", versionFilePath)
	return nil
}

// normalizeVersion returns a semver-compatible tag (with leading "v") when possible.
// Non-semver labels ("dev", "unknown", empty) return "".
func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "dev" || v == "unknown" {
		return ""
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	if !semver.IsValid(v) {
		return ""
	}
	return v
}

// runVersionMigrations applies install-state migrations between previous and new versions.
// When previous is unknown/dev, all migrations with Version <= new are applied (safe/idempotent ops only).
func runVersionMigrations(previous, newVersion string) error {
	from := normalizeVersion(previous)
	to := normalizeVersion(newVersion)

	// No comparable target version (dev builds): skip versioned steps; DB migrate still runs.
	if to == "" {
		fmt.Println("➜ Skipping versioned install migrations (non-semver build:", newVersion+")")
		return nil
	}

	applied := 0
	for _, m := range versionMigrations {
		mv := normalizeVersion(m.Version)
		if mv == "" {
			continue
		}
		// Only migrations that land at or before the new binary version.
		if semver.Compare(mv, to) > 0 {
			continue
		}
		// Skip if we already passed this migration on a previous upgrade.
		if from != "" && semver.Compare(from, mv) >= 0 {
			continue
		}
		fmt.Printf("➜ Applying install migration %s (%s)...\n", m.Version, m.Name)
		if err := m.Run(); err != nil {
			return fmt.Errorf("migration %s (%s): %w", m.Version, m.Name, err)
		}
		applied++
	}
	if applied == 0 {
		fmt.Println("➜ No versioned install migrations needed")
	} else {
		fmt.Printf("➜ Applied %d versioned install migration(s)\n", applied)
	}
	return nil
}
