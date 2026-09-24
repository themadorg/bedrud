package install

import (
	"fmt"
	"net/url"
	"os"
	"runtime"
	"strings"
)

// UpdateCheck is what "bedrud update --check" found: where the install stands,
// what an update would move it to, and the command that would do it. Nothing on
// the system is touched.
type UpdateCheck struct {
	InstalledVersion string `json:"installedVersion"`
	TargetVersion    string `json:"targetVersion,omitempty"`
	TargetNote       string `json:"targetNote,omitempty"`
	VersionOrigin    string `json:"versionOrigin,omitempty"`
	Source           string `json:"source"`
	ConfigPath       string `json:"configPath"`
	Command          string `json:"command"`
	UpToDate         bool   `json:"upToDate"`
}

// LinuxUpdateCheck resolves what an update would install, without changing
// anything. It avoids downloading release assets wherever the version can be
// read from metadata alone ("latest", GitHub release URLs).
func LinuxUpdateCheck(opts UpdateOptions) (UpdateCheck, error) {
	if runtime.GOOS != "linux" {
		return UpdateCheck{}, fmt.Errorf("only linux is supported")
	}
	if err := validateUpdateOptions(opts); err != nil {
		return UpdateCheck{}, err
	}

	cfgPath := opts.ConfigPath
	if cfgPath == "" {
		cfgPath = etcConfigPath
	}
	if err := requireExistingInstall(cfgPath); err != nil {
		return UpdateCheck{}, err
	}

	installed := readInstalledVersion()
	if installed == "" {
		installed = unknownVersion
	}

	target, source, note, err := checkTargetVersion(opts)
	if err != nil {
		return UpdateCheck{}, err
	}

	return UpdateCheck{
		InstalledVersion: installed,
		TargetVersion:    target.Version,
		TargetNote:       note,
		VersionOrigin:    target.Origin,
		Source:           source,
		ConfigPath:       cfgPath,
		Command:          updateCommand(opts),
		UpToDate:         target.Version != "" && installed != unknownVersion && target.Version == installed,
	}, nil
}

// TextReport renders the check as the aligned block the update banner uses.
func (c UpdateCheck) TextReport() string {
	target := describeTargetVersion(c.InstalledVersion, c.TargetVersion)
	switch {
	case c.TargetVersion == "" && c.TargetNote != "":
		target = fmt.Sprintf("%s (%s)", unknownVersion, c.TargetNote)
	case c.UpToDate:
		// The closing line already says the install is current.
		target = c.TargetVersion
	}

	lines := []string{
		"➜ Bedrud update check",
		updateField("Installed version", c.InstalledVersion),
		updateField("Target version", target),
		updateField("Source", c.Source),
		updateField("Config", c.ConfigPath),
		"",
	}
	switch {
	case c.UpToDate:
		lines = append(lines, "  Already up to date — nothing changed. Re-apply anyway with: "+c.Command)
	case c.TargetVersion != "":
		lines = append(lines, "  Update available — nothing changed. Apply it with: "+c.Command)
	default:
		lines = append(lines, "  Nothing changed. Apply the update with: "+c.Command)
	}
	return strings.Join(lines, "\n")
}

// checkTargetVersion resolves the target version of an update as cheaply as the
// source allows, returning the target, a source description, and a note
// explaining an unknown version.
func checkTargetVersion(opts UpdateOptions) (updateTarget, string, string, error) {
	if opts.SkipBinary {
		// Same resolution as the apply path, so a check never reports a
		// different version than the update that follows it.
		return resolveTargetVersion(opts, resolvedSource{}),
			describeUpdateSource(opts, resolvedSource{}), "", nil
	}

	if opts.Self {
		self, _ := os.Executable()
		desc := "this executable (--self)"
		if self != "" {
			desc = fmt.Sprintf("this executable (--self): %s", self)
		}
		version := strings.TrimSpace(opts.Version)
		if version == "" && self != "" {
			version = probeBinaryVersion(self)
		}
		return updateTarget{Version: version, Origin: originSelf}, desc, "", nil
	}

	src := strings.TrimSpace(opts.Source)

	if strings.EqualFold(src, "latest") {
		rel, err := fetchLatestRelease()
		if err != nil {
			return updateTarget{}, "", "", err
		}
		tag := strings.TrimSpace(rel.TagName)
		return updateTarget{Version: tag, Origin: originReleaseTag},
			fmt.Sprintf("latest GitHub release %s", tag), "", nil
	}

	if strings.HasPrefix(src, "https://") || strings.HasPrefix(src, "http://") {
		u, err := url.Parse(src)
		if err != nil {
			return updateTarget{}, "", "", fmt.Errorf("invalid URL: %w", err)
		}
		desc := fmt.Sprintf("URL %s", src)
		if v := versionFromGitHubURL(u); v != "" {
			return updateTarget{Version: v, Origin: originReleaseTag}, desc, "", nil
		}
		return updateTarget{}, desc, "URL carries no release tag; run the update to read it from the asset", nil
	}

	// Local file or archive: no network involved, so resolve it and ask the
	// binary itself.
	resolved, err := resolveUpdateSource(opts)
	if err != nil {
		return updateTarget{}, "", "", err
	}
	if resolved.Cleanup != nil {
		defer resolved.Cleanup()
	}
	// Reading the version means running the binary. An update is about to
	// install and run it anyway, but a check promises to change nothing, so
	// it executes a local source only once a checksum has matched — or once
	// the operator has explicitly vouched for it with --skip-checksum.
	if !resolved.Verified && !opts.SkipChecksum {
		return updateTarget{}, resolved.Description,
			"no SHA256SUMS alongside the source, so it is not executed here; " +
				"add one, pass --skip-checksum, or run the update to read its version", nil
	}
	return resolveTargetVersion(opts, resolved), resolved.Description, "", nil
}

// updateCommand is the command that applies what the check reported.
func updateCommand(opts UpdateOptions) string {
	cmd := "sudo bedrud update"
	switch {
	case opts.SkipBinary:
		cmd += " --skip-binary"
	case opts.Self:
		cmd += " --self"
	case strings.TrimSpace(opts.Source) != "":
		cmd += " " + strings.TrimSpace(opts.Source)
	}
	if opts.ConfigPath != "" && opts.ConfigPath != etcConfigPath {
		cmd += " --config " + opts.ConfigPath
	}
	// Carry the rest of the flags through: the command has to be the update
	// the operator asked to check, not a differently-behaving one.
	if opts.SkipChecksum {
		cmd += " --skip-checksum"
	}
	if opts.SkipMigrate {
		cmd += " --skip-migrate"
	}
	if opts.SkipRestart {
		cmd += " --skip-restart"
	}
	return cmd
}

// requireExistingInstall fails with install guidance when no config is present.
func requireExistingInstall(cfgPath string) error {
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
	return nil
}
