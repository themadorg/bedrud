package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"bedrud/internal/clioutput"
	"bedrud/internal/install"

	"github.com/spf13/cobra"
)

func init() {
	// Prefer in-process completion generation during install/update (no re-exec).
	install.SetCompletionGenerator(generateCompletions)
}

// newCompletionCmd prints shell tab-completion scripts for bash, zsh, and fish.
func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish]",
		Short: "Generate shell tab-completion scripts",
		Long: `Generate shell tab-completion scripts for bedrud.

To load completions in the current shell session:

  # bash
  source <(bedrud completion bash)

  # zsh
  source <(bedrud completion zsh)

  # fish
  bedrud completion fish | source

To install completions system-wide (requires root), or rely on
"bedrud install" / "bedrud update" which write the standard FHS paths:

  # bash
  bedrud completion bash | sudo tee /usr/share/bash-completion/completions/bedrud

  # zsh
  bedrud completion zsh | sudo tee /usr/share/zsh/site-functions/_bedrud

  # fish
  bedrud completion fish | sudo tee /usr/share/fish/vendor_completions.d/bedrud.fish
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish"},
		Args:                  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeCompletion(os.Stdout, args[0], argvBinaryName())
		},
	}
	return cmd
}

// newGenerateManCmd is a hidden helper for packagers: print the embedded man page.
func newGenerateManCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "generate-man",
		Short:  "Print the bedrud(1) man page (roff)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			man := install.EmbeddedManPage()
			if clioutput.JSON() {
				return clioutput.Success("", map[string]any{
					"man":    man,
					"binary": argvBinaryName(),
				})
			}
			_, err := io.WriteString(os.Stdout, man)
			return err
		},
	}
}

// newGenerateFishCompletionCmd is a hidden helper (alias for completion fish).
func newGenerateFishCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "generate-fish-completion",
		Short:  "Print fish completion script",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeCompletion(os.Stdout, "fish", argvBinaryName())
		},
	}
}

func argvBinaryName() string {
	if len(os.Args) == 0 {
		return "bedrud"
	}
	base := filepath.Base(os.Args[0])
	if base == "" || base == "." {
		return "bedrud"
	}
	return base
}

func writeCompletion(w io.Writer, shell, binaryName string) error {
	root := NewRootCmd()
	// Avoid recursing into completion while generating.
	root.Use = binaryName
	switch strings.ToLower(shell) {
	case "bash":
		return root.GenBashCompletionV2(w, true)
	case "zsh":
		return root.GenZshCompletion(w)
	case "fish":
		return root.GenFishCompletion(w, true)
	default:
		return fmt.Errorf("unsupported shell %q (want bash, zsh, or fish)", shell)
	}
}

func generateCompletions(binaryName string) (bash, zsh, fish []byte, err error) {
	var bb, zb, fb bytes.Buffer
	if err = writeCompletion(&bb, "bash", binaryName); err != nil {
		return nil, nil, nil, err
	}
	if err = writeCompletion(&zb, "zsh", binaryName); err != nil {
		return nil, nil, nil, err
	}
	if err = writeCompletion(&fb, "fish", binaryName); err != nil {
		return nil, nil, nil, err
	}
	return bb.Bytes(), zb.Bytes(), fb.Bytes(), nil
}
