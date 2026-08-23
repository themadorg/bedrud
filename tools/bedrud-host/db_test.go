package main

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "bedrud-host-exe")
	if err != nil {
		os.Exit(1)
	}
	p := filepath.Join(dir, "bedrud-host")
	if err := os.WriteFile(p, []byte("FAKE-ELF"), 0o755); err != nil {
		os.Exit(1)
	}
	selfPath = func() (string, error) { return p, nil }
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func TestRecordCreateTimeAverage(t *testing.T) {
	dir := t.TempDir()
	st, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	avg, n, err := st.recordCreateTime(10 * time.Second)
	if err != nil || n != 1 || avg != 10*time.Second {
		t.Fatalf("first %s n=%d %v", avg, n, err)
	}
	avg, n, err = st.recordCreateTime(20 * time.Second)
	if err != nil || n != 2 || avg != 15*time.Second {
		t.Fatalf("avg %s n=%d %v", avg, n, err)
	}
	got, n2, err := st.createStats()
	if err != nil || n2 != 2 || got != 15*time.Second {
		t.Fatalf("stats %s n=%d %v", got, n2, err)
	}
	if g := fmtDuration(90 * time.Second); g != "1m30s" {
		t.Fatalf("fmt %s", g)
	}
}

func TestEncryptRoundtrip(t *testing.T) {
	key := deriveKey("test-pass")
	plain := []byte("hello sqlite")
	blob, err := encryptBytes(key, plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(blob, plain) {
		t.Fatal("ciphertext equals plaintext")
	}
	got, err := decryptBytes(key, blob)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("%q", got)
	}
	if _, err := decryptBytes(deriveKey("other"), blob); err == nil {
		t.Fatal("expected wrong key to fail")
	}
}

