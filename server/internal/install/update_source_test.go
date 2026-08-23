package install

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ulikunitz/xz"
)

func TestInstallBinaryFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst", "bedrud")
	payload := []byte{0x7f, 'E', 'L', 'F', 1, 2, 3, 4}
	if err := os.WriteFile(src, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := installBinaryFile(dst, src); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("content mismatch")
	}
	st, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode()&0o111 == 0 {
		t.Fatalf("expected executable bits, mode=%v", st.Mode())
	}

	// overwrite (ETXTBSY-safe path)
	payload2 := []byte{0x7f, 'E', 'L', 'F', 9, 9}
	if err := os.WriteFile(src, payload2, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := installBinaryFile(dst, src); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(dst)
	if string(got) != string(payload2) {
		t.Fatalf("overwrite failed")
	}
}

func TestInstallBinaryBytesEmpty(t *testing.T) {
	if err := installBinaryBytes(filepath.Join(t.TempDir(), "x"), nil); err == nil {
		t.Fatal("expected empty binary error")
	}
}

func TestQuickELFCheckRejectsNonELF(t *testing.T) {
	p := filepath.Join(t.TempDir(), "not-elf")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := quickELFCheck(p); err == nil {
		t.Fatal("expected non-ELF rejection")
	}
}

func TestResolveUpdateSource_localBinary(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bedrud")
	if err := os.WriteFile(bin, []byte{0x7f, 'E', 'L', 'F', 0}, 0o700); err != nil {
		t.Fatal(err)
	}
	res, err := resolveUpdateSource(UpdateOptions{Source: bin, SkipChecksum: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.BinaryPath != bin {
		t.Fatalf("got %s", res.BinaryPath)
	}
	if !strings.Contains(res.Description, "local binary") {
		t.Fatalf("desc=%q", res.Description)
	}
}

func TestResolveUpdateSource_self(t *testing.T) {
	res, err := resolveUpdateSource(UpdateOptions{Self: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Cleanup != nil {
		defer res.Cleanup()
	}
	if err := quickELFCheck(res.BinaryPath); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Description, "--self") {
		t.Fatalf("desc=%q", res.Description)
	}
}

func TestResolveURL_withChecksum(t *testing.T) {
	dir := t.TempDir()
	archPath := filepath.Join(dir, "bedrud_test.tar.gz")
	payload := []byte{0x7f, 'E', 'L', 'F', 't', 'e', 's', 't'}
	if err := writeTarGz(archPath, map[string][]byte{"bedrud": payload}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(archPath)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	sumsBody := hex.EncodeToString(sum[:]) + "  bedrud_test.tar.gz\n"

	var assetHits, sumsHits int
	mux := http.NewServeMux()
	mux.HandleFunc("/release/bedrud_test.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		assetHits++
		_, _ = w.Write(data)
	})
	mux.HandleFunc("/release/SHA256SUMS", func(w http.ResponseWriter, r *http.Request) {
		sumsHits++
		_, _ = w.Write([]byte(sumsBody))
	})
	mux.HandleFunc("/bad/bedrud_test.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(data)
	})
	mux.HandleFunc("/bad/SHA256SUMS", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("0000000000000000000000000000000000000000000000000000000000000000  bedrud_test.tar.gz\n"))
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	oldClient := httpClient
	httpClient = srv.Client()
	t.Cleanup(func() { httpClient = oldClient })

	res, err := resolveURL(srv.URL+"/release/bedrud_test.tar.gz", false)
	if err != nil {
		t.Fatalf("resolveURL: %v", err)
	}
	if res.Cleanup != nil {
		defer res.Cleanup()
	}
	if assetHits < 1 || sumsHits < 1 {
		t.Fatalf("hits asset=%d sums=%d", assetHits, sumsHits)
	}
	got, err := os.ReadFile(res.BinaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("binary content mismatch")
	}

	if _, err := resolveURL(srv.URL+"/bad/bedrud_test.tar.gz", false); err == nil {
		t.Fatal("expected checksum mismatch")
	}
}

func TestResolveLatest_fullFlow(t *testing.T) {
	assetName, err := releaseAssetName()
	if err != nil {
		t.Skip(err)
	}

	payload := []byte{0x7f, 'E', 'L', 'F', 'L', 'a', 't', 'e', 's', 't'}
	archBytes, err := writeTarXZBytes(map[string][]byte{"bedrud": payload})
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(archBytes)
	sumsBody := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), assetName)

	var assetURL, sumsURL string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/releases/latest"):
			_, _ = fmt.Fprintf(w, `{
				"tag_name": "v9.9.9-test",
				"assets": [
					{"name": %q, "browser_download_url": %q},
					{"name": "SHA256SUMS", "browser_download_url": %q}
				]
			}`, assetName, assetURL, sumsURL)
		case r.URL.Path == "/asset":
			_, _ = w.Write(archBytes)
		case r.URL.Path == "/sums":
			_, _ = w.Write([]byte(sumsBody))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	assetURL = srv.URL + "/asset"
	sumsURL = srv.URL + "/sums"

	oldClient := httpClient
	oldLatest := githubLatestURL
	httpClient = srv.Client()
	githubLatestURL = srv.URL + "/repos/themadorg/bedrud/releases/latest"
	t.Cleanup(func() {
		httpClient = oldClient
		githubLatestURL = oldLatest
	})

	res, err := resolveGitHubChannel(false, false)
	if err != nil {
		t.Fatalf("resolveGitHubChannel stable: %v", err)
	}
	if res.Cleanup != nil {
		defer res.Cleanup()
	}
	if res.Version != "v9.9.9-test" {
		t.Fatalf("version=%q", res.Version)
	}
	got, err := os.ReadFile(res.BinaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("payload mismatch: %q", got)
	}

	// Missing SHA256SUMS asset
	srv2 := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `{
			"tag_name": "v1.0.0",
			"assets": [{"name": %q, "browser_download_url": %q}]
		}`, assetName, assetURL)
	}))
	t.Cleanup(srv2.Close)
	httpClient = srv2.Client()
	githubLatestURL = srv2.URL
	if _, err := resolveGitHubChannel(false, false); err == nil {
		t.Fatal("expected error when SHA256SUMS missing")
	}
}

