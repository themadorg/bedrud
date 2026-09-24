package install

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func stubProbe(t *testing.T, fn func(string) string) {
	t.Helper()
	prev := probeBinaryVersion
	probeBinaryVersion = fn
	t.Cleanup(func() { probeBinaryVersion = prev })
}

func TestResolveTargetVersionPrefersReleaseTag(t *testing.T) {
	stubProbe(t, func(string) string { return "v9.9.9" })

	got := resolveTargetVersion(
		UpdateOptions{Version: "v0.12.0", Source: "latest"},
		resolvedSource{Version: "v0.13.0", BinaryPath: "/tmp/bedrud"},
	)
	if got.Version != "v0.13.0" || got.Origin != originReleaseTag {
		t.Fatalf("got %+v, want v0.13.0 from %s", got, originReleaseTag)
	}
}

func TestResolveTargetVersionProbesForeignSource(t *testing.T) {
	stubProbe(t, func(path string) string {
		if path != "/tmp/bedrud" {
			t.Fatalf("probed %q", path)
		}
		return "v0.13.0"
	})

	// The running binary is v0.12.0; a local source must never inherit it.
	got := resolveTargetVersion(
		UpdateOptions{Version: "v0.12.0", Source: "/tmp/bedrud"},
		resolvedSource{BinaryPath: "/tmp/bedrud"},
	)
	if got.Version != "v0.13.0" || got.Origin != originSourceBinary {
		t.Fatalf("got %+v, want v0.13.0 from %s", got, originSourceBinary)
	}
}

func TestResolveTargetVersionUnknownWhenProbeFails(t *testing.T) {
	stubProbe(t, func(string) string { return "" })

	got := resolveTargetVersion(
		UpdateOptions{Version: "v0.12.0", Source: "/tmp/bedrud"},
		resolvedSource{BinaryPath: "/tmp/bedrud"},
	)
	if got.Version != "" {
		t.Fatalf("got %+v, want an unknown target", got)
	}
}

func TestResolveTargetVersionSelfDoesNotProbe(t *testing.T) {
	stubProbe(t, func(string) string {
		t.Fatal("must not probe when the running executable is the source")
		return ""
	})

	got := resolveTargetVersion(
		UpdateOptions{Version: "v0.13.0", Self: true},
		resolvedSource{BinaryPath: "/tmp/bedrud"},
	)
	if got.Version != "v0.13.0" || got.Origin != originSelf {
		t.Fatalf("got %+v, want v0.13.0 from %s", got, originSelf)
	}
}

func TestResolveTargetVersionSkipBinaryPrefersInstalledBinary(t *testing.T) {
	inService := runningBinaryPath()
	stubProbe(t, func(path string) string {
		if path != inService {
			t.Fatalf("probed %q, want the binary in service %q", path, inService)
		}
		return "v0.13.0"
	})

	// The package manager replaced the installed binary; the bedrud running
	// this command may be an older one earlier in PATH.
	got := resolveTargetVersion(UpdateOptions{Version: "v0.12.0", SkipBinary: true}, resolvedSource{})
	if got.Version != "v0.13.0" || got.Origin != originInstalledBinary {
		t.Fatalf("got %+v, want v0.13.0 from %s", got, originInstalledBinary)
	}
}

func TestResolveTargetVersionSkipBinaryFallsBackToRunningVersion(t *testing.T) {
	stubProbe(t, func(string) string { return "" })

	got := resolveTargetVersion(UpdateOptions{Version: "v0.12.0", SkipBinary: true}, resolvedSource{})
	if got.Version != "v0.12.0" || got.Origin != originSelf {
		t.Fatalf("got %+v, want v0.12.0 from %s", got, originSelf)
	}
}

func TestCheckTargetVersionSkipBinaryMatchesApplyPath(t *testing.T) {
	stubProbe(t, func(string) string { return "v0.13.0" })

	opts := UpdateOptions{Version: "v0.12.0", SkipBinary: true}
	target, source, note, err := checkTargetVersion(opts)
	if err != nil {
		t.Fatal(err)
	}
	if want := resolveTargetVersion(opts, resolvedSource{}); target != want {
		t.Fatalf("check reported %+v, apply would use %+v", target, want)
	}
	if note != "" {
		t.Fatalf("unexpected note %q", note)
	}
	if !strings.Contains(source, "--skip-binary") {
		t.Fatalf("got source %q", source)
	}
}

