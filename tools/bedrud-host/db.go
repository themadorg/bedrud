package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS hosts (
  fqdn TEXT PRIMARY KEY,
  host TEXT NOT NULL,
  zone TEXT NOT NULL,
  ipv4 TEXT,
  linode_id INTEGER,
  linode_label TEXT,
  admin_name TEXT,
  admin_email TEXT,
  admin_password TEXT,
  created_at TEXT NOT NULL,
  create_ms INTEGER,
  hourly_usd REAL,
  linode_type TEXT
);
CREATE TABLE IF NOT EXISTS settings (
  k TEXT PRIMARY KEY,
  v TEXT NOT NULL
);
`

const (
	setLinodeToken = "linode_token"
	setCFToken     = "cloudflare_token"
	setCFEmail     = "cloudflare_email"
	setCFKey       = "cloudflare_api_key"
	setCFDomain    = "cloudflare_domain"
	setCFZoneID    = "cloudflare_zone_id"
	setCreateAvgMS = "create_avg_ms"
	setCreateCount = "create_count"
)

type HostRow struct {
	ID            int
	FQDN          string
	Host          string
	Zone          string
	IPv4          string
	LinodeID      int
	LinodeLabel   string
	AdminName     string
	AdminEmail    string
	AdminPassword string
	CreatedAt     string
	CreateMS      int64
	HourlyUSD     float64
	LinodeType    string
}

type Store struct {
	dir     string
	key     []byte
	plainDB string
	db      *sql.DB
}

func openStore(dir string) (*Store, error) {
	if dir == "" {
		return nil, fmt.Errorf("config dir is empty")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	key, err := loadOrCreateKey()
	if err != nil {
		return nil, err
	}
	plain := filepath.Join(dir, "hosts.sqlite")
	enc := filepath.Join(dir, "hosts.sqlite.enc")
	if b, err := os.ReadFile(enc); err == nil {
		plainBytes, err := decryptBytes(key, b)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(plain, plainBytes, 0o600); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", plain)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, err
	}
	_, _ = db.Exec(`ALTER TABLE hosts ADD COLUMN create_ms INTEGER`)
	_, _ = db.Exec(`ALTER TABLE hosts ADD COLUMN hourly_usd REAL`)
	_, _ = db.Exec(`ALTER TABLE hosts ADD COLUMN linode_type TEXT`)
	s := &Store{dir: dir, key: key, plainDB: plain, db: db}
	if err := s.flush(); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	err := s.flush()
	cerr := s.db.Close()
	s.db = nil
	_ = os.Remove(s.plainDB)
	_ = os.Remove(s.plainDB + "-wal")
	_ = os.Remove(s.plainDB + "-shm")
	_ = os.Remove(s.plainDB + "-journal")
	if err != nil {
		return err
	}
	return cerr
}

func (s *Store) flush() error {
	if s.db == nil {
		return nil
	}
	if _, err := s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		// ignore if wal unused
		_ = err
	}
	raw, err := os.ReadFile(s.plainDB)
	if err != nil {
		return err
	}
	blob, err := encryptBytes(s.key, raw)
	if err != nil {
		return err
	}
	enc := filepath.Join(s.dir, "hosts.sqlite.enc")
	tmp := enc + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, enc)
}

func (s *Store) Upsert(h HostRow) error {
	if h.CreatedAt == "" {
		h.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := s.db.Exec(`
INSERT INTO hosts (fqdn, host, zone, ipv4, linode_id, linode_label, admin_name, admin_email, admin_password, created_at, create_ms, hourly_usd, linode_type)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(fqdn) DO UPDATE SET
  host=excluded.host, zone=excluded.zone, ipv4=excluded.ipv4,
  linode_id=excluded.linode_id, linode_label=excluded.linode_label,
  admin_name=excluded.admin_name, admin_email=excluded.admin_email,
  admin_password=excluded.admin_password, create_ms=excluded.create_ms,
  hourly_usd=excluded.hourly_usd, linode_type=excluded.linode_type
