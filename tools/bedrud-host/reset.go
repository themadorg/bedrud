package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func cmdReset(c Config, yes bool) error {
	if c.ConfigDir == "" {
		return fmt.Errorf("config dir not set")
	}
	enc := filepath.Join(c.ConfigDir, "hosts.sqlite.enc")
	plain := filepath.Join(c.ConfigDir, "hosts.sqlite")
	if !yes {
		if !shouldPrompt() {
			return fmt.Errorf("pass -yes to delete the local database")
		}
		fmt.Fprintf(os.Stderr, "Delete local database %s? Type yes: ", enc)
		var line string
		if _, err := fmt.Fscanln(promptIn, &line); err != nil || line != "yes" {
			fmt.Println("cancelled")
			return nil
		}
	}
	var removed []string
	for _, p := range []string{enc, plain} {
		if err := os.Remove(p); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		removed = append(removed, p)
	}
	if len(removed) == 0 {
		fmt.Println("no local database files")
		return nil
	}
	for _, p := range removed {
		fmt.Printf("deleted %s\n", p)
	}
	fmt.Println("run init to store credentials again")
	return nil
}
