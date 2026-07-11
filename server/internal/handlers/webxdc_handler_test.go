package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"
	wx "bedrud/internal/webxdc"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func makeMinimalXDC(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create("index.html")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write([]byte(`<!DOCTYPE html><html><head><script src="webxdc.js"></script></head><body>ok</body></html>`))
	m, err := w.Create("manifest.toml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = m.Write([]byte(`name = "Test App"` + "\n"))
	// malicious webxdc.js in ZIP — host must not serve this
	js, err := w.Create("webxdc.js")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = js.Write([]byte(`/* evil from zip */`))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func webxdcTestEnv(t *testing.T) (*fiber.App, *WebxdcHandler, *repository.RoomRepository, *config.Config, string) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	webxdcRepo := repository.NewWebxdcRepository(db)
	storageDir := t.TempDir()

	cfg := &config.Config{
		Server: config.ServerConfig{Domain: "example.com", EnableTLS: true},
		Auth:   config.AuthConfig{JWTSecret: "test-jwt-secret-for-webxdc-tests!!"},
		Webxdc: config.WebxdcConfig{
			Enabled:              true,
			BaseDomain:           "wx.example.com",
			UploadPolicy:         "owner_mod",
			StorageDir:           storageDir,
			TicketTTLMinutes:     10,
			MaxArchiveBytes:      10 << 20,
			MaxUncompressedTotal: 30 << 20,
			MaxEntries:           500,
			MaxSingleFileBytes:   5 << 20,
			StatusLogMaxUpdates:  500,
			SendUpdateMaxSize:    128000,
			SendUpdateIntervalMs: 10000,
		},
	}
	if err := cfg.Webxdc.Validate(cfg.Server.Domain); err != nil {
		t.Fatal(err)
	}
	config.SetForTest(cfg)

	userID := "user-webxdc-1"
	db.Create(&models.User{
		ID: userID, Email: "u@ex.com", Name: "User", Provider: "local",
		IsActive: true, Accesses: models.StringArray{"user"},
	})
	room, err := roomRepo.CreateRoom(userID, "webxdc-test-room", true, "meeting", 20, &models.RoomSettings{})
	if err != nil {
		t.Fatal(err)
	}

	h := NewWebxdcHandler(cfg, webxdcRepo, roomRepo)
	claims := &auth.Claims{
		UserID: userID, Email: "u@ex.com", Name: "User", Accesses: []string{"user"},
	}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		// Simulate Host for asset routes when set via header in tests
		c.Locals("user", claims)
		return c.Next()
	})
	app.Get("/api/webxdc/config", h.PublicConfig)
	app.Post("/api/rooms/:roomId/webxdc/packages", h.UploadPackage)
	app.Get("/api/rooms/:roomId/webxdc/packages", h.ListPackages)
	app.Post("/api/rooms/:roomId/webxdc/instances", h.CreateInstance)
	app.Get("/api/rooms/:roomId/webxdc/instances", h.ListInstances)
	app.Post("/api/rooms/:roomId/webxdc/instances/:instanceId/ticket", h.MintTicket)
	app.Post("/api/rooms/:roomId/webxdc/instances/:instanceId/close", h.CloseInstance)
	app.Post("/api/rooms/:roomId/webxdc/instances/:instanceId/updates", h.PostUpdate)
	app.Get("/api/rooms/:roomId/webxdc/instances/:instanceId/updates", h.ListUpdates)
	// Asset host: path-style test uses ServeHost with Host header
	app.Get("/*", func(c *fiber.Ctx) error {
		return h.ServeHost(c)
	})

	return app, h, roomRepo, cfg, room.ID
}

