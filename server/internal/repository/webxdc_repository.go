package repository

import (
	"errors"
	"time"

	"bedrud/internal/models"

	"gorm.io/gorm"
)

type WebxdcRepository struct {
	db *gorm.DB
}

func NewWebxdcRepository(db *gorm.DB) *WebxdcRepository {
	return &WebxdcRepository{db: db}
}

func (r *WebxdcRepository) CreatePackage(p *models.WebxdcPackage) error {
	return r.db.Create(p).Error
}

func (r *WebxdcRepository) GetPackage(id string) (*models.WebxdcPackage, error) {
	var p models.WebxdcPackage
	if err := r.db.Where("id = ?", id).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *WebxdcRepository) ListPackagesByRoom(roomID string) ([]models.WebxdcPackage, error) {
	var list []models.WebxdcPackage
	err := r.db.Where("room_id = ?", roomID).Order("created_at desc").Find(&list).Error
	return list, err
}

// ListInstanceCatalogPackages returns server-global packages (room_id empty).
func (r *WebxdcRepository) ListInstanceCatalogPackages() ([]models.WebxdcPackage, error) {
	var list []models.WebxdcPackage
	err := r.db.Where("room_id = ? OR room_id IS NULL", "").Order("created_at desc").Find(&list).Error
	return list, err
}

func (r *WebxdcRepository) DeletePackage(id string) error {
	return r.db.Where("id = ?", id).Delete(&models.WebxdcPackage{}).Error
}

// UpdatePackageMetadata updates display fields only (name, description, category, source URL).
// Never touches storage key, hash, room, or binary archive.
func (r *WebxdcRepository) UpdatePackageMetadata(id, name, description, category, sourceCodeURL string) error {
	return r.db.Model(&models.WebxdcPackage{}).Where("id = ?", id).Updates(map[string]interface{}{
		"name":            name,
		"description":     description,
		"category":        category,
		"source_code_url": sourceCodeURL,
	}).Error
}

// FindRoomPackageByHash returns a room package with the same content hash if any.
func (r *WebxdcRepository) FindRoomPackageByHash(roomID, contentHash string) (*models.WebxdcPackage, error) {
	var p models.WebxdcPackage
	err := r.db.Where("room_id = ? AND content_hash = ?", roomID, contentHash).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *WebxdcRepository) CreateInstance(inst *models.WebxdcInstance) error {
	return r.db.Create(inst).Error
}

func (r *WebxdcRepository) GetInstance(id string) (*models.WebxdcInstance, error) {
	var inst models.WebxdcInstance
	if err := r.db.Preload("Package").Where("id = ?", id).First(&inst).Error; err != nil {
		return nil, err
	}
	return &inst, nil
}

func (r *WebxdcRepository) ListInstancesByRoom(roomID string, includeClosed bool) ([]models.WebxdcInstance, error) {
	var list []models.WebxdcInstance
	q := r.db.Preload("Package").Where("room_id = ?", roomID)
	if !includeClosed {
		q = q.Where("closed_at IS NULL")
	}
	err := q.Order("created_at desc").Find(&list).Error
	return list, err
}

func (r *WebxdcRepository) CloseInstance(id string) error {
	now := time.Now().UTC()
	return r.db.Model(&models.WebxdcInstance{}).Where("id = ? AND closed_at IS NULL", id).
		Update("closed_at", now).Error
}

func (r *WebxdcRepository) CloseAllInstancesInRoom(roomID string) error {
	now := time.Now().UTC()
	return r.db.Model(&models.WebxdcInstance{}).Where("room_id = ? AND closed_at IS NULL", roomID).
		Update("closed_at", now).Error
}

