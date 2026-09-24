package install

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

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

// InstalledVersion returns the version recorded for the current install, or
// "" when the install has none. It is the value the last update wrote, not the
// version of the binary running this call.
func InstalledVersion() string {
	return readInstalledVersion()
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

// versionProbeTimeout bounds a version probe so a hung binary cannot stall an update.
const versionProbeTimeout = 15 * time.Second

// unknownVersion labels an install whose version could not be determined.
const unknownVersion = "unknown"

// probeBinaryVersion asks a bedrud binary for its own compiled-in version.
//
// This is the only source of truth for local files and archives, which carry no
// release tag. It returns "" when the binary cannot be executed (foreign
// architecture, noexec mount, a build without the subcommand) so callers fall
// back to whatever else they know. Only probe binaries that already passed
// checksum verification — this executes them.
//
// Overridable in tests.
var probeBinaryVersion = func(path string) string {
	if v := parseVersionJSON(runVersionProbe(path, "version", "--json")); v != "" {
		return v
	}
	return parseVersionText(runVersionProbe(path, "version"))
}

func runVersionProbe(path string, args ...string) []byte {
	ctx, cancel := context.WithTimeout(context.Background(), versionProbeTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, args...).Output()
	if err != nil {
		return nil
	}
	return out
}

// parseVersionJSON reads the clioutput envelope of "bedrud version --json".
func parseVersionJSON(out []byte) string {
	if len(out) == 0 {
		return ""
	}
	var payload struct {
		Data struct {
			Version string `json:"version"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Data.Version)
}

// parseVersionText reads the plain "bedrud <version>" line of "bedrud version".
func parseVersionText(out []byte) string {
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "bedrud" {
			return fields[1]
		}
	}
	return ""
}
