package install

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func confirmGitHubUpdate(opts UpdateOptions, previous, next, channel string) error {
	if channel == "" {
		return nil
	}
	if opts.Yes {
		fmt.Println("➜ Confirmed (-y)")
		return nil
	}

	kind := "stable"
	if channel == "nightly" {
		kind = "nightly / prerelease"
	}
	fmt.Println()
	fmt.Printf("About to update Bedrud %s → %s (%s).\n", previous, next, kind)
	fmt.Println("Services will stop, the binary will be replaced, the database migrated, then services restart.")
	fmt.Println("Config, secrets, and certificates are preserved.")
	if channel == "nightly" {
		fmt.Println("Nightly builds are not stable releases.")
	}

	if !stdinIsTTY() {
		return fmt.Errorf("refusing to update without confirmation (stdin is not a TTY); re-run with -y")
	}

	fmt.Print("Proceed? [y/N]: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return fmt.Errorf("read confirmation: %w", err)
	}
	ans := strings.TrimSpace(strings.ToLower(line))
	if ans != "y" && ans != "yes" {
		return fmt.Errorf("update cancelled")
	}
	return nil
}

func stdinIsTTY() bool {
	st, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}
