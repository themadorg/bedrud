package install

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestSafeArchivePath(t *testing.T) {
	dest := t.TempDir()
	destAbs, _ := filepath.Abs(dest)

	ok, err := safeArchivePath(destAbs, "bedrud")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(ok) != "bedrud" {
		t.Fatalf("got %s", ok)
	}

	for _, bad := range []string{
		"../etc/passwd",
		"/etc/passwd",
		"foo/../../etc/passwd",
		"..",
	} {
		if _, err := safeArchivePath(destAbs, bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestSafeExtractArchive_happy(t *testing.T) {
	dir := t.TempDir()
	arch := filepath.Join(dir, "test.tar.gz")
	if err := writeTarGz(arch, map[string][]byte{
		"bedrud": []byte("\x7fELFfake-binary-content-for-test"),
	}); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(dir, "out")
	bin, err := safeExtractArchive(arch, out)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte{0x7f, 'E', 'L', 'F'}) {
		t.Fatalf("unexpected content")
	}
}

func TestSafeExtractArchive_rejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	arch := filepath.Join(dir, "evil.tar.gz")
	if err := writeTarGz(arch, map[string][]byte{
		"../escape": []byte("nope"),
		"bedrud":    []byte("\x7fELF"),
	}); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if _, err := safeExtractArchive(arch, out); err == nil {
		t.Fatal("expected traversal rejection")
	}
}

func TestSafeExtractArchive_rejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	arch := filepath.Join(dir, "link.tar.gz")
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	hdr := &tar.Header{
		Name:     "evil-link",
		Typeflag: tar.TypeSymlink,
		Linkname: "/etc/passwd",
		Mode:     0o777,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	// also include bedrud so failure is specifically the link
	content := []byte("\x7fELF")
	if err := tw.WriteHeader(&tar.Header{Name: "bedrud", Size: int64(len(content)), Mode: 0o755, Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	_ = tw.Close()
	_ = gw.Close()
	if err := os.WriteFile(arch, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if _, err := safeExtractArchive(arch, out); err == nil {
		t.Fatal("expected symlink rejection")
	}
}

func writeTarGz(path string, files map[string][]byte) error {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for name, content := range files {
		hdr := &tar.Header{
			Name:     name,
			Size:     int64(len(content)),
			Mode:     0o755,
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := tw.Write(content); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := gw.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o600)
}

func TestParseSHA256SUMS(t *testing.T) {
	content := `
# comment
abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789  bedrud_linux_amd64.tar.xz
deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef *other.bin
`
	// invalid length on first - wait we used 64 hex for first
	sum, err := parseSHA256SUMS(content, "bedrud_linux_amd64.tar.xz")
	if err != nil {
		t.Fatal(err)
	}
	if sum != "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789" {
		t.Fatalf("got %s", sum)
	}
	if _, err := parseSHA256SUMS(content, "missing"); err == nil {
		t.Fatal("expected missing error")
	}
}
