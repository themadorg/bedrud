package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	wx "bedrud/internal/webxdc"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// WebxdcHandler serves experimental WebXDC API + asset host.
type WebxdcHandler struct {
	cfg          *config.Config
	repo         *repository.WebxdcRepository
	roomRepo     *repository.RoomRepository
	settingsRepo *repository.SettingsRepository // optional; for effective gallery flags
}

func NewWebxdcHandler(cfg *config.Config, repo *repository.WebxdcRepository, roomRepo *repository.RoomRepository) *WebxdcHandler {
	return &WebxdcHandler{cfg: cfg, repo: repo, roomRepo: roomRepo}
}

// WithSettings attaches system settings for gallery public config (admin overrides).
func (h *WebxdcHandler) WithSettings(settingsRepo *repository.SettingsRepository) *WebxdcHandler {
	if h != nil {
		h.settingsRepo = settingsRepo
	}
	return h
}

func (h *WebxdcHandler) active() bool {
	if h == nil || h.cfg == nil {
		return false
	}
	return h.cfg.Webxdc.Active(h.cfg.Server.Domain)
}

// effectiveLimits returns zip validation limits: admin system settings (MiB) override
// config.yaml when set (>0). Fixes "entry too large" for large apps without redeploy.
func (h *WebxdcHandler) effectiveLimits() wx.Limits {
	limits := wx.Limits{
		MaxArchiveBytes:      h.cfg.Webxdc.MaxArchiveBytes,
		MaxUncompressedTotal: h.cfg.Webxdc.MaxUncompressedTotal,
		MaxEntries:           h.cfg.Webxdc.MaxEntries,
		MaxSingleFile:        h.cfg.Webxdc.MaxSingleFileBytes,
	}
	const mib = int64(1 << 20)
	if h.settingsRepo != nil {
		if s, err := h.settingsRepo.GetEffectiveSettings(); err == nil && s != nil {
			// Prefer explicit DB values; GetEffectiveSettings already fills from config when 0.
			if s.WebxdcMaxArchiveMB > 0 {
				limits.MaxArchiveBytes = int64(s.WebxdcMaxArchiveMB) * mib
			}
			if s.WebxdcMaxUncompressedMB > 0 {
				limits.MaxUncompressedTotal = int64(s.WebxdcMaxUncompressedMB) * mib
			}
			if s.WebxdcMaxSingleFileMB > 0 {
				limits.MaxSingleFile = int64(s.WebxdcMaxSingleFileMB) * mib
			}
			if s.WebxdcMaxEntries > 0 {
				limits.MaxEntries = s.WebxdcMaxEntries
			}
		}
	}
	return limits
}

func (h *WebxdcHandler) notEnabled(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "WebXDC is not enabled"})
}

func (h *WebxdcHandler) PublicConfig(c *fiber.Ctx) error {
	enabled := h.active()
	out := fiber.Map{
		"enabled":      enabled,
		"experimental": true,
	}
	if enabled {
		out["baseDomain"] = h.cfg.Webxdc.BaseDomain
		out["sendUpdateMaxSize"] = h.cfg.Webxdc.SendUpdateMaxSize
		out["sendUpdateIntervalMs"] = h.cfg.Webxdc.SendUpdateIntervalMs
		out["devPathMode"] = h.cfg.Webxdc.UsePathMode()
		if h.cfg.Webxdc.UsePathMode() {
			out["publicBaseURL"] = h.cfg.Webxdc.PublicBaseURL
		}

		// Gallery: config defaults, overlaid by system settings when available.
		galleryEnabled, gallerySource, galleryRemoteURL, _, instanceCatalog := h.effectiveGallery()
		out["galleryEnabled"] = galleryEnabled
		out["gallerySource"] = gallerySource
		out["instanceCatalogEnabled"] = instanceCatalog
		if galleryRemoteURL != "" {
			out["galleryRemoteUrl"] = galleryRemoteURL
		}
	}
	return c.JSON(fiber.Map{"webxdc": out})
}

func (h *WebxdcHandler) claims(c *fiber.Ctx) (*auth.Claims, error) {
	v := c.Locals("user")
	if v == nil {
		return nil, fiber.ErrUnauthorized
	}
	cl, ok := v.(*auth.Claims)
	if !ok || cl == nil {
		return nil, fiber.ErrUnauthorized
	}
	return cl, nil
}

func (h *WebxdcHandler) canAccessRoom(claims *auth.Claims, room *models.Room) bool {
	if claims == nil || room == nil {
		return false
	}
	if containsAccess(claims.Accesses, "superadmin") {
		return true
	}
	if room.CreatedBy == claims.UserID || room.AdminID == claims.UserID {
		return true
	}
	if room.IsPublic {
		return true
	}
	ok, err := h.roomRepo.IsParticipant(room.ID, claims.UserID)
	return err == nil && ok
}

func (h *WebxdcHandler) canUpload(claims *auth.Claims, room *models.Room) bool {
	if !h.canAccessRoom(claims, room) {
		return false
	}
	policy := h.cfg.Webxdc.UploadPolicy
	if policy == "any_member" {
		return true
	}
	ownerID := room.AdminID
	if ownerID == "" {
		ownerID = room.CreatedBy
	}
	return isRoomModerator(claims, ownerID, room.ID, h.roomRepo)
}

func (h *WebxdcHandler) loadRoom(c *fiber.Ctx) (*models.Room, error) {
	roomID := c.Params("roomId")
	room, err := h.roomRepo.GetRoom(roomID)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusNotFound, "room not found")
	}
	return room, nil
}

