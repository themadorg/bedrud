package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// resolveInstalledBinary returns the path of the currently installed bedrud binary.
// Prefers package path when present, then /usr/local/bin, then ExecStart from units.
func resolveInstalledBinary() string {
	if _, err := os.Stat(binaryPackagePath); err == nil {
		return binaryPackagePath
	}
	if _, err := os.Stat(binaryLocalPath); err == nil {
		return binaryLocalPath
	}
	// Fall back to systemd unit ExecStart if present
	if data, err := os.ReadFile("/etc/systemd/system/bedrud.service"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "ExecStart=") {
				fields := strings.Fields(strings.TrimPrefix(line, "ExecStart="))
				if len(fields) > 0 {
					return fields[0]
				}
			}
		}
	}
	return binaryLocalPath
}

// isPackageManaged reports whether the installed binary is owned by a package manager
// (typically /usr/bin/bedrud from apt/dnf/apk). Update must not overwrite those paths.
func isPackageManaged(path string) bool {
	clean := filepath.Clean(path)
	if clean == binaryPackagePath {
		return true
	}
	// dpkg / rpm ownership check
	if _, err := exec.LookPath("dpkg"); err == nil {
		if out, err := exec.Command("dpkg", "-S", clean).CombinedOutput(); err == nil && len(out) > 0 {
			return true
		}
	}
	if _, err := exec.LookPath("rpm"); err == nil {
		if err := exec.Command("rpm", "-qf", clean).Run(); err == nil {
			return true
		}
	}
	return false
}

// readSelfBinary returns the bytes of the currently executing bedrud binary.
func readSelfBinary() ([]byte, error) {
	selfBytes, err := os.ReadFile("/proc/self/exe")
	if err != nil {
		execPath, errFallback := os.Executable()
		if errFallback != nil {
			return nil, fmt.Errorf("failed to get executable path: %w", errFallback)
		}
		selfBytes, err = os.ReadFile(execPath)
	}
	if err != nil || len(selfBytes) == 0 {
		return nil, fmt.Errorf("failed to read current binary: %w", err)
	}
	return selfBytes, nil
}

// sameFile reports whether two paths refer to the same inode (or same resolved path).
func sameFile(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA == nil && errB == nil && ra == rb {
		return true
	}
	sa, errA := os.Stat(a)
	sb, errB := os.Stat(b)
	if errA != nil || errB != nil {
		return false
	}
	return os.SameFile(sa, sb)
}

// installSelfBinary copies the running binary to targetPath.
// Removes the target first to avoid ETXTBSY when replacing a running binary.
func installSelfBinary(targetPath string) error {
	selfBytes, err := readSelfBinary()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create binary dir: %w", err)
	}
	// Avoid ETXTBSY: remove then write (same approach as LinuxInstall).
	_ = os.Remove(targetPath)
	if err := os.WriteFile(targetPath, selfBytes, 0o755); err != nil {
		return fmt.Errorf("failed to install binary to %s: %w", targetPath, err)
	}
	return nil
}

func runChown(userGroup, path string) error {
	if out, err := exec.Command("chown", userGroup, path).CombinedOutput(); err != nil {
		return fmt.Errorf("chown %s %s: %s: %w", userGroup, path, string(out), err)
	}
	return nil
}

func runChownR(userGroup, path string) error {
	if out, err := exec.Command("chown", "-R", userGroup, path).CombinedOutput(); err != nil {
		return fmt.Errorf("chown -R %s %s: %s: %w", userGroup, path, string(out), err)
	}
	return nil
}
