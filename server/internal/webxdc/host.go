package webxdc

import (
	"strings"
)

// ParseInstanceHost extracts the instance host label from a request Host.
// Expects: webxdc-<label>.<baseDomain> (optional port stripped by caller).
// Returns label or empty if not a webxdc host for this baseDomain.
func ParseInstanceHost(host, baseDomain string) (label string, ok bool) {
	host = strings.ToLower(strings.TrimSpace(host))
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	baseDomain = strings.ToLower(strings.TrimSpace(baseDomain))
	if host == "" || baseDomain == "" {
		return "", false
	}
	suffix := "." + baseDomain
	if !strings.HasSuffix(host, suffix) {
		return "", false
	}
	prefix := strings.TrimSuffix(host, suffix)
	const head = "webxdc-"
	if !strings.HasPrefix(prefix, head) {
		return "", false
	}
	label = strings.TrimPrefix(prefix, head)
	if label == "" || strings.Contains(label, ".") {
		return "", false
	}
	for _, c := range label {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return "", false
		}
	}
	return label, true
}

// InstanceOrigin builds https://webxdc-<label>.<baseDomain>[:port]
// port is optional; empty or "80"/"443" (matching scheme) omits the port.
// Local make dev: baseDomain "localhost", port "7071" → http://webxdc-id.localhost:7071
// Modern browsers resolve *.localhost → 127.0.0.1 (no /etc/hosts needed).
func InstanceOrigin(label, baseDomain string, secure bool, port string) string {
	scheme := "https"
	if !secure {
		scheme = "http"
	}
	host := "webxdc-" + label + "." + strings.TrimSpace(baseDomain)
	port = strings.TrimSpace(port)
	if port != "" {
		if secure && port != "443" {
			host = host + ":" + port
		}
		if !secure && port != "80" {
			host = host + ":" + port
		}
	}
	return scheme + "://" + host
}

// PathModePrefix is the API-host path prefix for local/dev path-mode packages.
const PathModePrefix = "/__webxdc"

// InstanceOriginPath builds {publicBase}/__webxdc/{label} for make-dev path mode
// (no wildcard DNS). publicBase must be absolute, e.g. http://localhost:7071.
func InstanceOriginPath(label, publicBase string) string {
	base := strings.TrimRight(strings.TrimSpace(publicBase), "/")
	if base == "" {
		base = "http://localhost:7071"
	}
	return base + PathModePrefix + "/" + strings.TrimSpace(label)
}

// ParsePathModeInstance extracts instance id from /__webxdc/{id}/... paths.
func ParsePathModeInstance(path string) (label string, assetPath string, ok bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", "", false
	}
	if !strings.HasPrefix(path, PathModePrefix+"/") && path != PathModePrefix {
		return "", "", false
	}
	rest := strings.TrimPrefix(path, PathModePrefix)
	rest = strings.TrimPrefix(rest, "/")
	if rest == "" {
		return "", "", false
	}
	parts := strings.SplitN(rest, "/", 2)
	label = parts[0]
	if label == "" || strings.Contains(label, ".") {
		return "", "", false
	}
	for _, c := range label {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return "", "", false
		}
	}
	assetPath = "/"
	if len(parts) == 2 && parts[1] != "" {
		assetPath = "/" + parts[1]
	}
	return label, assetPath, true
}
