package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileUnlessExists_writesWhenMissing(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	want := []byte("new-config\n")

	wrote, err := writeFileUnlessExists(p, want, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if !wrote {
		t.Fatal("expected write when file is missing")
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestWriteFileUnlessExists_preservesExisting(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	orig := []byte("keep-tls-and-secrets\n")
	if err := os.WriteFile(p, orig, 0o600); err != nil {
		t.Fatal(err)
	}

	wrote, err := writeFileUnlessExists(p, []byte("should-not-land\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if wrote {
		t.Fatal("must not overwrite an existing config file")
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(orig) {
		t.Fatalf("existing file changed: got %q", got)
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.yaml")
	ok, err := fileExists(p)
	if err != nil || ok {
		t.Fatalf("missing file: exists=%v err=%v", ok, err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	ok, err = fileExists(p)
	if err != nil || !ok {
		t.Fatalf("present file: exists=%v err=%v", ok, err)
	}
}
