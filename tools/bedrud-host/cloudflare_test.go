package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCloudflareDNS(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/zones", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("name") != "example.com" {
			t.Errorf("zone name %q", r.URL.Query().Get("name"))
		}
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"zid","name":"example.com"}]}`))
	})
	mux.HandleFunc("/zones/zid/dns_records", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			b, _ := io.ReadAll(r.Body)
			var in map[string]any
			_ = json.Unmarshal(b, &in)
			if in["type"] != "A" || in["content"] != "203.0.113.10" {
				t.Errorf("body %s", b)
			}
			_, _ = w.Write([]byte(`{"success":true,"result":{"id":"rid","type":"A","name":"meet.example.com","content":"203.0.113.10"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"rid","type":"A","name":"meet.example.com","content":"203.0.113.10"}]}`))
	})
	mux.HandleFunc("/zones/zid/dns_records/rid", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodDelete {
			t.Errorf("method %s", r.Method)
		}
		_, _ = w.Write([]byte(`{"success":true,"result":{"id":"rid"}}`))
	})
	s := httptest.NewServer(mux)
	defer s.Close()
	c := Config{CloudflareTok: "t", CloudflareBase: s.URL, Zone: "example.com"}

	zid, err := lookupZoneID(c)
	if err != nil || zid != "zid" {
		t.Fatalf("%s %v", zid, err)
	}
	rec, err := createADNS(c, zid, "meet.example.com", "203.0.113.10")
	if err != nil || rec.ID != "rid" {
		t.Fatalf("%+v %v", rec, err)
	}
	list, err := listDNS(c, zid)
	if err != nil || len(list) != 1 {
		t.Fatalf("%v %v", list, err)
	}
	if err := deleteDNS(c, zid, "rid"); err != nil {
		t.Fatal(err)
	}
}

func TestCfCheckFail(t *testing.T) {
	env := cfEnvelope[int]{Success: false, Errors: []cfErr{{Message: "bad token"}}}
	err := cfCheck(env)
	if err == nil || !strings.Contains(err.Error(), "bad token") {
		t.Fatalf("%v", err)
	}
	if err := cfCheck(cfEnvelope[int]{Success: false}); err == nil {
		t.Fatal("expected unsuccessful")
	}
	if err := cfCheck(cfEnvelope[int]{Success: true}); err != nil {
		t.Fatal(err)
	}
}

func TestLookupZoneNotFound(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"result":[]}`))
	}))
	defer s.Close()
	c := Config{CloudflareTok: "t", CloudflareBase: s.URL, Zone: "missing.example"}
	_, err := lookupZoneID(c)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("got %v", err)
	}
}

func TestCreateADNSUnsuccessful(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":false,"errors":[{"message":"record exists"}]}`))
	}))
	defer s.Close()
	c := Config{CloudflareTok: "t", CloudflareBase: s.URL}
	_, err := createADNS(c, "zid", "meet.example.com", "1.2.3.4")
	if err == nil || !strings.Contains(err.Error(), "record exists") {
		t.Fatalf("got %v", err)
	}
}
