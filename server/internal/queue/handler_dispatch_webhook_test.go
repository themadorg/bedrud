package queue

import (
	"bedrud/internal/models"
	"bedrud/internal/testutil"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func testWebhookJob(t *testing.T, url, secret, event string, body map[string]any) *models.Job {
	t.Helper()
	payload, _ := json.Marshal(WebhookPayload{
		URL:    url,
		Event:  event,
		Body:   body,
		Secret: secret,
	})
	return &models.Job{
		ID:      uuid.NewString(),
		Type:    "dispatch_webhook",
		Payload: string(payload),
	}
}

func TestDispatchWebhook_HMACMatch(t *testing.T) {
	received := make(chan struct {
		sig   string
		event string
		body  []byte
	}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		received <- struct {
			sig   string
			event string
			body  []byte
		}{
			sig:   r.Header.Get("X-Bedrud-Signature"),
			event: r.Header.Get("X-Bedrud-Event"),
			body:  body,
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	handler := NewDispatchWebhookHandler()
	ctx := context.Background()
	db := testutil.SetupTestDB(t)
	job := testWebhookJob(t, srv.URL, "test-secret", "room.created", map[string]any{"roomId": "room-1"})

	err := handler(ctx, db, job)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	select {
	case msg := <-received:
		// Verify HMAC
		mac := hmac.New(sha256.New, []byte("test-secret"))
		mac.Write(msg.body)
		expectedSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if msg.sig != expectedSig {
			t.Errorf("HMAC mismatch: got %q, expected %q", msg.sig, expectedSig)
		}
		if msg.event != "room.created" {
			t.Errorf("expected event room.created, got %q", msg.event)
		}

		// Verify envelope
		var envelope map[string]any
		json.Unmarshal(msg.body, &envelope)
		if envelope["event"] != "room.created" {
			t.Errorf("expected event room.created in body, got %v", envelope["event"])
		}
		if envelope["timestamp"] == "" {
			t.Error("expected timestamp to be set")
		}
		data, ok := envelope["data"].(map[string]any)
		if !ok || data["roomId"] != "room-1" {
			t.Errorf("expected data.roomId=room-1, got %v", data)
		}
	default:
		t.Fatal("handler did not send webhook request")
	}
}

func TestDispatchWebhook_HTTP200(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	handler := NewDispatchWebhookHandler()
	ctx := context.Background()
	db := testutil.SetupTestDB(t)
	job := testWebhookJob(t, srv.URL, "sec", "room.created", nil)

	err := handler(ctx, db, job)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !called {
		t.Fatal("expected handler to call the webhook URL")
	}
}

func TestDispatchWebhook_Non2xx(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 500, 502, 503} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
		}))

		handler := NewDispatchWebhookHandler()
		ctx := context.Background()
		db := testutil.SetupTestDB(t)
		job := testWebhookJob(t, srv.URL, "sec", "room.created", nil)

		err := handler(ctx, db, job)
		if err != nil {
			t.Fatalf("status %d: expected nil error, got %v", status, err)
		}
		srv.Close()
	}
}

func TestDispatchWebhook_NilBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var envelope map[string]any
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		json.Unmarshal(body, &envelope)

		data, ok := envelope["data"]
		if !ok {
			t.Error("expected 'data' field in envelope")
		}
		// Should be {} not null
		dataMap, ok := data.(map[string]any)
		if !ok || len(dataMap) != 0 {
			t.Errorf("expected data to be {}, got %v (type %T)", data, data)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	handler := NewDispatchWebhookHandler()
	ctx := context.Background()
	db := testutil.SetupTestDB(t)
	job := testWebhookJob(t, srv.URL, "sec", "room.created", nil)

	err := handler(ctx, db, job)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestDispatchWebhook_EmptySecret(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sig := r.Header.Get("X-Bedrud-Signature")

		// Verify HMAC with empty key produces a valid signature
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		mac := hmac.New(sha256.New, []byte(""))
		mac.Write(body)
		expectedSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if sig != expectedSig {
			t.Errorf("HMAC with empty key mismatch: got %q, expected %q", sig, expectedSig)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	handler := NewDispatchWebhookHandler()
	ctx := context.Background()
	db := testutil.SetupTestDB(t)
	job := testWebhookJob(t, srv.URL, "", "room.created", map[string]any{"roomId": "r1"})

	err := handler(ctx, db, job)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestDispatchWebhook_MalformedURL(t *testing.T) {
	handler := NewDispatchWebhookHandler()
	ctx := context.Background()
	db := testutil.SetupTestDB(t)
	job := testWebhookJob(t, "://invalid-url", "sec", "room.created", nil)

	err := handler(ctx, db, job)
	if err != nil {
		t.Fatalf("expected nil error for malformed URL, got %v", err)
	}
}

func TestDispatchWebhook_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	handler := NewDispatchWebhookHandler()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	db := testutil.SetupTestDB(t)
	job := testWebhookJob(t, srv.URL, "sec", "room.created", nil)

	err := handler(ctx, db, job)
	if err != nil {
		t.Fatalf("expected nil error on timeout, got %v", err)
	}
}

func TestDispatchWebhook_CancelledContext(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-block // never unblocked, ensures we don't send
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	close(block) // unblock immediately to allow clean shutdown, but context is already cancelled

	handler := NewDispatchWebhookHandler()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancelled
	db := testutil.SetupTestDB(t)
	job := testWebhookJob(t, srv.URL, "sec", "room.created", nil)

	err := handler(ctx, db, job)
	if err != nil {
		t.Fatalf("expected nil error on cancelled context, got %v", err)
	}
}

func TestDispatchWebhook_UnreachableHost(t *testing.T) {
	handler := NewDispatchWebhookHandler()
	ctx := context.Background()
	db := testutil.SetupTestDB(t)
	// Use a non-routable IP that will fail fast
	job := testWebhookJob(t, "http://10.255.255.1:9999/webhook", "sec", "room.created", nil)

	err := handler(ctx, db, job)
	if err != nil {
		t.Fatalf("expected nil error for unreachable host, got %v", err)
	}
}

func TestDispatchWebhook_ConcurrentDelivery(t *testing.T) {
	var mu sync.Mutex
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	handler := NewDispatchWebhookHandler()
	db := testutil.SetupTestDB(t)

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			job := testWebhookJob(t, srv.URL, "sec", "room.created", nil)
			handler(context.Background(), db, job)
		}()
	}
	wg.Wait()

	mu.Lock()
	if callCount != 10 {
		t.Fatalf("expected 10 deliveries, got %d", callCount)
	}
	mu.Unlock()
}