// UploadPackage POST /api/rooms/:roomId/webxdc/packages
func (h *WebxdcHandler) UploadPackage(c *fiber.Ctx) error {
	if !h.active() {
		return h.notEnabled(c)
	}
	claims, err := h.claims(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
	}
	room, err := h.loadRoom(c)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "room not found"})
	}
	if !h.canUpload(claims, room) {
		return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "file required"})
	}
	f, err := fileHeader.Open()
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "cannot open file"})
	}
	defer f.Close()

	// size guard before full read
	limits := h.effectiveLimits()
	if fileHeader.Size > limits.MaxArchiveBytes {
		return c.Status(413).JSON(fiber.Map{"error": "archive too large"})
	}
	data := make([]byte, 0, fileHeader.Size)
	buf := make([]byte, 32*1024)
	var total int64
	for {
		n, rerr := f.Read(buf)
		if n > 0 {
			total += int64(n)
			if total > limits.MaxArchiveBytes {
				return c.Status(413).JSON(fiber.Map{"error": "archive too large"})
			}
			data = append(data, buf[:n]...)
		}
		if rerr != nil {
			break
		}
	}

	if err := wx.ValidateZip(data, limits); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	pkgID := uuid.New().String()
	if err := os.MkdirAll(h.cfg.Webxdc.StorageDir, 0o750); err != nil {
		log.Error().Err(err).Msg("webxdc storage dir")
		return c.Status(500).JSON(fiber.Map{"error": "storage error"})
	}
	storageKey := filepath.Join(room.ID, pkgID+".xdc")
	abs := filepath.Join(h.cfg.Webxdc.StorageDir, storageKey)
	if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "storage error"})
	}
	if err := os.WriteFile(abs, data, 0o640); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "storage error"})
	}

	meta := wx.ParseManifestFromZip(data)
	name := meta.Name
	if name == "" {
		name = fileHeader.Filename
		if name == "" {
			name = "app.xdc"
		}
	}
	pkg := &models.WebxdcPackage{
		ID:            pkgID,
		RoomID:        room.ID,
		ContentHash:   hash,
		StorageKey:    storageKey,
		SizeBytes:     int64(len(data)),
		Name:          name,
		SourceCodeURL: meta.SourceCodeURL,
		IconPath:      meta.IconPath,
		UploadedBy:    claims.UserID,
	}
	if icon, ct, ierr := wx.ExtractSafeIcon(data); ierr == nil {
		_ = os.WriteFile(abs+wx.IconStorageSuffix, icon, 0o640)
		_ = os.WriteFile(abs+wx.IconTypeStorageSuffix, []byte(ct), 0o640)
		if pkg.IconPath == "" {
			pkg.IconPath = "icon.png"
		}
	}
	if err := h.repo.CreatePackage(pkg); err != nil {
		_ = os.Remove(abs)
		_ = os.Remove(abs + wx.IconStorageSuffix)
		_ = os.Remove(abs + wx.IconTypeStorageSuffix)
		log.Error().Err(err).Msg("webxdc create package")
		return c.Status(500).JSON(fiber.Map{"error": "db error"})
	}
	return c.Status(201).JSON(pkg)
}

// ListPackages GET /api/rooms/:roomId/webxdc/packages
func (h *WebxdcHandler) ListPackages(c *fiber.Ctx) error {
	if !h.active() {
		return h.notEnabled(c)
	}
	claims, err := h.claims(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
	}
	room, err := h.loadRoom(c)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "room not found"})
	}
	if !h.canAccessRoom(claims, room) {
		return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
	}
	list, err := h.repo.ListPackagesByRoom(room.ID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "db error"})
	}
	return c.JSON(fiber.Map{"packages": list})
}

// CreateInstance POST /api/rooms/:roomId/webxdc/instances
func (h *WebxdcHandler) CreateInstance(c *fiber.Ctx) error {
	if !h.active() {
		return h.notEnabled(c)
	}
	claims, err := h.claims(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
	}
	room, err := h.loadRoom(c)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "room not found"})
	}
	if !h.canAccessRoom(claims, room) {
		return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
	}
	var req struct {
		PackageID string `json:"packageId"`
	}
	if err := c.BodyParser(&req); err != nil || req.PackageID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "packageId required"})
	}
	pkg, err := h.repo.GetPackage(req.PackageID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "package not found"})
	}
	// Room package must match room; instance-catalog packages have empty RoomID (server-global).
	if pkg.RoomID != "" && pkg.RoomID != room.ID {
		return c.Status(404).JSON(fiber.Map{"error": "package not found"})
	}
	// Materialize global package into this room (same storage blob, new row).
	if pkg.RoomID == "" {
		if existing, err := h.repo.FindRoomPackageByHash(room.ID, pkg.ContentHash); err == nil && existing != nil {
			pkg = existing
		} else {
			clone := *pkg
			clone.ID = uuid.New().String()
			clone.RoomID = room.ID
			clone.UploadedBy = claims.UserID
			if err := h.repo.CreatePackage(&clone); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "db error"})
			}
			pkg = &clone
		}
	}

	label, err := randomHostLabel(16)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "id error"})
	}
	inst := &models.WebxdcInstance{
		ID:        label,
		RoomID:    room.ID,
		PackageID: pkg.ID,
		CreatedBy: claims.UserID,
	}
	if err := h.repo.CreateInstance(inst); err != nil {
		log.Error().Err(err).Msg("webxdc create instance")
		return c.Status(500).JSON(fiber.Map{"error": "db error"})
	}

	ticket, exp, iframeURL, origin, err := h.mintTicketResponse(claims, inst)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "ticket error"})
	}
	selfAddr, selfName := h.identityFor(claims, inst)
	return c.Status(201).JSON(fiber.Map{
		"id":           inst.ID,
		"roomId":       inst.RoomID,
		"packageId":    inst.PackageID,
		"name":         pkg.Name,
		"iframeOrigin": origin,
		"iframeUrl":    iframeURL,
		"ticket":       ticket,
		"expiresAt":    exp.UTC().Format(time.RFC3339),
		"selfAddr":     selfAddr,
		"selfName":     selfName,
		"experimental": true,
	})
}

