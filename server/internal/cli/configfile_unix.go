//go:build !windows

package cli

import (
	"os"
	"syscall"
)

// copyFileOwner gives dst the uid/gid of src so replacing an installed config
// as root does not hand the file to root and lock the service account out.
// A missing src (first write) or a chown the caller is not allowed to make is
// not an error: the file is still readable by whoever created it.
func copyFileOwner(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return nil
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	if err := os.Chown(dst, int(stat.Uid), int(stat.Gid)); err != nil && !os.IsPermission(err) {
		return err
	}
	return nil
}
