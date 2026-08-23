package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func cfLinodeMux(t *testing.T, posts *atomic.Int32, deletes *atomic.Int32) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/zones", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"zid","name":"example.com"}]}`))
	})
	var dnsGone atomic.Bool
	mux.HandleFunc("/zones/zid/dns_records/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			deletes.Add(1)
			dnsGone.Store(true)
			_, _ = w.Write([]byte(`{"success":true,"result":{"id":"rid"}}`))
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("/zones/zid/dns_records", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			posts.Add(1)
			dnsGone.Store(false)
			b, _ := io.ReadAll(r.Body)
			var in map[string]any
			_ = json.Unmarshal(b, &in)
			if in["proxied"] != false {
				t.Errorf("proxied %v", in["proxied"])
			}
			_, _ = w.Write([]byte(`{"success":true,"result":{"id":"new","type":"A","name":"meet.example.com","content":"203.0.113.10"}}`))
			return
		}
		if dnsGone.Load() {
			_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"rid","type":"A","name":"taken.example.com","content":"1.1.1.1"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"rid","type":"A","name":"taken.example.com","content":"1.1.1.1"},{"id":"mid","type":"A","name":"meet.example.com","content":"203.0.113.10"}]}`))
	})
	var instGone atomic.Bool
	mux.HandleFunc("/linode/instances/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			deletes.Add(1)
			instGone.Store(true)
			w.WriteHeader(200)
			return
		}
		if instGone.Load() {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"id":42,"label":"meet-example-com","status":"running","ipv4":["203.0.113.10"]}`))
	})
	mux.HandleFunc("/linode/types/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"g6-standard-1","label":"Linode 2GB","price":{"hourly":0.0075,"monthly":5}}`))
	})
	mux.HandleFunc("/linode/instances", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			posts.Add(1)
			b, _ := io.ReadAll(r.Body)
			var in map[string]any
			_ = json.Unmarshal(b, &in)
			if in["booted"] != true {
				t.Errorf("booted %v", in["booted"])
			}
			_, _ = w.Write([]byte(`{"id":42,"label":"meet-example-com","status":"running","ipv4":["203.0.113.10"]}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":42,"label":"meet-example-com","status":"running","ipv4":["203.0.113.10"]}]}`))
	})
	return mux
}

func testCfg(t *testing.T, base string) Config {
	t.Helper()
	dir := t.TempDir()
	pub := filepath.Join(dir, "id.pub")
	if err := os.WriteFile(pub, []byte("ssh-ed25519 AAAA test"), 0o644); err != nil {
		t.Fatal(err)
	}
	return Config{
		LinodeToken: "t", LinodeBase: base, LinodeRegion: "de-fra-2",
		LinodeType: "g6-standard-1", LinodeImage: "linode/debian13",
		CloudflareTok: "t", CloudflareBase: base, Zone: "example.com",
		SSHIdentity: filepath.Join(dir, "id"), SSHPubKeyPath: pub,
		ConfigDir: t.TempDir(),
	}
}

func TestCmdCreateNeedsInit(t *testing.T) {
	err := cmdCreate(Config{ConfigDir: t.TempDir()}, "meet", true)
	if err == nil || !strings.Contains(err.Error(), "bedrud-host init") {
		t.Fatalf("got %v", err)
	}
}

func TestCmdCreateDryRun(t *testing.T) {
	var p, d atomic.Int32
	s := httptestServer(cfLinodeMux(t, &p, &d))
	defer s.Close()
	c := testCfg(t, s.URL)
	if err := cmdCreate(c, "fresh", true); err != nil {
		t.Fatal(err)
	}
	if p.Load() != 0 {
		t.Fatalf("dry-run must not POST, got %d", p.Load())
	}
}

func TestCmdCreatePrefixTaken(t *testing.T) {
	var p, d atomic.Int32
	s := httptestServer(cfLinodeMux(t, &p, &d))
	defer s.Close()
	c := testCfg(t, s.URL)
	err := cmdCreate(c, "taken", true)
	if err == nil || !strings.Contains(err.Error(), "already has DNS") {
		t.Fatalf("got %v", err)
	}
}

func TestCmdCreateRandomAvoidsTaken(t *testing.T) {
	var p, d atomic.Int32
	s := httptestServer(cfLinodeMux(t, &p, &d))
	defer s.Close()
	c := testCfg(t, s.URL)
	if err := cmdCreate(c, "", true); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCreateFullMockedSSH(t *testing.T) {
	var p, d atomic.Int32
	s := httptestServer(cfLinodeMux(t, &p, &d))
	defer s.Close()
	c := testCfg(t, s.URL)

	oldW, oldR := waitSSHFn, sshRunFn
	t.Cleanup(func() { waitSSHFn = oldW; sshRunFn = oldR })
	var sshArgs []string
	waitSSHFn = func(identity, ip string, tries int) error {
		if ip != "203.0.113.10" {
			t.Errorf("wait ip %s", ip)
		}
		return nil
	}
	sshRunFn = func(identity, ip, script string, args []string, timeout time.Duration) error {
		sshArgs = append([]string{}, args...)
		if !strings.Contains(script, "bedrud install") {
			t.Error("missing install")
		}
		if timeout < time.Minute {
			t.Errorf("timeout %s", timeout)
		}
		return nil
	}

	if err := cmdCreate(c, "fresh", false); err != nil {
		t.Fatal(err)
	}
	if p.Load() < 2 {
		t.Fatalf("expected linode+dns POST, got %d", p.Load())
	}
	if len(sshArgs) != 5 || sshArgs[0] != "fresh.example.com" || sshArgs[1] != "203.0.113.10" {
		t.Fatalf("ssh args %v", sshArgs)
	}
	if !strings.HasPrefix(sshArgs[2], "admin@") {
		t.Fatalf("email %s", sshArgs[2])
	}
}

func TestCmdCreateSSHFail(t *testing.T) {
	var p, d atomic.Int32
	s := httptestServer(cfLinodeMux(t, &p, &d))
	defer s.Close()
	c := testCfg(t, s.URL)
	oldW := waitSSHFn
	t.Cleanup(func() { waitSSHFn = oldW })
	waitSSHFn = func(string, string, int) error { return errors.New("ssh down") }
	err := cmdCreate(c, "fresh", false)
	if err == nil || !strings.Contains(err.Error(), "ssh down") {
		t.Fatalf("got %v", err)
	}
}
