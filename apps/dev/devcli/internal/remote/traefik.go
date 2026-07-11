package remote

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bedrud/devcli/internal/logfmt"
)

// TraefikStaticSync uploads traefik.yml (entrypoints + optional ACME) and restarts Traefik.
func TraefikStaticSync(cfg *Config) error {
	if err := pingSSH(cfg); err != nil {
		return err
	}
	state := cfg.Provision.StateDir
	content := traefikStaticYAML(cfg)
	configPath := state + "/traefik.yml"
	hashPath := state + "/.traefik.yml.sha256"
	newHash := contentSHA256(content)
	storedHash := remoteStoredHash(cfg, hashPath)
	configChanged := storedHash == "" || storedHash != newHash

	if configChanged {
		if err := UploadContent(cfg, content, configPath, "644"); err != nil {
			return fmt.Errorf("upload traefik static config: %w", err)
		}
		if err := storeRemoteHash(cfg, newHash, hashPath); err != nil {
			return fmt.Errorf("upload traefik config hash: %w", err)
		}
		logfmt.Println("traefik", fmt.Sprintf("static config → %s/traefik.yml", state))
	} else {
		logfmt.Println("traefik", "static config unchanged — skipping upload")
	}
	if cfg.acmeEnabled() {
		if err := SSHSudo(cfg, fmt.Sprintf("touch %s/acme.json && chmod 600 %s/acme.json",
			state, state)); err != nil {
			return err
		}
		if err := SSHSudo(cfg, "ufw allow 443/tcp >/dev/null 2>&1 || true"); err != nil {
			return err
		}
	}
	if !configChanged {
		logfmt.Println("traefik", "static config unchanged — skipping restart")
		return nil
	}
	if err := SSHSudo(cfg, "systemctl restart bedrud-traefik"); err != nil {
		return fmt.Errorf("restart traefik: %w", err)
	}
	logfmt.Println("traefik", "static config applied (bedrud-traefik restarted)")
	return nil
}

// TraefikSync generates Traefik dynamic routes and uploads them to the remote server.
func TraefikSync(cfg *Config) error {
	if err := pingSSH(cfg); err != nil {
		return err
	}
	if cfg.acmeEnabled() {
		if err := TraefikStaticSync(cfg); err != nil {
			return err
		}
	}
	if err := MkdirRemote(cfg, cfg.Traefik.DynamicDir); err != nil {
		return err
	}

	tmp, err := os.MkdirTemp("", "bedrud-traefik-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	files := map[string]string{
		"bedrud-debug-web.yaml":     traefikWebYAML(cfg),
		"bedrud-debug-api.yaml":     traefikAPIYAML(cfg),
		"bedrud-debug-livekit.yaml": traefikLiveKitYAML(cfg),
		// WebXDC mini-app hosts: webxdc-<id>.{publicHost} → local API (Host header kept).
		"bedrud-debug-webxdc.yaml": traefikWebxdcYAML(cfg),
	}

	var combined strings.Builder
	names := []string{
		"bedrud-debug-web.yaml",
		"bedrud-debug-api.yaml",
		"bedrud-debug-livekit.yaml",
		"bedrud-debug-webxdc.yaml",
	}
	for _, name := range names {
		combined.WriteString(files[name])
	}
	newHash := contentSHA256(combined.String())
	hashPath := cfg.Provision.StateDir + "/.traefik-dynamic.sha256"
	if remoteStoredHash(cfg, hashPath) == newHash {
		logfmt.Println("traefik", "dynamic config unchanged — skipping upload")
		return nil
	}

	var paths []string
	for _, name := range names {
		content := files[name]
		p := filepath.Join(tmp, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			return err
		}
		paths = append(paths, p)
		logfmt.Println("traefik", fmt.Sprintf("%s → %s/%s", name, cfg.Traefik.DynamicDir, name))
	}

	if err := SCP(cfg, paths, cfg.Traefik.DynamicDir); err != nil {
		return fmt.Errorf("upload traefik config: %w", err)
	}
	if err := storeRemoteHash(cfg, newHash, hashPath); err != nil {
		return fmt.Errorf("upload traefik dynamic hash: %w", err)
	}
	logfmt.Println("traefik", "dynamic config synced (file provider will auto-reload)")
	return nil
}

