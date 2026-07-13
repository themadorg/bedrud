package webxdc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// WellKnownXdcgetLock is the public xdcget store lockfile (JSON app list with download URLs).
// Used when admins paste a human store page URL (e.g. https://webxdc.org/apps) instead of JSON.
const WellKnownXdcgetLock = "https://apps.testrun.org/xdcget-lock.json"

// GalleryEntry is a normalized catalog card (safe for JSON to the browser).
type GalleryEntry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
	// IconURL is only for remote/semi-remote HTTPS icons (not used for instance packages).
	IconURL string `json:"iconUrl,omitempty"`
	// HasIcon is true when a server-stored raster sidecar exists (instance/room packages).
	// Client loads it via authenticated GET /api/webxdc/packages/:id/icon — never raw paths.
	HasIcon bool `json:"hasIcon,omitempty"`
	XdcURL  string `json:"xdcUrl,omitempty"`
	// PackageID is set for instance-catalog (admin-uploaded) packages already on this server.
	PackageID     string `json:"packageId,omitempty"`
	SourceCodeURL string `json:"sourceCodeUrl,omitempty"`
	Origin        string `json:"origin"` // remote | local | semi-remote | instance
}

// FetchGalleryCatalog downloads and parses a remote catalog JSON (server-side only).
// Accepts a catalog JSON URL, or well-known store HTML URLs (auto-mapped to xdcget-lock.json).
func FetchGalleryCatalog(ctx context.Context, catalogURL string, maxBytes int64) ([]GalleryEntry, error) {
	if maxBytes <= 0 {
		maxBytes = 4 << 20 // 4 MiB (xdcget lock is larger than 2MiB sometimes)
	}
	resolved := resolveCatalogURL(catalogURL)
	raw, err := fetchHTTPS(ctx, resolved, maxBytes)
	if err != nil {
		return nil, err
	}
	// HTML mistake: try well-known lock once.
	if looksLikeHTML(raw) {
		if resolved != WellKnownXdcgetLock {
			raw2, err2 := fetchHTTPS(ctx, WellKnownXdcgetLock, maxBytes)
			if err2 != nil {
				return nil, fmt.Errorf("catalog URL returned HTML (need JSON catalog); fallback failed: %w", err2)
			}
			raw = raw2
			resolved = WellKnownXdcgetLock
		} else {
			return nil, fmt.Errorf("catalog URL returned HTML, not JSON")
		}
	}
	list, err := parseGalleryCatalog(raw)
	if err != nil {
		return nil, err
	}
	base, _ := url.Parse(resolved)
	for i := range list {
		list[i].IconURL = absolutizeURL(base, list[i].IconURL)
		list[i].XdcURL = absolutizeURL(base, list[i].XdcURL)
		list[i].SourceCodeURL = absolutizeURL(base, list[i].SourceCodeURL)
	}
	return list, nil
}

// FetchXdcArchive downloads a remote .xdc with size limit (server-side only).
func FetchXdcArchive(ctx context.Context, xdcURL string, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = 10 << 20
	}
	return fetchHTTPS(ctx, xdcURL, maxBytes)
}

// resolveCatalogURL maps human store pages to the JSON lockfile when possible.
func resolveCatalogURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return WellKnownXdcgetLock
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	host := strings.ToLower(u.Hostname())
	pathLower := strings.ToLower(strings.TrimRight(u.Path, "/"))
	// Common misconfig: paste the apps website instead of catalog JSON.
	if host == "webxdc.org" || host == "www.webxdc.org" || host == "webxdc.com" || host == "www.webxdc.com" {
		if pathLower == "" || pathLower == "/apps" || pathLower == "/apps/index.html" {
			return WellKnownXdcgetLock
		}
	}
	if host == "apps.testrun.org" && (pathLower == "" || pathLower == "/" || pathLower == "/apps") {
		return WellKnownXdcgetLock
	}
	return raw
}

