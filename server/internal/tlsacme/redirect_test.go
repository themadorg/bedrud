package tlsacme

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPRedirectHandler(t *testing.T) {
	h := HTTPRedirectHandler()

	req := httptest.NewRequest(http.MethodGet, "http://meet.example.com/path?q=1", nil)
	req.Host = "meet.example.com"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	res := rr.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusMovedPermanently {
		t.Fatalf("status=%d, want %d", res.StatusCode, http.StatusMovedPermanently)
	}
	loc := res.Header.Get("Location")
	want := "https://meet.example.com/path?q=1"
	if loc != want {
		t.Fatalf("Location=%q, want %q", loc, want)
	}
}

// TestStartHTTPRedirect_CustomPort ensures DNS-01 redirect binds the configured
// address (not only :80) — the core of issue #65 for StartHTTPRedirect.
func TestStartHTTPRedirect_CustomPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	StartHTTPRedirect(addr)

	client := &http.Client{
		Timeout: 2 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	var resp *http.Response
	deadline := time.Now().Add(2 * time.Second)
	url := "http://" + addr + "/acme-check?x=1"
	for time.Now().Before(deadline) {
		resp, err = client.Get(url)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("GET %s after wait: %v", url, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusMovedPermanently {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusMovedPermanently)
	}
	loc := resp.Header.Get("Location")
	// Host includes the non-80 port from the request URL.
	wantPrefix := "https://" + addr + "/acme-check?x=1"
	if loc != wantPrefix {
		t.Fatalf("Location=%q, want %q", loc, wantPrefix)
	}
}
