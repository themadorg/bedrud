package webxdc

import (
	"bytes"
	"net/url"
	"regexp"
	"strings"
)

// Match src= / href= attribute values in HTML (quoted). Used only to inject the
// short-lived asset ticket so relative CSS/JS loads work when third-party
// cookies are blocked and Referer is missing.
var htmlAttrURL = regexp.MustCompile(`(?i)(\b(?:src|href)\s*=\s*)(["'])([^"']*)(["'])`)

// InjectTicketIntoHTML rewrites relative src/href in HTML so subresources carry ?t=.
// Absolute, protocol-relative, data:, blob:, mailto:, javascript:, and fragment-only
// URLs are left unchanged.
func InjectTicketIntoHTML(html []byte, ticket string) []byte {
	ticket = strings.TrimSpace(ticket)
	if ticket == "" || len(html) == 0 {
		return html
	}
	return htmlAttrURL.ReplaceAllFunc(html, func(m []byte) []byte {
		parts := htmlAttrURL.FindSubmatch(m)
		if len(parts) != 5 {
			return m
		}
		raw := string(parts[3])
		next, ok := appendTicketQuery(raw, ticket)
		if !ok {
			return m
		}
		out := make([]byte, 0, len(m)+len(ticket)+4)
		out = append(out, parts[1]...)
		out = append(out, parts[2]...)
		out = append(out, next...)
		out = append(out, parts[4]...)
		return out
	})
}

// appendTicketQuery adds t=<ticket> to a relative URL. Returns ok=false if the
// URL must not be rewritten (external / special schemes / already has t=).
func appendTicketQuery(raw, ticket string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "#" || strings.HasPrefix(raw, "#") {
		return raw, false
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "data:") ||
		strings.HasPrefix(lower, "blob:") ||
		strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(lower, "javascript:") ||
		strings.HasPrefix(lower, "http:") ||
		strings.HasPrefix(lower, "https:") ||
		strings.HasPrefix(raw, "//") {
		return raw, false
	}
	// Already ticketed (idempotent).
	if u, err := url.Parse(raw); err == nil {
		if u.Query().Get("t") != "" {
			return raw, false
		}
		q := u.Query()
		q.Set("t", ticket)
		u.RawQuery = q.Encode()
		// url.Parse on relative paths keeps Path; String() is fine.
		return u.String(), true
	}
	sep := "?"
	if strings.Contains(raw, "?") {
		sep = "&"
	}
	return raw + sep + "t=" + url.QueryEscape(ticket), true
}

// IsHTMLEntry reports whether a zip entry should be HTML-rewritten for tickets.
func IsHTMLEntry(entry string) bool {
	base := strings.ToLower(entry)
	return strings.HasSuffix(base, ".html") || strings.HasSuffix(base, ".htm")
}

// IsScriptEntry reports JS assets that may assume Desktop top-level window.top.
func IsScriptEntry(entry string) bool {
	base := strings.ToLower(entry)
	return strings.HasSuffix(base, ".js") || strings.HasSuffix(base, ".mjs")
}

// SoftenCrossOriginTop rewrites a few Desktop-only window.top usages so apps
// (notably Quake/OpenArena) do not throw SecurityError inside Bedrud's
// cross-origin iframe. Only targeted patterns — never a blanket top→self rewrite.
func SoftenCrossOriginTop(body []byte) []byte {
	if len(body) == 0 {
		return body
	}
	// OpenArena index.html: pagehide notify on server stop (Desktop: top-level).
	body = bytes.ReplaceAll(body,
		[]byte("window.top.addEventListener('pagehide'"),
		[]byte("window.addEventListener('pagehide'"),
	)
	body = bytes.ReplaceAll(body,
		[]byte(`window.top.addEventListener("pagehide"`),
		[]byte(`window.addEventListener("pagehide"`),
	)
	// OpenArena override-webrtc.js: stash channel across reloads (Desktop top).
	// We expose the same property on window in HostBridgeJS.
	body = bytes.ReplaceAll(body,
		[]byte("window.top.__webxdcRealtimeChannel"),
		[]byte("window.__webxdcRealtimeChannel"),
	)
	return body
}
