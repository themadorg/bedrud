package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeCreate_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "newfile.txt")

	f, err := SafeCreate(path, 0o644)
	if err != nil {
		t.Fatalf("SafeCreate failed: %v", err)
	}
	f.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("file not found: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Fatalf("expected perms 0o644, got 0o%o", perm)
	}
}

func TestSafeCreate_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "existing.txt")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := SafeCreate(path, 0o644)
	if err == nil {
		t.Fatal("expected error for existing file")
	}
}

func TestSafeCreate_ExistingSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target.txt")
	if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	_, err := SafeCreate(link, 0o644)
	if err == nil {
		t.Fatal("expected error for existing symlink")
	}
}

func TestSafeCreate_DanglingSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	link := filepath.Join(tmpDir, "dangling.txt")
	if err := os.Symlink("/nonexistent/target", link); err != nil {
		t.Fatal(err)
	}

	_, err := SafeCreate(link, 0o644)
	if err == nil {
		t.Fatal("expected error for dangling symlink")
	}
}

func TestSafeCreate_NonexistentParent(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subdir", "file.txt")

	_, err := SafeCreate(path, 0o644)
	if err == nil {
		t.Fatal("expected error for nonexistent parent dir")
	}
}

func TestSafeCreate_ParentIsSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	realDir := t.TempDir()
	linkDir := filepath.Join(tmpDir, "linkeddir")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(linkDir, "file.txt")
	f, err := SafeCreate(path, 0o644)
	if err != nil {
		t.Fatalf("expected success when parent dir is a resolved symlink, got: %v", err)
	}
	f.Close()
}

func TestSafeCreate_WritesData(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "write.txt")

	f, err := SafeCreate(path, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("hello")
	f.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("expected 'hello', got '%s'", string(data))
	}
}

func TestSafeOpenAppend_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "newlog.txt")

	f, err := SafeOpenAppend(path, 0o644)
	if err != nil {
		t.Fatalf("SafeOpenAppend failed: %v", err)
	}
	_, _ = f.WriteString("first\n")
	f.Close()
}

func TestSafeOpenAppend_Restart(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "restart.log")

	// First "start": create new file and write
	f, err := SafeOpenAppend(path, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("line1\n")
	f.Close()

	// Second "start": reopen existing file and append
	f, err = SafeOpenAppend(path, 0o644)
	if err != nil {
		t.Fatalf("reopen existing file should succeed (restart), got: %v", err)
	}
	_, _ = f.WriteString("line2\n")
	f.Close()

	// Verify both lines present — original content preserved, new content appended
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "line1\nline2\n" {
		t.Fatalf("expected 'line1\\nline2\\n', got '%s'", string(data))
	}
}

func TestSafeOpenAppend_ExistingSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sneaky.log")

	target := filepath.Join(tmpDir, "target.txt")
	if err := os.WriteFile(target, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("real"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}

	f, err := SafeOpenAppend(path, 0o644)
	if err != nil {
		t.Fatalf("expected success for symlink to regular file, got: %v", err)
	}
	f.Close()
}

func TestSafeOpenAppend_DanglingSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "dangling.log")
	if err := os.Symlink("/nonexistent/target", path); err != nil {
		t.Fatal(err)
	}

	_, err := SafeOpenAppend(path, 0o644)
	if err == nil {
		t.Fatal("expected error for dangling symlink")
	}
}

func TestSafeOpenAppend_NotRegular(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "mydir")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := SafeOpenAppend(path, 0o644)
	if err == nil {
		t.Fatal("expected error for directory path")
	}
}

func TestSafeOpenAppend_NonexistentParent(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nope", "file.txt")

	_, err := SafeOpenAppend(path, 0o644)
	if err == nil {
		t.Fatal("expected error for nonexistent parent dir")
	}
}

func TestSafeOpenAppend_ParentIsSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	realDir := t.TempDir()
	linkDir := filepath.Join(tmpDir, "linkeddir")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(linkDir, "file.log")
	f, err := SafeOpenAppend(path, 0o644)
	if err != nil {
		t.Fatalf("expected success when parent dir is a resolved symlink, got: %v", err)
	}
	f.Close()
}
