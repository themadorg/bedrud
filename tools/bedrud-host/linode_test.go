package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateAndGetLinode(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/linode/instances", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			b, _ := io.ReadAll(r.Body)
			var in map[string]any
			_ = json.Unmarshal(b, &in)
			if in["region"] != "de-fra-2" {
				t.Errorf("region %v", in["region"])
			}
			if in["label"] != "meet-example-com" {
				t.Errorf("label %v", in["label"])
			}
			_, _ = w.Write([]byte(`{"id":99,"label":"meet-example-com","status":"provisioning","ipv4":["203.0.113.10"]}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":99,"label":"meet-example-com","status":"running","ipv4":["203.0.113.10"]}]}`))
	})
	mux.HandleFunc("/linode/instances/99", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			w.WriteHeader(200)
			return
		}
		_, _ = w.Write([]byte(`{"id":99,"label":"meet-example-com","status":"running","ipv4":["203.0.113.10"]}`))
	})
	s := httptest.NewServer(mux)
	defer s.Close()
	c := Config{LinodeToken: "t", LinodeBase: s.URL, LinodeRegion: "de-fra-2", LinodeType: "g6-standard-1", LinodeImage: "linode/debian13"}

	inst, err := createLinode(c, "meet-example-com", "RootPass1234!x", []string{"ssh-ed25519 AAAA"})
	if err != nil {
		t.Fatal(err)
	}
	if inst.ID != 99 || inst.IPv4[0] != "203.0.113.10" {
		t.Fatalf("%+v", inst)
	}
	got, err := getLinode(c, 99)
	if err != nil || got.Status != "running" {
		t.Fatalf("%+v %v", got, err)
	}
	list, err := listLinodes(c)
	if err != nil || len(list) != 1 {
		t.Fatalf("%v %v", list, err)
	}
	found, ok, err := findLinodeByLabelOrIP(c, "meet-example-com", "")
	if err != nil || !ok || found.ID != 99 {
		t.Fatalf("find %+v %v %v", found, ok, err)
	}
	if err := deleteLinode(c, 99); err != nil {
		t.Fatal(err)
	}
}

func TestLinodeTypePrice(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/types/g6-standard-1" {
			t.Errorf("path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"g6-standard-1","price":{"hourly":0.0075,"monthly":5}}`))
	}))
	defer s.Close()
	c := Config{LinodeToken: "t", LinodeBase: s.URL}
	h, m, err := linodeTypePrice(c, "g6-standard-1")
	if err != nil || h != 0.0075 || m != 5 {
		t.Fatalf("%v %v %v", h, m, err)
	}
	s2 := fmtUSDRate(0.0075, 5, "g6-standard-1")
	if s2 != "$0.0075/hr  (~$5.00/mo, g6-standard-1)" {
		t.Fatalf("%q", s2)
	}
}

func TestWaitLinodeIP(t *testing.T) {
	old := linodePoll
	linodePoll = 0
	t.Cleanup(func() { linodePoll = old })
	n := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		n++
		if n == 1 {
			_, _ = w.Write([]byte(`{"id":1,"status":"provisioning","ipv4":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":1,"status":"running","ipv4":["203.0.113.9"]}`))
	}))
	defer s.Close()
	c := Config{LinodeToken: "t", LinodeBase: s.URL}
	// shrink sleep by using 2 tries; waitLinodeIP sleeps 5s between - too slow.
	// call get twice via wait with tries=1 first empty then we test get only.
	ip, err := waitLinodeIP(c, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if ip != "203.0.113.9" {
		t.Fatalf("ip %s", ip)
	}
}

func TestWaitLinodeIPNone(t *testing.T) {
	old := linodePoll
	linodePoll = 0
	t.Cleanup(func() { linodePoll = old })
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1,"status":"provisioning","ipv4":[]}`))
	}))
	defer s.Close()
	c := Config{LinodeToken: "t", LinodeBase: s.URL}
	_, err := waitLinodeIP(c, 1, 2)
	if err == nil {
		t.Fatal("expected timeout")
	}
}

func TestFindLinodeByIP(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":3,"label":"x","ipv4":["198.51.100.1"]}]}`))
	}))
	defer s.Close()
	c := Config{LinodeToken: "t", LinodeBase: s.URL}
	inst, ok, err := findLinodeByLabelOrIP(c, "nope", "198.51.100.1")
	if err != nil || !ok || inst.ID != 3 {
		t.Fatalf("%+v %v %v", inst, ok, err)
	}
	_, ok, err = findLinodeByLabelOrIP(c, "nope", "0.0.0.0")
	if err != nil || ok {
		t.Fatalf("expected not found")
	}
}
