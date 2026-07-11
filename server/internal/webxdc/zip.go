package webxdc

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
)

// Default package limits (config can override later).
const (
	DefaultMaxArchiveBytes      = 10 << 20 // 10 MiB
	DefaultMaxUncompressedTotal = 30 << 20 // 30 MiB
	DefaultMaxEntries           = 500
	DefaultMaxSingleFile        = 5 << 20 // 5 MiB
)

var (
	ErrTooLarge          = errors.New("webxdc: archive too large")
	ErrTooManyEntries    = errors.New("webxdc: too many entries")
	// ErrEntryTooLarge is returned when a single zip entry exceeds MaxSingleFile.
	// Prefer fmt.Errorf with the limit for user-facing handlers; this sentinel remains for tests.
	ErrEntryTooLarge = errors.New("webxdc: entry too large")
	ErrUncompressedTotal = errors.New("webxdc: uncompressed size exceeds limit")
	ErrMissingIndex      = errors.New("webxdc: missing index.html at package root")
	ErrZipSlip           = errors.New("webxdc: invalid entry path")
	ErrEmptyArchive      = errors.New("webxdc: empty archive")
	ErrInvalidZip        = errors.New("webxdc: invalid zip")
)

// Limits for ValidateZip.
type Limits struct {
	MaxArchiveBytes      int64
	MaxUncompressedTotal int64
	MaxEntries           int
	MaxSingleFile        int64
}

func (l Limits) withDefaults() Limits {
	if l.MaxArchiveBytes <= 0 {
		l.MaxArchiveBytes = DefaultMaxArchiveBytes
	}
	if l.MaxUncompressedTotal <= 0 {
		l.MaxUncompressedTotal = DefaultMaxUncompressedTotal
	}
	if l.MaxEntries <= 0 {
		l.MaxEntries = DefaultMaxEntries
	}
	if l.MaxSingleFile <= 0 {
		l.MaxSingleFile = DefaultMaxSingleFile
	}
	return l
}

// ValidateZip checks a .xdc ZIP for size, zip-slip, and required index.html.
func ValidateZip(data []byte, limits Limits) error {
	limits = limits.withDefaults()
	if int64(len(data)) > limits.MaxArchiveBytes {
		return ErrTooLarge
	}
	if len(data) == 0 {
		return ErrEmptyArchive
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidZip, err)
	}
	if len(r.File) == 0 {
		return ErrEmptyArchive
	}
	if len(r.File) > limits.MaxEntries {
		return ErrTooManyEntries
	}

	var totalUncompressed int64
	hasIndex := false
	for _, f := range r.File {
		name := f.Name
		if err := ValidateEntryPath(name); err != nil {
			return err
		}
		// Directories may end with /
		if strings.HasSuffix(name, "/") {
			continue
		}
		if f.UncompressedSize64 > uint64(limits.MaxSingleFile) {
			return fmt.Errorf("%w (file %q is %d bytes; max single entry %d bytes — raise Admin → WebXDC → Max single file)",
				ErrEntryTooLarge, path.Base(name), f.UncompressedSize64, limits.MaxSingleFile)
		}
		totalUncompressed += int64(f.UncompressedSize64)
		if totalUncompressed > limits.MaxUncompressedTotal {
			return fmt.Errorf("%w (total uncompressed %d > max %d — raise Admin → WebXDC → Max uncompressed total)",
				ErrUncompressedTotal, totalUncompressed, limits.MaxUncompressedTotal)
		}
		clean := path.Clean("/" + strings.ReplaceAll(name, "\\", "/"))
		clean = strings.TrimPrefix(clean, "/")
		if clean == "index.html" {
			hasIndex = true
		}
	}
	if !hasIndex {
		return ErrMissingIndex
	}
	return nil
}

// ValidateEntryPath rejects zip-slip and absolute paths.
func ValidateEntryPath(name string) error {
	if name == "" {
		return ErrZipSlip
	}
	// Disallow absolute / drive paths and null bytes.
	if strings.Contains(name, "\x00") {
		return ErrZipSlip
	}
	n := strings.ReplaceAll(name, "\\", "/")
	if strings.HasPrefix(n, "/") || strings.HasPrefix(n, "../") || strings.Contains(n, "/../") {
		return ErrZipSlip
	}
	// path.Clean tricks: "foo/../../etc/passwd"
	cleaned := path.Clean("/" + n)
	if !strings.HasPrefix(cleaned, "/") {
		return ErrZipSlip
	}
	// After clean, must not escape root.
	rel := strings.TrimPrefix(cleaned, "/")
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return ErrZipSlip
	}
	// Reject Windows absolute "C:/..."
	if len(n) >= 2 && n[1] == ':' {
		return ErrZipSlip
	}
	return nil
}

// SafeJoinEntry maps a request path to a zip entry name under root.
// Returns ErrZipSlip if the path would escape.
func SafeJoinEntry(requestPath string) (string, error) {
	p := strings.TrimSpace(requestPath)
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		p = "index.html"
	}
	if err := ValidateEntryPath(p); err != nil {
		return "", err
	}
	cleaned := path.Clean("/" + p)
	rel := strings.TrimPrefix(cleaned, "/")
	if rel == "" || rel == "." {
		return "index.html", nil
	}
	if err := ValidateEntryPath(rel); err != nil {
		return "", err
	}
	return rel, nil
}

// ReadZipEntry returns the contents of a single entry (after validation).
func ReadZipEntry(data []byte, entry string, maxSingle int64) ([]byte, error) {
	if maxSingle <= 0 {
		maxSingle = DefaultMaxSingleFile
	}
	entry, err := SafeJoinEntry(entry)
	if err != nil {
		return nil, err
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidZip, err)
	}
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "/") {
			continue
		}
		clean := path.Clean("/" + strings.ReplaceAll(f.Name, "\\", "/"))
		clean = strings.TrimPrefix(clean, "/")
		if clean != entry {
			continue
		}
		if f.UncompressedSize64 > uint64(maxSingle) {
			return nil, ErrEntryTooLarge
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		limited := io.LimitReader(rc, maxSingle+1)
		b, err := io.ReadAll(limited)
		if err != nil {
			return nil, err
		}
		if int64(len(b)) > maxSingle {
			return nil, ErrEntryTooLarge
		}
		return b, nil
	}
	return nil, fmt.Errorf("webxdc: entry not found: %s", entry)
}
