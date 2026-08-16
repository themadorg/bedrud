//go:build ignore

// Incomplete: needs useTestInstallRoot (path override helper) before it can compile.

package install

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"bedrud/config"
	"bedrud/internal/database"
)

// stubSystemHooks disables root/systemd side effects for integration tests.
func stubSystemHooks(t *testing.T) {
	t.Helper()
	prevCreate, prevChownR, prevChown := createUserHook, chownRHook, chownHook
	prevStop, prevRefresh, prevPkg := stopServicesHook, refreshServicesHook, packageManagedHook
	prevMandb, prevGen := mandbRunner, generateCompletionsFn
	prevArgs := os.Args

	// argv basename drives man/completion filenames (must be "bedrud", not *.test).
	os.Args = []string{"bedrud"}

	createUserHook = func() error { return nil }
	chownRHook = func(string, string) error { return nil }
	chownHook = func(string, string) error { return nil }
	stopServicesHook = func([]string) {}
	refreshServicesHook = func(string, bool) error { return nil }
	packageManagedHook = func(string) bool { return false }
	mandbRunner = func() {}
	generateCompletionsFn = func(binaryName string) (bash, zsh, fish []byte, err error) {
		return []byte("# bash " + binaryName + "\ncomplete -F _bedrud bedrud\n"),
			[]byte("#compdef " + binaryName + "\n"),
			[]byte("# fish " + binaryName + "\ncomplete -c " + binaryName + "\n"),
			nil
	}

	config.ResetLoadForTest()
	database.ResetForTest()

	t.Cleanup(func() {
		createUserHook, chownRHook, chownHook = prevCreate, prevChownR, prevChown
		stopServicesHook, refreshServicesHook, packageManagedHook = prevStop, prevRefresh, prevPkg
		mandbRunner, generateCompletionsFn = prevMandb, prevGen
		os.Args = prevArgs
		config.ResetLoadForTest()
		database.ResetForTest()
	})
}

func writeMinimalInstallConfig(t *testing.T, cfgPath, dbPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `server:
  port: "8090"
  host: "127.0.0.1"
database:
  type: "sqlite"
  path: "` + dbPath + `"
auth:
  jwtSecret: "test-jwt-secret-with-enough-length-32chars"
  sessionSecret: "test-session-secret-long-enough-32chars"
livekit:
  host: "ws://127.0.0.1:7880"
  internalHost: "http://127.0.0.1:7880"
  apiKey: "devkey"
  apiSecret: "CHANGE_ME_LIVEKIT_SECRET_LONG_ENOUGH"
  external: true
logger:
  level: "error"
`
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	config.ResetLoadForTest()
	database.ResetForTest()
}

func elfPayload(tag byte) []byte {
	// Minimal ELF magic + marker so installBinaryFile/quickELFCheck accept it.
	return []byte{0x7f, 'E', 'L', 'F', tag, 't', 'e', 's', 't'}
}

func assertUpdateArtifacts(t *testing.T, root string) {
	t.Helper()
	examples := filepath.Join(root, "usr", "share", "doc", "bedrud", "examples")
	for _, name := range []string{"config.yaml.example", "livekit.yaml.example", "README"} {
		p := filepath.Join(examples, name)
		st, err := os.Stat(p)
		if err != nil || st.Size() == 0 {
			t.Fatalf("missing or empty example %s: %v", p, err)
		}
	}
	man := filepath.Join(root, "usr", "share", "man", "man1", "bedrud.1")
	manData, err := os.ReadFile(man)
	if err != nil {
		t.Fatalf("man page: %v", err)
	}
	if !strings.Contains(string(manData), `.TH "bedrud" "1"`) {
		t.Fatalf("unexpected man content")
	}
	for _, p := range []string{
		filepath.Join(root, "usr", "share", "bash-completion", "completions", "bedrud"),
		filepath.Join(root, "usr", "share", "zsh", "site-functions", "_bedrud"),
		filepath.Join(root, "usr", "share", "fish", "vendor_completions.d", "bedrud.fish"),
	} {
		b, err := os.ReadFile(p)
		if err != nil || len(b) == 0 {
			t.Fatalf("completion missing %s: %v", p, err)
		}
	}
}

