//go:build !linux

package tray

import "fmt"

func startXEmbed(cb Callbacks) (func(), error) {
	return nil, fmt.Errorf("xembed tray: linux only")
}
