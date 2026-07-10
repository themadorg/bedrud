package storage

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	avatarMaxBytes   = 2 * 1024 * 1024
	avatarMaxDim     = 1024
	defaultAvatarDir = "./data/uploads/avatars"
)

var avatarDir = defaultAvatarDir

func AvatarMaxBytes() int64 { return avatarMaxBytes }

func AvatarDir() string {
	return avatarDir
}

// SetAvatarDirForTest overrides the avatar directory; restore() reverts it.
func SetAvatarDirForTest(dir string) (restore func()) {
	prev := avatarDir
	avatarDir = dir
	return func() { avatarDir = prev }
}

func SaveUserAvatar(userID string, data []byte) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("user id required")
	}
	if len(data) == 0 {
		return "", fmt.Errorf("empty upload")
	}
	if len(data) > avatarMaxBytes {
		return "", fmt.Errorf("avatar too large (max 2 MB)")
	}

	mime, err := SniffMime(data)
	if err != nil {
		return "", err
	}
	ext, ok := allowedMimeTypes[mime]
	if !ok {
		return "", fmt.Errorf("unsupported image type")
	}

	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		if cfg.Width > avatarMaxDim || cfg.Height > avatarMaxDim {
			return "", fmt.Errorf("image dimensions too large (max %dx%d)", avatarMaxDim, avatarMaxDim)
		}
	}

	dir := AvatarDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create avatar dir: %w", err)
	}

	for _, oldExt := range []string{".png", ".jpg", ".gif", ".webp"} {
		oldPath := filepath.Join(dir, userID+oldExt)
		if oldExt != ext {
			_ = os.Remove(oldPath)
		}
	}

	path := filepath.Join(dir, userID+ext)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("failed to write avatar: %w", err)
	}

	version := strconv.FormatInt(time.Now().Unix(), 10)
	return "/uploads/avatars/" + userID + ext + "?v=" + version, nil
}

func DeleteUserAvatarFiles(userID string) error {
	if userID == "" {
		return nil
	}
	dir := AvatarDir()
	for _, ext := range []string{".png", ".jpg", ".gif", ".webp"} {
		_ = os.Remove(filepath.Join(dir, userID+ext))
	}
	return nil
}

func ResolveAvatarFile(pathParam string) (string, error) {
	pathParam = strings.Split(pathParam, "?")[0]
	if pathParam == "" || strings.Contains(pathParam, "..") {
		return "", fmt.Errorf("invalid path")
	}
	dir := filepath.Clean(AvatarDir())
	resolved := filepath.Join(dir, pathParam)
	if !strings.HasPrefix(resolved, dir+string(os.PathSeparator)) && resolved != dir {
		return "", fmt.Errorf("invalid path")
	}
	return resolved, nil
}
