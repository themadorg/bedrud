package install

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	githubAPILatest   = "https://api.github.com/repos/themadorg/bedrud/releases/latest"
	githubAPIReleases = "https://api.github.com/repos/themadorg/bedrud/releases?per_page=30"
	downloadTimeout   = 10 * time.Minute
	maxDownloadBytes  = 512 << 20 // 512 MiB
	httpUserAgent     = "bedrud-update/1.0 (+https://github.com/themadorg/bedrud)"
)

// Overridable for tests.
var (
	httpClient = &http.Client{Timeout: downloadTimeout}
	// githubLatestURL can be overridden in tests.
	githubLatestURL = githubAPILatest
	// githubReleasesURL lists recent releases (stable + prerelease) for --nightly.
	githubReleasesURL = githubAPIReleases
)

// githubRelease is the GitHub Releases API subset we need.
type githubRelease struct {
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// resolvedSource is the result of resolving an update source to a local binary.
type resolvedSource struct {
	BinaryPath  string
	Cleanup     func()
	Version     string // optional, from release tag
	Description string
	// Channel is "stable" or "nightly" for GitHub fetches; empty for local/--self.
	Channel string
}

// resolveUpdateSource turns UpdateOptions into a local binary path.
// Caller must run Cleanup when done (if non-nil).
func resolveUpdateSource(opts UpdateOptions) (resolvedSource, error) {
	if opts.Self {
		self, _ := os.Executable()
		tmp, cleanup, err := writeSelfToTemp()
		if err != nil {
			return resolvedSource{}, fmt.Errorf("resolve self binary: %w", err)
		}
		desc := "this executable (--self)"
		if self != "" {
			desc = fmt.Sprintf("this executable (--self): %s", self)
		}
		return resolvedSource{
			BinaryPath:  tmp,
			Cleanup:     cleanup,
			Description: desc,
		}, nil
	}

	src := strings.TrimSpace(opts.Source)

	if opts.Nightly {
		if src != "" && !strings.EqualFold(src, "latest") {
			return resolvedSource{}, fmt.Errorf("--nightly cannot be combined with a local path or URL")
		}
		return resolveGitHubChannel(opts.SkipChecksum, true)
	}

	if src == "" {
		return resolvedSource{}, fmt.Errorf("empty source")
	}

	if strings.EqualFold(src, "latest") {
		return resolveGitHubChannel(opts.SkipChecksum, false)
	}

	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		return resolveURL(src, opts.SkipChecksum)
	}

	return resolveLocalPath(src, opts.SkipChecksum)
}

func writeSelfToTemp() (path string, cleanup func(), err error) {
	data, err := readSelfBinary()
	if err != nil {
		return "", nil, err
	}
	dir, err := os.MkdirTemp("", "bedrud-update-self-*")
	if err != nil {
		return "", nil, err
	}
	path = filepath.Join(dir, "bedrud")
	if err := os.WriteFile(path, data, 0o700); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, err
	}
	return path, func() { _ = os.RemoveAll(dir) }, nil
}

func resolveLocalPath(src string, skipChecksum bool) (resolvedSource, error) {
	st, err := os.Stat(src)
	if err != nil {
		return resolvedSource{}, fmt.Errorf("source path: %w", err)
	}
	if st.IsDir() {
		return resolvedSource{}, fmt.Errorf("source is a directory: %s", src)
	}

	if !skipChecksum {
		if err := verifyLocalChecksumIfPresent(src); err != nil {
			return resolvedSource{}, err
		}
	}

	if isArchivePath(src) {
		return extractArchiveToResolved(src, fmt.Sprintf("local archive %s", src), "")
	}

	// Bare binary
	if err := quickELFCheck(src); err != nil {
		return resolvedSource{}, err
	}
	return resolvedSource{
		BinaryPath:  src,
		Cleanup:     nil,
		Description: fmt.Sprintf("local binary %s", src),
	}, nil
}

