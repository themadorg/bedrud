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
	installed := resolveInstalledBinary()
	stubProbe(t, func(path string) string {
		if path != installed {
			t.Fatalf("probed %q, want the installed binary %q", path, installed)
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
