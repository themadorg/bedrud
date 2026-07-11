// Package webxdc holds pure helpers for serving and validating WebXDC packages.
// Full HTTP routes land later; these invariants are tested now so hosts cannot
// regress CSP / MIME / zip rules from docs/plan/webxdc.
package webxdc

import "strings"

// CSP directives for the mini-app document itself (network isolation).
// frame-ancestors is filled by BuildCSP — 'self' alone is wrong because the
// Bedrud SPA is always a *different* origin (localhost:7070 vs webxdc-*.localhost:7071,
// or app.example.com vs webxdc-id.wx.example.com).
//
// WebRTC: emit CSP `webrtc 'block'` (Chromium honors it; other engines may ignore
// with a console warning). Combined with hostbridge.js stubs, this stops the
// classic STUN/srflx IP sidechannel from webxdc-test.
//
// WebAssembly: 'wasm-unsafe-eval' (safer than full 'unsafe-eval'). Same-origin
// .wasm fetch needs connect-src 'self'; workers need worker-src. External
// network stays blocked — no https: / * in connect-src or script-src.
const cspBase = "" +
	"default-src 'none'; " +
	"base-uri 'none'; " +
	"form-action 'none'; " +
	"frame-src 'none'; " +
	"child-src 'none'; " +
	"object-src 'none'; " +
	"worker-src 'self' blob:; " +
	"manifest-src 'none'; " +
	"media-src 'self' data: blob:; " +
	"font-src 'self' data: blob:; " +
	"img-src 'self' data: blob:; " +
	"style-src 'self' 'unsafe-inline' blob:; " +
	"script-src 'self' 'unsafe-inline' blob: 'wasm-unsafe-eval'; " +
	// data: blob: match Delta Chat Desktop — apps (e.g. OpenArena FakeWebSocket)
	// use fetch("data:application/octet-stream;base64,...") for local mock signaling
	// over joinRealtimeChannel. Still no https: / wss: / * (no real network).
	"connect-src 'self' data: blob:; " +
	"webrtc 'block'"

// DefaultCSP is the CSP with local-dev + self frame-ancestors (tests / fallback).
var DefaultCSP = BuildCSP(nil)

// PermissionsPolicy denies powerful features by default (Desktop allowlists
// only pointer lock + fullscreen at the OS webview layer).
const PermissionsPolicy = "" +
	"accelerometer=(), autoplay=(), camera=(), display-capture=(), " +
	"encrypted-media=(), fullscreen=(), geolocation=(), gyroscope=(), " +
	"magnetometer=(), microphone=(), midi=(), payment=(), " +
	"picture-in-picture=(), publickey-credentials-get=(), " +
	"screen-wake-lock=(), usb=(), web-share=(), xr-spatial-tracking=()"

// BuildCSP returns a full Content-Security-Policy for WebXDC assets.
// frameAncestors lists absolute SPA origins allowed to embed the mini-app.
// Plan 02: when webxdc host ≠ SPA host, frame-ancestors must be the SPA origin
// (not merely 'self' on the webxdc host).
func BuildCSP(frameAncestors []string) string {
	anc := make([]string, 0, len(frameAncestors)+2)
	seen := map[string]bool{}
	add := func(o string) {
		o = strings.TrimSpace(o)
		if o == "" || seen[o] {
			return
		}
		seen[o] = true
		anc = append(anc, o)
	}
	for _, a := range frameAncestors {
		add(a)
	}
	// Never empty: if misconfigured, block embedding rather than open the world.
	if len(anc) == 0 {
		add("'none'")
	}
	return cspBase + "; frame-ancestors " + strings.Join(anc, " ")
}

// SecurityHeaders returns headers that MUST be set on every WebXDC asset
// response, including 404/500 (XDC-01-002: missing CSP on errors).
// Prefer SecurityHeadersFor with the SPA origin in production.
func SecurityHeaders() map[string]string {
	return SecurityHeadersFor(nil)
}

// SecurityHeadersFor builds headers with explicit SPA origins that may iframe the app.
func SecurityHeadersFor(frameAncestors []string) map[string]string {
	return map[string]string{
		"Content-Security-Policy": BuildCSP(frameAncestors),
		"X-Content-Type-Options":  "nosniff",
		// same-origin (not no-referrer): relative CSS/JS requests send Referer with
		// the document's ?t= ticket when cookies are blocked in a cross-site iframe.
		// Cross-origin requests still get no referrer.
		"Referrer-Policy": "same-origin",
		// Parent SPA is a different origin — must not use same-origin CORP.
		"Cross-Origin-Resource-Policy": "cross-origin",
		"Permissions-Policy":           PermissionsPolicy,
		// Do NOT set X-Frame-Options: SAMEORIGIN — that blocks embedding from the
		// meeting SPA (always a different origin). CSP frame-ancestors is the control.
	}
}

// FrameAncestorsFromFrontendURL expands a frontendURL into CSP frame-ancestors.
// Local make dev also allows the Vite pair (localhost / 127.0.0.1 :7070).
func FrameAncestorsFromFrontendURL(frontendURL string) []string {
	frontendURL = strings.TrimSpace(frontendURL)
	if frontendURL == "" {
		// Safe local-dev default only — production should set auth.frontendURL.
		return []string{"http://localhost:7070", "http://127.0.0.1:7070"}
	}
	// Strip path; keep scheme://host[:port]
	u := frontendURL
	if i := strings.Index(u, "://"); i >= 0 {
		rest := u[i+3:]
		if j := strings.IndexAny(rest, "/?#"); j >= 0 {
			u = u[:i+3] + rest[:j]
		}
	}
	out := []string{u}
	// Vite HMR often uses either hostname for the same machine.
	if strings.Contains(u, "localhost") || strings.Contains(u, "127.0.0.1") {
		out = append(out, "http://localhost:7070", "http://127.0.0.1:7070")
	}
	return out
}

// CSPContainsRequiredDirectives reports whether csp includes the directives we
// treat as non-negotiable for the WebXDC privacy promise.
func CSPContainsRequiredDirectives(csp string) bool {
	needles := []string{
		"connect-src",
		"default-src 'none'",
		"object-src 'none'",
		"base-uri 'none'",
		"form-action 'none'",
		"frame-ancestors",
		"wasm-unsafe-eval", // WebAssembly (many .xdc games/tools)
		"webrtc 'block'",   // STUN/srflx IP sidechannel (Chromium)
	}
	for _, n := range needles {
		if !strings.Contains(csp, n) {
			return false
		}
	}
	// Must not open the open web for scripts or XHR.
	if strings.Contains(csp, "script-src") && strings.Contains(csp, "https:") {
		return false
	}
	if strings.Contains(csp, "connect-src *") || strings.Contains(csp, "connect-src https:") {
		return false
	}
	return true
}
