package install

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed examples/*
var examplesFS embed.FS

// docDir / docExamplesDir are the FHS locations for package/docs examples.
// Overridable in tests via setDocExamplesBase.
var (
	docDir         = "/usr/share/doc/bedrud"
	docExamplesDir = "/usr/share/doc/bedrud/examples"
)

func setDocExamplesBase(base string) {
	docDir = base
	docExamplesDir = filepath.Join(base, "examples")
}

// installDocExamples writes embedded config examples under docExamplesDir.
// Safe to call on every install/update (overwrites with current package contents).
func installDocExamples() error {
	if err := os.MkdirAll(docExamplesDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", docExamplesDir, err)
	}

	return fs.WalkDir(examplesFS, "examples", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(path, "examples/")
		if rel == "" || rel == path {
			return nil
		}
		data, err := examplesFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embed %s: %w", path, err)
		}
		dest := filepath.Join(docExamplesDir, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(dest), err)
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
		return nil
	})
}

// removeDocTree removes /usr/share/doc/bedrud (binary/tarball uninstall).
func removeDocTree() error {
	if err := os.RemoveAll(docDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", docDir, err)
	}
	return nil
}
