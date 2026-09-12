package install

import (
	"fmt"

	"bedrud/internal/clioutput"
)

// VersionsList implements `bedrud versions list`.
func VersionsList(remote bool) error {
	root := installRoot()
	local, err := listInstalled(root)
	if err != nil {
		return err
	}
	if !remote {
		if clioutput.JSON() {
			return clioutput.Success("", map[string]any{
				"install_root": root,
				"versions":     local,
			})
		}
		clioutput.Println()
		clioutput.Printf("Install root: %s\n", root)
		if len(local) == 0 {
			clioutput.Println("  (no versions installed)")
		} else {
			clioutput.Printf("%-16s %-8s %s\n", "VERSION", "ACTIVE", "PATH")
			for _, v := range local {
				mark := ""
				if v.Active {
					mark = "*"
				}
				clioutput.Printf("%-16s %-8s %s\n", v.Version, mark, v.Binary)
			}
		}
		clioutput.Println()
		return nil
	}

	remoteList, err := fetchRemoteReleases()
	if err != nil {
		return fmt.Errorf("versions list --remote: %w", err)
	}
	merged := mergeLocalAndRemote(local, remoteList)
	if clioutput.JSON() {
		return clioutput.Success("", map[string]any{
			"install_root": root,
			"remote":       true,
			"versions":     merged,
		})
	}
	clioutput.Println()
	clioutput.Printf("Install root: %s\n", root)
	clioutput.Printf("%-16s %-8s %-8s %s\n", "VERSION", "SOURCE", "ACTIVE", "NOTES")
	for _, e := range merged {
		notes := []string{}
		if e.RemoteLatest {
			notes = append(notes, "remote latest")
		}
		if e.Installed {
			notes = append(notes, "installed")
		} else {
			notes = append(notes, "available")
		}
		if e.Source == "local" {
			notes = append(notes, "local-only")
		}
		mark := ""
		if e.Active {
			mark = "*"
		}
		clioutput.Printf("%-16s %-8s %-8s %s\n", e.Version, e.Source, mark, joinNotes(notes))
	}
	clioutput.Println()
	return nil
}

func joinNotes(notes []string) string {
	out := ""
	for i, n := range notes {
		if i > 0 {
			out += "; "
		}
		out += n
	}
	return out
}

// VersionsCurrent implements `bedrud versions current`.
func VersionsCurrent() error {
	root := installRoot()
	active, err := resolveActiveVersion(root)
	if err != nil {
		return err
	}
	var path string
	if active != "" {
		path = versionBinaryPath(root, active)
	}
	if clioutput.JSON() {
		var ver any
		if active != "" {
			ver = active
		} else {
			ver = nil
		}
		return clioutput.Success("", map[string]any{
			"install_root": root,
			"version":      ver,
			"path":         pathOrNil(path),
		})
	}
	if active == "" {
		clioutput.Println("No active version under the install root.")
		return nil
	}
	clioutput.Printf("Active version: %s\n", active)
	clioutput.Printf("Path: %s\n", path)
	return nil
}

func pathOrNil(p string) any {
	if p == "" {
		return nil
	}
	return p
}

// VersionsUse implements `bedrud versions use`.
func VersionsUse(version string) error {
	root := installRoot()
	id, err := sanitizeVersionID(version)
	if err != nil {
		return err
	}
	prev, _ := resolveActiveVersion(root)
	if err := activateVersion(root, id, versionManagerManagesServices()); err != nil {
		return err
	}
	bin := versionBinaryPath(root, id)
	return clioutput.Success(fmt.Sprintf("✓ Active version is now %s", id), map[string]any{
		"version":  id,
		"previous": prev,
		"path":     bin,
	})
}

// VersionsPrune implements `bedrud versions prune`.
func VersionsPrune(keep int) error {
	if keep < 0 {
		keep = defaultVersionKeep
	}
	root := installRoot()
	removed, err := pruneVersions(root, keep)
	if err != nil {
		return err
	}
	return clioutput.Success(
		fmt.Sprintf("✓ Pruned %d version(s) (keep non-active=%d)", len(removed), keep),
		map[string]any{"removed": removed, "keep": keep},
	)
}

// VersionsRemove implements `bedrud versions remove`.
func VersionsRemove(version string) error {
	root := installRoot()
	if err := removeVersion(root, version); err != nil {
		return err
	}
	return clioutput.Success(fmt.Sprintf("✓ Removed version %s", version), map[string]any{
		"removed": version,
	})
}

// VersionsPath implements `bedrud versions path`.
func VersionsPath(version string) error {
	root := installRoot()
	var id string
	var err error
	if version == "" {
		id, err = resolveActiveVersion(root)
		if err != nil {
			return err
		}
		if id == "" {
			return fmt.Errorf("no active version; pass an explicit version")
		}
	} else {
		id, err = sanitizeVersionID(version)
		if err != nil {
			return err
		}
	}
	path := versionBinaryPath(root, id)
	if clioutput.JSON() {
		return clioutput.Success("", map[string]any{
			"path":         path,
			"install_root": root,
			"version_dir":  versionDir(root, id),
		})
	}
	clioutput.Println(path)
	return nil
}
