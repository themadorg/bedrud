package main

import (
	"crypto/rand"
	"fmt"
	"strings"
)

const letters = "abcdefghijklmnopqrstuvwxyz"

func randomLabel(n int) (string, error) {
	if n < 3 {
		n = 3
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b), nil
}

func randomPassword(n int) (string, error) {
	if n < 12 {
		n = 12
	}
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#%^*+-"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	// guarantee mixed classes
	b[0] = 'A'
	b[1] = 'a'
	b[2] = '7'
	b[3] = '!'
	return string(b), nil
}

func hostFromArg(arg, zone string) string {
	arg = strings.ToLower(strings.Trim(strings.TrimSpace(arg), "."))
	z := strings.ToLower(strings.Trim(zone, "."))
	if strings.HasSuffix(arg, "."+z) {
		return strings.TrimSuffix(arg, "."+z)
	}
	return arg
}

func fqdn(host, zone string) string {
	return host + "." + strings.Trim(zone, ".")
}

func linodeLabel(host, zone string) string {
	s := strings.ReplaceAll(fqdn(host, zone), ".", "-")
	if len(s) > 32 {
		s = s[:32]
	}
	return s
}

func adminEmail(host, zone string) string {
	return fmt.Sprintf("admin@%s", fqdn(host, zone))
}

func splitFQDN(arg string) (host, zone, name string, err error) {
	arg = strings.ToLower(strings.Trim(strings.TrimSpace(arg), "."))
	i := strings.IndexByte(arg, '.')
	if i <= 0 || i == len(arg)-1 {
		return "", "", "", fmt.Errorf("need a full host name (subdomain.zone)")
	}
	return arg[:i], arg[i+1:], arg, nil
}

func resolveName(arg, zone string) (host, z, name string, err error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", "", "", fmt.Errorf("missing host")
	}
	if strings.TrimSpace(zone) != "" {
		h := hostFromArg(arg, zone)
		return h, strings.Trim(zone, "."), fqdn(h, zone), nil
	}
	return splitFQDN(arg)
}
