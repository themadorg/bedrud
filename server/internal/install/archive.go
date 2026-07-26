package install

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ulikunitz/xz"
)

const (
	maxArchiveEntries   = 64
	maxArchiveTotalSize = 512 << 20 // 512 MiB uncompressed total
	maxArchiveFileSize  = 256 << 20 // 256 MiB single member
)

// safeExtractArchive extracts a .tar.xz / .tar.gz / .tgz archive into destDir
// with path-traversal, symlink, device, and size protections.
// Returns the absolute path of a member named "bedrud" (or basename bedrud) if found.
func safeExtractArchive(archivePath, destDir string) (binaryPath string, err error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	var r io.Reader = f
	name := strings.ToLower(archivePath)
	switch {
	case strings.HasSuffix(name, ".tar.xz") || strings.HasSuffix(name, ".txz"):
		xzr, err := xz.NewReader(f)
		if err != nil {
			return "", fmt.Errorf("xz reader: %w", err)
		}
		r = xzr
	case strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz"):
		gzr, err := gzip.NewReader(f)
		if err != nil {
			return "", fmt.Errorf("gzip reader: %w", err)
		}
		defer gzr.Close()
		r = gzr
	case strings.HasSuffix(name, ".tar"):
		// plain tar
	default:
		return "", fmt.Errorf("unsupported archive type (want .tar.xz, .tar.gz, .tgz, or .tar): %s", filepath.Base(archivePath))
	}

	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return "", err
	}
	destAbs, err := filepath.Abs(destDir)
	if err != nil {
		return "", err
	}

	tr := tar.NewReader(r)
	var (
		entries int
		total   int64
		found   string
	)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("tar read: %w", err)
		}
		entries++
		if entries > maxArchiveEntries {
			return "", fmt.Errorf("archive has too many entries (max %d)", maxArchiveEntries)
		}

		// Security: reject links and special files entirely.
		switch hdr.Typeflag {
		case tar.TypeReg, tar.TypeRegA:
			// ok
		case tar.TypeDir:
			// only create dirs we need under dest
			clean, err := safeArchivePath(destAbs, hdr.Name)
			if err != nil {
				return "", err
			}
			if err := os.MkdirAll(clean, 0o700); err != nil {
				return "", err
			}
			continue
		case tar.TypeSymlink, tar.TypeLink:
			return "", fmt.Errorf("archive contains link %q (not allowed)", hdr.Name)
		default:
			return "", fmt.Errorf("archive contains unsupported entry type %q (%v)", hdr.Name, hdr.Typeflag)
		}

		if hdr.Size < 0 || hdr.Size > maxArchiveFileSize {
			return "", fmt.Errorf("archive member %q exceeds size limit", hdr.Name)
		}
		if total+hdr.Size > maxArchiveTotalSize {
			return "", fmt.Errorf("archive uncompressed size exceeds limit")
		}

		target, err := safeArchivePath(destAbs, hdr.Name)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return "", err
		}

		// Write with restricted mode; never preserve setuid or attacker mode bits.
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return "", err
		}
		n, copyErr := io.Copy(out, io.LimitReader(tr, hdr.Size+1))
		_ = out.Close()
		if copyErr != nil {
			return "", fmt.Errorf("extract %s: %w", hdr.Name, copyErr)
		}
		if n > hdr.Size {
			return "", fmt.Errorf("archive member %q larger than declared size", hdr.Name)
		}
		total += n

		base := filepath.Base(target)
		if base == "bedrud" || base == "bedrud.exe" {
			found = target
		}
	}

	if found == "" {
		return "", fmt.Errorf("archive does not contain a bedrud binary at the top level or in a subpath")
	}
	return found, nil
}

// safeArchivePath joins destAbs with name and ensures the result stays under destAbs.
// Rejects absolute paths and any ".." segment before join (Clean alone is not enough:
// Clean("/../etc/passwd") becomes "/etc/passwd", which would join under dest).
func safeArchivePath(destAbs, name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	if name == "" {
		return "", fmt.Errorf("empty archive entry name")
	}
	if strings.HasPrefix(name, "/") || filepath.IsAbs(name) {
		return "", fmt.Errorf("absolute path in archive: %q", name)
	}
	// Windows-style absolute (C:...)
	if len(name) >= 2 && name[1] == ':' {
		return "", fmt.Errorf("absolute path in archive: %q", name)
	}

	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return "", fmt.Errorf("path traversal in archive: %q", name)
		}
	}

	// path.Clean on relative name: "a/./b" → "a/b"; never introduces ".." if none present.
	cleanName := filepath.Clean(name)
	if cleanName == ".." || strings.HasPrefix(cleanName, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal in archive: %q", name)
	}
	// filepath.Clean(".") for empty-ish names
	if cleanName == "." {
		return "", fmt.Errorf("invalid archive entry name: %q", name)
	}

	target := filepath.Join(destAbs, cleanName)
	rel, err := filepath.Rel(destAbs, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes extract root: %q", name)
	}
	return target, nil
}

func isArchivePath(path string) bool {
	l := strings.ToLower(path)
	return strings.HasSuffix(l, ".tar.xz") ||
		strings.HasSuffix(l, ".txz") ||
		strings.HasSuffix(l, ".tar.gz") ||
		strings.HasSuffix(l, ".tgz") ||
		strings.HasSuffix(l, ".tar")
}