func extractArchiveToResolved(archivePath, desc, version string) (resolvedSource, error) {
	dir, err := os.MkdirTemp("", "bedrud-update-extract-*")
	if err != nil {
		return resolvedSource{}, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	bin, err := safeExtractArchive(archivePath, dir)
	if err != nil {
		cleanup()
		return resolvedSource{}, err
	}
	if err := quickELFCheck(bin); err != nil {
		cleanup()
		return resolvedSource{}, err
	}
	return resolvedSource{
		BinaryPath:  bin,
		Cleanup:     cleanup,
		Version:     version,
		Description: desc,
	}, nil
}

func resolveURL(raw string, skipChecksum bool) (resolvedSource, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return resolvedSource{}, fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "https" {
		return resolvedSource{}, fmt.Errorf("only https URLs are allowed (got %q)", u.Scheme)
	}

	requireChecksum := isGitHubReleaseURL(u) && !skipChecksum
	if !isGitHubReleaseURL(u) && !skipChecksum {
		// Arbitrary HTTPS: require checksum skip only if operator opts in, else fail closed
		// unless we can find SHA256SUMS next to the asset (same directory URL).
		requireChecksum = true
	}

	dir, err := os.MkdirTemp("", "bedrud-update-dl-*")
	if err != nil {
		return resolvedSource{}, err
	}
	cleanupAll := func() { _ = os.RemoveAll(dir) }

	base := path.Base(u.Path)
	if base == "" || base == "." || base == "/" {
		base = "bedrud-download"
	}
	dest := filepath.Join(dir, base)

	if err := downloadFile(raw, dest); err != nil {
		cleanupAll()
		return resolvedSource{}, err
	}

	version := versionFromGitHubURL(u)

	if requireChecksum {
		sumsURL := siblingSHA256SUMS(u)
		if sumsURL == "" {
			cleanupAll()
			return resolvedSource{}, fmt.Errorf(
				"remote download requires checksum verification; could not derive SHA256SUMS URL\n" +
					"Use a GitHub release asset, or a local path, or pass --skip-checksum for a trusted local/operator URL",
			)
		}
		if err := verifyRemoteFile(dest, base, sumsURL); err != nil {
			cleanupAll()
			return resolvedSource{}, err
		}
	}

	if isArchivePath(dest) {
		res, err := extractArchiveToResolved(dest, fmt.Sprintf("URL %s", raw), version)
		if err != nil {
			cleanupAll()
			return resolvedSource{}, err
		}
		// Combine cleanups
		inner := res.Cleanup
		res.Cleanup = func() {
			if inner != nil {
				inner()
			}
			cleanupAll()
		}
		return res, nil
	}

	if err := quickELFCheck(dest); err != nil {
		cleanupAll()
		return resolvedSource{}, err
	}
	return resolvedSource{
		BinaryPath:  dest,
		Cleanup:     cleanupAll,
		Version:     version,
		Description: fmt.Sprintf("URL %s", raw),
	}, nil
}

func resolveLatest(skipChecksum bool) (resolvedSource, error) {
	return resolveGitHubChannel(skipChecksum, false)
}

func resolveGitHubChannel(skipChecksum, nightly bool) (resolvedSource, error) {
	channel := "stable"
	label := "latest"
	if nightly {
		channel = "nightly"
		label = "nightly"
	}
	if skipChecksum {
		return resolvedSource{}, fmt.Errorf("--skip-checksum is not allowed with %q (checksum required)", label)
	}

	if nightly {
		fmt.Println("➜ Fetching latest nightly GitHub release...")
	} else {
		fmt.Println("➜ Fetching latest stable GitHub release...")
	}

	assetName, err := releaseAssetName()
	if err != nil {
		return resolvedSource{}, err
	}

	rel, err := fetchGitHubRelease(nightly)
	if err != nil {
		return resolvedSource{}, err
	}

	if nightly {
		fmt.Println("➜ Nightly release:", rel.TagName)
	} else {
		fmt.Println("➜ Stable release:", rel.TagName)
	}

	var assetURL, sumsURL string
	for _, a := range rel.Assets {
		switch a.Name {
		case assetName:
			assetURL = a.BrowserDownloadURL
		case "SHA256SUMS", "SHA256SUMS.txt":
			sumsURL = a.BrowserDownloadURL
		}
	}
	if assetURL == "" {
		return resolvedSource{}, fmt.Errorf("%s release %s has no asset %q", label, rel.TagName, assetName)
	}
	if sumsURL == "" {
		return resolvedSource{}, fmt.Errorf(
			"%s release %s has no SHA256SUMS asset; cannot verify integrity\n"+
				"Download the release manually and run: sudo bedrud update /path/to/archive-or-binary",
			label, rel.TagName,
		)
	}

	dir, err := os.MkdirTemp("", "bedrud-update-"+label+"-*")
	if err != nil {
		return resolvedSource{}, err
	}
	cleanupAll := func() { _ = os.RemoveAll(dir) }

	dest := filepath.Join(dir, assetName)
	if err := downloadFile(assetURL, dest); err != nil {
		cleanupAll()
		return resolvedSource{}, err
	}
	if err := verifyRemoteFile(dest, assetName, sumsURL); err != nil {
		cleanupAll()
		return resolvedSource{}, err
	}

	res, err := extractArchiveToResolved(dest, fmt.Sprintf("%s %s (%s)", label, rel.TagName, assetName), rel.TagName)
	if err != nil {
		cleanupAll()
		return resolvedSource{}, err
	}
	res.Channel = channel
	inner := res.Cleanup
	res.Cleanup = func() {
		if inner != nil {
			inner()
		}
		cleanupAll()
	}
	return res, nil
}

func fetchGitHubRelease(nightly bool) (githubRelease, error) {
	if !nightly {
		rel, err := getJSONRelease(githubLatestURL)
		if err != nil {
			return githubRelease{}, err
		}
		if !isStableRelease(rel) {
			return githubRelease{}, fmt.Errorf(
				"GitHub latest is not a stable release (%s); refusing\n"+
					"Use --nightly for prereleases, or wait for a stable tag",
				rel.TagName,
			)
		}
		return rel, nil
	}

	list, err := getJSONReleaseList(githubReleasesURL)
	if err != nil {
		return githubRelease{}, err
	}
	for _, rel := range list {
		if isNightlyRelease(rel) {
			return rel, nil
		}
	}
	return githubRelease{}, fmt.Errorf(
		"no nightly/prerelease found on GitHub\n" +
			"Use: sudo bedrud update latest   for the latest stable release",
	)
}

func githubAPIGet(urlStr string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", httpUserAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		return nil, fmt.Errorf("github %s: HTTP %d: %s", urlStr, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return resp, nil
}

func getJSONRelease(urlStr string) (githubRelease, error) {
	resp, err := githubAPIGet(urlStr)
	if err != nil {
		return githubRelease{}, fmt.Errorf("github latest: %w", err)
	}
	defer resp.Body.Close()
	var rel githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rel); err != nil {
		return githubRelease{}, fmt.Errorf("decode release JSON: %w", err)
	}
	return rel, nil
}