func TestDescribeTargetVersion(t *testing.T) {
	cases := []struct {
		name, installed, target, want string
	}{
		{"upgrade", "v0.12.0", "v0.13.0", "v0.13.0"},
		{"unknown", "v0.12.0", "", unknownVersion + " (source carries no version information)"},
		{"same", "v0.13.0", "v0.13.0", "v0.13.0 (no version change)"},
		{"downgrade", "v0.13.0", "v0.11.0", "v0.11.0 (downgrade from v0.13.0)"},
		{"unknown installed", unknownVersion, "v0.13.0", "v0.13.0"},
		{"non-semver", "dev", "dev", "dev (no version change)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := describeTargetVersion(c.installed, c.target); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestDescribeUpdateSource(t *testing.T) {
	if got := describeUpdateSource(UpdateOptions{SkipBinary: true}, resolvedSource{}); !strings.Contains(got, "--skip-binary") {
		t.Fatalf("got %q", got)
	}
	if got := describeUpdateSource(UpdateOptions{Source: "latest"}, resolvedSource{Description: "latest v0.13.0"}); got != "latest v0.13.0" {
		t.Fatalf("got %q", got)
	}
}

func TestUpdateField(t *testing.T) {
	got := updateField("Target version", "v0.13.0")
	if got != "  Target version:    v0.13.0" {
		t.Fatalf("misaligned field line: %q", got)
	}
}

func TestParseVersionJSON(t *testing.T) {
	out := []byte(`{"ok": true, "data": {"name": "bedrud", "version": "v0.13.0"}}`)
	if got := parseVersionJSON(out); got != "v0.13.0" {
		t.Fatalf("got %q", got)
	}
	if got := parseVersionJSON([]byte("not json")); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
	if got := parseVersionJSON(nil); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestParseVersionText(t *testing.T) {
	if got := parseVersionText([]byte("bedrud v0.13.0\n")); got != "v0.13.0" {
		t.Fatalf("got %q", got)
	}
	if got := parseVersionText([]byte("some warning\nbedrud dev\n")); got != "dev" {
		t.Fatalf("got %q", got)
	}
	if got := parseVersionText([]byte("unexpected output\n")); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestProbeBinaryVersionRunsTheBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("probe executes a POSIX shell script")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "bedrud")
	script := "#!/bin/sh\n" +
		"if [ \"$2\" = \"--json\" ]; then\n" +
		"  echo '{\"ok\":true,\"data\":{\"name\":\"bedrud\",\"version\":\"v0.13.0\"}}'\n" +
		"else\n" +
		"  echo 'bedrud v0.13.0'\n" +
		"fi\n"
	if err := os.WriteFile(fake, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if got := probeBinaryVersion(fake); got != "v0.13.0" {
		t.Fatalf("got %q, want v0.13.0", got)
	}
}

func TestProbeBinaryVersionUnrunnable(t *testing.T) {
	dir := t.TempDir()
	notABinary := filepath.Join(dir, "bedrud")
	if err := os.WriteFile(notABinary, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := probeBinaryVersion(notABinary); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestUpdateCommand(t *testing.T) {
	cases := []struct {
		opts UpdateOptions
		want string
	}{
		{UpdateOptions{Source: "latest"}, "sudo bedrud update latest"},
		{UpdateOptions{Self: true}, "sudo bedrud update --self"},
		{UpdateOptions{SkipBinary: true}, "sudo bedrud update --skip-binary"},
		{UpdateOptions{Source: "latest", ConfigPath: "/srv/bedrud.yaml"}, "sudo bedrud update latest --config /srv/bedrud.yaml"},
		{UpdateOptions{Source: "/tmp/bedrud", SkipChecksum: true}, "sudo bedrud update /tmp/bedrud --skip-checksum"},
		{UpdateOptions{Source: "latest", SkipMigrate: true}, "sudo bedrud update latest --skip-migrate"},
		{UpdateOptions{Source: "latest", SkipRestart: true}, "sudo bedrud update latest --skip-restart"},
		{
			UpdateOptions{Source: "/tmp/bedrud", ConfigPath: "/srv/bedrud.yaml", SkipChecksum: true, SkipMigrate: true, SkipRestart: true},
			"sudo bedrud update /tmp/bedrud --config /srv/bedrud.yaml --skip-checksum --skip-migrate --skip-restart",
		},
	}
	for _, c := range cases {
		if got := updateCommand(c.opts); got != c.want {
			t.Fatalf("got %q, want %q", got, c.want)
		}
	}
}

func TestUpdateCheckTextReport(t *testing.T) {
	check := UpdateCheck{
		InstalledVersion: "v0.12.0",
		TargetVersion:    "v0.13.0",
		Source:           "latest GitHub release v0.13.0",
		ConfigPath:       etcConfigPath,
		Command:          "sudo bedrud update latest",
	}
	report := check.TextReport()
	for _, want := range []string{
		"  Installed version: v0.12.0",
		"  Target version:    v0.13.0",
		"sudo bedrud update latest",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("report missing %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "Already up to date") {
		t.Fatalf("unexpected up-to-date line:\n%s", report)
	}

	check.UpToDate = true
	if !strings.Contains(check.TextReport(), "Already up to date") {
		t.Fatalf("missing up-to-date line:\n%s", check.TextReport())
	}
}

// TestResolveLocalArchiveIsProbeable covers the regression that made every
// local archive report an unknown version: members are extracted 0600, so the
// probe could not run the binary it had just unpacked.
func TestResolveLocalArchiveIsProbeable(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "bedrud_linux_amd64.tar.xz")
	data, err := writeTarXZBytes(map[string][]byte{"bedrud": {0x7f, 'E', 'L', 'F', 'v'}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive, data, 0o600); err != nil {
		t.Fatal(err)
	}

	resolved, err := resolveUpdateSource(UpdateOptions{Source: archive})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Cleanup != nil {
		defer resolved.Cleanup()
	}

	st, err := os.Stat(resolved.BinaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && st.Mode().Perm()&0o100 == 0 {
		t.Fatalf("extracted binary is not executable: mode %v", st.Mode().Perm())
	}

	// The probe stub stands in for exec: a fake ELF cannot actually run, but
	// it must be handed an executable file.
	stubProbe(t, func(path string) string {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("probe got an unusable path %q: %v", path, err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm()&0o100 == 0 {
			t.Fatalf("probe got a non-executable file: mode %v", info.Mode().Perm())
		}
		return "v0.13.0"
	})

	got := resolveTargetVersion(UpdateOptions{Version: "v0.12.0", Source: archive}, resolved)
	if got.Version != "v0.13.0" || got.Origin != originSourceBinary {
		t.Fatalf("got %+v, want v0.13.0 from %s", got, originSourceBinary)
	}
}

func TestCheckTargetVersionRefusesUnverifiedLocalSource(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bedrud")
	if err := os.WriteFile(bin, []byte{0x7f, 'E', 'L', 'F', 'v'}, 0o700); err != nil {
		t.Fatal(err)
	}
	stubProbe(t, func(string) string {
		t.Fatal("a check must not execute an unverified source")
		return ""
	})

	target, _, note, err := checkTargetVersion(UpdateOptions{Version: "v0.12.0", Source: bin})
	if err != nil {
		t.Fatal(err)
	}
	if target.Version != "" {
		t.Fatalf("got %+v, want an unknown target", target)
	}
	if !strings.Contains(note, "SHA256SUMS") {
		t.Fatalf("note does not explain why: %q", note)
	}
}

func TestCheckTargetVersionProbesWhenOperatorVouches(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bedrud")
	if err := os.WriteFile(bin, []byte{0x7f, 'E', 'L', 'F', 'v'}, 0o700); err != nil {
		t.Fatal(err)
	}
	stubProbe(t, func(string) string { return "v0.13.0" })

	target, _, note, err := checkTargetVersion(UpdateOptions{Version: "v0.12.0", Source: bin, SkipChecksum: true})
	if err != nil {
		t.Fatal(err)
	}
	if target.Version != "v0.13.0" || target.Origin != originSourceBinary {
		t.Fatalf("got %+v, want v0.13.0 from %s", target, originSourceBinary)
	}
	if note != "" {
		t.Fatalf("unexpected note %q", note)
	}
}

func TestProbeBinaryVersionCapsOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("probe executes a POSIX shell script")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "bedrud")
	// Floods stdout well past the cap and prints the version line last, so a
	// version only comes back if the output was buffered without bound. Pair
	// with TestProbeBinaryVersionSurvivesOutputFlood, which proves hitting the
	// cap does not fail the probe outright.
	script := "#!/bin/sh\n" +
		"i=0\n" +
		"while [ $i -lt 2000 ]; do\n" +
		"  echo xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n" +
		"  i=$((i+1))\n" +
		"done\n" +
		"echo 'bedrud v0.13.0'\n"
	if err := os.WriteFile(fake, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if got := probeBinaryVersion(fake); got != "" {
		t.Fatalf("got %q, want the flooded output dropped at the cap", got)
	}
}

// TestProbeBinaryVersionSurvivesOutputFlood covers the short-write bug: when
// the cap is reached, the capped buffer must keep reporting full writes, or
// io.Copy stops with ErrShortWrite, the child takes SIGPIPE, and the probe
// throws away a buffer that already holds the version.
func TestProbeBinaryVersionSurvivesOutputFlood(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("probe executes a POSIX shell script")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "bedrud")
	script := "#!/bin/sh\n" +
		"echo 'bedrud v0.13.0'\n" +
		"i=0\n" +
		"while [ $i -lt 2000 ]; do\n" +
		"  echo xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n" +
		"  i=$((i+1))\n" +
		"done\n"
	if err := os.WriteFile(fake, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if got := probeBinaryVersion(fake); got != "v0.13.0" {
		t.Fatalf("got %q, want v0.13.0 — output past the cap must be dropped, not fatal", got)
	}
}