func TestWebxdcPublicConfig_Enabled(t *testing.T) {
	app, _, _, _, _ := webxdcTestEnv(t)
	req := httptest.NewRequest("GET", "/api/webxdc/config", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var body map[string]map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	wx := body["webxdc"]
	if wx["enabled"] != true || wx["experimental"] != true {
		t.Fatalf("%v", wx)
	}
	if wx["baseDomain"] != "wx.example.com" {
		t.Fatalf("baseDomain %v", wx["baseDomain"])
	}
}

func TestWebxdcPublicConfig_Disabled(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	repo := repository.NewWebxdcRepository(db)
	cfg := &config.Config{
		Server: config.ServerConfig{Domain: "example.com"},
		Webxdc: config.WebxdcConfig{Enabled: false},
	}
	config.SetForTest(cfg)
	h := NewWebxdcHandler(cfg, repo, roomRepo)
	app := fiber.New()
	app.Get("/api/webxdc/config", h.PublicConfig)
	resp, err := app.Test(httptest.NewRequest("GET", "/api/webxdc/config", nil))
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["webxdc"]["enabled"] != false {
		t.Fatal("expected disabled")
	}
}

func TestWebxdcUploadCreateUpdateHostServe(t *testing.T) {
	app, h, _, cfg, roomID := webxdcTestEnv(t)
	xdc := makeMinimalXDC(t)

	// multipart upload
	var body bytesBuffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", "demo.xdc")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(xdc); err != nil {
		t.Fatal(err)
	}
	_ = mw.Close()

	req := httptest.NewRequest("POST", "/api/rooms/"+roomID+"/webxdc/packages", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 201 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload status %d: %s", resp.StatusCode, b)
	}
	var pkg models.WebxdcPackage
	_ = json.NewDecoder(resp.Body).Decode(&pkg)
	if pkg.Name != "Test App" || pkg.ID == "" {
		t.Fatalf("pkg=%+v", pkg)
	}
	// blob on disk
	if _, err := os.Stat(filepath.Join(cfg.Webxdc.StorageDir, pkg.StorageKey)); err != nil {
		t.Fatal(err)
	}

	// list packages
	resp, err = app.Test(httptest.NewRequest("GET", "/api/rooms/"+roomID+"/webxdc/packages", nil))
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("list packages %v %d", err, resp.StatusCode)
	}

	// create instance
	reqBody := strings.NewReader(`{"packageId":"` + pkg.ID + `"}`)
	req = httptest.NewRequest("POST", "/api/rooms/"+roomID+"/webxdc/instances", reqBody)
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 201 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("create instance %d: %s", resp.StatusCode, b)
	}
	var start map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&start)
	instID, _ := start["id"].(string)
	ticket, _ := start["ticket"].(string)
	iframeURL, _ := start["iframeUrl"].(string)
	if instID == "" || ticket == "" || !strings.Contains(iframeURL, "webxdc-"+instID) {
		t.Fatalf("start=%v", start)
	}

	// post update
	upd := strings.NewReader(`{"payload":{"votes":3},"info":"Alice voted","href":"index.html#x"}`)
	req = httptest.NewRequest("POST", "/api/rooms/"+roomID+"/webxdc/instances/"+instID+"/updates", upd)
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 201 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("post update %d: %s", resp.StatusCode, b)
	}
	var postRes map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&postRes)
	if postRes["serial"] != float64(1) {
		t.Fatalf("serial %v", postRes["serial"])
	}

	// absolute href rejected
	upd = strings.NewReader(`{"payload":1,"href":"https://evil.example"}`)
	req = httptest.NewRequest("POST", "/api/rooms/"+roomID+"/webxdc/instances/"+instID+"/updates", upd)
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req)
	if err != nil || resp.StatusCode != 400 {
		t.Fatalf("expected 400 for absolute href, got %v %d", err, resp.StatusCode)
	}

	// list updates
	resp, err = app.Test(httptest.NewRequest("GET", "/api/rooms/"+roomID+"/webxdc/instances/"+instID+"/updates?after=0", nil))
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("list updates %v %d", err, resp.StatusCode)
	}
	var listRes map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&listRes)
	updates, _ := listRes["updates"].([]interface{})
	if len(updates) != 1 {
		t.Fatalf("updates=%v", listRes)
	}

	// serve index via Host
	req = httptest.NewRequest("GET", "/index.html?t="+ticket, nil)
	req.Host = "webxdc-" + instID + ".wx.example.com"
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("index status %d: %s", resp.StatusCode, b)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("content-type %s", ct)
	}
	if csp := resp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "connect-src 'self'") {
		t.Fatalf("missing CSP connect-src self: %s", csp)
	}
	if csp := resp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "wasm-unsafe-eval") {
		t.Fatalf("missing wasm CSP: %s", csp)
	}
	htmlBody, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(htmlBody, []byte("webxdc.js")) {
		t.Fatal("index should reference webxdc.js")
	}
	// Relative assets must be rewritten with ?t= (cross-site iframe often blocks cookie).
	if !bytes.Contains(htmlBody, []byte("t="+ticket)) && !bytes.Contains(htmlBody, []byte("webxdc.js?t=")) {
		t.Fatalf("index HTML should inject ticket into relative URLs, body=%s", htmlBody)
	}

	// Subresource with no ?t= but Referer carrying document ticket (same-origin policy).
	req = httptest.NewRequest("GET", "/webxdc.js", nil)
	req.Host = "webxdc-" + instID + ".wx.example.com"
	req.Header.Set("Referer", "http://webxdc-"+instID+".wx.example.com/?t="+ticket)
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("webxdc.js via Referer ticket: status %d: %s", resp.StatusCode, b)
	}
	_, _ = io.ReadAll(resp.Body)

	// host webxdc.js must be host bridge, not ZIP evil
	req = httptest.NewRequest("GET", "/webxdc.js?t="+ticket, nil)
	req.Host = "webxdc-" + instID + ".wx.example.com"
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("webxdc.js %d", resp.StatusCode)
	}
	jsBody, _ := io.ReadAll(resp.Body)
	if bytes.Contains(jsBody, []byte("evil from zip")) {
		t.Fatal("must not serve ZIP webxdc.js")
	}
	if !bytes.Contains(jsBody, []byte("window.webxdc")) {
		t.Fatal("expected host bridge")
	}
	if !bytes.Contains(jsBody, []byte("bedrud-webxdc")) {
		t.Fatal("expected channel marker in host bridge")
	}
	_ = wx.HostBridgeJS

	// bad ticket
	req = httptest.NewRequest("GET", "/index.html?t=bad", nil)
	req.Host = "webxdc-" + instID + ".wx.example.com"
	resp, err = app.Test(req, -1)
	if err != nil || resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %v %d", err, resp.StatusCode)
	}

	// CSP on 404
	req = httptest.NewRequest("GET", "/missing.txt?t="+ticket, nil)
	req.Host = "webxdc-" + instID + ".wx.example.com"
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 404 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if csp := resp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "connect-src 'self'") {
		t.Fatalf("404 must have CSP: %q", csp)
	}

	// close
	req = httptest.NewRequest("POST", "/api/rooms/"+roomID+"/webxdc/instances/"+instID+"/close", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req)
	if err != nil || resp.StatusCode != 204 {
		t.Fatalf("close %v %d", err, resp.StatusCode)
	}

	// closed instance asset 404
	req = httptest.NewRequest("GET", "/index.html?t="+ticket, nil)
	req.Host = "webxdc-" + instID + ".wx.example.com"
	resp, err = app.Test(req, -1)
	if err != nil || resp.StatusCode != 404 {
		t.Fatalf("closed asset %v %d", err, resp.StatusCode)
	}

	_ = h
}

