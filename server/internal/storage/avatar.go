package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

const avatarMaxBytes int64 = 2 * 1024 * 1024

func AvatarMaxBytes() int64 {
	return avatarMaxBytes
}

func AvatarDir() string {
	return filepath.Clean("./data/uploads/avatars")
}

func SaveUserAvatar(userID string, data []byte) (string, error) {
	if _, err := uuid.Parse(userID); err != nil {
		return "", fmt.Errorf("invalid user ID")
	}
	mime, err := SniffMime(data)
	if err != nil {
		return "", err
	}
	ext := allowedMimeTypes[mime]
	if err := os.MkdirAll(AvatarDir(), 0o755); err != nil {
		return "", fmt.Errorf("failed to create avatar dir: %w", err)
	}
	_ = DeleteUserAvatarFiles(userID)
	name := userID + ext
	path := filepath.Join(AvatarDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("failed to write avatar: %w", err)
	}
	return "/uploads/avatars/" + name, nil
}

func DeleteUserAvatarFiles(userID string) error {
	if _, err := uuid.Parse(userID); err != nil {
		return fmt.Errorf("invalid user ID")
	}
	var firstErr error
	for _, ext := range allowedMimeTypes {
		if err := os.Remove(filepath.Join(AvatarDir(), userID+ext)); err != nil && !os.IsNotExist(err) && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func ResolveAvatarFile(name string) (string, error) {
	if name == "" || strings.ContainsAny(name, `/\\`) || filepath.Base(name) != name {
		return "", fmt.Errorf("invalid path")
	}
	path := filepath.Join(AvatarDir(), name)
	if !strings.HasPrefix(path, AvatarDir()+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid path")
	}
	return path, nil
}
