package services

import (
	"os"
	"path/filepath"
	"testing"

	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"

	"github.com/google/uuid"
)

func TestRoomCleanupService_WebxdcCascade(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	webxdcRepo := repository.NewWebxdcRepository(db)
	dir := t.TempDir()

	userID := uuid.New().String()
	db.Create(&models.User{
		ID: userID, Email: "c@ex.com", Name: "C", Provider: "local",
		IsActive: true, Accesses: models.StringArray{"user"},
	})
	room, err := roomRepo.CreateRoom(userID, "cleanup-wx-room", true, "meeting", 10, &models.RoomSettings{})
	if err != nil {
		t.Fatal(err)
	}

	pkgID := uuid.New().String()
	key := room.ID + "/" + pkgID + ".xdc"
	abs := filepath.Join(dir, key)
	if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte("fake-xdc"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := webxdcRepo.CreatePackage(&models.WebxdcPackage{
		ID: pkgID, RoomID: room.ID, ContentHash: "hh", StorageKey: key,
		SizeBytes: 8, Name: "n", UploadedBy: userID,
	}); err != nil {
		t.Fatal(err)
	}
	instID := "deadbeefdeadbeef"
	if err := webxdcRepo.CreateInstance(&models.WebxdcInstance{
		ID: instID, RoomID: room.ID, PackageID: pkgID, CreatedBy: userID,
	}); err != nil {
		t.Fatal(err)
	}
	if err := webxdcRepo.AppendStatusUpdate(&models.WebxdcStatusUpdate{
		InstanceID: instID, Serial: 1, PayloadJSON: `{"payload":1}`, ByteSize: 12,
	}); err != nil {
		t.Fatal(err)
	}

	svc := NewRoomCleanupService(roomRepo, nil, testutil.NewMockRoomService(), nil, "k", "s", nil)
	svc.SetWebxdcCleanup(webxdcRepo, dir)

	if err := svc.cleanupWebxdc(room.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(abs); !os.IsNotExist(err) {
		t.Fatal("expected blob removed")
	}
	if _, err := webxdcRepo.GetPackage(pkgID); !repository.IsNotFound(err) {
		t.Fatalf("package still present: %v", err)
	}
	if _, err := webxdcRepo.GetInstance(instID); !repository.IsNotFound(err) {
		t.Fatalf("instance still present: %v", err)
	}
}
