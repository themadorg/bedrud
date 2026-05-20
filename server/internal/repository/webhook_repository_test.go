package repository

import (
	"bedrud/internal/models"
	"bedrud/internal/testutil"
	"testing"
	"time"

	"github.com/google/uuid"
)

func newTestWebhook() *models.Webhook {
	events := []string{"room.created", "room.ended"}
	return &models.Webhook{
		ID:        uuid.NewString(),
		Name:      "Test Webhook",
		URL:       "https://example.com/webhook",
		Secret:    "test-secret-32-chars-long-xxxx",
		Events:    events,
		IsActive:  true,
		CreatedBy: "admin-user",
	}
}

func TestWebhookRepository_Create(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewWebhookRepository(db)

	w := newTestWebhook()
	err := repo.Create(w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify persisted
	var count int64
	db.Model(&models.Webhook{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 webhook, got %d", count)
	}
}

func TestWebhookRepository_GetByID_Found(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewWebhookRepository(db)

	w := newTestWebhook()
	_ = repo.Create(w)

	found, err := repo.GetByID(w.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found.URL != w.URL {
		t.Fatalf("expected URL '%s', got '%s'", w.URL, found.URL)
	}
	if found.Secret != w.Secret {
		t.Fatalf("expected secret '%s', got '%s'", w.Secret, found.Secret)
	}
}

func TestWebhookRepository_GetByID_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewWebhookRepository(db)

	_, err := repo.GetByID("nonexistent")
	if err != ErrWebhookNotFound {
		t.Fatalf("expected ErrWebhookNotFound, got %v", err)
	}
}

func TestWebhookRepository_List(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewWebhookRepository(db)

	_ = repo.Create(newTestWebhook())
	_ = repo.Create(newTestWebhook())
	_ = repo.Create(newTestWebhook())

	webhooks, err := repo.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(webhooks) != 3 {
		t.Fatalf("expected 3 webhooks, got %d", len(webhooks))
	}
}

func TestWebhookRepository_ListActive_ByEvent(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewWebhookRepository(db)

	// Webhook subscribed to room.created + room.ended
	w1 := newTestWebhook()
	w1.Events = []string{"room.created", "room.ended"}
	_ = repo.Create(w1)

	// Webhook subscribed to participant.joined only
	w2 := newTestWebhook()
	w2.Events = []string{"participant.joined"}
	_ = repo.Create(w2)

	// Inactive webhook — GORM ignores `false` on Create for bools with default:true,
	// so create active then update
	w3 := newTestWebhook()
	w3.Events = []string{"room.created"}
	_ = repo.Create(w3)
	db.Model(w3).Update("is_active", false)

	// Should return w1 only
	active, err := repo.ListActive("room.created")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active webhook for room.created, got %d", len(active))
	}
	if active[0].ID != w1.ID {
		t.Fatalf("expected webhook %s, got %s", w1.ID, active[0].ID)
	}
}

func TestWebhookRepository_ListActive_AllEvents(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewWebhookRepository(db)

	_ = repo.Create(newTestWebhook())
	_ = repo.Create(newTestWebhook())

	inactive := newTestWebhook()
	_ = repo.Create(inactive)
	db.Model(inactive).Update("is_active", false)

	// Empty event string returns all active regardless of subscription
	active, err := repo.ListActive("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(active) != 2 {
		t.Fatalf("expected 2 active webhooks, got %d", len(active))
	}
}

func TestWebhookRepository_Update(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewWebhookRepository(db)

	w := newTestWebhook()
	_ = repo.Create(w)

	w.Name = "Updated Name"
	w.URL = "https://updated.example.com/webhook"
	err := repo.Update(w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify persisted
	var saved models.Webhook
	db.First(&saved, "id = ?", w.ID)
	if saved.Name != "Updated Name" {
		t.Fatalf("expected name 'Updated Name', got '%s'", saved.Name)
	}
	if saved.URL != "https://updated.example.com/webhook" {
		t.Fatalf("expected updated URL, got '%s'", saved.URL)
	}
}

func TestWebhookRepository_Delete(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewWebhookRepository(db)

	w := newTestWebhook()
	_ = repo.Create(w)

	err := repo.Delete(w.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var count int64
	db.Model(&models.Webhook{}).Count(&count)
	if count != 0 {
		t.Fatalf("expected 0 webhooks after delete, got %d", count)
	}
}

func TestWebhookRepository_Delete_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewWebhookRepository(db)

	err := repo.Delete("nonexistent")
	if err != ErrWebhookNotFound {
		t.Fatalf("expected ErrWebhookNotFound, got %v", err)
	}
}

func TestWebhookRepository_UpdateSecret(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewWebhookRepository(db)

	w := newTestWebhook()
	_ = repo.Create(w)

	newSecret := "new-secret-32-chars-long-xxxxx"
	err := repo.UpdateSecret(w.ID, newSecret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var saved models.Webhook
	db.First(&saved, "id = ?", w.ID)
	if saved.Secret != newSecret {
		t.Fatalf("expected secret '%s', got '%s'", newSecret, saved.Secret)
	}
}

func TestWebhookRepository_UpdateSecret_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewWebhookRepository(db)

	err := repo.UpdateSecret("nonexistent", "new-secret")
	if err != ErrWebhookNotFound {
		t.Fatalf("expected ErrWebhookNotFound, got %v", err)
	}
}

func TestWebhookRepository_UpdateLastSeen(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewWebhookRepository(db)

	w := newTestWebhook()
	_ = repo.Create(w)

	now := time.Now()
	err := repo.UpdateLastSeen(w.ID, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var saved models.Webhook
	db.First(&saved, "id = ?", w.ID)
	if saved.LastSeen == nil {
		t.Fatal("expected LastSeen to be set")
	}
}
