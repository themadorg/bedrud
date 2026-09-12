package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

const defaultVersionKeep = 5

// VersionMeta is stored as versions/<id>/meta.json.
type VersionMeta struct {
	Version     string `json:"version"`
	InstalledAt string `json:"installed_at,omitempty"`
	Source      string `json:"source,omitempty"`
	SourceURL   string `json:"source_url,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
	Variant     string `json:"variant,omitempty"`
	OS          string `json:"os,omitempty"`
	Channel     string `json:"channel,omitempty"`
}

// InstalledVersion is one directory under {root}/versions.
type InstalledVersion struct {
	Version string       `json:"version"`
	Path    string       `json:"path"`
	Binary  string       `json:"binary"`
	Active  bool         `json:"active"`
	Meta    *VersionMeta `json:"meta,omitempty"`
}

// VersionListEntry is a row for `versions list` / `--remote`.
type VersionListEntry struct {
	Version      string  `json:"version"`
	Source       string  `json:"source"`
	Active       bool    `json:"active"`
	Installed    bool    `json:"installed"`
	RemoteLatest bool    `json:"remote_latest"`
	PublishedAt  *string `json:"published_at,omitempty"`
	Asset        *string `json:"asset,omitempty"`
	AssetSize    *uint64 `json:"asset_size,omitempty"`
}

// RemoteRelease is GitHub metadata used by `versions list --remote`.
type RemoteRelease struct {
	Version     string
	PublishedAt *string
	Asset       *string
	AssetSize   *uint64
	IsLatest    bool
}

func installRoot() string {
	if p := strings.TrimSpace(os.Getenv(envInstallRoot)); p != "" {
		return p
	}
	return defaultInstallRoot
}

func versionsDir(root string) string {
	return filepath.Join(root, "versions")
}

func versionDir(root, id string) string {
	return filepath.Join(versionsDir(root), id)
}

func versionBinaryPath(root, id string) string {
	return filepath.Join(versionDir(root, id), "bedrud")
}

func currentLinkPath(root string) string {
	return filepath.Join(root, "current")
}

func stableBinaryPath() string {
	if p := strings.TrimSpace(os.Getenv("BEDRUD_STABLE_BIN")); p != "" {
		return p
	}
	return binaryLocalPath
}

func sanitizeVersionID(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("version id is empty")
	}
	if strings.Contains(s, "/") || strings.Contains(s, `\`) || strings.Contains(s, "..") {
		return "", fmt.Errorf("invalid version id (path separators not allowed): %s", s)
	}
	for _, c := range s {
		if unicode.IsLetter(c) || unicode.IsDigit(c) || c == '.' || c == '_' || c == '+' || c == '-' {
			continue
		}
		return "", fmt.Errorf("invalid version id (allowed [0-9A-Za-z._+-]): %s", s)
	}
	return s, nil
}

func ensureInstallLayout(root string) error {
	return os.MkdirAll(versionsDir(root), 0o755)
}

func writeVersionMeta(root, id string, meta VersionMeta) error {
	if err := os.MkdirAll(versionDir(root, id), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(versionDir(root, id), "meta.json"), append(data, '\n'), 0o644)
}

func readVersionMeta(root, id string) *VersionMeta {
	data, err := os.ReadFile(filepath.Join(versionDir(root, id), "meta.json"))
	if err != nil {
		return nil
	}
	var m VersionMeta
	if json.Unmarshal(data, &m) != nil {
		return nil
	}
	return &m
}

func resolveActiveVersion(root string) (string, error) {
	current := currentLinkPath(root)
	if target, err := os.Readlink(current); err == nil {
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(current), target)
		}
		base := filepath.Base(target)
		if st, err := os.Stat(versionDir(root, base)); err == nil && st.IsDir() {
			return base, nil
		}
		parent := filepath.Base(filepath.Dir(target))
		if st, err := os.Stat(versionDir(root, parent)); err == nil && st.IsDir() {
			return parent, nil
		}
	}
	if real, err := filepath.EvalSymlinks(stableBinaryPath()); err == nil {
		if id := versionIDFromBinaryPath(root, real); id != "" {
			return id, nil
		}
	}
	return "", nil
}

func versionIDFromBinaryPath(root, binary string) string {
	parent := filepath.Dir(binary)
	id := filepath.Base(parent)
	if filepath.Dir(parent) == versionsDir(root) {
		return id
	}
	if strings.HasPrefix(parent, versionsDir(root)+string(os.PathSeparator)) {
		return id
	}
	return ""
}

func listInstalled(root string) ([]InstalledVersion, error) {
	vdir := versionsDir(root)
	ents, err := os.ReadDir(vdir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	active, _ := resolveActiveVersion(root)
	var out []InstalledVersion
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		id, err := sanitizeVersionID(e.Name())
		if err != nil {
			continue
		}
		bin := versionBinaryPath(root, id)
		st, err := os.Stat(bin)
		if err != nil || st.IsDir() {
			continue
		}
		out = append(out, InstalledVersion{
			Version: id,
			Path:    versionDir(root, id),
			Binary:  bin,
			Active:  id == active,
			Meta:    readVersionMeta(root, id),
		})
	}
	// newest-looking first (string desc)
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Version > out[i].Version {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}

func installCandidate(root, versionID, src string, meta VersionMeta) (string, error) {
	id, err := sanitizeVersionID(versionID)
	if err != nil {
		return "", err
	}
	if err := ensureInstallLayout(root); err != nil {
		return "", err
	}
	destDir := versionDir(root, id)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	dest := versionBinaryPath(root, id)
	if fi, err := os.Lstat(dest); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("refusing to write through symlink at %s", dest)
	}
	staging := filepath.Join(destDir, fmt.Sprintf(".staging-%d", os.Getpid()))
	data, err := os.ReadFile(src)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", src, err)
	}
	if err := os.WriteFile(staging, data, 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(staging, dest); err != nil {
		_ = os.Remove(staging)
		return "", err
	}
	meta.Version = id
	if meta.InstalledAt == "" {
		meta.InstalledAt = time.Now().UTC().Format(time.RFC3339)
	}
	if err := writeVersionMeta(root, id, meta); err != nil {
		return "", err
	}
	grantNetBindCapability(dest)
	return dest, nil
}

func setActive(root, versionID, stablePath string) error {
	id, err := sanitizeVersionID(versionID)
	if err != nil {
		return err
	}
	bin := versionBinaryPath(root, id)
	fi, err := os.Lstat(bin)
	if err != nil {
		return fmt.Errorf("version %s binary not found at %s", id, bin)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("version %s binary at %s is a symlink; refusing to activate", id, bin)
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("version %s binary at %s is not a regular file", id, bin)
	}
	if err := ensureInstallLayout(root); err != nil {
		return err
	}
	_ = os.Chmod(bin, 0o755)
	if err := atomicSymlink(versionDir(root, id), currentLinkPath(root)); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(stablePath), 0o755); err != nil {
		return err
	}
	return atomicSymlink(bin, stablePath)
}

func atomicSymlink(target, link string) error {
	parent := filepath.Dir(link)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(parent, fmt.Sprintf(".bedrud-link-%d-%d", os.Getpid(), time.Now().UnixNano()))
	_ = os.Remove(tmp)
	if err := os.Symlink(target, tmp); err != nil {
		return fmt.Errorf("symlink %s -> %s: %w", tmp, target, err)
	}
	if err := os.Rename(tmp, link); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("activate link %s -> %s: %w", link, target, err)
	}
	return nil
}

func pruneVersions(root string, keep int) ([]string, error) {
	installed, err := listInstalled(root)
	if err != nil {
		return nil, err
	}
	active, _ := resolveActiveVersion(root)
	// newest mtime first
	type row struct {
		InstalledVersion
		mod time.Time
	}
	rows := make([]row, 0, len(installed))
	for _, v := range installed {
		mod := time.Time{}
		if st, err := os.Stat(v.Path); err == nil {
			mod = st.ModTime()
		}
		rows = append(rows, row{v, mod})
	}
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[j].mod.After(rows[i].mod) {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
	var removed []string
	kept := 0
	for _, v := range rows {
		if active != "" && v.Version == active {
			continue
		}
		if keep > 0 && kept < keep {
			kept++
			continue
		}
		left, err := listInstalled(root)
		if err != nil {
			return removed, err
		}
		if len(left) <= 1 {
			break
		}
		if err := removeVersion(root, v.Version); err != nil {
			return removed, err
		}
		removed = append(removed, v.Version)
	}
	return removed, nil
}

func removeVersion(root, versionID string) error {
	id, err := sanitizeVersionID(versionID)
	if err != nil {
		return err
	}
	active, _ := resolveActiveVersion(root)
	if active == id {
		return fmt.Errorf("refusing to remove active version %s", id)
	}
	dir := versionDir(root, id)
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return fmt.Errorf("version %s not found", id)
	}
	installed, err := listInstalled(root)
	if err != nil {
		return err
	}
	if len(installed) == 1 && installed[0].Version == id {
		return fmt.Errorf("refusing to remove the only remaining version %s", id)
	}
	return os.RemoveAll(dir)
}

func mergeLocalAndRemote(local []InstalledVersion, remote []RemoteRelease) []VersionListEntry {
	type acc struct {
		VersionListEntry
	}
	m := map[string]*acc{}
	order := []string{}
	put := func(id string) *acc {
		if e, ok := m[id]; ok {
			return e
		}
		e := &acc{VersionListEntry{Version: id}}
		m[id] = e
		order = append(order, id)
		return e
	}
	for _, l := range local {
		e := put(l.Version)
		e.Source = "local"
		e.Active = l.Active
		e.Installed = true
	}
	for _, r := range remote {
		e := put(r.Version)
		if e.Installed {
			e.Source = "both"
		} else {
			e.Source = "remote"
		}
		e.RemoteLatest = r.IsLatest
		e.PublishedAt = r.PublishedAt
		e.Asset = r.Asset
		e.AssetSize = r.AssetSize
	}
	out := make([]VersionListEntry, 0, len(order))
	for _, id := range order {
		out = append(out, m[id].VersionListEntry)
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Version > out[i].Version {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func normalizeReleaseTag(tag string) string {
	t := strings.TrimSpace(tag)
	return strings.TrimPrefix(t, "v")
}

func parseGitHubReleasesJSON(body, hostAsset string) ([]RemoteRelease, error) {
	var arr []githubRelease
	if err := json.Unmarshal([]byte(body), &arr); err != nil {
		return nil, fmt.Errorf("github releases json: %w", err)
	}
	var out []RemoteRelease
	first := true
	for _, item := range arr {
		if !isStableRelease(item) {
			continue
		}
		version, err := sanitizeVersionID(normalizeReleaseTag(item.TagName))
		if err != nil {
			continue
		}
		var asset *string
		var size *uint64
		for _, a := range item.Assets {
			if a.Name == hostAsset || strings.Contains(a.Name, hostAsset) {
				n := a.Name
				asset = &n
				break
			}
		}
		out = append(out, RemoteRelease{
			Version:   version,
			Asset:     asset,
			AssetSize: size,
			IsLatest:  first,
		})
		first = false
		if len(out) >= 20 {
			break
		}
	}
	return out, nil
}

func fetchRemoteReleases() ([]RemoteRelease, error) {
	list, err := getJSONReleaseList(githubReleasesURL)
	if err != nil {
		return nil, err
	}
	asset, err := releaseAssetName()
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(list)
	if err != nil {
		return nil, err
	}
	return parseGitHubReleasesJSON(string(data), asset)
}

func archiveCurrentIfNeeded(root, versionID, stablePath string) {
	id, err := sanitizeVersionID(versionID)
	if err != nil || id == "" {
		id = fmt.Sprintf("unknown-%d", time.Now().Unix())
	}
	fi, err := os.Lstat(stablePath)
	if err != nil {
		return
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return // already version-managed
	}
	if !fi.Mode().IsRegular() {
		return
	}
	dest := versionBinaryPath(root, id)
	if st, err := os.Stat(dest); err == nil && st.Mode().IsRegular() {
		return
	}
	meta := VersionMeta{Version: id, Source: "archive-previous", OS: "linux"}
	if _, err := installCandidate(root, id, stablePath, meta); err != nil {
		fmt.Printf("⚠ Warning: could not archive current binary as version %s: %v\n", id, err)
	} else {
		fmt.Println("➜ Archived previous binary as", id)
	}
}

func activateVersion(root, id string, manageServices bool) error {
	bin := versionBinaryPath(root, id)
	if err := quickELFCheck(bin); err != nil {
		return err
	}
	fi, err := os.Lstat(bin)
	if err != nil {
		return fmt.Errorf("version %s not found at %s", id, bin)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("version %s binary at %s is a symlink; refusing to activate", id, bin)
	}
	prev, _ := resolveActiveVersion(root)
	stable := stableBinaryPath()
	if manageServices {
		stopServicesHook([]string{"bedrud", "livekit"})
	}
	if err := setActive(root, id, stable); err != nil {
		if manageServices {
			_ = refreshServicesHook(stable, false)
		}
		return err
	}
	if err := quickELFCheck(bin); err != nil {
		if prev != "" {
			_ = setActive(root, prev, stable)
		}
		if manageServices {
			_ = refreshServicesHook(stable, false)
		}
		return fmt.Errorf("smoke check failed after switch: %w", err)
	}
	if manageServices {
		isExt := false
		if _, err := os.Stat(etcConfigPath); err == nil {
			isExt, _ = isExternalLiveKitFromConfig(etcConfigPath)
		}
		if err := refreshServicesHook(stable, isExt); err != nil {
			return err
		}
	}
	return nil
}

func versionManagerManagesServices() bool {
	return os.Getenv(envNoVMService) == ""
}
