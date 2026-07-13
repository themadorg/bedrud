package webxdc

import (
	"path"
	"strings"
)

// ContentTypeForEntry returns a safe Content-Type for a path inside an .xdc.
// PDF is never served as application/pdf (XDC-01-005 PDF viewer CSP bypass).
func ContentTypeForEntry(filename string) string {
	base := path.Base(filename)
	ext := strings.ToLower(path.Ext(base))
	switch ext {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".txt", ".md", ".toml":
		return "text/plain; charset=utf-8"
	case ".wasm":
		return "application/wasm"
	case ".pdf":
		// Force download / non-viewer type (XDC-01-005).
		return "application/octet-stream"
	case ".mp3":
		return "audio/mpeg"
	case ".ogg":
		return "audio/ogg"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	default:
		return "application/octet-stream"
	}
}

// IsHostProvidedPath is true for paths the host must never take from the ZIP.
func IsHostProvidedPath(filename string) bool {
	// Normalize and compare basename so "./webxdc.js" still matches.
	base := strings.ToLower(path.Base(strings.ReplaceAll(filename, "\\", "/")))
	return base == "webxdc.js"
}
