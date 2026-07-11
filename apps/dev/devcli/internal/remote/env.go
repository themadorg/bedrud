package remote

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	envSSHHost              = "REMOTE_DEBUG_SSH_HOST"
	envSSHUser              = "REMOTE_DEBUG_SSH_USER"
	envSSHPort              = "REMOTE_DEBUG_SSH_PORT"
	envSSHIdentityFile      = "REMOTE_DEBUG_SSH_IDENTITY_FILE"
	envWGEndpoint           = "REMOTE_DEBUG_WG_ENDPOINT"
	envLiveKitAPISecret     = "REMOTE_DEBUG_LIVEKIT_API_SECRET"
	envACMEEmail            = "REMOTE_DEBUG_ACME_EMAIL"
	envTunnelToken          = "REMOTE_DEBUG_TUNNEL_TOKEN"
	envTunnelTLSFingerprint = "REMOTE_DEBUG_TUNNEL_TLS_FINGERPRINT"
	envWebxdcWildcardCert   = "REMOTE_DEBUG_WEBXDC_WILDCARD_CERT"
	envWebxdcWildcardKey    = "REMOTE_DEBUG_WEBXDC_WILDCARD_KEY"
)

// EnvPath returns server/.env under repo root.
func EnvPath(repo string) string {
	return filepath.Join(repo, "server", ".env")
}

func loadSSHFromEnv(repo string, cfg *Config) error {
	vals := map[string]string{}

	// Process environment takes precedence over .env file.
	for _, key := range []string{envSSHHost, envSSHUser, envSSHPort, envSSHIdentityFile} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			vals[key] = v
		}
	}

	path := EnvPath(repo)
	fileVals, err := parseDotEnv(path)
	if err != nil {
		return err
	}
	for k, v := range fileVals {
		if _, set := vals[k]; !set && v != "" {
			vals[k] = v
		}
	}

	if host := vals[envSSHHost]; host != "" {
		cfg.SSH.Host = host
	}
	if user := vals[envSSHUser]; user != "" {
		cfg.SSH.User = user
	}
	if port := vals[envSSHPort]; port != "" {
		n, err := strconv.Atoi(port)
		if err != nil {
			return fmt.Errorf("%s must be a number (got %q)", envSSHPort, port)
		}
		cfg.SSH.Port = n
	}
	if id := vals[envSSHIdentityFile]; id != "" {
		cfg.SSH.IdentityFile = id
	}

	return nil
}

func loadProvisionFromEnv(repo string, cfg *Config) error {
	vals := map[string]string{}
	for _, key := range []string{
		envWGEndpoint, envLiveKitAPISecret, envACMEEmail, envTunnelToken, envTunnelTLSFingerprint,
		envWebxdcWildcardCert, envWebxdcWildcardKey,
	} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			vals[key] = v
		}
	}
	fileVals, err := parseDotEnv(EnvPath(repo))
	if err != nil {
		return err
	}
	for k, v := range fileVals {
		if _, set := vals[k]; !set && v != "" {
			vals[k] = v
		}
	}
	if ep := vals[envWGEndpoint]; ep != "" {
		cfg.Provision.WGEndpoint = ep
	}
	if secret := vals[envLiveKitAPISecret]; secret != "" {
		cfg.LiveKit.APISecret = secret
	}
	if email := vals[envACMEEmail]; email != "" {
		cfg.Provision.ACMEmail = email
	}
	if token := vals[envTunnelToken]; token != "" {
		cfg.Tunnel.DevTunnel.Token = token
	}
	if fp := vals[envTunnelTLSFingerprint]; fp != "" {
		cfg.Tunnel.DevTunnel.TLSFingerprint = fp
	}
	// Wildcard cert paths fill empty YAML only (YAML can pin remote paths).
	if cert := vals[envWebxdcWildcardCert]; cert != "" && cfg.Provision.WebxdcWildcardCert == "" {
		cfg.Provision.WebxdcWildcardCert = cert
	}
	if key := vals[envWebxdcWildcardKey]; key != "" && cfg.Provision.WebxdcWildcardKey == "" {
		cfg.Provision.WebxdcWildcardKey = key
	}
	return nil
}

func parseDotEnv(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	out := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		if key != "" {
			out[key] = val
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return out, nil
}

// UpsertDotEnv sets key=value in server/.env (creates the file if missing).
// Existing keys are replaced in place; new keys are appended. Comments and
// unrelated lines are preserved. Values with spaces or # are double-quoted.
func UpsertDotEnv(repo, key, value string) error {
	if repo == "" {
		return fmt.Errorf("upsert .env: empty repo path")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("upsert .env: empty key")
	}
	path := EnvPath(repo)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	var lines []string
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else if len(data) > 0 {
		// Preserve trailing newline handling via Split (no trailing empty from final \n).
		text := string(data)
		text = strings.TrimSuffix(text, "\n")
		if text != "" {
			lines = strings.Split(text, "\n")
		}
	}

	encoded := encodeDotEnvValue(value)
	assignment := key + "=" + encoded
	replaced := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		exportPrefixed := false
		work := trimmed
		if strings.HasPrefix(work, "export ") {
			exportPrefixed = true
			work = strings.TrimSpace(strings.TrimPrefix(work, "export "))
		}
		k, _, ok := strings.Cut(work, "=")
		if !ok || strings.TrimSpace(k) != key {
			continue
		}
		if exportPrefixed {
			lines[i] = "export " + assignment
		} else {
			lines[i] = assignment
		}
		replaced = true
		// Keep replacing all occurrences so duplicates stay consistent.
	}
	if !replaced {
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		lines = append(lines, assignment)
	}

	out := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(out), 0o600)
}

func encodeDotEnvValue(v string) string {
	if v == "" {
		return ""
	}
	if strings.ContainsAny(v, " \t#\"'\\") || strings.Contains(v, "\n") {
		escaped := strings.ReplaceAll(v, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		return `"` + escaped + `"`
	}
	return v
}

// persistRemoteSecrets writes generated secrets into server/.env when repo is known.
func persistRemoteSecrets(cfg *Config, pairs map[string]string) {
	if len(pairs) == 0 {
		return
	}
	repo, err := findRepoForDetached(cfg)
	if err != nil {
		fmt.Printf("warning: could not locate repo to update server/.env: %v\n", err)
		return
	}
	for k, v := range pairs {
		if strings.TrimSpace(v) == "" {
			continue
		}
		if err := UpsertDotEnv(repo, k, v); err != nil {
			fmt.Printf("warning: failed to write %s to server/.env: %v\n", k, err)
			continue
		}
		fmt.Printf("wrote %s to server/.env\n", k)
	}
}