func TestStoreUpsertListDelete(t *testing.T) {
	dir := t.TempDir()
	st, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Upsert(HostRow{
		FQDN: "meet.example.com", Host: "meet", Zone: "example.com",
		IPv4: "203.0.113.10", LinodeID: 9, LinodeLabel: "meet-example-com",
		AdminName: "admin-meet", AdminEmail: "admin@meet.example.com",
		AdminPassword: "secret",
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	enc := filepath.Join(dir, "hosts.sqlite.enc")
	if _, err := os.Stat(enc); err != nil {
		t.Fatal("missing encrypted db")
	}
	for _, n := range []string{"hosts.sqlite", "hosts.sqlite-wal", "hosts.sqlite-shm", "hosts.sqlite-journal"} {
		if _, err := os.Stat(filepath.Join(dir, n)); !os.IsNotExist(err) {
			t.Fatalf("plaintext leftover %s", n)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "db.key")); !os.IsNotExist(err) {
		t.Fatal("must not write db.key")
	}

	st2, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	h, ok, err := st2.Get("meet.example.com")
	if err != nil || !ok {
		t.Fatalf("%v %v", ok, err)
	}
	if h.LinodeID != 9 || h.AdminPassword != "secret" {
		t.Fatalf("%+v", h)
	}
	if h.ID == 0 {
		t.Fatal("expected row id")
	}
	byID, ok, err := st2.GetByID(h.ID)
	if err != nil || !ok || byID.FQDN != h.FQDN {
		t.Fatalf("GetByID %+v %v %v", byID, ok, err)
	}
	list, err := st2.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("%v %v", list, err)
	}
	if err := st2.Delete("meet.example.com"); err != nil {
		t.Fatal(err)
	}
	_, ok, err = st2.Get("meet.example.com")
	if err != nil || ok {
		t.Fatal("expected gone")
	}
}

func TestOpenStoreWrongKey(t *testing.T) {
	dir := t.TempDir()
	st, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Upsert(HostRow{FQDN: "a.example.com", Host: "a", Zone: "example.com"}); err != nil {
		t.Fatal(err)
	}
	_ = st.Close()

	other := filepath.Join(t.TempDir(), "other-bin")
	if err := os.WriteFile(other, []byte("OTHER"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := selfPath
	selfPath = func() (string, error) { return other, nil }
	t.Cleanup(func() { selfPath = old })
	if _, err := openStore(dir); err == nil {
		t.Fatal("expected decrypt failure with a different binary key")
	}
}

func TestLoadOrCreateKeyAppendsToBinary(t *testing.T) {
	p := filepath.Join(t.TempDir(), "app")
	body := []byte("ELF-PLACEHOLDER")
	if err := os.WriteFile(p, body, 0o755); err != nil {
		t.Fatal(err)
	}
	old := selfPath
	selfPath = func() (string, error) { return p, nil }
	t.Cleanup(func() { selfPath = old })

	k1, err := loadOrCreateKey()
	if err != nil || len(k1) != 32 {
		t.Fatalf("%v %d", err, len(k1))
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != len(body)+trailerSize {
		t.Fatalf("want len %d got %d", len(body)+trailerSize, len(raw))
	}
	if !bytes.HasSuffix(raw, []byte(keyMagic)) {
		t.Fatal("missing trailer magic")
	}
	if !bytes.HasPrefix(raw, body) {
		t.Fatal("original bytes must be preserved")
	}
	k2, err := loadOrCreateKey()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(k1, k2) {
		t.Fatal("second load must not mint a new key")
	}
	raw2, _ := os.ReadFile(p)
	if len(raw2) != len(raw) {
		t.Fatal("must not append twice")
	}
}

func TestCmdInit(t *testing.T) {
	dir := t.TempDir()
	c := Config{ConfigDir: dir, CloudflareBase: "http://127.0.0.1:1"}
	err := cmdInit(c, initArgs{})
	if err == nil {
		t.Fatal("expected missing creds")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/zones", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"zid123","name":"example.com"}]}`))
	})
	s := httptestServer(mux)
	defer s.Close()
	c.CloudflareBase = s.URL
	if err := cmdInit(c, initArgs{
		LinodeToken: "linode-secret-token",
		CFToken:     "cf-secret-token",
		CFDomain:    "example.com",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "hosts.sqlite.enc")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "db.key")); !os.IsNotExist(err) {
		t.Fatal("must not write db.key")
	}
	st, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	v, ok, err := st.GetSetting(setLinodeToken)
	if err != nil || !ok || v != "linode-secret-token" {
		t.Fatalf("linode token %q %v %v", v, ok, err)
	}
	v, ok, err = st.GetSetting(setCFZoneID)
	if err != nil || !ok || v != "zid123" {
		t.Fatalf("zone id %q %v %v", v, ok, err)
	}
}

func TestCmdInitInteractive(t *testing.T) {
	dir := t.TempDir()
	mux := http.NewServeMux()
	mux.HandleFunc("/zones", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"zid-int","name":"example.com"}]}`))
	})
	s := httptestServer(mux)
	defer s.Close()
	oldIn, oldErr := promptIn, promptErr
	promptIn = strings.NewReader("tok-linode\ntok-cf\nexample.com\n\n")
	promptErr = io.Discard
	t.Cleanup(func() { promptIn = oldIn; promptErr = oldErr })
	c := Config{ConfigDir: dir, CloudflareBase: s.URL}
	if err := cmdInit(c, initArgs{}); err != nil {
		t.Fatal(err)
	}
	st, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	v, ok, err := st.GetSetting(setLinodeToken)
	if err != nil || !ok || v != "tok-linode" {
		t.Fatalf("%q %v %v", v, ok, err)
	}
}

func TestCmdListEmpty(t *testing.T) {
	c := Config{ConfigDir: t.TempDir()}
	if err := cmdList(c); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultConfigDir(t *testing.T) {
	t.Setenv("BEDRUD_HOST_CONFIG_DIR", "/tmp/bh-test")
	if g := defaultConfigDir("/home/x"); g != "/tmp/bh-test" {
		t.Fatalf("%s", g)
	}
	t.Setenv("BEDRUD_HOST_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if g := defaultConfigDir("/home/x"); g != "/xdg/bedrud-host" {
		t.Fatalf("%s", g)
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	if g := defaultConfigDir("/home/x"); g != "/home/x/.config/bedrud-host" {
		t.Fatalf("%s", g)
	}
}

func TestCmdDeleteUsesDBLinodeID(t *testing.T) {
	dir := t.TempDir()
	st, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Upsert(HostRow{
		FQDN: "meet.example.com", Host: "meet", Zone: "example.com",
		IPv4: "203.0.113.10", LinodeID: 42, LinodeLabel: "meet-example-com",
	}); err != nil {
		t.Fatal(err)
	}
	_ = st.Close()

	var posts, deletes atomic.Int32
	s := httptestServer(cfLinodeMux(t, &posts, &deletes))
	defer s.Close()
	c := testCfg(t, s.URL)
	c.ConfigDir = dir
	if err := cmdDelete(c, "meet", true, false); err != nil {
		t.Fatal(err)
	}
	st2, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	_, ok, err := st2.Get("meet.example.com")
	if err != nil || ok {
		t.Fatal("row should be removed")
	}
}

func TestCmdRecordViewAdmin(t *testing.T) {
	dir := t.TempDir()
	c := Config{ConfigDir: dir}
	err := cmdRecord(c, recordArgs{
		Host: "meet.example.com", IPv4: "203.0.113.10", LinodeID: 7,
		AdminName: "admin-meet", AdminEmail: "admin@meet.example.com", AdminPassword: "pw",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := cmdView(c, "meet.example.com"); err != nil {
		t.Fatal(err)
	}
	st, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	h, ok, err := st.Get("meet.example.com")
	_ = st.Close()
	if err != nil || !ok || h.ID == 0 {
		t.Fatalf("id %+v %v %v", h, ok, err)
	}
	if err := cmdView(c, strconv.Itoa(h.ID)); err != nil {
		t.Fatal(err)
	}
	if err := cmdAdmin(c, strconv.Itoa(h.ID)); err != nil {
		t.Fatal(err)
	}
	if err := cmdAdmin(c, "meet.example.com"); err != nil {
		t.Fatal(err)
	}
	if err := cmdList(c); err != nil {
		t.Fatal(err)
	}
	if err := cmdView(c, "missing.example.com"); err == nil {
		t.Fatal("expected missing")
	}
}
