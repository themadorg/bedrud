//go:build windows

package cli

// copyFileOwner is a no-op on Windows: file ownership is carried by ACLs that
// a replacement file inherits from its directory.
func copyFileOwner(src, dst string) error { return nil }