// ListInstances GET /api/rooms/:roomId/webxdc/instances
func (h *WebxdcHandler) ListInstances(c *fiber.Ctx) error {
	if !h.active() {
		return h.notEnabled(c)
	}
	claims, err := h.claims(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
	}
	room, err := h.loadRoom(c)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "room not found"})
	}
	if !h.canAccessRoom(claims, room) {
		return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
	}
	list, err := h.repo.ListInstancesByRoom(room.ID, false)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "db error"})
	}
	return c.JSON(fiber.Map{"instances": list})
}

// MintTicket POST /api/rooms/:roomId/webxdc/instances/:instanceId/ticket
func (h *WebxdcHandler) MintTicket(c *fiber.Ctx) error {
	if !h.active() {
		return h.notEnabled(c)
	}
	claims, err := h.claims(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
	}
	room, err := h.loadRoom(c)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "room not found"})
	}
	if !h.canAccessRoom(claims, room) {
		return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
	}
	inst, err := h.repo.GetInstance(c.Params("instanceId"))
	if err != nil || inst.RoomID != room.ID {
		return c.Status(404).JSON(fiber.Map{"error": "instance not found"})
	}
	if inst.ClosedAt != nil {
		return c.Status(410).JSON(fiber.Map{"error": "instance closed"})
	}
	ticket, exp, iframeURL, origin, err := h.mintTicketResponse(claims, inst)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "ticket error"})
	}
	selfAddr, selfName := h.identityFor(claims, inst)
	return c.JSON(fiber.Map{
		"ticket":       ticket,
		"expiresAt":    exp.UTC().Format(time.RFC3339),
		"iframeOrigin": origin,
		"iframeUrl":    iframeURL,
		"selfAddr":     selfAddr,
		"selfName":     selfName,
	})
}

// CloseInstance POST .../close
func (h *WebxdcHandler) CloseInstance(c *fiber.Ctx) error {
	if !h.active() {
		return h.notEnabled(c)
	}
	claims, err := h.claims(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
	}
	room, err := h.loadRoom(c)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "room not found"})
	}
	ownerID := room.AdminID
	if ownerID == "" {
		ownerID = room.CreatedBy
	}
	if !isRoomModerator(claims, ownerID, room.ID, h.roomRepo) && claims.UserID != room.CreatedBy {
		// allow instance creator
		inst, ierr := h.repo.GetInstance(c.Params("instanceId"))
		if ierr != nil || inst.CreatedBy != claims.UserID {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
	}
	inst, err := h.repo.GetInstance(c.Params("instanceId"))
	if err != nil || inst.RoomID != room.ID {
		return c.Status(404).JSON(fiber.Map{"error": "instance not found"})
	}
	_ = h.repo.CloseInstance(inst.ID)
	return c.SendStatus(204)
}

// PostUpdate POST .../updates
func (h *WebxdcHandler) PostUpdate(c *fiber.Ctx) error {
	if !h.active() {
		return h.notEnabled(c)
	}
	claims, err := h.claims(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
	}
	room, err := h.loadRoom(c)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "room not found"})
	}
	if !h.canAccessRoom(claims, room) {
		return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
	}
	inst, err := h.repo.GetInstance(c.Params("instanceId"))
	if err != nil || inst.RoomID != room.ID {
		return c.Status(404).JSON(fiber.Map{"error": "instance not found"})
	}
	if inst.ClosedAt != nil {
		return c.Status(410).JSON(fiber.Map{"error": "instance closed"})
	}

	body := c.Body()
	if len(body) > h.cfg.Webxdc.SendUpdateMaxSize {
		return c.Status(413).JSON(fiber.Map{"error": "update too large"})
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid json"})
	}
	if _, ok := raw["payload"]; !ok {
		return c.Status(400).JSON(fiber.Map{"error": "payload required"})
	}
	// strip absolute href
	if href, ok := raw["href"].(string); ok {
		if strings.Contains(href, "://") || strings.HasPrefix(href, "//") {
			return c.Status(400).JSON(fiber.Map{"error": "href must be relative"})
		}
	}
	serialized, _ := json.Marshal(raw)
	if len(serialized) > h.cfg.Webxdc.SendUpdateMaxSize {
		return c.Status(413).JSON(fiber.Map{"error": "update too large"})
	}

	serial, err := h.repo.NextSerial(inst.ID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "serial error"})
	}
	u := &models.WebxdcStatusUpdate{
		InstanceID:     inst.ID,
		Serial:         serial,
		SenderUserID:   claims.UserID,
		SenderIdentity: claims.UserID,
		PayloadJSON:    string(serialized),
		ByteSize:       len(serialized),
	}
	if err := h.repo.AppendStatusUpdate(u); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "db error"})
	}
	_ = h.repo.TrimStatusLog(inst.ID, h.cfg.Webxdc.StatusLogMaxUpdates)

	// chrome fields
	doc, _ := raw["document"].(string)
	sum, _ := raw["summary"].(string)
	info, _ := raw["info"].(string)
	_ = h.repo.UpdateInstanceChrome(inst.ID, doc, sum, info)

	return c.Status(201).JSON(fiber.Map{
		"serial":    serial,
		"maxSerial": serial,
		"update":    raw,
		"ts":        time.Now().UTC().UnixMilli(),
		"nudge": fiber.Map{
			"v": 1, "kind": "nudge", "appId": inst.ID, "maxSerial": serial,
		},
	})
}

// ListUpdates GET .../updates?after=N
func (h *WebxdcHandler) ListUpdates(c *fiber.Ctx) error {
	if !h.active() {
		return h.notEnabled(c)
	}
	claims, err := h.claims(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
	}
	room, err := h.loadRoom(c)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "room not found"})
	}
	if !h.canAccessRoom(claims, room) {
		return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
	}
	inst, err := h.repo.GetInstance(c.Params("instanceId"))
	if err != nil || inst.RoomID != room.ID {
		return c.Status(404).JSON(fiber.Map{"error": "instance not found"})
	}
	after := int64(0)
	if v := c.Query("after"); v != "" {
		fmt.Sscan(v, &after)
	}
	list, maxSerial, err := h.repo.ListStatusUpdatesAfter(inst.ID, after, 200)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "db error"})
	}
	out := make([]fiber.Map, 0, len(list))
	for _, u := range list {
		var payload interface{}
		_ = json.Unmarshal([]byte(u.PayloadJSON), &payload)
		m := fiber.Map{
			"serial":     u.Serial,
			"max_serial": maxSerial,
			"payload":    nil,
		}
		if pm, ok := payload.(map[string]interface{}); ok {
			for k, v := range pm {
				m[k] = v
			}
		} else {
			m["payload"] = payload
		}
		m["serial"] = u.Serial
		m["max_serial"] = maxSerial
		out = append(out, m)
	}
	return c.JSON(fiber.Map{"updates": out, "maxSerial": maxSerial})
}