`, h.FQDN, h.Host, h.Zone, h.IPv4, h.LinodeID, h.LinodeLabel, h.AdminName, h.AdminEmail, h.AdminPassword, h.CreatedAt, h.CreateMS, h.HourlyUSD, h.LinodeType)
	if err != nil {
		return err
	}
	return s.flush()
}

const hostCols = `rowid, fqdn, host, zone, ipv4, linode_id, linode_label, admin_name, admin_email, admin_password, created_at, COALESCE(create_ms,0), COALESCE(hourly_usd,0), COALESCE(linode_type,'')`

func scanHost(s interface{ Scan(...any) error }, h *HostRow) error {
	return s.Scan(&h.ID, &h.FQDN, &h.Host, &h.Zone, &h.IPv4, &h.LinodeID, &h.LinodeLabel,
		&h.AdminName, &h.AdminEmail, &h.AdminPassword, &h.CreatedAt, &h.CreateMS, &h.HourlyUSD, &h.LinodeType)
}

func (s *Store) Get(fqdn string) (HostRow, bool, error) {
	var h HostRow
	err := scanHost(s.db.QueryRow(`SELECT `+hostCols+` FROM hosts WHERE fqdn = ?`, fqdn), &h)
	if err == sql.ErrNoRows {
		return HostRow{}, false, nil
	}
	if err != nil {
		return HostRow{}, false, err
	}
	return h, true, nil
}

func (s *Store) GetByID(id int) (HostRow, bool, error) {
	var h HostRow
	err := scanHost(s.db.QueryRow(`SELECT `+hostCols+` FROM hosts WHERE rowid = ?`, id), &h)
	if err == sql.ErrNoRows {
		return HostRow{}, false, nil
	}
	if err != nil {
		return HostRow{}, false, err
	}
	return h, true, nil
}

func (s *Store) Delete(fqdn string) error {
	if _, err := s.db.Exec(`DELETE FROM hosts WHERE fqdn = ?`, fqdn); err != nil {
		return err
	}
	return s.flush()
}

func (s *Store) List() ([]HostRow, error) {
	rows, err := s.db.Query(`SELECT ` + hostCols + ` FROM hosts ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HostRow
	for rows.Next() {
		var h HostRow
		if err := scanHost(rows, &h); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Store) SetSetting(k, v string) error {
	_, err := s.db.Exec(`INSERT INTO settings (k, v) VALUES (?, ?)
ON CONFLICT(k) DO UPDATE SET v=excluded.v`, k, v)
	if err != nil {
		return err
	}
	return s.flush()
}

func (s *Store) GetSetting(k string) (string, bool, error) {
	var v string
	err := s.db.QueryRow(`SELECT v FROM settings WHERE k = ?`, k).Scan(&v)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

func (s *Store) createStats() (avg time.Duration, n int, err error) {
	vs, ok, err := s.GetSetting(setCreateAvgMS)
	if err != nil {
		return 0, 0, err
	}
	cs, _, err := s.GetSetting(setCreateCount)
	if err != nil {
		return 0, 0, err
	}
	if !ok || vs == "" {
		return 0, 0, nil
	}
	var avgMS int64
	fmt.Sscanf(vs, "%d", &avgMS)
	fmt.Sscanf(cs, "%d", &n)
	return time.Duration(avgMS) * time.Millisecond, n, nil
}

func (s *Store) recordCreateTime(d time.Duration) (avg time.Duration, n int, err error) {
	prev, n, err := s.createStats()
	if err != nil {
		return 0, 0, err
	}
	ms := d.Milliseconds()
	if ms < 0 {
		ms = 0
	}
	total := prev.Milliseconds()*int64(n) + ms
	n++
	avgMS := total / int64(n)
	if err := s.SetSetting(setCreateAvgMS, fmt.Sprintf("%d", avgMS)); err != nil {
		return 0, 0, err
	}
	if err := s.SetSetting(setCreateCount, fmt.Sprintf("%d", n)); err != nil {
		return 0, 0, err
	}
	return time.Duration(avgMS) * time.Millisecond, n, nil
}

func fmtDuration(d time.Duration) string {
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}
	return d.Round(time.Second).String()
}
