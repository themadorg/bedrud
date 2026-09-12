package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

var (
	promptIn  io.Reader = os.Stdin
	promptErr io.Writer = os.Stderr
)

func readLine(prompt string) (string, error) {
	fmt.Fprint(promptErr, prompt)
	var b []byte
	tmp := make([]byte, 1)
	for {
		n, err := promptIn.Read(tmp)
		if n == 1 {
			if tmp[0] == '\n' {
				break
			}
			if tmp[0] != '\r' {
				b = append(b, tmp[0])
			}
		}
		if err != nil {
			if err == io.EOF && len(b) > 0 {
				break
			}
			return "", err
		}
	}
	return strings.TrimSpace(string(b)), nil
}

func readSecret(prompt string) (string, error) {
	f, ok := promptIn.(*os.File)
	if ok && term.IsTerminal(int(f.Fd())) {
		fmt.Fprint(promptErr, prompt)
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(promptErr)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	return readLine(prompt)
}

func maskSecret(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "(not set)"
	}
	if len(s) <= 4 {
		return "****"
	}
	return "****" + s[len(s)-4:]
}
