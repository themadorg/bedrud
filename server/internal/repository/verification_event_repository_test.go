package repository

import (
	"testing"

	"bedrud/internal/models"
	"bedrud/internal/testutil"
)

func TestVerificationEventRepository_RecordAndList(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewVerificationEventRepository(db)

	if err := repo.RecordEvent("u1", "a@ex.com", models.VerificationSent, "1.2.3.4", "meta"); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordEvent("u2", "b@ex.com", models.VerificationSuccess, "", ""); err != nil {
		t.Fatal(err)
	}

	recent, err := repo.GetRecentEvents(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 2 {
		t.Fatalf("expected 2 recent, got %d", len(recent))
	}

	byUser, err := repo.GetEventsByUser("u1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(byUser) != 1 || byUser[0].Email != "a@ex.com" {
		t.Fatalf("by user: %+v", byUser)
	}
}
