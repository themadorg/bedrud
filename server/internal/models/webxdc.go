package models

import "time"

// WebxdcPackage is a stored .xdc archive for a room (or global when RoomID empty).
type WebxdcPackage struct {
	ID            string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	RoomID        string    `json:"roomId" gorm:"type:varchar(36);index"`
	ContentHash   string    `json:"contentHash" gorm:"not null;type:varchar(64);index"`
	StorageKey    string    `json:"-" gorm:"not null;type:varchar(512)"`
	SizeBytes     int64     `json:"sizeBytes" gorm:"not null;default:0"`
	Name          string    `json:"name" gorm:"type:varchar(255)"`
	// Description is admin-editable catalog blurb (instance packages; shown in meeting gallery).
	Description string `json:"description,omitempty" gorm:"type:text"`
	// Category is admin-editable catalog tag (e.g. tools, games).
	Category      string    `json:"category,omitempty" gorm:"type:varchar(64)"`
	SourceCodeURL string    `json:"sourceCodeUrl,omitempty" gorm:"type:varchar(512)"`
	IconPath      string    `json:"iconPath,omitempty" gorm:"type:varchar(255)"`
	UploadedBy    string    `json:"uploadedBy" gorm:"type:varchar(36);index"`
	CreatedAt     time.Time `json:"createdAt" gorm:"autoCreateTime;not null"`
}

func (WebxdcPackage) TableName() string { return "webxdc_packages" }

// WebxdcInstance is a running app instance in a room (host label = ID).
type WebxdcInstance struct {
	ID        string     `json:"id" gorm:"primaryKey;type:varchar(32)"` // host label, hex
	RoomID    string     `json:"roomId" gorm:"not null;type:varchar(36);index"`
	PackageID string     `json:"packageId" gorm:"not null;type:varchar(36);index"`
	CreatedBy string     `json:"createdBy" gorm:"type:varchar(36)"`
	Document  string     `json:"document,omitempty" gorm:"type:varchar(64)"`
	Summary   string     `json:"summary,omitempty" gorm:"type:varchar(64)"`
	LastInfo  string     `json:"lastInfo,omitempty" gorm:"type:varchar(128)"`
	ClosedAt  *time.Time `json:"closedAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt" gorm:"autoCreateTime;not null"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"autoUpdateTime;not null"`
	Package   *WebxdcPackage `json:"package,omitempty" gorm:"foreignKey:PackageID"`
}

func (WebxdcInstance) TableName() string { return "webxdc_instances" }

// WebxdcStatusUpdate is one serial entry in the server status log.
type WebxdcStatusUpdate struct {
	ID             uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	InstanceID     string    `json:"instanceId" gorm:"not null;type:varchar(32);uniqueIndex:idx_webxdc_inst_serial,priority:1"`
	Serial         int64     `json:"serial" gorm:"not null;uniqueIndex:idx_webxdc_inst_serial,priority:2"`
	SenderUserID   string    `json:"senderUserId,omitempty" gorm:"type:varchar(36)"`
	SenderIdentity string    `json:"senderIdentity,omitempty" gorm:"type:varchar(255)"`
	PayloadJSON    string    `json:"payloadJson" gorm:"type:text;not null"`
	ByteSize       int       `json:"byteSize" gorm:"not null;default:0"`
	CreatedAt      time.Time `json:"createdAt" gorm:"autoCreateTime;not null"`
}

func (WebxdcStatusUpdate) TableName() string { return "webxdc_status_updates" }
