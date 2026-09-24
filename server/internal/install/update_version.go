package install

import (
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

// updateFieldWidth aligns the labels of the update/check summary block.
const updateFieldWidth = 18

// updateField renders one aligned "  Label: value" line of that block.
func updateField(label, value string) string {
	return fmt.Sprintf("  %-*s %s", updateFieldWidth, label+":", value)
}

// updateTarget is the version an update moves the install to, plus where that
// number came from. Version is empty when no source could name it.
type updateTarget struct {
	Version string
	Origin  string
}

// Origins reported by resolveTargetVersion.
const (
	originReleaseTag      = "release tag"
	originSelf            = "this executable"
	originSourceBinary    = "source binary"
	originInstalledBinary = "installed binary"
)

// resolveTargetVersion decides which version an update installs, in order of
// authority: the release tag of the resolved source, the binary already on
// disk for --skip-binary, the running executable when it is itself the source
// (--self), then the resolved binary's own answer. It never falls back to the
// running binary's version for a foreign source — that is what made "New
// version" echo the installed one.
func resolveTargetVersion(opts UpdateOptions, src resolvedSource) updateTarget {
	if v := strings.TrimSpace(src.Version); v != "" {
		return updateTarget{Version: v, Origin: originReleaseTag}
	}
	// --skip-binary installs nothing, so the version in play is whatever is
	// already installed — not necessarily the binary running this command,
	// since PATH can resolve to an older one than the package-managed install.
	if opts.SkipBinary {
		if v := probeBinaryVersion(resolveInstalledBinary()); v != "" {
			return updateTarget{Version: v, Origin: originInstalledBinary}
		}
	}
	if opts.Self || opts.SkipBinary {
		if v := strings.TrimSpace(opts.Version); v != "" {
			return updateTarget{Version: v, Origin: originSelf}
		}
	}
	if src.BinaryPath != "" {
		if v := probeBinaryVersion(src.BinaryPath); v != "" {
			return updateTarget{Version: v, Origin: originSourceBinary}
		}
	}
	return updateTarget{}
}

// describeTargetVersion spells out what the target version means for this
// install, so equal versions and downgrades do not look like a mistake.
func describeTargetVersion(installed, target string) string {
	if target == "" {
		return unknownVersion + " (source carries no version information)"
	}
	if installed != "" && installed != unknownVersion && installed == target {
		return target + " (no version change)"
	}
	from, to := normalizeVersion(installed), normalizeVersion(target)
	if from != "" && to != "" && semver.Compare(to, from) < 0 {
		return fmt.Sprintf("%s (downgrade from %s)", target, installed)
	}
	return target
}

// describeUpdateSource names where the new binary comes from, including the
// case where no binary is installed at all.
func describeUpdateSource(opts UpdateOptions, src resolvedSource) string {
	if opts.SkipBinary {
		return "none — keeping the installed binary (--skip-binary)"
	}
	return src.Description
}
