package hostbin

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestTrailerOfBytes(t *testing.T) {
	if trailerOfBytes([]byte("short")) != nil {
		t.Fatal("short")
	}
	tbytes := append(bytes.Repeat([]byte{1}, 32), []byte(keyMagic)...)
	body := append([]byte("ELF"), tbytes...)
	got := trailerOfBytes(body)
	if !bytes.Equal(got, tbytes) {
		t.Fatalf("%q", got)
	}
}

func TestInstallPreservesExistingTrailer(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "bedrud-host")
	tr := append(bytes.Repeat([]byte{9}, 32), []byte(keyMagic)...)
	if err := os.WriteFile(old, append([]byte("OLDBIN"), tr...), 0o755); err != nil {
		t.Fatal(err)
	}
	got := trailerOf(old)
	if !bytes.Equal(got, tr) {
		t.Fatalf("%v", got)
	}
}