func looksLikeHTML(raw []byte) bool {
	s := strings.TrimSpace(string(raw))
	if len(s) < 15 {
		return false
	}
	low := strings.ToLower(s[:min(64, len(s))])
	return strings.HasPrefix(low, "<!doctype html") || strings.HasPrefix(low, "<html") || strings.Contains(low, "<head>")
}

func absolutizeURL(base *url.URL, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" || base == nil {
		return ref
	}
	if strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "http://") {
		return ref
	}
	// Relative icon_relname like "foo-icon.png" → same directory as catalog.
	u, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	abs := base.ResolveReference(u)
	// For bare filenames, ResolveReference may drop path incorrectly if base is .../file.json
	if !strings.Contains(ref, "/") && base.Path != "" {
		dir := path.Dir(base.Path)
		abs.Path = path.Join(dir, ref)
	}
	return abs.String()
}

func fetchHTTPS(ctx context.Context, rawURL string, maxBytes int64) ([]byte, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, fmt.Errorf("url must be https (got %q)", rawURL)
	}
	if err := rejectPrivateHost(u.Hostname()); err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			if req.URL.Scheme != "https" {
				return fmt.Errorf("redirect must stay on https")
			}
			return rejectPrivateHost(req.URL.Hostname())
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Bedrud-WebXDC-Gallery/1.0")
	req.Header.Set("Accept", "application/json, application/zip, application/octet-stream, */*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch status %d", resp.StatusCode)
	}
	if resp.ContentLength > maxBytes {
		return nil, fmt.Errorf("response too large")
	}
	limited := io.LimitReader(resp.Body, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("response too large")
	}
	return data, nil
}

func rejectPrivateHost(host string) error {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") || host == "0.0.0.0" {
		return fmt.Errorf("blocked host")
	}
	// Strip IPv6 brackets if present
	h := strings.Trim(host, "[]")
	if ip := net.ParseIP(h); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("blocked address")
		}
		return nil
	}
	// Resolve hostname and reject private answers
	ips, err := net.LookupIP(h)
	if err != nil {
		return fmt.Errorf("host resolve failed: %w", err)
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("blocked address")
		}
	}
	return nil
}

func parseGalleryCatalog(raw []byte) ([]GalleryEntry, error) {
	// Accept { apps: [...] }, { entries: [...] }, or a bare array (xdcget-lock.json).
	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("invalid catalog json: %w", err)
	}

	var items []any
	switch v := root.(type) {
	case []any:
		items = v
	case map[string]any:
		for _, key := range []string{"apps", "entries", "items", "packages"} {
			if arr, ok := v[key].([]any); ok {
				items = arr
				break
			}
		}
		if items == nil {
			return nil, fmt.Errorf("catalog missing apps array")
		}
	default:
		return nil, fmt.Errorf("unsupported catalog shape")
	}

	out := make([]GalleryEntry, 0, len(items))
	for i, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		name := strField(m, "name", "title", "appName", "app_name")
		xdc := strField(m, "xdcUrl", "xdc_url", "url", "download", "downloadUrl", "download_url", "xdc")
		id := strField(m, "id", "slug", "key", "app_id", "appId")
		if name == "" {
			name = id
		}
		if name == "" && xdc == "" {
			continue
		}
		if name == "" {
			name = fmt.Sprintf("App %d", i+1)
		}
		if id == "" {
			id = fmt.Sprintf("remote-%d", i)
		}
		// Prefer absolute icon fields; icon_relname resolved by caller against catalog base.
		icon := strField(m, "iconUrl", "icon_url", "icon", "image", "icon_relname")
		out = append(out, GalleryEntry{
			ID:            id,
			Name:          name,
			Description:   strField(m, "description", "desc", "summary"),
			Category:      strField(m, "category", "cat", "type"),
			IconURL:       icon,
			XdcURL:        xdc,
			SourceCodeURL: strField(m, "sourceCodeUrl", "source_code_url", "sourceCode", "source"),
			Origin:        "semi-remote",
		})
	}
	return out, nil
}

func strField(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					return s
				}
			}
		}
	}
	return ""
}