// TraefikStatus checks remote Traefik and lists synced route files.
func TraefikStatus(cfg *Config) error {
	if err := pingSSH(cfg); err != nil {
		return err
	}
	out, err := SSHOutput(cfg, fmt.Sprintf("ls -1 %s 2>/dev/null || true", shellQuote(cfg.Traefik.DynamicDir)))
	if err != nil {
		return err
	}
	if out == "" {
		fmt.Printf("traefik | no files in %s (run: devcli remote traefik sync)\n", cfg.Traefik.DynamicDir)
	} else {
		fmt.Printf("traefik | %s:\n", cfg.Traefik.DynamicDir)
		for _, line := range strings.Split(out, "\n") {
			if line != "" {
				fmt.Printf("  • %s\n", line)
			}
		}
	}
	// Common Traefik deployments: systemd unit or docker container
	for _, probe := range []struct {
		label string
		cmd   string
	}{
		{"systemd", "systemctl is-active bedrud-traefik 2>/dev/null || systemctl is-active traefik 2>/dev/null || true"},
		{"docker", "docker ps --filter name=traefik --format '{{.Names}} {{.Status}}' 2>/dev/null || true"},
	} {
		status, _ := SSHOutput(cfg, probe.cmd)
		if status != "" && status != "inactive" && status != "unknown" {
			fmt.Printf("traefik | %s: %s\n", probe.label, status)
		}
	}
	return nil
}

// TraefikShow prints the generated YAML without uploading.
func TraefikShow(cfg *Config) {
	fmt.Println("--- bedrud-debug-web.yaml ---")
	fmt.Println(traefikWebYAML(cfg))
	fmt.Println("--- bedrud-debug-api.yaml ---")
	fmt.Println(traefikAPIYAML(cfg))
	fmt.Println("--- bedrud-debug-livekit.yaml ---")
	fmt.Println(traefikLiveKitYAML(cfg))
	fmt.Println("--- bedrud-debug-webxdc.yaml ---")
	fmt.Println(traefikWebxdcYAML(cfg))
}

func traefikTLSBlock(cfg *Config) string {
	if cfg.acmeEnabled() {
		return "      tls:\n        certResolver: letsencrypt\n"
	}
	return ""
}

// traefikWebxdcTLSBlock attaches TLS for webxdc-<id>.publicHost.
// HTTP-01 ACME cannot issue *.publicHost; prefer an explicit wildcard cert
// (provision.webxdcWildcardCert / webxdcWildcardKey) when present.
func traefikWebxdcTLSBlock(cfg *Config) string {
	cert := strings.TrimSpace(cfg.Provision.WebxdcWildcardCert)
	key := strings.TrimSpace(cfg.Provision.WebxdcWildcardKey)
	if cert != "" && key != "" {
		// Use default certificate store via dynamic tls.certificates (see traefikWebxdcYAML top-level tls).
		// Router still needs tls: {} so HTTPS is terminated.
		return "      tls: {}\n"
	}
	if cfg.acmeEnabled() {
		// Per-host HTTP-01 only works for Host(`exact`) rules, not HostRegexp.
		// Still enable TLS termination; expect a default/wildcard cert or browser warnings
		// until a wildcard is installed (see remote-debug.yaml.example).
		return "      tls:\n        certResolver: letsencrypt\n"
	}
	return ""
}

func traefikWebYAML(cfg *Config) string {
	host := cfg.URLs.PublicHost
	ep := cfg.EffectiveTraefikEntryPoint()
	return fmt.Sprintf("# Generated by devcli — routes public HTTPS to local Vite over tunnel\n"+
		"http:\n"+
		"  routers:\n"+
		"    bedrud-debug-web:\n"+
		"      rule: \"Host(`%s`)\"\n"+
		"      entryPoints:\n"+
		"        - %s\n"+
		"      service: bedrud-debug-web\n"+
		"      priority: 1\n"+
		"%s"+
		"  services:\n"+
		"    bedrud-debug-web:\n"+
		"      loadBalancer:\n"+
		"        servers:\n"+
		"          - url: %q\n",
		host, ep, traefikTLSBlock(cfg), cfg.WebBackend())
}

