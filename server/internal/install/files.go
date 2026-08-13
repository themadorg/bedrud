package install

import (
	"fmt"
	"os"
)

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// writeFileUnlessExists writes data only when path is absent.
// Existing files are left unchanged (install re-run without --fresh).
func writeFileUnlessExists(path string, data []byte, perm os.FileMode) (wrote bool, err error) {
	exists, err := fileExists(path)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}
	if err := os.WriteFile(path, data, perm); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}