// bytesBuffer is a thin alias so multipart write works with httptest
type bytesBuffer = bytes.Buffer

func TestWebxdcUpload_RejectsInvalidZip(t *testing.T) {
	app, _, _, _, roomID := webxdcTestEnv(t)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, _ := mw.CreateFormFile("file", "bad.xdc")
	_, _ = part.Write([]byte("not a zip"))
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/api/rooms/"+roomID+"/webxdc/packages", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := app.Test(req, -1)
	if err != nil || resp.StatusCode != 400 {
		t.Fatalf("got %v %d", err, resp.StatusCode)
	}
}

func TestWebxdcUpload_ForbiddenForNonModWhenOwnerModPolicy(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	webxdcRepo := repository.NewWebxdcRepository(db)
	storageDir := t.TempDir()
	cfg := &config.Config{
		Server: config.ServerConfig{Domain: "example.com"},
		Auth:   config.AuthConfig{JWTSecret: "secret-secret-secret-secret!!"},
		Webxdc: config.WebxdcConfig{
			Enabled: true, BaseDomain: "wx.example.com", UploadPolicy: "owner_mod", StorageDir: storageDir,
			TicketTTLMinutes: 10, MaxArchiveBytes: 10 << 20, MaxUncompressedTotal: 30 << 20,
			MaxEntries: 500, MaxSingleFileBytes: 5 << 20, StatusLogMaxUpdates: 500,
			SendUpdateMaxSize: 128000, SendUpdateIntervalMs: 10000,
		},
	}
	config.SetForTest(cfg)

	owner := "owner-1"
	member := "member-1"
	db.Create(&models.User{ID: owner, Email: "o@ex.com", Name: "O", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}})
	db.Create(&models.User{ID: member, Email: "m@ex.com", Name: "M", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}})
	room, err := roomRepo.CreateRoom(owner, "private-room-wx", false, "meeting", 20, &models.RoomSettings{})
	if err != nil {
		t.Fatal(err)
	}
	_ = roomRepo.AddParticipant(room.ID, member)

	h := NewWebxdcHandler(cfg, webxdcRepo, roomRepo)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{UserID: member, Accesses: []string{"user"}})
		return c.Next()
	})
	app.Post("/api/rooms/:roomId/webxdc/packages", h.UploadPackage)

	xdc := makeMinimalXDC(t)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, _ := mw.CreateFormFile("file", "demo.xdc")
	_, _ = part.Write(xdc)
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/api/rooms/"+room.ID+"/webxdc/packages", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := app.Test(req, -1)
	if err != nil || resp.StatusCode != 403 {
		t.Fatalf("expected 403, got %v %d", err, resp.StatusCode)
	}
}

