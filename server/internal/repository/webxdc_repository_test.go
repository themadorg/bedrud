package repository

import (
	"testing"
	"time"

	"bedrud/internal/models"
	"bedrud/internal/testutil"

	"github.com/google/uuid"
)

func TestWebxdcRepository_PackageInstanceStatusCascade(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewWebxdcRepository(db)

	roomID := uuid.New().String()
	pkgID := uuid.New().String()
	if err := repo.CreatePackage(&models.WebxdcPackage{
		ID: pkgID, RoomID: roomID, ContentHash: "abc", StorageKey: roomID + "/p.xdc",
		SizeBytes: 10, Name: "demo", UploadedBy: "u1",
	}); err != nil {
		t.Fatal(err)
	}

	instID := "a1b2c3d4e5f67890"
	if err := repo.CreateInstance(&models.WebxdcInstance{
		ID: instID, RoomID: roomID, PackageID: pkgID, CreatedBy: "u1",
	}); err != nil {
		t.Fatal(err)
	}

	serial, err := repo.NextSerial(instID)
	if err != nil || serial != 1 {
		t.Fatalf("serial=%d err=%v", serial, err)
	}
	if err := repo.AppendStatusUpdate(&models.WebxdcStatusUpdate{
		InstanceID: instID, Serial: 1, PayloadJSON: `{"payload":1}`, ByteSize: 12,
	}); err != nil {
		t.Fatal(err)
	}
	serial2, err := repo.NextSerial(instID)
	if err != nil || serial2 != 2 {
		t.Fatalf("serial2=%d err=%v", serial2, err)
	}

	list, maxS, err := repo.ListStatusUpdatesAfter(instID, 0, 10)
	if err != nil || len(list) != 1 || maxS != 1 {
		t.Fatalf("list=%d max=%d err=%v", len(list), maxS, err)
	}

	if err := repo.CloseInstance(instID); err != nil {
		t.Fatal(err)
	}
	inst, err := repo.GetInstance(instID)
	if err != nil || inst.ClosedAt == nil {
		t.Fatal("expected closed")
	}

	open, err := repo.ListInstancesByRoom(roomID, false)
	if err != nil || len(open) != 0 {
		t.Fatalf("open=%d", len(open))
	}
	all, err := repo.ListInstancesByRoom(roomID, true)
	if err != nil || len(all) != 1 {
		t.Fatalf("all=%d", len(all))
	}

	keys, err := repo.DeleteAllForRoom(roomID)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0] != roomID+"/p.xdc" {
		t.Fatalf("keys=%v", keys)
	}
	if _, err := repo.GetPackage(pkgID); !IsNotFound(err) {
		t.Fatalf("package should be gone: %v", err)
	}
}

func TestWebxdcRepository_TrimStatusLog(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewWebxdcRepository(db)
	instID := "trimtestinstance01"
	roomID := uuid.New().String()
	pkgID := uuid.New().String()
	_ = repo.CreatePackage(&models.WebxdcPackage{
		ID: pkgID, RoomID: roomID, ContentHash: "h", StorageKey: "k", SizeBytes: 1, Name: "n", UploadedBy: "u",
	})
	_ = repo.CreateInstance(&models.WebxdcInstance{ID: instID, RoomID: roomID, PackageID: pkgID, CreatedBy: "u"})

	for i := int64(1); i <= 5; i++ {
		if err := repo.AppendStatusUpdate(&models.WebxdcStatusUpdate{
			InstanceID: instID, Serial: i, PayloadJSON: `{"payload":true}`, ByteSize: 10,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.TrimStatusLog(instID, 2); err != nil {
		t.Fatal(err)
	}
	list, maxS, err := repo.ListStatusUpdatesAfter(instID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || maxS != 5 {
		t.Fatalf("len=%d max=%d", len(list), maxS)
	}
	if list[0].Serial != 4 || list[1].Serial != 5 {
		t.Fatalf("kept wrong serials: %d %d", list[0].Serial, list[1].Serial)
	}
}

func TestWebxdcRepository_CloseAllInstancesInRoom(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewWebxdcRepository(db)
	roomID := uuid.New().String()
	pkgID := uuid.New().String()
	_ = repo.CreatePackage(&models.WebxdcPackage{
		ID: pkgID, RoomID: roomID, ContentHash: "h", StorageKey: "k", SizeBytes: 1, Name: "n", UploadedBy: "u",
	})
	for _, id := range []string{"instaaaaaaaaaaaaaa", "instbbbbbbbbbbbbbb"} {
		if err := repo.CreateInstance(&models.WebxdcInstance{
			ID: id, RoomID: roomID, PackageID: pkgID, CreatedBy: "u",
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.CloseAllInstancesInRoom(roomID); err != nil {
		t.Fatal(err)
	}
	n, err := repo.CountOpenInstances(roomID)
	if err != nil || n != 0 {
		t.Fatalf("open=%d err=%v", n, err)
	}
	// second close is fine
	_ = time.Now()
	if err := repo.CloseAllInstancesInRoom(roomID); err != nil {
		t.Fatal(err)
	}
}