func TestResolveLatestRejectsPrerelease(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9-rc.1","prerelease":true,"assets":[]}`))
	}))
	t.Cleanup(srv.Close)
	oldClient, oldLatest := httpClient, githubLatestURL
	httpClient = srv.Client()
	githubLatestURL = srv.URL
	t.Cleanup(func() {
		httpClient = oldClient
		githubLatestURL = oldLatest
	})
	if _, err := resolveGitHubChannel(false, false); err == nil {
		t.Fatal("expected stable channel to reject prerelease")
	}
}

func TestResolveNightlyPicksPrerelease(t *testing.T) {
	assetName, err := releaseAssetName()
	if err != nil {
		t.Skip(err)
	}
	payload := []byte{0x7f, 'E', 'L', 'F', 'n', 't'}
	archBytes, err := writeTarXZBytes(map[string][]byte{"bedrud": payload})
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(archBytes)
	sumsBody := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), assetName)

	var assetURL, sumsURL string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/releases") && !strings.Contains(r.URL.Path, "/latest"):
			_, _ = fmt.Fprintf(w, `[
				{"tag_name":"v9.9.9","prerelease":false,"draft":false,"assets":[]},
				{"tag_name":"v9.9.10-nightly","prerelease":true,"draft":false,"assets":[
					{"name":%q,"browser_download_url":%q},
					{"name":"SHA256SUMS","browser_download_url":%q}
				]}
			]`, assetName, assetURL, sumsURL)
		case r.URL.Path == "/asset":
			_, _ = w.Write(archBytes)
		case r.URL.Path == "/sums":
			_, _ = w.Write([]byte(sumsBody))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	assetURL = srv.URL + "/asset"
	sumsURL = srv.URL + "/sums"

	oldClient, oldList := httpClient, githubReleasesURL
	httpClient = srv.Client()
	githubReleasesURL = srv.URL + "/repos/themadorg/bedrud/releases"
	t.Cleanup(func() {
		httpClient = oldClient
		githubReleasesURL = oldList
	})

	res, err := resolveGitHubChannel(false, true)
	if err != nil {
		t.Fatalf("nightly: %v", err)
	}
	if res.Cleanup != nil {
		defer res.Cleanup()
	}
	if res.Version != "v9.9.10-nightly" {
		t.Fatalf("version=%q", res.Version)
	}
	if res.Channel != "nightly" {
		t.Fatalf("channel=%q", res.Channel)
	}
}