func TestLinuxUpdate_binaryPathInstallsAndWritesDocs(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("LinuxUpdate is linux-only")
	}
	root := t.TempDir()
	restore := useTestInstallRoot(root)
	t.Cleanup(restore)
	stubSystemHooks(t)

	dbPath := filepath.Join(varLibDir, "test.db")
	writeMinimalInstallConfig(t, etcConfigPath, dbPath)

	// Existing installed binary (old)
	if err := os.MkdirAll(filepath.Dir(binaryLocalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryLocalPath, elfPayload('O'), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeInstalledVersion("v0.1.0"); err != nil {
		t.Fatal(err)
	}

	// New binary source path
	src := filepath.Join(root, "download", "bedrud-new")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	newPayload := elfPayload('N')
	if err := os.WriteFile(src, newPayload, 0o755); err != nil {
		t.Fatal(err)
	}

	err := LinuxUpdate(UpdateOptions{
		Version:      "v9.9.9-test",
		ConfigPath:   etcConfigPath,
		Source:       src,
		SkipChecksum: true,
		SkipRestart:  true,
		// Run real DB migrations against temp sqlite — validates config load path.
	})
	if err != nil {
		t.Fatalf("LinuxUpdate: %v", err)
	}

	got, err := os.ReadFile(binaryLocalPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(newPayload) {
		t.Fatalf("installed binary not replaced: got %q want %q", got, newPayload)
	}
	if v := readInstalledVersion(); v != "v9.9.9-test" {
		t.Fatalf("version file = %q", v)
	}
	assertUpdateArtifacts(t, root)
}

func TestLinuxUpdate_selfInstallsAndWritesDocs(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("LinuxUpdate is linux-only")
	}
	root := t.TempDir()
	restore := useTestInstallRoot(root)
	t.Cleanup(restore)
	stubSystemHooks(t)

	dbPath := filepath.Join(varLibDir, "test-self.db")
	writeMinimalInstallConfig(t, etcConfigPath, dbPath)

	if err := os.MkdirAll(filepath.Dir(binaryLocalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryLocalPath, elfPayload('O'), 0o755); err != nil {
		t.Fatal(err)
	}

	err := LinuxUpdate(UpdateOptions{
		Version:     "v-self-test",
		ConfigPath:  etcConfigPath,
		Self:        true,
		SkipRestart: true,
	})
	if err != nil {
		t.Fatalf("LinuxUpdate --self: %v", err)
	}

	// Self install copies the test binary (ELF) over target.
	got, err := os.ReadFile(binaryLocalPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 4 || got[0] != 0x7f || got[1] != 'E' {
		t.Fatalf("self-installed binary does not look like ELF: %q", got[:min(8, len(got))])
	}
	assertUpdateArtifacts(t, root)
}

func TestLinuxUpdate_skipBinaryStillWritesDocs(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("LinuxUpdate is linux-only")
	}
	root := t.TempDir()
	restore := useTestInstallRoot(root)
	t.Cleanup(restore)
	stubSystemHooks(t)

	dbPath := filepath.Join(varLibDir, "test-skip.db")
	writeMinimalInstallConfig(t, etcConfigPath, dbPath)
	if err := os.MkdirAll(filepath.Dir(binaryLocalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	old := elfPayload('K')
	if err := os.WriteFile(binaryLocalPath, old, 0o755); err != nil {
		t.Fatal(err)
	}

	err := LinuxUpdate(UpdateOptions{
		Version:     "v-skip",
		ConfigPath:  etcConfigPath,
		SkipBinary:  true,
		SkipRestart: true,
	})
	if err != nil {
		t.Fatalf("LinuxUpdate --skip-binary: %v", err)
	}
	got, _ := os.ReadFile(binaryLocalPath)
	if string(got) != string(old) {
		t.Fatal("skip-binary should not replace binary")
	}
	assertUpdateArtifacts(t, root)
}

func TestPostInstallArtifactsHelpers(t *testing.T) {
	// Documents the checklist: after installDocExamples + installCLIDocs,
	// examples, man, and completions exist (same helpers used by install/update).
	root := t.TempDir()
	restore := useTestInstallRoot(root)
	t.Cleanup(restore)
	stubSystemHooks(t)

	if err := installDocExamples(); err != nil {
		t.Fatal(err)
	}
	if err := installCLIDocsFor("bedrud"); err != nil {
		t.Fatal(err)
	}
	assertUpdateArtifacts(t, root)
}
