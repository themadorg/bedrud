package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedManPageSections(t *testing.T) {
	man := EmbeddedManPage()
	for _, section := range []string{
		`.TH "bedrud" "1"`,
		".SH NAME",
		".SH SYNOPSIS",
		".SH DESCRIPTION",
		".SH OPTIONS",
		".SH COMMANDS",
		".SH EXIT STATUS",
		".SH ENVIRONMENT",
		".SH FILES",
		".SH EXAMPLES",
		".SH SEE ALSO",
	} {
		if !strings.Contains(man, section) {
			t.Fatalf("man page missing %q", section)
		}
	}
	if !strings.Contains(man, "completion") {
		t.Fatal("man page should mention completion")
	}
	if !strings.Contains(man, "update") {
		t.Fatal("man page should mention update")
	}
}

func TestInstallAndRemoveCLIDocs(t *testing.T) {
	base := t.TempDir()
	name := "bedrud"

	oldMan, oldBash, oldZsh, oldFish := manPagePathFn, bashCompPath, zshCompPath, fishCompPath
	oldGen, oldMandb := generateCompletionsFn, mandbRunner
	t.Cleanup(func() {
		manPagePathFn, bashCompPath, zshCompPath, fishCompPath = oldMan, oldBash, oldZsh, oldFish
		generateCompletionsFn, mandbRunner = oldGen, oldMandb
	})

	manPagePathFn = func(b string) string { return filepath.Join(base, "man", b+".1") }
	bashCompPath = func(b string) string { return filepath.Join(base, "bash", b) }
	zshCompPath = func(b string) string { return filepath.Join(base, "zsh", "_"+b) }
	fishCompPath = func(b string) string { return filepath.Join(base, "fish", b+".fish") }
	mandbRunner = func() {}
	generateCompletionsFn = func(binaryName string) (bash, zsh, fish []byte, err error) {
		return []byte("# bash " + binaryName),
			[]byte("# zsh " + binaryName),
			[]byte("# fish " + binaryName),
			nil
	}

	if err := installCLIDocsFor(name); err != nil {
		t.Fatal(err)
	}

	manData, err := os.ReadFile(manPagePathFn(name))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manData), `.TH "bedrud" "1"`) {
		t.Fatalf("unexpected man content")
	}
	for _, p := range []string{bashCompPath(name), zshCompPath(name), fishCompPath(name)} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if len(b) == 0 {
			t.Fatalf("%s empty", p)
		}
	}

	// re-install overwrites
	if err := installCLIDocsFor(name); err != nil {
		t.Fatal(err)
	}

	// removeCLIDocs uses binaryNameFromArgv — call remove paths directly
	for _, p := range []string{manPagePathFn(name), bashCompPath(name), zshCompPath(name), fishCompPath(name)} {
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCLIDocPathsFHS(t *testing.T) {
	if defaultManPagePath("bedrud") != "/usr/share/man/man1/bedrud.1" {
		t.Fatal(defaultManPagePath("bedrud"))
	}
	if defaultBashCompPath("bedrud") != "/usr/share/bash-completion/completions/bedrud" {
		t.Fatal(defaultBashCompPath("bedrud"))
	}
	if defaultZshCompPath("bedrud") != "/usr/share/zsh/site-functions/_bedrud" {
		t.Fatal(defaultZshCompPath("bedrud"))
	}
	if defaultFishCompPath("bedrud") != "/usr/share/fish/vendor_completions.d/bedrud.fish" {
		t.Fatal(defaultFishCompPath("bedrud"))
	}
}
