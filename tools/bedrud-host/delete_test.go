package main

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fastPoll(t *testing.T) {
	t.Helper()
	old := linodePoll
	linodePoll = time.Millisecond
	t.Cleanup(func() { linodePoll = old })
}

func TestCmdDeleteInteractiveYes(t *testing.T) {
	fastPoll(t)
	var posts, deletes atomic.Int32
	s := httptestServer(cfLinodeMux(t, &posts, &deletes))
	defer s.Close()
	c := testCfg(t, s.URL)
	oldIn, oldErr := promptIn, promptErr
	promptIn = strings.NewReader("yes\n")
	promptErr = io.Discard
	t.Cleanup(func() { promptIn = oldIn; promptErr = oldErr })
	if err := cmdDelete(c, "meet.example.com", false, false); err != nil {
		t.Fatal(err)
	}
	if deletes.Load() < 1 {
		t.Fatalf("expected delete after yes, got %d", deletes.Load())
	}
}

func TestCmdDeleteInteractiveNo(t *testing.T) {
	fastPoll(t)
	var posts, deletes atomic.Int32
	s := httptestServer(cfLinodeMux(t, &posts, &deletes))
	defer s.Close()
	c := testCfg(t, s.URL)
	oldIn, oldErr := promptIn, promptErr
	promptIn = strings.NewReader("no\n")
	promptErr = io.Discard
	t.Cleanup(func() { promptIn = oldIn; promptErr = oldErr })
	if err := cmdDelete(c, "meet.example.com", false, false); err != nil {
		t.Fatal(err)
	}
	if deletes.Load() != 0 {
		t.Fatalf("cancelled must not delete, got %d", deletes.Load())
	}
}

func TestCmdDeleteByLocalID(t *testing.T) {
	fastPoll(t)
	var posts, deletes atomic.Int32
	s := httptestServer(cfLinodeMux(t, &posts, &deletes))
	defer s.Close()
	c := testCfg(t, s.URL)
	if err := cmdRecord(c, recordArgs{
		Host: "meet.example.com", IPv4: "203.0.113.10", LinodeID: 42,
	}); err != nil {
		t.Fatal(err)
	}
	st, err := openStore(c.ConfigDir)
	if err != nil {
		t.Fatal(err)
	}
	h, ok, err := st.Get("meet.example.com")
	_ = st.Close()
	if err != nil || !ok {
		t.Fatal(err)
	}
	if err := cmdDelete(c, strconv.Itoa(h.ID), true, false); err != nil {
		t.Fatal(err)
	}
	st2, err := openStore(c.ConfigDir)
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	_, ok, err = st2.Get("meet.example.com")
	if err != nil || ok {
		t.Fatal("expected removed")
	}
}

func TestCmdDeleteYes(t *testing.T) {
	fastPoll(t)
	var posts, deletes atomic.Int32
	s := httptestServer(cfLinodeMux(t, &posts, &deletes))
	defer s.Close()
	c := testCfg(t, s.URL)
	if err := cmdDelete(c, "meet", true, false); err != nil {
		t.Fatal(err)
	}
	if deletes.Load() < 2 {
		t.Fatalf("expected dns+linode delete, got %d", deletes.Load())
	}
}

func TestCmdDeleteNothing(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/zones", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"zid","name":"example.com"}]}`))
	})
	mux.HandleFunc("/zones/zid/dns_records", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"result":[]}`))
	})
	mux.HandleFunc("/linode/instances", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	s := httptestServer(mux)
	defer s.Close()
	c := testCfg(t, s.URL)
	err := cmdDelete(c, "ghost", true, false)
	if err == nil || !strings.Contains(err.Error(), "nothing to delete") {
		t.Fatalf("got %v", err)
	}
}

func TestCmdDeleteByIPWhenLabelDiffers(t *testing.T) {
	fastPoll(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/zones", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"zid","name":"example.com"}]}`))
	})
	dnsGone := false
	mux.HandleFunc("/zones/zid/dns_records", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if dnsGone {
			_, _ = w.Write([]byte(`{"success":true,"result":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"rid","type":"A","name":"meet.example.com","content":"198.51.100.9"}]}`))
	})
	mux.HandleFunc("/zones/zid/dns_records/rid", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		dnsGone = true
		_, _ = w.Write([]byte(`{"success":true,"result":{"id":"rid"}}`))
	})
	mux.HandleFunc("/linode/instances", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":8,"label":"other-name","status":"running","ipv4":["198.51.100.9"]}]}`))
	})
	deleted := false
	mux.HandleFunc("/linode/instances/8", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = true
			w.WriteHeader(200)
			return
		}
		if deleted {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"id":8,"label":"other-name","status":"running","ipv4":["198.51.100.9"]}`))
	})
	s := httptestServer(mux)
	defer s.Close()
	c := testCfg(t, s.URL)
	if err := cmdDelete(c, "meet.example.com", true, false); err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Fatal("expected linode delete by ipv4")
	}
}
