package hostbin

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	keyMagic    = "BHKEY1\x00\x00"
	keySize     = 32
	trailerSize = keySize + len(keyMagic)
)

func Ensure() (string, error) {
	payload := Bin
	if len(payload) < 64 {
		if b, err := os.ReadFile(siblingHost()); err == nil && len(b) >= 64 {
			payload = b
		}
	}
	if len(payload) < 64 {
		return "", fmt.Errorf("bedrud-host is not embedded; run make in tools/bedrud-gui")
	}

	dir, err := destDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	dest := filepath.Join(dir, "bedrud-host")
	trailer := trailerOf(dest)
	if trailer == nil {
		trailer = trailerOf(siblingHost())
	}
	if t := trailerOfBytes(payload); t != nil {
		trailer = t
		payload = payload[:len(payload)-trailerSize]
	}
	if trailer != nil {
		payload = append(append([]byte{}, payload...), trailer...)
	}
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return dest, nil
}

func destDir() (string, error) {
	if x := os.Getenv("XDG_CACHE_HOME"); x != "" {
		return filepath.Join(x, "bedrud-gui"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "bedrud-gui"), nil
}

func siblingHost() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "..", "bedrud-host", "bedrud-host")
}

func trailerOf(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return trailerOfBytes(b)
}

func trailerOfBytes(b []byte) []byte {
	if len(b) < trailerSize {
		return nil
	}
	t := b[len(b)-trailerSize:]
	if string(t[keySize:]) != keyMagic {
		return nil
	}
	out := make([]byte, trailerSize)
	copy(out, t)
	return out
}