func (r *WebxdcRepository) UpdateInstanceChrome(id, document, summary, lastInfo string) error {
	updates := map[string]interface{}{}
	if document != "" {
		updates["document"] = document
	}
	if summary != "" {
		updates["summary"] = summary
	}
	if lastInfo != "" {
		updates["last_info"] = lastInfo
	}
	if len(updates) == 0 {
		return nil
	}
	return r.db.Model(&models.WebxdcInstance{}).Where("id = ?", id).Updates(updates).Error
}

func (r *WebxdcRepository) NextSerial(instanceID string) (int64, error) {
	var maxSerial *int64
	err := r.db.Model(&models.WebxdcStatusUpdate{}).
		Where("instance_id = ?", instanceID).
		Select("MAX(serial)").Scan(&maxSerial).Error
	if err != nil {
		return 0, err
	}
	if maxSerial == nil {
		return 1, nil
	}
	return *maxSerial + 1, nil
}

func (r *WebxdcRepository) AppendStatusUpdate(u *models.WebxdcStatusUpdate) error {
	return r.db.Create(u).Error
}

func (r *WebxdcRepository) ListStatusUpdatesAfter(instanceID string, afterSerial int64, limit int) ([]models.WebxdcStatusUpdate, int64, error) {
	if limit <= 0 {
		limit = 200
	}
	var maxSerial int64
	_ = r.db.Model(&models.WebxdcStatusUpdate{}).
		Where("instance_id = ?", instanceID).
		Select("COALESCE(MAX(serial), 0)").Scan(&maxSerial)

	var list []models.WebxdcStatusUpdate
	err := r.db.Where("instance_id = ? AND serial > ?", instanceID, afterSerial).
		Order("serial asc").Limit(limit).Find(&list).Error
	return list, maxSerial, err
}

func (r *WebxdcRepository) TrimStatusLog(instanceID string, maxUpdates int) error {
	if maxUpdates <= 0 {
		return nil
	}
	var count int64
	if err := r.db.Model(&models.WebxdcStatusUpdate{}).Where("instance_id = ?", instanceID).Count(&count).Error; err != nil {
		return err
	}
	if count <= int64(maxUpdates) {
		return nil
	}
	// Delete oldest serials beyond cap.
	toDelete := count - int64(maxUpdates)
	var ids []uint64
	if err := r.db.Model(&models.WebxdcStatusUpdate{}).
		Where("instance_id = ?", instanceID).
		Order("serial asc").Limit(int(toDelete)).Pluck("id", &ids).Error; err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	return r.db.Where("id IN ?", ids).Delete(&models.WebxdcStatusUpdate{}).Error
}

// DeleteAllForRoom removes all WebXDC data for a room (meeting-end cascade).
func (r *WebxdcRepository) DeleteAllForRoom(roomID string) (storageKeys []string, err error) {
	var packages []models.WebxdcPackage
	if err := r.db.Where("room_id = ?", roomID).Find(&packages).Error; err != nil {
		return nil, err
	}
	for _, p := range packages {
		storageKeys = append(storageKeys, p.StorageKey)
	}

	var instanceIDs []string
	if err := r.db.Model(&models.WebxdcInstance{}).Where("room_id = ?", roomID).Pluck("id", &instanceIDs).Error; err != nil {
		return nil, err
	}
	if len(instanceIDs) > 0 {
		if err := r.db.Where("instance_id IN ?", instanceIDs).Delete(&models.WebxdcStatusUpdate{}).Error; err != nil {
			return nil, err
		}
	}
	if err := r.db.Where("room_id = ?", roomID).Delete(&models.WebxdcInstance{}).Error; err != nil {
		return nil, err
	}
	if err := r.db.Where("room_id = ?", roomID).Delete(&models.WebxdcPackage{}).Error; err != nil {
		return nil, err
	}
	return storageKeys, nil
}

func (r *WebxdcRepository) CountOpenInstances(roomID string) (int64, error) {
	var n int64
	err := r.db.Model(&models.WebxdcInstance{}).Where("room_id = ? AND closed_at IS NULL", roomID).Count(&n).Error
	return n, err
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
