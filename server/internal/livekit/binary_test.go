package livekit

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// The embedded LiveKit binary has to reach disk before it can be executed, and
// the export runs on every start — including a restart while the previous
// process is still holding the old inode. These cover the file mechanics, not
// the binary's contents: the test build embeds whatever placeholder the //go:embed
// directive found, so the assertions compare against the embedded bytes rather
// than assuming a real executable.

func embeddedBytes(t *testing.T) []byte {
	t.Helper()

	data, err := Bin.ReadFile(lkBinKey)
	if err != nil {
		t.Fatalf("read embedded binary %q: %v", lkBinKey, err)
	}
	return data
}

func TestExportBinary_WritesTheEmbeddedBytes(t *testing.T) {
	dest := filepath.Join(t.TempDir(), lkExeName)

	if err := ExportBinary(dest); err != nil {
		t.Fatalf("ExportBinary: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read exported binary: %v", err)
	}
	if want := embeddedBytes(t); !bytes.Equal(got, want) {
		t.Errorf("exported %d bytes, embedded copy is %d — they must match exactly", len(got), len(want))
	}
}

func TestExportBinary_IsExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows decides executability by extension, not by mode bits")
	}

	dest := filepath.Join(t.TempDir(), lkExeName)
	if err := ExportBinary(dest); err != nil {
		t.Fatalf("ExportBinary: %v", err)
	}

	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("stat exported binary: %v", err)
	}
	// Written 0o755 rather than chmod'd afterwards, because the caller treats a
	// failed chmod as a warning and carries on — so the mode set here is the one
	// that has to be right.
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("mode = %v, want the owner execute bit set", info.Mode().Perm())
	}
}

func TestExportBinary_OverwritesAnExistingFile(t *testing.T) {
	dest := filepath.Join(t.TempDir(), lkExeName)

	if err := os.WriteFile(dest, []byte("a stale binary from an older version"), 0o755); err != nil {
		t.Fatalf("seed a stale file: %v", err)
	}

	if err := ExportBinary(dest); err != nil {
		t.Fatalf("ExportBinary over an existing file: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read exported binary: %v", err)
	}
	if want := embeddedBytes(t); !bytes.Equal(got, want) {
		t.Error("the stale contents survived the export")
	}
}

func TestExportBinary_ReportsAnUnwritableDestination(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory mode bits do not gate writes the same way on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root, which ignores the directory mode")
	}

	dir := filepath.Join(t.TempDir(), "readonly")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatalf("create read-only dir: %v", err)
	}

	// resolveLiveKitPath walks a list of candidate directories and moves on when
	// one fails, so a refusal has to arrive as an error rather than a panic or a
	// silent success.
	if err := ExportBinary(filepath.Join(dir, lkExeName)); err == nil {
		t.Error("exporting into an unwritable directory reported success")
	}
}

func TestTempDirPath_FollowsTheEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.TempDir reads TMP/TEMP on Windows, not TMPDIR")
	}

	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)

	got := tempDirPath()

	if want := filepath.Join(dir, lkExeName); got != want {
		t.Errorf("tempDirPath() = %q, want %q", got, want)
	}
}

// The candidates exist so that a locked-down temp dir, a read-only install
// directory or a missing HOME each fall through to the next rather than
// stopping the server from starting.
func TestPathCandidates_AreDistinctAndNamed(t *testing.T) {
	candidates := map[string]func() string{
		"tempDir":   tempDirPath,
		"userCache": userCachePath,
		"exeDir":    exeDirPath,
		"cwd":       cwdPath,
	}

	seen := map[string]string{}
	for name, fn := range candidates {
		p := fn()
		if p == "" {
			// Legitimate: os.UserCacheDir fails with no HOME, os.Executable can
			// fail on some platforms. resolveLiveKitPath skips those.
			continue
		}
		if filepath.Base(p) != lkExeName {
			t.Errorf("%s() = %q, want it to end in %q", name, p, lkExeName)
		}
		if !filepath.IsAbs(p) {
			t.Errorf("%s() = %q, want an absolute path — it is handed to exec", name, p)
		}
		if prev, dup := seen[p]; dup {
			t.Errorf("%s() and %s() both resolve to %q, so one is not a fallback for the other", name, prev, p)
		}
		seen[p] = name
	}

	if len(seen) == 0 {
		t.Fatal("no candidate produced a path; resolveLiveKitPath would always fall back to $PATH")
	}
}

// The unlink in ExportBinary is the one line here with a production reason
// behind it: a restart re-exports over a path the outgoing LiveKit process may
// still have mapped as its text image, and on Linux writing to that file fails
// with ETXTBSY. Truncating an idle file works either way, so overwriting a
// *running* one is the only thing that tells the two apart.
//
// The embedded binary is a placeholder in a test build and cannot be executed,
// so the running executable here is a copy of the test binary itself, re-invoked
// as a helper — the standard os/exec pattern.
func TestExportBinary_OverwritesARunningExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows locks a running image outright; unlink-then-write does not apply")
	}

	self, err := os.Executable()
	if err != nil {
		t.Skipf("cannot locate the test binary: %v", err)
	}
	image, err := os.ReadFile(self)
	if err != nil {
		t.Skipf("cannot read the test binary: %v", err)
	}

	dest := filepath.Join(t.TempDir(), lkExeName)
	if err := os.WriteFile(dest, image, 0o755); err != nil {
		t.Fatalf("stage a runnable file at the export path: %v", err)
	}

	cmd := exec.Command(dest, "-test.run=TestExportBinaryHelperProcess")
	cmd.Env = append(os.Environ(), helperEnv+"=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("pipe helper stdout: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start the helper: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	// Wait for the child to announce itself, so the kernel has certainly mapped
	// the file. Starting the process is not enough — without this the write can
	// land before the image is held and the test passes for the wrong reason.
	ready := make(chan struct{})
	go func() {
		buf := make([]byte, len(helperReady))
		if _, err := io.ReadFull(stdout, buf); err == nil && string(buf) == helperReady {
			close(ready)
		}
	}()
	select {
	case <-ready:
	case <-time.After(30 * time.Second):
		t.Fatal("the helper process never signalled readiness")
	}

	// Sanity check that this is genuinely the hard case: a plain write must fail
	// here, or the test proves nothing about the unlink.
	if err := os.WriteFile(dest, []byte("x"), 0o755); err == nil {
		t.Skip("this platform allows writing to a running executable, so ETXTBSY is not reachable")
	}

	if err := ExportBinary(dest); err != nil {
		t.Fatalf("ExportBinary over a running executable: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read exported binary: %v", err)
	}
	if want := embeddedBytes(t); !bytes.Equal(got, want) {
		t.Error("the running image survived the export")
	}
}

const (
	helperEnv   = "BEDRUD_LIVEKIT_EXPORT_HELPER"
	helperReady = "helper-running\n"
)

// TestExportBinaryHelperProcess is not a test. It is the child process spawned
// by TestExportBinary_OverwritesARunningExecutable, and it does nothing but stay
// alive while its own image is overwritten.
func TestExportBinaryHelperProcess(t *testing.T) {
	if os.Getenv(helperEnv) != "1" {
		t.Skip("helper process, driven by TestExportBinary_OverwritesARunningExecutable")
	}
	fmt.Print(helperReady)
	os.Stdout.Sync()
	time.Sleep(60 * time.Second)
}