func TestIsStableAndNightlyRelease(t *testing.T) {
	if isStableRelease(githubRelease{TagName: "v1.0.0", Prerelease: true}) {
		t.Fatal("prerelease is not stable")
	}
	if isStableRelease(githubRelease{TagName: "v1.0.0-nightly"}) {
		t.Fatal("nightly tag is not stable")
	}
	if !isStableRelease(githubRelease{TagName: "v1.2.3"}) {
		t.Fatal("v1.2.3 should be stable")
	}
	if !isNightlyRelease(githubRelease{TagName: "v1.0.0-rc.1", Prerelease: true}) {
		t.Fatal("prerelease should count as nightly channel")
	}
	if isNightlyRelease(githubRelease{TagName: "v1.2.3"}) {
		t.Fatal("stable should not be nightly")
	}
}

func TestConfirmGitHubUpdateYesSkipsPrompt(t *testing.T) {
	if err := confirmGitHubUpdate(UpdateOptions{Yes: true}, "a", "b", "stable"); err != nil {
		t.Fatal(err)
	}
	if err := confirmGitHubUpdate(UpdateOptions{}, "a", "b", ""); err != nil {
		t.Fatal(err)
	}
}

func TestResolveUpdateSource_latestKeyword(t *testing.T) {
	// ensure "latest" routes to resolveLatest (skip-checksum path rejects)
	_, err := resolveUpdateSource(UpdateOptions{Source: "latest", SkipChecksum: true})
	if err == nil {
		t.Fatal("expected latest+skip-checksum error")
	}
}

func TestSafeExtractArchive_missingBinary(t *testing.T) {
	dir := t.TempDir()
	arch := filepath.Join(dir, "nobin.tar.gz")
	if err := writeTarGz(arch, map[string][]byte{"readme.txt": []byte("hi")}); err != nil {
		t.Fatal(err)
	}
	if _, err := safeExtractArchive(arch, filepath.Join(dir, "out")); err == nil {
		t.Fatal("expected missing bedrud error")
	}
}

func TestSafeExtractArchive_absolutePath(t *testing.T) {
	dir := t.TempDir()
	arch := filepath.Join(dir, "abs.tar.gz")
	if err := writeTarGz(arch, map[string][]byte{
		"/etc/passwd": []byte("x"),
		"bedrud":      {0x7f, 'E', 'L', 'F'},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := safeExtractArchive(arch, filepath.Join(dir, "out")); err == nil {
		t.Fatal("expected absolute path rejection")
	}
}

func TestSafeExtractArchive_xzHappy(t *testing.T) {
	dir := t.TempDir()
	payload := []byte{0x7f, 'E', 'L', 'F', 'x', 'z'}
	data, err := writeTarXZBytes(map[string][]byte{"bedrud": payload})
	if err != nil {
		t.Fatal(err)
	}
	arch := filepath.Join(dir, "a.tar.xz")
	if err := os.WriteFile(arch, data, 0o600); err != nil {
		t.Fatal(err)
	}
	bin, err := safeExtractArchive(arch, filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("got %q", got)
	}
}

func TestIsArchivePath(t *testing.T) {
	for _, p := range []string{"a.tar.xz", "a.tar.gz", "a.tgz", "a.tar", "A.TAR.XZ"} {
		if !isArchivePath(p) {
			t.Fatalf("expected archive: %s", p)
		}
	}
	if isArchivePath("bedrud") || isArchivePath("bedrud.exe") {
		t.Fatal("bare binary should not be archive")
	}
}

func TestSiblingSHA256SUMSAndGitHub(t *testing.T) {
	u, err := url.Parse("https://github.com/themadorg/bedrud/releases/download/v1.2.3/bedrud_linux_amd64.tar.xz")
	if err != nil {
		t.Fatal(err)
	}
	got := siblingSHA256SUMS(u)
	if !strings.HasSuffix(got, "/v1.2.3/SHA256SUMS") {
		t.Fatalf("got %s", got)
	}
	if !isGitHubReleaseURL(u) {
		t.Fatal("expected github release url")
	}
	if v := versionFromGitHubURL(u); v != "v1.2.3" {
		t.Fatalf("version=%q", v)
	}

	other, _ := url.Parse("https://example.com/files/bedrud.tar.gz")
	if isGitHubReleaseURL(other) {
		t.Fatal("example.com should not be github release")
	}
}

func writeTarXZBytes(files map[string][]byte) ([]byte, error) {
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	for name, content := range files {
		hdr := &tar.Header{
			Name:     name,
			Size:     int64(len(content)),
			Mode:     0o755,
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write(content); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	var xzBuf bytes.Buffer
	xw, err := xz.NewWriter(&xzBuf)
	if err != nil {
		return nil, err
	}
	if _, err := xw.Write(tarBuf.Bytes()); err != nil {
		return nil, err
	}
	if err := xw.Close(); err != nil {
		return nil, err
	}
	return xzBuf.Bytes(), nil
}
