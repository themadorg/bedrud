package install

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateUpdateOptions(t *testing.T) {
	if err := validateUpdateOptions(UpdateOptions{}); err == nil {
		t.Fatal("expected error for empty options")
	}
	if err := validateUpdateOptions(UpdateOptions{Self: true}); err != nil {
		t.Fatal(err)
	}
	if err := validateUpdateOptions(UpdateOptions{Source: "/tmp/x"}); err != nil {
		t.Fatal(err)
	}
	if err := validateUpdateOptions(UpdateOptions{SkipBinary: true}); err != nil {
		t.Fatal(err)
	}
	if err := validateUpdateOptions(UpdateOptions{Nightly: true}); err != nil {
		t.Fatal(err)
	}
	if err := validateUpdateOptions(UpdateOptions{Nightly: true, Source: "latest"}); err != nil {
		t.Fatal(err)
	}
	if err := validateUpdateOptions(UpdateOptions{Self: true, Source: "x"}); err == nil {
		t.Fatal("expected self+source error")
	}
	if err := validateUpdateOptions(UpdateOptions{SkipBinary: true, Source: "x"}); err == nil {
		t.Fatal("expected skip-binary+source error")
	}
	if err := validateUpdateOptions(UpdateOptions{Nightly: true, Self: true}); err == nil {
		t.Fatal("expected nightly+self error")
	}
	if err := validateUpdateOptions(UpdateOptions{Nightly: true, Source: "/tmp/x"}); err == nil {
		t.Fatal("expected nightly+path error")
	}
}

func TestResolveLocalBinary(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bedrud")
	// minimal ELF magic
	if err := os.WriteFile(bin, []byte{0x7f, 'E', 'L', 'F', 0, 1, 2, 3}, 0o700); err != nil {
		t.Fatal(err)
	}
	res, err := resolveLocalPath(bin, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.BinaryPath != bin {
		t.Fatalf("got %s", res.BinaryPath)
	}
}

func TestResolveLocalArchive(t *testing.T) {
	dir := t.TempDir()
	arch := filepath.Join(dir, "rel.tar.gz")
	if err := writeTarGz(arch, map[string][]byte{
		"bedrud": {0x7f, 'E', 'L', 'F', 0},
	}); err != nil {
		t.Fatal(err)
	}
	res, err := resolveLocalPath(arch, true)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Cleanup()
	if err := quickELFCheck(res.BinaryPath); err != nil {
		t.Fatal(err)
	}
}

func TestResolveLocalChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bedrud")
	data := []byte{0x7f, 'E', 'L', 'F', 9}
	if err := os.WriteFile(bin, data, 0o700); err != nil {
		t.Fatal(err)
	}
	// wrong sum
	sums := "0000000000000000000000000000000000000000000000000000000000000000  bedrud\n"
	if err := os.WriteFile(filepath.Join(dir, "SHA256SUMS"), []byte(sums), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveLocalPath(bin, false); err == nil {
		t.Fatal("expected checksum mismatch")
	}
	// correct sum
	h := sha256.Sum256(data)
	sums = hex.EncodeToString(h[:]) + "  bedrud\n"
	if err := os.WriteFile(filepath.Join(dir, "SHA256SUMS"), []byte(sums), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveLocalPath(bin, false); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPURLRejected(t *testing.T) {
	_, err := resolveURL("http://example.com/bedrud", true)
	if err == nil {
		t.Fatal("expected http reject")
	}
}

func TestLatestSkipChecksumRejected(t *testing.T) {
	_, err := resolveLatest(true)
	if err == nil {
		t.Fatal("expected skip-checksum reject for latest")
	}
}
