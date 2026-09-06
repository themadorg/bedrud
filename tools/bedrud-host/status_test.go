package main

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestCmdStatusMissing(t *testing.T) {
	c := Config{ConfigDir: t.TempDir()}
	err := cmdStatus(c, false)
	if err == nil {
		t.Fatal("expected failed status")
	}
}

func TestCmdStatusOK(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer lt" {
			t.Errorf("linode auth %s", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"username":"u"}`))
	})
	mux.HandleFunc("/zones", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"zid","name":"example.com"}]}`))
	})
	s := httptestServer(mux)
	defer s.Close()
	dir := t.TempDir()
	st, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = st.SetSetting(setLinodeToken, "lt")
	_ = st.SetSetting(setCFToken, "ct")
	_ = st.SetSetting(setCFDomain, "example.com")
	_ = st.SetSetting(setCFZoneID, "zid")
	_ = st.Close()
	c := Config{
		ConfigDir: dir, LinodeToken: "lt", LinodeBase: s.URL,
		CloudflareTok: "ct", CloudflareBase: s.URL, Zone: "example.com", ZoneID: "zid",
	}
	if err := cmdStatus(c, false); err != nil {
		t.Fatal(err)
	}
}

func TestCmdStatusLocalSkipsAPI(t *testing.T) {
	dir := t.TempDir()
	st, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = st.SetSetting(setLinodeToken, "lt")
	_ = st.SetSetting(setCFToken, "ct")
	_ = st.SetSetting(setCFDomain, "example.com")
	_ = st.Close()
	c := Config{
		ConfigDir: dir, LinodeToken: "lt", LinodeBase: "http://127.0.0.1:1",
		CloudflareTok: "ct", CloudflareBase: "http://127.0.0.1:1", Zone: "example.com",
	}
	if err := cmdStatus(c, true); err != nil {
		t.Fatal(err)
	}
}

func TestPingLinodeFail(t *testing.T) {
	s := httptestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer s.Close()
	c := Config{LinodeToken: "x", LinodeBase: s.URL}
	if err := pingLinode(c); err == nil {
		t.Fatal("expected error")
	}
}

func TestStatusBinaryKeyOnFakeExe(t *testing.T) {
	p, err := selfPath()
	if err != nil {
		t.Fatal(err)
	}
	_, ok, err := readKeyTrailer(p)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("test fake exe should have a key after store opens")
	}
	if _, err := os.Stat(filepath.Clean(p)); err != nil {
		t.Fatal(err)
	}
}
