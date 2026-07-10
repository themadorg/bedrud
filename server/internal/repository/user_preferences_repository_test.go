package repository

import (
	"testing"

	"bedrud/internal/testutil"
)

func TestUserPreferencesRepository_GetMissing(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserPreferencesRepository(db)

	p, err := repo.GetByUserID("missing")
	if err != nil {
		t.Fatal(err)
	}
	if p != nil {
		t.Fatalf("expected nil, got %+v", p)
	}
}

func TestUserPreferencesRepository_UpsertAndGet(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserPreferencesRepository(db)

	if err := repo.Upsert("u1", `{"theme":"dark"}`); err != nil {
		t.Fatal(err)
	}
	p, err := repo.GetByUserID("u1")
	if err != nil || p == nil {
		t.Fatalf("get: %v %+v", err, p)
	}
	if p.PreferencesJSON != `{"theme":"dark"}` {
		t.Fatalf("got %q", p.PreferencesJSON)
	}

	if err := repo.Upsert("u1", `{"theme":"light"}`); err != nil {
		t.Fatal(err)
	}
	p, err = repo.GetByUserID("u1")
	if err != nil || p == nil {
		t.Fatalf("get after upsert: %v %+v", err, p)
	}
	if p.PreferencesJSON != `{"theme":"light"}` {
		t.Fatalf("got %q", p.PreferencesJSON)
	}
}

func TestUserPreferencesRepository_TwoUserIsolation(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserPreferencesRepository(db)

	if err := repo.Upsert("a", `{"a":1}`); err != nil {
		t.Fatal(err)
	}
	if err := repo.Upsert("b", `{"b":2}`); err != nil {
		t.Fatal(err)
	}
	pa, _ := repo.GetByUserID("a")
	pb, _ := repo.GetByUserID("b")
	if pa == nil || pb == nil || pa.PreferencesJSON != `{"a":1}` || pb.PreferencesJSON != `{"b":2}` {
		t.Fatalf("isolation failed: a=%+v b=%+v", pa, pb)
	}
}

func TestUserPreferencesRepository_Delete(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserPreferencesRepository(db)

	_ = repo.Upsert("u1", `{}`)
	if err := repo.DeleteByUserID("u1"); err != nil {
		t.Fatal(err)
	}
	p, err := repo.GetByUserID("u1")
	if err != nil {
		t.Fatal(err)
	}
	if p != nil {
		t.Fatalf("expected nil after delete, got %+v", p)
	}
}
