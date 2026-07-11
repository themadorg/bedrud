package webxdc

import (
	"archive/zip"
	"bytes"
	"io"
	"regexp"
	"strings"
)

var (
	reName           = regexp.MustCompile(`(?m)^\s*name\s*=\s*"([^"]*)"`)
	reSourceCodeURL  = regexp.MustCompile(`(?m)^\s*source_code_url\s*=\s*"([^"]*)"`)
)

// ManifestMeta extracted from package root.
type ManifestMeta struct {
	Name          string
	SourceCodeURL string
	IconPath      string
}

// ParseManifestFromZip reads optional manifest.toml and icon from ZIP bytes.
func ParseManifestFromZip(data []byte) ManifestMeta {
	var meta ManifestMeta
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return meta
	}
	for _, f := range r.File {
		name := strings.TrimPrefix(strings.ReplaceAll(f.Name, "\\", "/"), "/")
		if name == "manifest.toml" {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			b, _ := io.ReadAll(io.LimitReader(rc, 64*1024))
			_ = rc.Close()
			s := string(b)
			if m := reName.FindStringSubmatch(s); len(m) > 1 {
				meta.Name = m[1]
			}
			if m := reSourceCodeURL.FindStringSubmatch(s); len(m) > 1 {
				meta.SourceCodeURL = m[1]
			}
		}
		if name == "icon.png" || name == "icon.jpg" {
			meta.IconPath = name
		}
	}
	return meta
}