// ServeHost handles requests on webxdc-<id>.baseDomain (assets + webxdc.js).
func (h *WebxdcHandler) ServeHost(c *fiber.Ctx) error {
	// Always apply security headers (including errors) — XDC-01-002
	h.applyWebxdcSecurityHeaders(c)

	if !h.active() {
		return c.Status(404).SendString("not found")
	}
	label, ok := wx.ParseInstanceHost(c.Hostname(), h.cfg.Webxdc.BaseDomain)
	if !ok {
		return c.Next()
	}
	return h.serveInstanceAssets(c, label, c.Path(), "/")
}

// ServeHostPath handles local/dev path mode: /__webxdc/{instanceId}/...
func (h *WebxdcHandler) ServeHostPath(c *fiber.Ctx) error {
	h.applyWebxdcSecurityHeaders(c)
	if !h.active() {
		return c.Status(404).SendString("not found")
	}
	label, assetPath, ok := wx.ParsePathModeInstance(c.Path())
	if !ok {
		return c.Status(404).SendString("not found")
	}
	cookiePath := wx.PathModePrefix + "/" + label
	return h.serveInstanceAssets(c, label, assetPath, cookiePath)
}

func (h *WebxdcHandler) serveInstanceAssets(c *fiber.Ctx, label, reqPath, cookiePath string) error {
	inst, err := h.repo.GetInstance(label)
	if err != nil || inst.ClosedAt != nil {
		return h.webxdcNotFound(c, reqPath)
	}

	ticket := extractWebxdcTicket(c)
	secret := h.cfg.Auth.JWTSecret
	if secret == "" {
		secret = h.cfg.Auth.SessionSecret
	}
	if _, err := wx.VerifyTicket(secret, ticket, label); err != nil {
		// Missing/invalid ticket → text/plain 401 causes CSS MIME errors in the browser.
		return h.webxdcUnauthorized(c, reqPath)
	}

	// Cookie for subsequent relative loads (CSS/JS without ?t=).
	// Cross-site iframes (SPA origin ≠ webxdc host) often block this cookie —
	// HTML rewrite + Referer fallback are the reliable paths.
	if ticket != "" {
		secure := c.Protocol() == "https"
		sameSite := "Lax"
		if secure {
			// Cross-site SPA parent + mini-app host needs None; requires Secure.
			sameSite = "None"
		}
		if cookiePath == "" {
			cookiePath = "/"
		}
		c.Cookie(&fiber.Cookie{
			Name:     "webxdc_ticket",
			Value:    ticket,
			Path:     cookiePath,
			HTTPOnly: true,
			Secure:   secure,
			SameSite: sameSite,
			MaxAge:   h.cfg.Webxdc.TicketTTLMinutes * 60,
		})
	}

	path := reqPath
	if path == "" || path == "/" {
		path = "/index.html"
	}
	entry, err := wx.SafeJoinEntry(strings.TrimPrefix(path, "/"))
	if err != nil {
		return h.webxdcBadRequest(c, reqPath)
	}

	if wx.IsHostProvidedPath(entry) {
		c.Type("js")
		c.Set("Content-Type", "text/javascript; charset=utf-8")
		return c.SendString(wx.HostBridgeJS)
	}

	if inst.Package == nil {
		pkg, perr := h.repo.GetPackage(inst.PackageID)
		if perr != nil {
			return h.webxdcNotFound(c, reqPath)
		}
		inst.Package = pkg
	}
	abs := filepath.Join(h.cfg.Webxdc.StorageDir, inst.Package.StorageKey)
	data, err := os.ReadFile(abs)
	if err != nil {
		log.Error().Err(err).Str("path", abs).Msg("webxdc read package")
		return h.webxdcNotFound(c, reqPath)
	}
	body, err := wx.ReadZipEntry(data, entry, h.effectiveLimits().MaxSingleFile)
	if err != nil {
		return h.webxdcNotFound(c, reqPath)
	}
	// Inject ?t= into relative HTML links so CSS/JS/webxdc.js do not 401 when
	// the ticket cookie is blocked (cross-site iframe) and Referer is absent.
	if ticket != "" && wx.IsHTMLEntry(entry) {
		body = wx.InjectTicketIntoHTML(body, ticket)
	}
	// Desktop runs webxdc as top-level; Bedrud uses a cross-origin iframe.
	// Soften a few known window.top patterns (OpenArena pagehide / realtime stash).
	if wx.IsHTMLEntry(entry) || wx.IsScriptEntry(entry) {
		body = wx.SoftenCrossOriginTop(body)
	}
	// Fiber Type() can help some clients; always set full Content-Type for charset.
	setWebxdcContentType(c, entry)
	return c.Send(body)
}

// extractWebxdcTicket prefers ?t=, then cookie, then Referer ?t= (relative CSS/JS after HTML load).
func extractWebxdcTicket(c *fiber.Ctx) string {
	if t := c.Query("t"); t != "" {
		return t
	}
	if t := c.Cookies("webxdc_ticket"); t != "" {
		return t
	}
	// Relative asset loads often send the document URL as Referer, which still has ?t=.
	ref := c.Get("Referer")
	if ref == "" {
		return ""
	}
	if u, err := url.Parse(ref); err == nil {
		return u.Query().Get("t")
	}
	return ""
}

