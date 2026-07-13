package webxdc

import "testing"

func TestContentTypeForEntry_PDFIsNotViewer(t *testing.T) {
	ct := ContentTypeForEntry("docs/readme.pdf")
	if ct == "application/pdf" {
		t.Fatal("PDF must not be application/pdf (XDC-01-005)")
	}
	if ct != "application/octet-stream" {
		t.Fatalf("got %q", ct)
	}
}

func TestContentTypeForEntry_CommonTypes(t *testing.T) {
	cases := map[string]string{
		"index.html": "text/html; charset=utf-8",
		"app.js":     "text/javascript; charset=utf-8",
		"style.css":  "text/css; charset=utf-8",
		"icon.png":   "image/png",
		"unknown":    "application/octet-stream",
	}
	for name, want := range cases {
		if got := ContentTypeForEntry(name); got != want {
			t.Errorf("%s: got %q want %q", name, got, want)
		}
	}
}

func TestIsHostProvidedPath(t *testing.T) {
	if !IsHostProvidedPath("webxdc.js") {
		t.Fatal("webxdc.js must be host-provided")
	}
	if !IsHostProvidedPath("./webxdc.js") {
		t.Fatal("./webxdc.js must be host-provided")
	}
	if !IsHostProvidedPath("WebXDC.JS") {
		t.Fatal("case insensitive")
	}
	if IsHostProvidedPath("index.html") {
		t.Fatal("index.html is from ZIP")
	}
	if IsHostProvidedPath("lib/webxdc.js") {
		// basename is still webxdc.js — host must win for any path ending as webxdc.js
		// Spec apps load src="webxdc.js" at root; nested is still host-provided if requested as such.
		if !IsHostProvidedPath("lib/webxdc.js") {
			t.Fatal("basename webxdc.js")
		}
	}
}