func traefikAPIYAML(cfg *Config) string {
	host := cfg.URLs.PublicHost
	ep := cfg.EffectiveTraefikEntryPoint()
	return fmt.Sprintf("# Generated by devcli — routes /api to local Go API over tunnel\n"+
		"http:\n"+
		"  routers:\n"+
		"    bedrud-debug-api:\n"+
		"      rule: \"Host(`%s`) && PathPrefix(`/api`)\"\n"+
		"      entryPoints:\n"+
		"        - %s\n"+
		"      service: bedrud-debug-api\n"+
		"      priority: 100\n"+
		"%s"+
		"  services:\n"+
		"    bedrud-debug-api:\n"+
		"      loadBalancer:\n"+
		"        servers:\n"+
		"          - url: %q\n",
		host, ep, traefikTLSBlock(cfg), cfg.APIBackend())
}

func traefikLiveKitYAML(cfg *Config) string {
	host := cfg.URLs.PublicHost
	ep := cfg.EffectiveTraefikEntryPoint()
	return fmt.Sprintf("# Generated by devcli — routes /livekit to remote LiveKit (WebSocket + HTTP)\n"+
		"http:\n"+
		"  middlewares:\n"+
		"    bedrud-debug-livekit-strip:\n"+
		"      stripPrefix:\n"+
		"        prefixes:\n"+
		"          - /livekit\n"+
		"  routers:\n"+
		"    bedrud-debug-livekit:\n"+
		"      rule: \"Host(`%s`) && PathPrefix(`/livekit`)\"\n"+
		"      entryPoints:\n"+
		"        - %s\n"+
		"      middlewares:\n"+
		"        - bedrud-debug-livekit-strip\n"+
		"      service: bedrud-debug-livekit\n"+
		"      priority: 100\n"+
		"%s"+
		"  services:\n"+
		"    bedrud-debug-livekit:\n"+
		"      loadBalancer:\n"+
		"        serversTransport: bedrud-debug-livekit-transport\n"+
		"        servers:\n"+
		"          - url: %q\n"+
		"  serversTransports:\n"+
		"    bedrud-debug-livekit-transport:\n"+
		"      forwardingTimeouts:\n"+
		"        dialTimeout: 30s\n"+
		"        responseHeaderTimeout: 0s\n",
		host, ep, traefikTLSBlock(cfg), cfg.LiveKitBackend())
}

// traefikWebxdcYAML routes webxdc-<instanceId>.publicHost → local API over the tunnel.
// The Go server matches Host and serves package assets + host webxdc.js.
func traefikWebxdcYAML(cfg *Config) string {
	host := cfg.URLs.PublicHost
	ep := cfg.EffectiveTraefikEntryPoint()
	// Traefik v3 HostRegexp uses Go regexp (not v2 {name:pat} placeholders).
	// Escape dots in publicHost for literal match.
	escapedHost := strings.ReplaceAll(host, `.`, `\.`)
	rule := fmt.Sprintf("HostRegexp(`^webxdc-[a-zA-Z0-9]+\\.%s$`)", escapedHost)

	// Optional file cert (typically LE DNS-01 or Cloudflare Origin for *.publicHost).
	// Added as extra SNI certs — does not replace ACME default for the apex host.
	tlsStore := ""
	cert := strings.TrimSpace(cfg.Provision.WebxdcWildcardCert)
	key := strings.TrimSpace(cfg.Provision.WebxdcWildcardKey)
	if cert != "" && key != "" {
		tlsStore = fmt.Sprintf("tls:\n"+
			"  certificates:\n"+
			"    - certFile: %q\n"+
			"      keyFile: %q\n",
			cert, key)
	}

	return fmt.Sprintf("# Generated by devcli — WebXDC hosts webxdc-<id>.%s → local API\n"+
		"# Requires DNS *.%s → this server. Wildcard TLS: provision.webxdcWildcardCert/Key\n"+
		"# (HTTP-01 ACME on bedrud-debug-web does NOT cover subdomains.)\n"+
		"%s"+
		"http:\n"+
		"  routers:\n"+
		"    bedrud-debug-webxdc:\n"+
		"      rule: %q\n"+
		"      entryPoints:\n"+
		"        - %s\n"+
		"      service: bedrud-debug-webxdc\n"+
		"      priority: 50\n"+
		"%s"+
		"  services:\n"+
		"    bedrud-debug-webxdc:\n"+
		"      loadBalancer:\n"+
		"        servers:\n"+
		"          - url: %q\n",
		host, host, tlsStore, rule, ep, traefikWebxdcTLSBlock(cfg), cfg.APIBackend())
}