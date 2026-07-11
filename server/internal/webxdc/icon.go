package webxdc

import (
	"bytes"
	"fmt"
)

const MaxIconBytes = 256 * 1024 // 256 KiB — enough for app icons, blocks zip bombs via ReadZipEntry

// ExtractSafeIcon reads a package icon from zip bytes.
// Security:
//   - only fixed root paths icon.png|jpg|jpeg|webp|gif (via SafeJoinEntry / ReadZipEntry)
//   - max size MaxIconBytes
//   - content sniffed; SVG and unknown types rejected (no scriptable image types)
func ExtractSafeIcon(zipData []byte) (data []byte, contentType string, err error) {
	candidates := []string{"icon.png", "icon.jpg", "icon.jpeg", "icon.webp", "icon.gif"}
	for _, name := range candidates {
		b, rerr := ReadZipEntry(zipData, name, MaxIconBytes)
		if rerr != nil {
			continue
		}
		ct := SniffImageContentType(b)
		if ct == "" {
			continue
		}
		return b, ct, nil
	}
	return nil, "", fmt.Errorf("webxdc: no safe icon found")
}

// SniffImageContentType returns a safe raster Content-Type, or empty if not allowed.
func SniffImageContentType(b []byte) string {
	if len(b) < 12 {
		return ""
	}
	// PNG
	if bytes.HasPrefix(b, []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}) {
		return "image/png"
	}
	// JPEG
	if bytes.HasPrefix(b, []byte{0xff, 0xd8, 0xff}) {
		return "image/jpeg"
	}
	// GIF
	if bytes.HasPrefix(b, []byte("GIF87a")) || bytes.HasPrefix(b, []byte("GIF89a")) {
		return "image/gif"
	}
	// WEBP (RIFF....WEBP)
	if bytes.HasPrefix(b, []byte("RIFF")) && bytes.Equal(b[8:12], []byte("WEBP")) {
		return "image/webp"
	}
	// Deliberately no image/svg+xml — SVG can carry script / XSS when served.
	return ""
}

// IconStorageSuffix is appended to the package storage path for the extracted icon blob.
const IconStorageSuffix = ".icon"

// IconTypeStorageSuffix is appended for a one-line content-type sidecar (ASCII only).
const IconTypeStorageSuffix = ".icon.ct"
