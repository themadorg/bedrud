package clioutput

import (
	"encoding/json"
	"fmt"
	"os"
	"sync/atomic"
)

var jsonOutput atomic.Bool

func SetJSON(enabled bool) {
	jsonOutput.Store(enabled)
}

func JSON() bool {
	return jsonOutput.Load()
}

func Success(message string, data any) error {
	if !JSON() {
		if message != "" {
			fmt.Println(message)
		}
		return nil
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{
		"ok":      true,
		"message": message,
		"data":    data,
	})
}

func EmitError(err error) {
	if err == nil {
		return
	}
	if JSON() {
		_ = json.NewEncoder(os.Stderr).Encode(map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}
	fmt.Fprintln(os.Stderr, err)
}

func Printf(format string, args ...any) {
	fmt.Printf(format, args...)
}

func Println(args ...any) {
	fmt.Println(args...)
}
