package install

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed man/bedrud.1
var embeddedManPage string

// EmbeddedManPage returns the packaged bedrud(1) roff source.
func EmbeddedManPage() string {
	return embeddedManPage
}

// Overridable in tests.
var (
	manPagePathFn = defaultManPagePath
	bashCompPath  = defaultBashCompPath
	zshCompPath   = defaultZshCompPath
	fishCompPath  = defaultFishCompPath
	// generateCompletionsFn produces bash, zsh, fish scripts for binaryName.
	generateCompletionsFn = generateCompletionsViaSelf
	// mandbRunner refreshes the man database (no-op if mandb missing).
	mandbRunner = runMandb
)

func defaultManPagePath(binaryName string) string {
	return filepath.Join("/usr/share/man/man1", binaryName+".1")
}

func defaultBashCompPath(binaryName string) string {
	return filepath.Join("/usr/share/bash-completion/completions", binaryName)
}

func defaultZshCompPath(binaryName string) string {
	return filepath.Join("/usr/share/zsh/site-functions", "_"+binaryName)
}

func defaultFishCompPath(binaryName string) string {
	return filepath.Join("/usr/share/fish/vendor_completions.d", binaryName+".fish")
}

// binaryNameFromArgv returns the basename of argv[0] (e.g. "bedrud").
func binaryNameFromArgv() string {
	if len(os.Args) == 0 {
		return "bedrud"
	}
	base := filepath.Base(os.Args[0])
	if base == "" || base == "." {
		return "bedrud"
	}
	return base
}

// installCLIDocs writes the man page and shell completion scripts to FHS paths.
func installCLIDocs() error {
	return installCLIDocsFor(binaryNameFromArgv())
}

func installCLIDocsFor(binaryName string) error {
	if binaryName == "" {
		binaryName = "bedrud"
	}

	manPath := manPagePathFn(binaryName)
	if err := writeFileWithDirs(manPath, []byte(embeddedManPage), 0o644); err != nil {
		return fmt.Errorf("man page: %w", err)
	}

	bash, zsh, fish, err := generateCompletionsFn(binaryName)
	if err != nil {
		return fmt.Errorf("generate completions: %w", err)
	}
	if err := writeFileWithDirs(bashCompPath(binaryName), bash, 0o644); err != nil {
		return fmt.Errorf("bash completion: %w", err)
	}
	if err := writeFileWithDirs(zshCompPath(binaryName), zsh, 0o644); err != nil {
		return fmt.Errorf("zsh completion: %w", err)
	}
	if err := writeFileWithDirs(fishCompPath(binaryName), fish, 0o644); err != nil {
		return fmt.Errorf("fish completion: %w", err)
	}

	mandbRunner()
	return nil
}

// removeCLIDocs removes man page and completion scripts for the argv binary name.
func removeCLIDocs() error {
	name := binaryNameFromArgv()
	var errs []error
	for _, p := range []string{
		manPagePathFn(name),
		bashCompPath(name),
		zshCompPath(name),
		fishCompPath(name),
	} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("remove %s: %w", p, err))
		}
	}
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = e.Error()
	}
	return fmt.Errorf("%s", strings.Join(msgs, "; "))
}

func writeFileWithDirs(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, mode)
}

func generateCompletionsViaSelf(binaryName string) (bash, zsh, fish []byte, err error) {
	if completionGenerator != nil {
		return completionGenerator(binaryName)
	}
	self, err := os.Executable()
	if err != nil {
		return nil, nil, nil, err
	}
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}
	run := func(shell string) ([]byte, error) {
		cmd := exec.Command(self, "completion", shell)
		out, err := cmd.Output()
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				return nil, fmt.Errorf("%s completion: %w: %s", shell, err, strings.TrimSpace(string(ee.Stderr)))
			}
			return nil, fmt.Errorf("%s completion: %w", shell, err)
		}
		return out, nil
	}
	if bash, err = run("bash"); err != nil {
		return nil, nil, nil, err
	}
	if zsh, err = run("zsh"); err != nil {
		return nil, nil, nil, err
	}
	if fish, err = run("fish"); err != nil {
		return nil, nil, nil, err
	}
	return bash, zsh, fish, nil
}

// CompletionGenerator produces shell completion scripts for a binary name.
type CompletionGenerator func(binaryName string) (bash, zsh, fish []byte, err error)

// completionGenerator, when set (by the cli package), is used instead of exec.
var completionGenerator CompletionGenerator

// SetCompletionGenerator registers an in-process completion generator.
// The cli package should call this from init to avoid recursive exec.
func SetCompletionGenerator(fn CompletionGenerator) {
	completionGenerator = fn
}

func runMandb() {
	if _, err := exec.LookPath("mandb"); err != nil {
		return
	}
	_ = exec.Command("mandb", "-q").Run()
}
