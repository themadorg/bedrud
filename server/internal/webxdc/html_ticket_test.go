package webxdc

import (
	"strings"
	"testing"
)

func TestInjectTicketIntoHTML(t *testing.T) {
	ticket := "abc.def"
	in := []byte(`<!doctype html><script src="webxdc.js"></script>` +
		`<link href="/assets/a.css" rel="stylesheet">` +
		`<script src="assets/app.js"></script>` +
		`<a href="https://evil.example/x">x</a>` +
		`<img src="data:image/png;base64,xx">` +
		`<a href="#top">t</a>`)
	out := string(InjectTicketIntoHTML(in, ticket))
	if !strings.Contains(out, `src="webxdc.js?t=abc.def"`) && !strings.Contains(out, `src="webxdc.js?t=abc%2Edef"`) {
		// QueryEscape leaves dots as-is usually
		if !strings.Contains(out, "webxdc.js?") || !strings.Contains(out, "t=") {
			t.Fatalf("webxdc.js not ticketed: %s", out)
		}
	}
	if !strings.Contains(out, "/assets/a.css?") || !strings.Contains(out, "t=") {
		t.Fatalf("css not ticketed: %s", out)
	}
	if strings.Contains(out, "https://evil.example/x?t=") {
		t.Fatal("must not ticket absolute https")
	}
	if strings.Contains(out, "data:image") && strings.Contains(out, "data:image/png;base64,xx?t=") {
		t.Fatal("must not ticket data:")
	}
	if strings.Contains(out, `href="#top?t=`) {
		t.Fatal("must not ticket fragment-only")
	}
}

func TestInjectTicketIntoHTML_Idempotent(t *testing.T) {
	in := []byte(`<script src="webxdc.js?t=one"></script>`)
	out := string(InjectTicketIntoHTML(in, "two"))
	if strings.Contains(out, "two") {
		t.Fatalf("should not replace existing t=: %s", out)
	}
}

func TestIsHTMLEntry(t *testing.T) {
	if !IsHTMLEntry("index.html") || !IsHTMLEntry("a/B.HTM") {
		t.Fatal("expected html")
	}
	if IsHTMLEntry("a.js") {
		t.Fatal("js is not html")
	}
}