func getJSONReleaseList(urlStr string) ([]githubRelease, error) {
	resp, err := githubAPIGet(urlStr)
	if err != nil {
		return nil, fmt.Errorf("github releases: %w", err)
	}
	defer resp.Body.Close()
	var list []githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&list); err != nil {
		return nil, fmt.Errorf("decode releases JSON: %w", err)
	}
	return list, nil
}

func isStableRelease(r githubRelease) bool {
	if r.Draft || r.Prerelease || r.TagName == "" {
		return false
	}
	t := strings.ToLower(r.TagName + " " + r.Name)
	return !strings.Contains(t, "nightly")
}

func isNightlyRelease(r githubRelease) bool {
	if r.Draft || r.TagName == "" {
		return false
	}
	t := strings.ToLower(r.TagName + " " + r.Name)
	return r.Prerelease || strings.Contains(t, "nightly")
}

func releaseAssetName() (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("update download only supports linux (got %s)", runtime.GOOS)
	}
	switch runtime.GOARCH {
	case "amd64":
		return "bedrud_linux_amd64.tar.xz", nil
	case "arm64":
		return "bedrud_linux_arm64.tar.xz", nil
	default:
		return "", fmt.Errorf("unsupported architecture %s (need amd64 or arm64)", runtime.GOARCH)
	}
}