func setWebxdcContentType(c *fiber.Ctx, entry string) {
	ct := wx.ContentTypeForEntry(entry)
	c.Set("Content-Type", ct)
	// Prevent middleware from re-sniffing as text/plain.
	c.Set("X-Content-Type-Options", "nosniff")
}

func (h *WebxdcHandler) webxdcNotFound(c *fiber.Ctx, reqPath string) error {
	h.applyWebxdcSecurityHeaders(c)
	setWebxdcContentType(c, reqPath)
	return c.Status(fiber.StatusNotFound).Send([]byte("not found"))
}

func (h *WebxdcHandler) webxdcUnauthorized(c *fiber.Ctx, reqPath string) error {
	h.applyWebxdcSecurityHeaders(c)
	setWebxdcContentType(c, reqPath)
	return c.Status(fiber.StatusUnauthorized).Send([]byte("unauthorized"))
}

func (h *WebxdcHandler) webxdcBadRequest(c *fiber.Ctx, reqPath string) error {
	h.applyWebxdcSecurityHeaders(c)
	setWebxdcContentType(c, reqPath)
	return c.Status(fiber.StatusBadRequest).Send([]byte("bad path"))
}

// identityFor returns opaque per-app selfAddr (HMAC) and display selfName for the host bridge.
func (h *WebxdcHandler) identityFor(claims *auth.Claims, inst *models.WebxdcInstance) (selfAddr, selfName string) {
	secret := h.cfg.Auth.JWTSecret
	if secret == "" {
		secret = h.cfg.Auth.SessionSecret
	}
	selfAddr = wx.SelfAddr(secret, inst.RoomID, inst.ID, claims.UserID)
	selfName = strings.TrimSpace(claims.Name)
	if selfName == "" {
		selfName = claims.UserID
	}
	return selfAddr, selfName
}

func (h *WebxdcHandler) mintTicketResponse(claims *auth.Claims, inst *models.WebxdcInstance) (ticket string, exp time.Time, iframeURL, origin string, err error) {
	secret := h.cfg.Auth.JWTSecret
	if secret == "" {
		secret = h.cfg.Auth.SessionSecret
	}
	jti := uuid.New().String()
	ttl := time.Duration(h.cfg.Webxdc.TicketTTLMinutes) * time.Minute
	ticket, exp, err = wx.MintTicket(secret, jti, inst.ID, inst.RoomID, claims.UserID, ttl)
	if err != nil {
		return
	}
	if h.cfg.Webxdc.UsePathMode() {
		// Browser origin is scheme+host+port only (no path). postMessage must match that.
		origin = strings.TrimRight(strings.TrimSpace(h.cfg.Webxdc.PublicBaseURL), "/")
		if origin == "" {
			origin = "http://localhost:7071"
		}
		iframeURL = wx.InstanceOriginPath(inst.ID, origin) + "/?t=" + ticket + "&appId=" + inst.ID
		return
	}
	secure := true
	if !h.cfg.Server.EnableTLS && !h.cfg.Server.BehindProxy {
		// dev may use http — still prefer https when domain set
		secure = false
	}
	if h.cfg.Server.BehindProxy || h.cfg.Server.EnableTLS {
		secure = true
	}
	// Local: http://webxdc-{id}.localhost:7071 (*.localhost → 127.0.0.1 in browsers)
	port := strings.TrimSpace(h.cfg.Webxdc.PublicPort)
	origin = wx.InstanceOrigin(inst.ID, h.cfg.Webxdc.BaseDomain, secure, port)
	iframeURL = origin + "/?t=" + ticket + "&appId=" + inst.ID
	return
}

// normalizeGallerySource maps UI aliases onto fetch modes.
// semi-remote = server fetches catalog JSON + native Bedrud UI (same fetch path as remote).
func normalizeGallerySource(source string) string {
	source = strings.TrimSpace(strings.ToLower(source))
	switch source {
	case "", "local":
		return "local"
	case "semi-remote", "remote":
		return "remote"
	case "both":
		return "both"
	default:
		return source
	}
}

// effectiveGallery returns gallery flags merged from config + system settings.
func (h *WebxdcHandler) effectiveGallery() (enabled bool, source, remoteURL string, allowDownload, instanceCatalog bool) {
	enabled = h.cfg.Webxdc.Gallery.Enabled
	source = strings.TrimSpace(h.cfg.Webxdc.Gallery.Source)
	remoteURL = strings.TrimSpace(h.cfg.Webxdc.Gallery.RemoteCatalogURL)
	allowDownload = h.cfg.Webxdc.Gallery.AllowRemoteDownload
	if source == "" {
		source = "local"
	}
	if h.settingsRepo != nil {
		if s, err := h.settingsRepo.GetEffectiveSettings(); err == nil && s != nil {
			enabled = s.WebxdcGalleryEnabled
			if src := strings.TrimSpace(s.WebxdcGallerySource); src != "" {
				source = src
			}
			if u := strings.TrimSpace(s.WebxdcGalleryRemoteCatalogURL); u != "" {
				remoteURL = u
			}
			allowDownload = s.WebxdcGalleryAllowRemoteDownload
			instanceCatalog = s.WebxdcGalleryInstanceCatalogEnabled
		}
	}
	// Whenever the gallery is enabled, include admin-uploaded instance packages.
	// Previously only "local" forced this flag, so semi-remote/remote hid uploads
	// when webxdcGalleryInstanceCatalogEnabled was false (common after the
	// semi-remote preset). Packages still live under _instance/ — just not listed.
	if enabled {
		instanceCatalog = true
	}
	// Default semi-remote catalog when remote mode is on but URL empty.
	if (source == "remote" || source == "semi-remote" || source == "both") && remoteURL == "" {
		remoteURL = wx.WellKnownXdcgetLock
	}
	source = normalizeGallerySource(source)
	return
}

