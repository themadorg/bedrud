package webxdc

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestParseManifestFromZip(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("index.html")
	_, _ = f.Write([]byte("<html></html>"))
	m, _ := w.Create("manifest.toml")
	_, _ = m.Write([]byte(`
name = "Poll Demo"
source_code_url = "https://example.org/poll"
`))
	ic, _ := w.Create("icon.png")
	_, _ = ic.Write([]byte{0x89, 0x50, 0x4e, 0x47})
	_ = w.Close()

	meta := ParseManifestFromZip(buf.Bytes())
	if meta.Name != "Poll Demo" {
		t.Fatalf("name=%q", meta.Name)
	}
	if meta.SourceCodeURL != "https://example.org/poll" {
		t.Fatalf("url=%q", meta.SourceCodeURL)
	}
	if meta.IconPath != "icon.png" {
		t.Fatalf("icon=%q", meta.IconPath)
	}
}

func TestParseManifestFromZip_Invalid(t *testing.T) {
	meta := ParseManifestFromZip([]byte("not-zip"))
	if meta.Name != "" || meta.IconPath != "" {
		t.Fatalf("%+v", meta)
	}
}

func TestParseManifestFromZip_Empty(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("index.html")
	_, _ = f.Write([]byte("x"))
	_ = w.Close()
	meta := ParseManifestFromZip(buf.Bytes())
	if meta.Name != "" {
		t.Fatal(meta.Name)
	}
}
