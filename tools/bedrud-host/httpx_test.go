package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCfHeadersBearerVsKey(t *testing.T) {
	c := Config{CloudflareTok: "tok"}
	h := c.cfHeaders()
	if h["Authorization"] != "Bearer tok" {
		t.Fatalf("%v", h)
	}
	c = Config{CloudflareEmail: "a@b.c", CloudflareKey: "glob"}
	h = c.cfHeaders()
	if h["X-Auth-Email"] != "a@b.c" || h["X-Auth-Key"] != "glob" {
		t.Fatalf("%v", h)
	}
}

func TestMaskSecret(t *testing.T) {
	if maskSecret("") != "(not set)" {
		t.Fatal("empty")
	}
	if g := maskSecret("abcdefghij"); g != "****ghij" {
		t.Fatalf("%s", g)
	}
}

func TestDoJSONOK(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer x" {
			t.Errorf("auth %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":7}`))
	}))
	defer s.Close()
	var out struct {
		ID int `json:"id"`
	}
	err := doJSON("GET", s.URL+"/x", map[string]string{"Authorization": "Bearer x"}, nil, &out)
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != 7 {
		t.Fatalf("id %d", out.ID)
	}
}

func TestDoJSONError(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer s.Close()
	err := doJSON("GET", s.URL, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDoJSONPostAndEmpty(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("ct %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(204)
	}))
	defer s.Close()
	if err := doJSON("POST", s.URL, nil, map[string]string{"a": "b"}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestDoJSONBadURL(t *testing.T) {
	err := doJSON("GET", "http://127.0.0.1:1/", nil, nil, nil)
	if err == nil {
		t.Fatal("expected network error")
	}
}
