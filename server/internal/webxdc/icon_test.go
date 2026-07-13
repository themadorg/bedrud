package webxdc

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestSniffImageContentType(t *testing.T) {
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0}
	if SniffImageContentType(png) != "image/png" {
		t.Fatal("png")
	}
	jpg := []byte{0xff, 0xd8, 0xff, 0xe0, 0, 0, 0, 0, 0, 0, 0, 0}
	if SniffImageContentType(jpg) != "image/jpeg" {
		t.Fatal("jpg")
	}
	// SVG must never be accepted
	svg := []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"></svg>`)
	if SniffImageContentType(svg) != "" {
		t.Fatal("svg should be rejected")
	}
}

func TestExtractSafeIcon_PNG(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	// minimal PNG header padded
	png := make([]byte, 32)
	copy(png, []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a})
	fw, _ := w.Create("icon.png")
	_, _ = fw.Write(png)
	_ = w.Close()

	data, ct, err := ExtractSafeIcon(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if ct != "image/png" || len(data) < 8 {
		t.Fatalf("ct=%s len=%d", ct, len(data))
	}
}

func TestExtractSafeIcon_ZipSlipRejected(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	fw, _ := w.Create("../evil.png")
	png := make([]byte, 32)
	copy(png, []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a})
	_, _ = fw.Write(png)
	_ = w.Close()
	_, _, err := ExtractSafeIcon(buf.Bytes())
	if err == nil {
		t.Fatal("expected no icon from traversal path")
	}
}