func TestWebxdcAPI_NotEnabled(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	repo := repository.NewWebxdcRepository(db)
	cfg := &config.Config{Server: config.ServerConfig{Domain: "example.com"}, Webxdc: config.WebxdcConfig{Enabled: false}}
	config.SetForTest(cfg)
	h := NewWebxdcHandler(cfg, repo, roomRepo)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{UserID: "u", Accesses: []string{"user"}})
		return c.Next()
	})
	app.Get("/api/rooms/:roomId/webxdc/packages", h.ListPackages)
	resp, err := app.Test(httptest.NewRequest("GET", "/api/rooms/"+uuid.New().String()+"/webxdc/packages", nil))
	if err != nil || resp.StatusCode != 404 {
		t.Fatalf("expected 404 when disabled, got %v %d", err, resp.StatusCode)
	}
}

func TestWebxdcCleanup_DeletesBlobs(t *testing.T) {
	app, h, roomRepo, cfg, roomID := webxdcTestEnv(t)
	xdc := makeMinimalXDC(t)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, _ := mw.CreateFormFile("file", "demo.xdc")
	_, _ = part.Write(xdc)
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/api/rooms/"+roomID+"/webxdc/packages", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := app.Test(req, -1)
	if err != nil || resp.StatusCode != 201 {
		t.Fatalf("upload %v %d", err, resp.StatusCode)
	}
	var pkg models.WebxdcPackage
	_ = json.NewDecoder(resp.Body).Decode(&pkg)
	// StorageKey is not in JSON; locate blob under storage dir for this room
	roomDir := filepath.Join(cfg.Webxdc.StorageDir, roomID)
	entries, err := os.ReadDir(roomDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected blob dir: %v", err)
	}
	abs := filepath.Join(roomDir, entries[0].Name())

	keys, err := h.repo.DeleteAllForRoom(roomID)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("keys=%v", keys)
	}
	for _, k := range keys {
		_ = os.Remove(filepath.Join(cfg.Webxdc.StorageDir, k))
	}
	if _, err := os.Stat(abs); !os.IsNotExist(err) {
		t.Fatal("blob should be gone")
	}
	_ = roomRepo
	_ = pkg
}