// ListGallery GET /api/webxdc/gallery — catalog cards only (no raw external HTML).
func (h *WebxdcHandler) ListGallery(c *fiber.Ctx) error {
	if !h.active() {
		return h.notEnabled(c)
	}
	enabled, source, remoteURL, _, instanceCatalog := h.effectiveGallery()
	if !enabled {
		return c.Status(404).JSON(fiber.Map{"error": "gallery disabled"})
	}
	if _, err := h.claims(c); err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
	}

	entries := []wx.GalleryEntry{}
	var warning string
	var catalogURL string

	// Instance catalog first (admin-uploaded packages on this server).
	if instanceCatalog {
		if list, err := h.repo.ListInstanceCatalogPackages(); err == nil {
			for _, p := range list {
				desc := strings.TrimSpace(p.Description)
				if desc == "" {
					desc = "Instance catalog"
				}
				cat := strings.TrimSpace(p.Category)
				if cat == "" {
					cat = "instance"
				}
				entries = append(entries, wx.GalleryEntry{
					ID:            "instance-" + p.ID,
					Name:          p.Name,
					PackageID:     p.ID,
					Origin:        "instance",
					Description:   desc,
					Category:      cat,
					SourceCodeURL: p.SourceCodeURL,
					HasIcon:      h.packageHasSafeIcon(&p),
				})
			}
		}
	}

	// Remote / semi-remote catalog metadata (server-side fetch only).
	if source == "remote" || source == "both" {
		catalogURL = remoteURL
		if catalogURL == "" {
			catalogURL = wx.WellKnownXdcgetLock
		}
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		list, err := wx.FetchGalleryCatalog(ctx, catalogURL, 4<<20)
		if err != nil {
			log.Warn().Err(err).Str("url", catalogURL).Msg("webxdc gallery catalog fetch failed")
			warning = "remote catalog unavailable: " + err.Error()
		} else {
			entries = append(entries, list...)
		}
	}

	out := fiber.Map{
		"entries":                 entries,
		"source":                  source,
		"instanceCatalogEnabled":  instanceCatalog,
	}
	if catalogURL != "" {
		out["catalogUrl"] = catalogURL
	}
	if warning != "" {
		out["warning"] = warning
	}
	return c.JSON(out)
}

// ImportGalleryApp POST /api/rooms/:roomId/webxdc/gallery/import
// Server downloads .xdc (SSRF-safe), validates like upload, stores as room package.
// Client then starts instance + claims stage — same secure host path as manual upload.
//
// Authz: any room member may import from the gallery (not limited to uploadPolicy owner_mod).
// Custom ZIP upload still uses canUpload / uploadPolicy.
func (h *WebxdcHandler) ImportGalleryApp(c *fiber.Ctx) error {
	if !h.active() {
		return h.notEnabled(c)
	}
	enabled, source, _, allowDownload, _ := h.effectiveGallery()
	if !enabled {
		return c.Status(404).JSON(fiber.Map{"error": "gallery disabled"})
	}
	// source is already normalized (semi-remote → remote).
	if source != "remote" && source != "both" {
		return c.Status(400).JSON(fiber.Map{"error": "remote gallery import not enabled (source is local)"})
	}
	if !allowDownload {
		return c.Status(403).JSON(fiber.Map{"error": "remote .xdc download is disabled by admin (enable in Settings → WebXDC)"})
	}
	claims, err := h.claims(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
	}
	room, err := h.loadRoom(c)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "room not found"})
	}
	// Prefer gallery start for any room participant, not only moderators.
	if !h.canAccessRoom(claims, room) {
		return c.Status(403).JSON(fiber.Map{"error": "forbidden: no access to this room"})
	}

	var req struct {
		XdcURL string `json:"xdcUrl"`
		Name   string `json:"name"`
	}
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.XdcURL) == "" {
		return c.Status(400).JSON(fiber.Map{"error": "xdcUrl required"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	limits := h.effectiveLimits()
	data, err := wx.FetchXdcArchive(ctx, req.XdcURL, limits.MaxArchiveBytes)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "download failed: " + err.Error()})
	}

	if err := wx.ValidateZip(data, limits); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	pkgID := uuid.New().String()
	if err := os.MkdirAll(h.cfg.Webxdc.StorageDir, 0o750); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "storage error"})
	}
	storageKey := filepath.Join(room.ID, pkgID+".xdc")
	abs := filepath.Join(h.cfg.Webxdc.StorageDir, storageKey)
	if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "storage error"})
	}
	if err := os.WriteFile(abs, data, 0o640); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "storage error"})
	}

	meta := wx.ParseManifestFromZip(data)
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = meta.Name
	}
	if name == "" {
		name = "gallery-app.xdc"
	}
	pkg := &models.WebxdcPackage{
		ID:            pkgID,
		RoomID:        room.ID,
		ContentHash:   hash,
		StorageKey:    storageKey,
		SizeBytes:     int64(len(data)),
		Name:          name,
		SourceCodeURL: meta.SourceCodeURL,
		IconPath:      meta.IconPath,
		UploadedBy:    claims.UserID,
	}
	if icon, ct, ierr := wx.ExtractSafeIcon(data); ierr == nil {
		_ = os.WriteFile(abs+wx.IconStorageSuffix, icon, 0o640)
		_ = os.WriteFile(abs+wx.IconTypeStorageSuffix, []byte(ct), 0o640)
		if pkg.IconPath == "" {
			pkg.IconPath = "icon.png"
		}
	}
	if err := h.repo.CreatePackage(pkg); err != nil {
		_ = os.Remove(abs)
		_ = os.Remove(abs + wx.IconStorageSuffix)
		_ = os.Remove(abs + wx.IconTypeStorageSuffix)
		return c.Status(500).JSON(fiber.Map{"error": "db error"})
	}
	return c.Status(201).JSON(pkg)
}

// AdminListInstanceCatalog GET /api/admin/webxdc/catalog
func (h *WebxdcHandler) AdminListInstanceCatalog(c *fiber.Ctx) error {
	if !h.active() {
		return h.notEnabled(c)
	}
	list, err := h.repo.ListInstanceCatalogPackages()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "db error"})
	}
	return c.JSON(fiber.Map{"packages": list})
}

