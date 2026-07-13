package webxdc

import (
	"archive/zip"
	"bytes"
	"errors"
	"testing"
)

func TestValidateZip_Empty(t *testing.T) {
	if err := ValidateZip(nil, Limits{}); !errors.Is(err, ErrEmptyArchive) {
		t.Fatalf("%v", err)
	}
	if err := ValidateZip([]byte{}, Limits{}); !errors.Is(err, ErrEmptyArchive) {
		t.Fatalf("%v", err)
	}
}

func TestValidateZip_EntryTooLarge(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("index.html")
	big := bytes.Repeat([]byte("a"), 100)
	_, _ = f.Write(big)
	_ = w.Close()
	err := ValidateZip(buf.Bytes(), Limits{MaxSingleFile: 10, MaxArchiveBytes: 1 << 20, MaxUncompressedTotal: 1 << 20, MaxEntries: 10})
	if !errors.Is(err, ErrEntryTooLarge) {
		t.Fatalf("%v", err)
	}
}

func TestReadZipEntry_NotFound(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("index.html")
	_, _ = f.Write([]byte("hi"))
	_ = w.Close()
	_, err := ReadZipEntry(buf.Bytes(), "nope.js", 0)
	if err == nil {
		t.Fatal("expected not found")
	}
}

func TestValidateEntryPath_OK(t *testing.T) {
	if err := ValidateEntryPath("assets/app.js"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateEntryPath("index.html"); err != nil {
		t.Fatal(err)
	}
}
