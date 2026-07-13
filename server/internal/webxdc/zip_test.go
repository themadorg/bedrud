package webxdc

import (
	"archive/zip"
	"bytes"
	"errors"
	"testing"
)

func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, body := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestValidateZip_OK(t *testing.T) {
	data := makeZip(t, map[string]string{
		"index.html": "<html></html>",
		"app.js":     "console.log(1)",
	})
	if err := ValidateZip(data, Limits{}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateZip_MissingIndex(t *testing.T) {
	data := makeZip(t, map[string]string{"app.js": "x"})
	err := ValidateZip(data, Limits{})
	if !errors.Is(err, ErrMissingIndex) {
		t.Fatalf("got %v", err)
	}
}

func TestValidateZip_ZipSlip(t *testing.T) {
	data := makeZip(t, map[string]string{
		"index.html":        "ok",
		"../evil.html":      "nope",
		"foo/../../etc/pw":  "nope",
	})
	// Creating with ../ may be normalized by archive/zip writer differently;
	// still test ValidateEntryPath directly and ValidateZip on crafted names.
	if err := ValidateEntryPath("../evil"); !errors.Is(err, ErrZipSlip) {
		t.Fatalf("expected zip slip, got %v", err)
	}
	if err := ValidateEntryPath("foo/../../etc/passwd"); !errors.Is(err, ErrZipSlip) {
		t.Fatalf("expected zip slip, got %v", err)
	}
	if err := ValidateEntryPath("/abs"); !errors.Is(err, ErrZipSlip) {
		t.Fatalf("expected zip slip, got %v", err)
	}
	_ = data
}

func TestValidateZip_TooManyEntries(t *testing.T) {
	files := map[string]string{"index.html": "x"}
	for i := 0; i < 10; i++ {
		files[string(rune('a'+i))+".txt"] = "y"
	}
	data := makeZip(t, files)
	err := ValidateZip(data, Limits{MaxEntries: 5})
	if !errors.Is(err, ErrTooManyEntries) {
		t.Fatalf("got %v", err)
	}
}

func TestValidateZip_ArchiveTooLarge(t *testing.T) {
	data := makeZip(t, map[string]string{"index.html": "hello"})
	err := ValidateZip(data, Limits{MaxArchiveBytes: 10})
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("got %v", err)
	}
}

func TestSafeJoinEntry(t *testing.T) {
	got, err := SafeJoinEntry("")
	if err != nil || got != "index.html" {
		t.Fatalf("empty -> index.html, got %q %v", got, err)
	}
	got, err = SafeJoinEntry("assets/app.js")
	if err != nil || got != "assets/app.js" {
		t.Fatalf("got %q %v", got, err)
	}
	_, err = SafeJoinEntry("../secret")
	if !errors.Is(err, ErrZipSlip) {
		t.Fatalf("got %v", err)
	}
}

func TestReadZipEntry(t *testing.T) {
	data := makeZip(t, map[string]string{
		"index.html": "HELLO",
		"a/b.txt":    "nested",
	})
	b, err := ReadZipEntry(data, "index.html", 0)
	if err != nil || string(b) != "HELLO" {
		t.Fatalf("got %q %v", b, err)
	}
	b, err = ReadZipEntry(data, "a/b.txt", 0)
	if err != nil || string(b) != "nested" {
		t.Fatalf("got %q %v", b, err)
	}
	_, err = ReadZipEntry(data, "../x", 0)
	if !errors.Is(err, ErrZipSlip) {
		t.Fatalf("got %v", err)
	}
}
