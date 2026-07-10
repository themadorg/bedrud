package storage

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.White)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func withTempAvatarDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	restore := SetAvatarDirForTest(dir)
	t.Cleanup(restore)
	return dir
}

func TestAvatarDir_DefaultAndOverride(t *testing.T) {
	if AvatarDir() != defaultAvatarDir {
		t.Fatalf("default AvatarDir = %q, want %q", AvatarDir(), defaultAvatarDir)
	}
	dir := t.TempDir()
	restore := SetAvatarDirForTest(dir)
	if AvatarDir() != dir {
		t.Fatalf("override AvatarDir = %q, want %q", AvatarDir(), dir)
	}
	restore()
	if AvatarDir() != defaultAvatarDir {
		t.Fatalf("restored AvatarDir = %q, want %q", AvatarDir(), defaultAvatarDir)
	}
}

func TestSaveUserAvatar_EmptyUser(t *testing.T) {
	withTempAvatarDir(t)
	_, err := SaveUserAvatar("", testJPEG(t, 1, 1))
	if err == nil || !strings.Contains(err.Error(), "user id") {
		t.Fatalf("expected user id error, got %v", err)
	}
}

func TestSaveUserAvatar_EmptyBytes(t *testing.T) {
	withTempAvatarDir(t)
	_, err := SaveUserAvatar("u1", nil)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected empty error, got %v", err)
	}
}

func TestSaveUserAvatar_Oversize(t *testing.T) {
	withTempAvatarDir(t)
	data := make([]byte, avatarMaxBytes+1)
	_, err := SaveUserAvatar("u1", data)
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected too large error, got %v", err)
	}
}

func TestSaveUserAvatar_BadMIME(t *testing.T) {
	withTempAvatarDir(t)
	_, err := SaveUserAvatar("u1", []byte("not-an-image"))
	if err == nil {
		t.Fatal("expected MIME/sniff error")
	}
}

func TestSaveUserAvatar_ValidJPEGURL(t *testing.T) {
	dir := withTempAvatarDir(t)
	url, err := SaveUserAvatar("user-a", testJPEG(t, 32, 32))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, "/uploads/avatars/user-a.jpg") {
		t.Fatalf("url = %q", url)
	}
	if _, err := os.Stat(filepath.Join(dir, "user-a.jpg")); err != nil {
		t.Fatal(err)
	}
}

func TestSaveUserAvatar_DimensionReject(t *testing.T) {
	withTempAvatarDir(t)
	_, err := SaveUserAvatar("u1", testJPEG(t, 1025, 10))
	if err == nil || !strings.Contains(err.Error(), "dimensions") {
		t.Fatalf("expected dimensions error, got %v", err)
	}
}

func TestSaveUserAvatar_ReplaceOtherExt(t *testing.T) {
	dir := withTempAvatarDir(t)
	if _, err := SaveUserAvatar("u1", testPNG(t, 8, 8)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "u1.png")); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveUserAvatar("u1", testJPEG(t, 8, 8)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "u1.jpg")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "u1.png")); !os.IsNotExist(err) {
		t.Fatalf("old png should be removed, err=%v", err)
	}
}

func TestDeleteUserAvatarFiles(t *testing.T) {
	dir := withTempAvatarDir(t)
	if _, err := SaveUserAvatar("u1", testJPEG(t, 4, 4)); err != nil {
		t.Fatal(err)
	}
	if err := DeleteUserAvatarFiles("u1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "u1.jpg")); !os.IsNotExist(err) {
		t.Fatalf("expected deleted, err=%v", err)
	}
}

func TestResolveAvatarFile_Traversal(t *testing.T) {
	withTempAvatarDir(t)
	if _, err := ResolveAvatarFile("../etc/passwd"); err == nil {
		t.Fatal("expected traversal reject")
	}
	if _, err := ResolveAvatarFile("foo/../../etc/passwd"); err == nil {
		t.Fatal("expected traversal reject")
	}
	path, err := ResolveAvatarFile("user-a.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "user-a.jpg") {
		t.Fatalf("path = %q", path)
	}
}