func downloadFile(urlStr, dest string) error {
	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", httpUserAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", urlStr, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", urlStr, resp.StatusCode)
	}

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()

	n, err := io.Copy(out, io.LimitReader(resp.Body, maxDownloadBytes+1))
	if err != nil {
		return fmt.Errorf("download body: %w", err)
	}
	if n > maxDownloadBytes {
		return fmt.Errorf("download exceeds size limit (%d bytes)", maxDownloadBytes)
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func verifyRemoteFile(localPath, assetName, sumsURL string) error {
	sumsPath := localPath + ".SHA256SUMS"
	if err := downloadFile(sumsURL, sumsPath); err != nil {
		return fmt.Errorf("download SHA256SUMS: %w", err)
	}
	data, err := os.ReadFile(sumsPath)
	if err != nil {
		return err
	}
	want, err := parseSHA256SUMS(string(data), assetName)
	if err != nil {
		return err
	}
	got, err := fileSHA256(localPath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("checksum mismatch for %s: got %s want %s", assetName, got, want)
	}
	fmt.Println("➜ Checksum verified:", assetName)
	return nil
}

func verifyLocalChecksumIfPresent(src string) error {
	// Optional: adjacent SHA256SUMS or src.sha256
	dir := filepath.Dir(src)
	base := filepath.Base(src)
	for _, name := range []string{"SHA256SUMS", "SHA256SUMS.txt"} {
		sumsPath := filepath.Join(dir, name)
		data, err := os.ReadFile(sumsPath)
		if err != nil {
			continue
		}
		want, err := parseSHA256SUMS(string(data), base)
		if err != nil {
			// file present but no line for this asset — ignore
			continue
		}
		got, err := fileSHA256(src)
		if err != nil {
			return err
		}
		if !strings.EqualFold(got, want) {
			return fmt.Errorf("checksum mismatch for %s: got %s want %s", base, got, want)
		}
		fmt.Println("➜ Checksum verified (local SHA256SUMS):", base)
		return nil
	}
	return nil
}

// parseSHA256SUMS finds the hex digest for filename in GNU-style SHA256SUMS content.
func parseSHA256SUMS(content, filename string) (string, error) {
	filename = filepath.Base(filename)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// formats: "<hex>  <name>" or "<hex> *<name>"
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		hexSum := fields[0]
		name := fields[len(fields)-1]
		name = strings.TrimPrefix(name, "*")
		if filepath.Base(name) == filename {
			if len(hexSum) != 64 {
				return "", fmt.Errorf("invalid sha256 length in SUMS for %s", filename)
			}
			return hexSum, nil
		}
	}
	return "", fmt.Errorf("no SHA256SUMS entry for %s", filename)
}

func isGitHubReleaseURL(u *url.URL) bool {
	if u == nil {
		return false
	}
	host := strings.ToLower(u.Host)
	if host != "github.com" && host != "objects.githubusercontent.com" && !strings.HasSuffix(host, ".githubusercontent.com") {
		return false
	}
	return strings.Contains(u.Path, "/themadorg/bedrud/") || strings.Contains(u.Path, "themadorg/bedrud")
}

func siblingSHA256SUMS(u *url.URL) string {
	if u == nil {
		return ""
	}
	// .../download/v1.2.3/bedrud_linux_amd64.tar.xz -> .../download/v1.2.3/SHA256SUMS
	dir := path.Dir(u.Path)
	if dir == "." || dir == "/" {
		return ""
	}
	cp := *u
	cp.Path = path.Join(dir, "SHA256SUMS")
	cp.RawQuery = ""
	cp.Fragment = ""
	return cp.String()
}

func versionFromGitHubURL(u *url.URL) string {
	// /themadorg/bedrud/releases/download/v1.2.3/asset
	parts := strings.Split(u.Path, "/")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "download" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// quickELFCheck ensures the file looks like a Linux ELF binary (best-effort).
func quickELFCheck(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var magic [4]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil {
		return fmt.Errorf("read binary magic: %w", err)
	}
	if magic[0] != 0x7f || magic[1] != 'E' || magic[2] != 'L' || magic[3] != 'F' {
		return fmt.Errorf("%s is not an ELF binary (expected Linux bedrud binary)", path)
	}
	return nil
}
