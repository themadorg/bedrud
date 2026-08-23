package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	encMagic    = "BH1\n"
	keySize     = 32
	keyMagic    = "BHKEY1\x00\x00" // 8 bytes at EOF
	trailerSize = keySize + len(keyMagic)
)

// selfPath is the running executable. Tests replace this with a temp file.
var selfPath = os.Executable

func deriveKey(material string) []byte {
	sum := sha256.Sum256([]byte(material))
	return sum[:]
}

func loadOrCreateKey() ([]byte, error) {
	path, err := selfPath()
	if err != nil {
		return nil, fmt.Errorf("executable path: %w", err)
	}
	if k, ok, err := readKeyTrailer(path); err != nil {
		return nil, err
	} else if ok {
		return k, nil
	}
	k := make([]byte, keySize)
	if _, err := rand.Read(k); err != nil {
		return nil, err
	}
	if err := appendKeyTrailer(path, k); err != nil {
		return nil, fmt.Errorf("append key to binary %s: %w", path, err)
	}
	return k, nil
}

func readKeyTrailer(path string) ([]byte, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, false, err
	}
	if st.Size() < int64(trailerSize) {
		return nil, false, nil
	}
	buf := make([]byte, trailerSize)
	if _, err := f.ReadAt(buf, st.Size()-int64(trailerSize)); err != nil {
		return nil, false, err
	}
	if string(buf[keySize:]) != keyMagic {
		return nil, false, nil
	}
	k := make([]byte, keySize)
	copy(k, buf[:keySize])
	return k, true, nil
}

func appendKeyTrailer(path string, key []byte) error {
	if len(key) != keySize {
		return fmt.Errorf("key must be %d bytes", keySize)
	}
	if k, ok, err := readKeyTrailer(path); err != nil {
		return err
	} else if ok && len(k) == keySize {
		return nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	out := append(append(body, key...), []byte(keyMagic)...)
	tmp := path + ".keytmp"
	if err := os.WriteFile(tmp, out, st.Mode()); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func encryptBytes(key, plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	out := append([]byte(encMagic), nonce...)
	return append(out, gcm.Seal(nil, nonce, plain, []byte(encMagic))...), nil
}

func decryptBytes(key, blob []byte) ([]byte, error) {
	if !strings.HasPrefix(string(blob), encMagic) {
		return nil, fmt.Errorf("not an encrypted bedrud-host db")
	}
	blob = blob[len(encMagic):]
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(blob) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ct := blob[:ns], blob[ns:]
	plain, err := gcm.Open(nil, nonce, ct, []byte(encMagic))
	if err != nil {
		return nil, fmt.Errorf("decrypt failed (wrong key?)")
	}
	return plain, nil
}