// AdminUpdateInstanceCatalog PUT /api/admin/webxdc/catalog/:id
// Updates display metadata only (name, description, category, sourceCodeUrl).
// Never accepts storage paths, archive bytes, or HTML — plain text fields with max lengths.
func (h *WebxdcHandler) AdminUpdateInstanceCatalog(c *fiber.Ctx) error {
	if !h.active() {
		return h.notEnabled(c)
	}
	id := c.Params("id")
	if !safePackageID(id) {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}
	pkg, err := h.repo.GetPackage(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "package not found"})
	}
	if pkg.RoomID != "" {
		return c.Status(400).JSON(fiber.Map{"error": "not an instance-catalog package"})
	}

	var req struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		Category      string `json:"category"`
		SourceCodeURL string `json:"sourceCodeUrl"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}

	name, err := sanitizeCatalogText(req.Name, 255, true)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "name: " + err.Error()})
	}
	desc, err := sanitizeCatalogText(req.Description, 2000, false)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "description: " + err.Error()})
	}
	category, err := sanitizeCatalogText(req.Category, 64, false)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "category: " + err.Error()})
	}
	srcURL, err := sanitizeOptionalHTTPSURL(req.SourceCodeURL, 512)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "sourceCodeUrl: " + err.Error()})
	}

	if err := h.repo.UpdatePackageMetadata(id, name, desc, category, srcURL); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "db error"})
	}
	updated, err := h.repo.GetPackage(id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "db error"})
	}
	return c.JSON(updated)
}

// AdminUploadInstanceCatalog POST /api/admin/webxdc/catalog (multipart file)
func (h *WebxdcHandler) AdminUploadInstanceCatalog(c *fiber.Ctx) error {
	if !h.active() {
		return h.notEnabled(c)
	}
	claims, err := h.claims(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "file required"})
	}
	f, err := fileHeader.Open()
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "cannot open file"})
	}
	defer f.Close()
	limits := h.effectiveLimits()
	if fileHeader.Size > limits.MaxArchiveBytes {
		return c.Status(413).JSON(fiber.Map{"error": "archive too large"})
	}
	data := make([]byte, 0, fileHeader.Size)
	buf := make([]byte, 32*1024)
	var total int64
	for {
		n, rerr := f.Read(buf)
		if n > 0 {
			total += int64(n)
			if total > limits.MaxArchiveBytes {
				return c.Status(413).JSON(fiber.Map{"error": "archive too large"})
			}
			data = append(data, buf[:n]...)
		}
		if rerr != nil {
			break
		}
	}
	if err := wx.ValidateZip(data, limits); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	pkgID := uuid.New().String()
	if err := os.MkdirAll(h.cfg.Webxdc.StorageDir, 0o750); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "storage error"})
	}
	// Global packages live under _instance/ (not a room id).
	storageKey := filepath.Join("_instance", pkgID+".xdc")
	abs := filepath.Join(h.cfg.Webxdc.StorageDir, storageKey)
	if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "storage error"})
	}
	if err := os.WriteFile(abs, data, 0o640); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "storage error"})
	}
	meta := wx.ParseManifestFromZip(data)
	name := meta.Name
	if name == "" {
		name = fileHeader.Filename
		if name == "" {
			name = "app.xdc"
		}
	}
	pkg := &models.WebxdcPackage{
		ID:            pkgID,
		RoomID:        "", // instance / global catalog
		ContentHash:   hash,
		StorageKey:    storageKey,
		SizeBytes:     int64(len(data)),
		Name:          name,
		SourceCodeURL: meta.SourceCodeURL,
		IconPath:      meta.IconPath,
		UploadedBy:    claims.UserID,
	}
	// Extract raster icon only (no SVG) into a sidecar next to the archive.
	if icon, ct, ierr := wx.ExtractSafeIcon(data); ierr == nil {
		_ = os.WriteFile(abs+wx.IconStorageSuffix, icon, 0o640)
		_ = os.WriteFile(abs+wx.IconTypeStorageSuffix, []byte(ct), 0o640)
		if pkg.IconPath == "" {
			pkg.IconPath = "icon.png"
		}
	}
	if err := h.repo.CreatePackage(pkg); err != nil {
		_ = os.Remove(abs)
		_ = os.Remove(abs + wx.IconStorageSuffix)
		_ = os.Remove(abs + wx.IconTypeStorageSuffix)
		return c.Status(500).JSON(fiber.Map{"error": "db error"})
	}
	return c.Status(201).JSON(pkg)
}

// AdminDeleteInstanceCatalog DELETE /api/admin/webxdc/catalog/:id
func (h *WebxdcHandler) AdminDeleteInstanceCatalog(c *fiber.Ctx) error {
	if !h.active() {
		return h.notEnabled(c)
	}
	id := c.Params("id")
	if !safePackageID(id) {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}
	pkg, err := h.repo.GetPackage(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "package not found"})
	}
	if pkg.RoomID != "" {
		return c.Status(400).JSON(fiber.Map{"error": "not an instance-catalog package"})
	}
	abs, err := safeUnderStorage(h.cfg.Webxdc.StorageDir, pkg.StorageKey)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "storage path error"})
	}
	_ = os.Remove(abs)
	_ = os.Remove(abs + wx.IconStorageSuffix)
	_ = os.Remove(abs + wx.IconTypeStorageSuffix)
	if err := h.repo.DeletePackage(id); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "db error"})
	}
	return c.JSON(fiber.Map{"ok": true})
}

// AdminGetInstanceCatalogIcon GET /api/admin/webxdc/catalog/:id/icon
// Admin-only; instance catalog packages only.
func (h *WebxdcHandler) AdminGetInstanceCatalogIcon(c *fiber.Ctx) error {
	if !h.active() {
		return h.notEnabled(c)
	}
	id := c.Params("id")
	if !safePackageID(id) {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}
	pkg, err := h.repo.GetPackage(id)
	if err != nil || pkg.RoomID != "" {
		return c.Status(404).JSON(fiber.Map{"error": "not found"})
	}
	return h.sendPackageIcon(c, pkg)
}

// GetPackageIcon GET /api/webxdc/packages/:id/icon
// Authenticated meeting/admin clients. Instance catalog icons for any signed-in user;
// room package icons only if the user can access that room. Raster only (no SVG).
func (h *WebxdcHandler) GetPackageIcon(c *fiber.Ctx) error {
	if !h.active() {
		return h.notEnabled(c)
	}
	claims, err := h.claims(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
	}
	id := c.Params("id")
	if !safePackageID(id) {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}
	pkg, err := h.repo.GetPackage(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "not found"})
	}
	// Instance catalog (empty room_id): any authenticated user may view the icon.
	// Room packages: require membership/access to that room.
	if pkg.RoomID != "" {
		room, rerr := h.roomRepo.GetRoom(pkg.RoomID)
		if rerr != nil || room == nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		if !h.canAccessRoom(claims, room) {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
	}
	return h.sendPackageIcon(c, pkg)
}

// packageHasSafeIcon reports whether a re-sniffable raster sidecar exists.
func (h *WebxdcHandler) packageHasSafeIcon(pkg *models.WebxdcPackage) bool {
	if pkg == nil {
		return false
	}
	abs, err := safeUnderStorage(h.cfg.Webxdc.StorageDir, pkg.StorageKey)
	if err != nil {
		return false
	}
	data, err := os.ReadFile(abs + wx.IconStorageSuffix)
	if err != nil || len(data) == 0 {
		return false
	}
	return wx.SniffImageContentType(data) != ""
}

// sendPackageIcon serves the extracted raster icon (nosniff). Storage key comes from DB only.
func (h *WebxdcHandler) sendPackageIcon(c *fiber.Ctx, pkg *models.WebxdcPackage) error {
	abs, err := safeUnderStorage(h.cfg.Webxdc.StorageDir, pkg.StorageKey)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "not found"})
	}
	data, err := os.ReadFile(abs + wx.IconStorageSuffix)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "no icon"})
	}
	// Re-sniff — never trust stored type alone for Content-Type.
	ct := wx.SniffImageContentType(data)
	if ct == "" {
		return c.Status(404).JSON(fiber.Map{"error": "invalid icon"})
	}
	c.Set("Content-Type", ct)
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("Cache-Control", "private, max-age=3600")
	c.Set("Content-Security-Policy", "default-src 'none'")
	return c.Send(data)
}

func safePackageID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	// UUID or hex-like only — no path separators or traversal.
	for _, r := range id {
		if (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return false
	}
	return true
}

// sanitizeCatalogText trims, strips control chars (except newline/tab in multi-line), enforces max len.
// required=true rejects empty after trim. Never interprets as path/HTML/shell.
func sanitizeCatalogText(s string, max int, required bool) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		if required {
			return "", fmt.Errorf("required")
		}
		return "", nil
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		// Allow printable Unicode + common whitespace; drop other controls.
		if r == '\n' || r == '\r' || r == '\t' || r >= 32 {
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	// Collapse excessive newlines for description safety.
	out = strings.ReplaceAll(out, "\r\n", "\n")
	out = strings.ReplaceAll(out, "\r", "\n")
	if len(out) > max {
		return "", fmt.Errorf("too long (max %d)", max)
	}
	if required && out == "" {
		return "", fmt.Errorf("required")
	}
	return out, nil
}

// sanitizeOptionalHTTPSURL allows empty or absolute https URLs only (blocks javascript:, file:, relative).
func sanitizeOptionalHTTPSURL(raw string, max int) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if len(raw) > max {
		return "", fmt.Errorf("too long (max %d)", max)
	}
	// Reject control characters and whitespace inside URL.
	for _, r := range raw {
		if r < 32 || r == 127 {
			return "", fmt.Errorf("invalid characters")
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid URL")
	}
	if u.Scheme != "https" {
		return "", fmt.Errorf("must be https")
	}
	if u.Host == "" {
		return "", fmt.Errorf("missing host")
	}
	// Block userinfo@ to reduce credential-smuggling in links.
	if u.User != nil {
		return "", fmt.Errorf("credentials not allowed")
	}
	return u.String(), nil
}

// safeUnderStorage joins root + relative key and rejects escapes (path traversal).
func safeUnderStorage(root, key string) (string, error) {
	root = filepath.Clean(root)
	key = filepath.Clean(key)
	if key == "." || key == ".." || strings.HasPrefix(key, ".."+string(os.PathSeparator)) || filepath.IsAbs(key) {
		return "", fmt.Errorf("invalid storage key")
	}
	abs := filepath.Join(root, key)
	// Ensure abs is strictly under root.
	rel, err := filepath.Rel(root, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path escape")
	}
	return abs, nil
}

func (h *WebxdcHandler) applyWebxdcSecurityHeaders(c *fiber.Ctx) {
	// SPA origin(s) that embed the mini-app (never same-origin as webxdc host).
	ancestors := wx.FrameAncestorsFromFrontendURL(hFrontendURL(h.cfg))
	for k, v := range wx.SecurityHeadersFor(ancestors) {
		c.Set(k, v)
	}
}

func hFrontendURL(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	if u := strings.TrimSpace(cfg.Auth.FrontendURL); u != "" {
		return u
	}
	// Local make dev default
	return "http://localhost:7070"
}

func randomHostLabel(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// SelfAddrFor returns opaque selfAddr for bridge init (used by clients via API if needed).
func (h *WebxdcHandler) SelfAddrFor(roomID, instanceID, userID string) string {
	secret := h.cfg.Auth.JWTSecret
	if secret == "" {
		secret = h.cfg.Auth.SessionSecret
	}
	return wx.SelfAddr(secret, roomID, instanceID, userID)
}
