package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeVersionID(t *testing.T) {
	if _, err := sanitizeVersionID("../x"); err == nil {
		t.Fatal("expected reject")
	}
	if _, err := sanitizeVersionID("a/b"); err == nil {
		t.Fatal("expected reject")
	}
	if _, err := sanitizeVersionID("2.20.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := sanitizeVersionID("v1.2.3"); err != nil {
		t.Fatal(err)
	}
	if _, err := sanitizeVersionID("dev-551a2f9-dirty"); err != nil {
		t.Fatal(err)
	}
}

func TestVersionTreeInstallListActivatePrune(t *testing.T) {
	root := t.TempDir()
	stable := filepath.Join(t.TempDir(), "bin", "bedrud")
	t.Setenv(envInstallRoot, root)
	t.Setenv("BEDRUD_STABLE_BIN", stable)
	t.Setenv(envNoVMService, "1")

	writeELF := func(name string, extra byte) string {
		p := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(p, []byte{0x7f, 'E', 'L', 'F', extra}, 0o700); err != nil {
			t.Fatal(err)
		}
		return p
	}

	a := writeELF("a", 1)
	b := writeELF("b", 2)
	c := writeELF("c", 3)

	if _, err := installCandidate(root, "1.0.0", a, VersionMeta{Source: "test"}); err != nil {
		t.Fatal(err)
	}
	if _, err := installCandidate(root, "1.1.0", b, VersionMeta{Source: "test"}); err != nil {
		t.Fatal(err)
	}
	if _, err := installCandidate(root, "1.2.0", c, VersionMeta{Source: "test"}); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Dir(stable), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := setActive(root, "1.2.0", stable); err != nil {
		t.Fatal(err)
	}
	got, err := resolveActiveVersion(root)
	if err != nil || got != "1.2.0" {
		t.Fatalf("active=%q err=%v", got, err)
	}
	list, err := listInstalled(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("len=%d", len(list))
	}

	if err := removeVersion(root, "1.2.0"); err == nil {
		t.Fatal("should refuse removing active")
	}
	if err := activateVersion(root, "1.1.0", false); err != nil {
		t.Fatal(err)
	}
	removed, err := pruneVersions(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) == 0 {
		t.Fatal("expected prune to remove non-active")
	}
	left, _ := listInstalled(root)
	if len(left) != 1 || left[0].Version != "1.1.0" {
		t.Fatalf("left=%+v", left)
	}
	if err := removeVersion(root, "1.1.0"); err == nil {
		t.Fatal("should refuse removing last version")
	}
}

func TestParseGitHubReleasesSkipsPrerelease(t *testing.T) {
	body := `[
	  {"tag_name":"v9.0.0-rc.1","prerelease":true,"draft":false,"assets":[]},
	  {"tag_name":"v8.1.0","prerelease":false,"draft":false,"assets":[{"name":"bedrud_linux_amd64.tar.xz"}]},
	  {"tag_name":"v8.0.0","prerelease":false,"draft":false,"assets":[]}
	]`
	got, err := parseGitHubReleasesJSON(body, "bedrud_linux_amd64.tar.xz")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Version != "8.1.0" || !got[0].IsLatest {
		t.Fatalf("%+v", got)
	}
}

func TestMergeLocalAndRemote(t *testing.T) {
	local := []InstalledVersion{{Version: "8.0.0", Active: true}}
	pub := "2026-01-01"
	remote := []RemoteRelease{
		{Version: "8.1.0", IsLatest: true, PublishedAt: &pub},
		{Version: "8.0.0"},
	}
	m := mergeLocalAndRemote(local, remote)
	if len(m) != 2 {
		t.Fatalf("len=%d", len(m))
	}
	var both, rem VersionListEntry
	for _, e := range m {
		switch e.Version {
		case "8.0.0":
			both = e
		case "8.1.0":
			rem = e
		}
	}
	if both.Source != "both" || !both.Installed || !both.Active {
		t.Fatalf("both=%+v", both)
	}
	if rem.Source != "remote" || rem.Installed || !rem.RemoteLatest {
		t.Fatalf("rem=%+v", rem)
	}
}
