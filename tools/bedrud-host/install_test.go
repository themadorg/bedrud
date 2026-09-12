package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoteInstallScript(t *testing.T) {
	s := remoteInstallScript()
	for _, want := range []string{
		"bedrud install --no-tls --behind-proxy",
		"certbot --nginx",
		"proxy_pass http://127.0.0.1:7880/",
		"bedrud user create",
		"--admin",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("script missing %q", want)
		}
	}
}

func TestReadPubKey(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "id.pub")
	if err := os.WriteFile(p, []byte("ssh-ed25519 AAAA demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := readPubKey(p)
	if err != nil {
		t.Fatal(err)
	}
	if s != "ssh-ed25519 AAAA demo" {
		t.Fatalf("%q", s)
	}
	if _, err := readPubKey(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("expected error")
	}
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, []byte("  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readPubKey(empty); err == nil {
		t.Fatal("expected empty error")
	}
}

func TestCmdDeleteDryRun(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/zones", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"zid","name":"example.com"}]}`))
	})
	mux.HandleFunc("/zones/zid/dns_records", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"rid","type":"A","name":"meet.example.com","content":"203.0.113.10"}]}`))
	})
	mux.HandleFunc("/linode/instances", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":5,"label":"meet-example-com","status":"running","ipv4":["203.0.113.10"]}]}`))
	})
	s := httptestServer(mux)
	defer s.Close()
	c := Config{
		LinodeToken: "t", LinodeBase: s.URL,
		CloudflareTok: "t", CloudflareBase: s.URL, Zone: "example.com",
		ConfigDir: t.TempDir(),
	}
	if err := cmdDelete(c, "meet.example.com", false, true); err != nil {
		t.Fatal(err)
	}
	err := cmdDelete(c, "meet.example.com", false, false)
	if err == nil || !strings.Contains(err.Error(), "-yes") {
		t.Fatalf("expected refuse without -yes when not a TTY, got %v", err)
	}
}
