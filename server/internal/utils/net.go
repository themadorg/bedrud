package utils

import (
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
)

func OutboundIP() net.IP {
	conn, err := net.Dial("udp4", "8.8.8.8:80")
	if err != nil {
		return net.ParseIP("127.0.0.1")
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP
}

// BindError annotates a listen failure. Binding a port below 1024 as a non-root
// user fails with EACCES, which on its own reads as a generic "permission denied".
func BindError(addr string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EACCES) {
		exe, _ := os.Executable()
		if exe == "" {
			exe = "/usr/local/bin/bedrud"
		}
		return fmt.Errorf("%w — binding %s needs privileges. Grant them once with: sudo setcap cap_net_bind_service=+ep %s (systemd units installed by `bedrud install` already set AmbientCapabilities=CAP_NET_BIND_SERVICE), or run behind a reverse proxy on a port above 1024", err, addr, exe)
	}
	return err
}

func DisplayAddr(host, port string) string {
	if host == "0.0.0.0" || host == "" {
		return OutboundIP().String() + ":" + port
	}
	return host + ":" + port
}
