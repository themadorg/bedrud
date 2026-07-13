package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeVersion(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"dev", ""},
		{"unknown", ""},
		{"1.2.3", "v1.2.3"},
		{"v1.2.3", "v1.2.3"},
		{"v1.2.3-rc.1", "v1.2.3-rc.1"},
		{"not-a-version", ""},
	}
	for _, tc := range cases {
		if got := normalizeVersion(tc.in); got != tc.want {
			t.Errorf("normalizeVersion(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRunVersionMigrationsOrdering(t *testing.T) {
	var ran []string
	orig := versionMigrations
	t.Cleanup(func() { versionMigrations = orig })

	versionMigrations = []versionMigration{
		{Version: "v1.0.0", Name: "a", Run: func() error { ran = append(ran, "a"); return nil }},
		{Version: "v1.1.0", Name: "b", Run: func() error { ran = append(ran, "b"); return nil }},
		{Version: "v2.0.0", Name: "c", Run: func() error { ran = append(ran, "c"); return nil }},
	}

	// From v1.0.0 to v1.1.0 → only b
	ran = nil
	if err := runVersionMigrations("v1.0.0", "v1.1.0"); err != nil {
		t.Fatal(err)
	}
	if len(ran) != 1 || ran[0] != "b" {
		t.Fatalf("expected [b], got %v", ran)
	}

	// Unknown previous → all ≤ target
	ran = nil
	if err := runVersionMigrations("unknown", "v1.1.0"); err != nil {
		t.Fatal(err)
	}
	if len(ran) != 2 || ran[0] != "a" || ran[1] != "b" {
		t.Fatalf("expected [a b], got %v", ran)
	}

	// Already at latest → none
	ran = nil
	if err := runVersionMigrations("v2.0.0", "v2.0.0"); err != nil {
		t.Fatal(err)
	}
	if len(ran) != 0 {
		t.Fatalf("expected none, got %v", ran)
	}

	// Dev target → skip all
	ran = nil
	if err := runVersionMigrations("v1.0.0", "dev"); err != nil {
		t.Fatal(err)
	}
	if len(ran) != 0 {
		t.Fatalf("expected none for dev, got %v", ran)
	}
}

func TestWriteAndReadInstalledVersion(t *testing.T) {
	dir := t.TempDir()
	// Override paths via writing directly to a temp version file helper.
	// writeInstalledVersion uses fixed path; test normalize + file IO separately.
	path := filepath.Join(dir, "version")
	if err := os.WriteFile(path, []byte("v9.9.9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := normalizeVersion(string(b)); got != "v9.9.9" {
		// normalizeVersion trims; string may have newline
		if normalizeVersion("v9.9.9\n") != "v9.9.9" {
			t.Fatalf("got %q", got)
		}
	}
}
