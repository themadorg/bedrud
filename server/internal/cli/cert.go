package cli

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"bedrud/config"
	"bedrud/internal/clioutput"
	"bedrud/internal/utils"

	"github.com/spf13/cobra"
)

const defaultEtcConfig = "/etc/bedrud/config.yaml"

func newCertCmd() *cobra.Command {
	// Primary name is "certificate"; "cert" remains a short alias.
	cmd := &cobra.Command{
		Use:     "certificate",
		Aliases: []string{"cert"},
		Short:   "Manage TLS certificates",
		Long: `Manage self-signed TLS certificates used by Bedrud (and embedded LiveKit TURN when TLS is enabled).

When WebXDC is enabled, regenerated certificates include a wildcard SAN for
webxdc instance hosts (*.{webxdc.baseDomain}), which is required for
subdomain mini-apps.`,
	}
	cmd.AddCommand(
		newCertRegenerateCmd(),
		newCertRenewCmd(),
		newCertInfoCmd(),
	)
	return cmd
}

func newCertRegenerateCmd() *cobra.Command {
	var (
		algo  string
		force bool
	)
	cmd := &cobra.Command{
		Use:   "regenerate",
		Short: "Regenerate self-signed TLS cert with current config SANs (incl. WebXDC wildcard)",
		Long: `Regenerate the self-signed certificate and private key from the current
configuration.

SAN hosts always include:
  - server.domain (if set)
  - server.host (when it is a non-loopback IP)
  - outbound IP (when discoverable)
  - localhost, 127.0.0.1, ::1

When webxdc.enabled is true and webxdc.baseDomain is set (and not path-mode
only), also includes:
  - webxdc.baseDomain
  - *.{webxdc.baseDomain}   (required for webxdc-<id>.{baseDomain} hosts)

Unlike "renew", regenerate will create the cert pair if it does not exist yet,
and always rebuilds SANs from the live config (so enabling WebXDC later can
be fixed without a full reinstall).

After regenerating, restart the bedrud (and livekit) service so processes
reload the new files:

  sudo systemctl restart livekit bedrud`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCertRegenerate(resolveConfigPath(defaultEtcConfig), algo, force)
		},
	}
	cmd.Flags().StringVar(&algo, "algo", "", "Key algorithm: ed25519, ecdsa256, rsa2048, rsa4096 (default: config certAlgorithm, existing cert, or ed25519)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing cert even when ACME/useACME is enabled (self-signed only; does not talk to Let's Encrypt)")
	return cmd
}

func newCertRenewCmd() *cobra.Command {
	var algo string
	cmd := &cobra.Command{
		Use:   "renew",
		Short: "Renew the self-signed TLS certificate (same SANs as regenerate)",
		Long: `Force-renew the self-signed TLS certificate. Equivalent to regenerate for
existing cert files: rebuilds SANs from config (including WebXDC wildcard
when enabled) and writes a new cert with ~5 year validity.

Prefer "certificate regenerate" when you need to create a missing cert or
pick up WebXDC SANs after enabling mini-apps.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCertRegenerate(resolveConfigPath(defaultEtcConfig), algo, true)
		},
	}
	cmd.Flags().StringVar(&algo, "algo", "", "Key algorithm: ed25519, ecdsa256, rsa2048, rsa4096 (default: detect existing)")
	return cmd
}

func newCertInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show TLS certificate status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCertInfo(resolveConfigPath(defaultEtcConfig))
		},
	}
}

func runCertRegenerate(configPath, algoStr string, force bool) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	certFile, keyFile := resolveCertPaths(cfg)

	if cfg.Server.UseACME && !force {
		return fmt.Errorf(
			"server.useACME is enabled — this command only manages self-signed certificates.\n" +
				"ACME certs are renewed automatically by the server.\n" +
				"To force a self-signed cert anyway: bedrud certificate regenerate --force",
		)
	}

	hosts, webxdcWildcard := buildCertSANHosts(cfg)
	algo, algoSource, err := resolveCertAlgorithm(algoStr, certFile, cfg)
	if err != nil {
		return err
	}

	exists := fileExists(certFile) && fileExists(keyFile)
	if exists {
		if err := utils.RenewSelfSignedCertWithAlgo(certFile, keyFile, algo, hosts...); err != nil {
			return fmt.Errorf("regenerating certificate: %w", err)
		}
	} else {
		if err := utils.GenerateSelfSignedCertWithAlgo(certFile, keyFile, algo, hosts...); err != nil {
			return fmt.Errorf("generating certificate: %w", err)
		}
	}

	// Best-effort ownership for installed layouts
	_ = chownIfPossible(certFile, "bedrud:bedrud")
	_ = chownIfPossible(keyFile, "bedrud:bedrud")

	action := "regenerated"
	if !exists {
		action = "generated"
	}
	msg := fmt.Sprintf("Self-signed TLS certificate %s successfully", action)
	if webxdcWildcard != "" {
		msg += " (includes WebXDC wildcard SAN " + webxdcWildcard + ")"
	}

	data := map[string]any{
		"certFile":        certFile,
		"keyFile":         keyFile,
		"sans":            hosts,
		"validDays":       utils.SelfSignedCertDays,
		"algorithm":       string(algo),
		"algorithmSource": algoSource,
		"created":         !exists,
		"webxdcWildcard":  webxdcWildcard,
	}
	if !clioutput.JSON() {
		fmt.Printf("Certificate:  %s\n", certFile)
		fmt.Printf("Private key:  %s\n", keyFile)
		fmt.Printf("Algorithm:    %s (%s)\n", algo, algoSource)
		fmt.Printf("Valid days:   %d\n", utils.SelfSignedCertDays)
		fmt.Printf("SANs:         %s\n", strings.Join(hosts, ", "))
		if webxdcWildcard != "" {
			fmt.Printf("WebXDC:       wildcard %s (webxdc-*.%s)\n",
				webxdcWildcard, strings.TrimPrefix(webxdcWildcard, "*."))
		}
		fmt.Println()
		fmt.Println("Restart services to load the new certificate:")
		fmt.Println("  sudo systemctl restart livekit bedrud")
	}
	return clioutput.Success(msg, data)
}

func runCertInfo(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if !cfg.Server.EnableTLS || cfg.Server.DisableTLS {
		return clioutput.Success("TLS: not enabled", map[string]any{"enabled": false})
	}
	certFile, keyFile := resolveCertPaths(cfg)
	info, err := utils.ValidateTLSCertPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("TLS certificate: %w", err)
	}

	expected, webxdcWildcard := buildCertSANHosts(cfg)
	missing := missingSANs(info.SANs, expected)

	data := map[string]any{
		"enabled":        true,
		"subject":        info.Subject,
		"issuer":         info.Issuer,
		"notBefore":      info.NotBefore.Format(time.RFC3339),
		"notAfter":       info.NotAfter.Format(time.RFC3339),
		"daysRemaining":  info.DaysRemaining,
		"status":         info.Status,
		"sans":           info.SANs,
		"expectedSans":   expected,
		"missingSans":    missing,
		"webxdcWildcard": webxdcWildcard,
		"certFile":       certFile,
		"keyFile":        keyFile,
	}
	if !clioutput.JSON() {
		fmt.Printf("Subject:        %s\n", info.Subject)
		fmt.Printf("Issuer:         %s\n", info.Issuer)
		fmt.Printf("Not Before:     %s\n", info.NotBefore.Format("2006-01-02 15:04:05 MST"))
		fmt.Printf("Not After:      %s\n", info.NotAfter.Format("2006-01-02 15:04:05 MST"))
		fmt.Printf("Days Remaining: %d\n", info.DaysRemaining)
		fmt.Printf("Status:         %s\n", info.Status)
		fmt.Printf("SANs:           %v\n", info.SANs)
		if webxdcWildcard != "" {
			fmt.Printf("WebXDC expect:  %s\n", webxdcWildcard)
		}
		if len(missing) > 0 {
			fmt.Printf("Missing SANs:   %v\n", missing)
			fmt.Println("Hint: run  bedrud certificate regenerate  to rebuild SANs from config")
		}
	}
	return clioutput.Success("", data)
}

func resolveCertPaths(cfg *config.Config) (certFile, keyFile string) {
	certFile = cfg.Server.CertFile
	keyFile = cfg.Server.KeyFile
	if certFile == "" {
		certFile = "/etc/bedrud/cert.pem"
	}
	if keyFile == "" {
		keyFile = "/etc/bedrud/key.pem"
	}
	return certFile, keyFile
}

// buildCertSANHosts derives DNS/IP SANs for a self-signed cert from config.
// When WebXDC is active (enabled + valid), adds baseDomain and *.{baseDomain}.
// Returns the list and the wildcard string (empty if not applicable).
func buildCertSANHosts(cfg *config.Config) (hosts []string, webxdcWildcard string) {
	seen := make(map[string]struct{})
	add := func(h string) {
		h = strings.TrimSpace(h)
		if h == "" {
			return
		}
		// Normalize wildcard form
		if strings.HasPrefix(h, "*.") {
			// keep as-is
		}
		key := strings.ToLower(h)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		hosts = append(hosts, h)
	}

	if cfg.Server.Domain != "" {
		add(cfg.Server.Domain)
	}
	if ip := net.ParseIP(cfg.Server.Host); ip != nil && !ip.IsLoopback() && !ip.IsUnspecified() {
		add(cfg.Server.Host)
	} else if host := strings.TrimSpace(cfg.Server.Host); host != "" && net.ParseIP(host) == nil {
		// Host may be a hostname (not IP)
		add(host)
	}
	if outIP := utils.OutboundIP(); outIP != nil && !outIP.IsLoopback() && !outIP.IsUnspecified() {
		add(outIP.String())
	}
	add("localhost")
	add("127.0.0.1")
	add("::1")

	// WebXDC: instance hosts are webxdc-<id>.{baseDomain} → need *.{baseDomain}
	if cfg.Webxdc.Active(cfg.Server.Domain) && !cfg.Webxdc.UsePathMode() {
		base := strings.TrimSpace(cfg.Webxdc.BaseDomain)
		// Strip accidental wildcard prefix if user put "*.wx.example.com" as base
		base = strings.TrimPrefix(base, "*.")
		if base != "" {
			add(base)
			webxdcWildcard = "*." + base
			add(webxdcWildcard)
		}
	}

	return hosts, webxdcWildcard
}

func resolveCertAlgorithm(algoStr, certFile string, cfg *config.Config) (utils.KeyAlgorithm, string, error) {
	if algoStr != "" {
		algo := utils.KeyAlgorithm(algoStr)
		if err := validateKeyAlgorithm(algo); err != nil {
			return "", "", err
		}
		return algo, "flag", nil
	}
	if a := strings.TrimSpace(cfg.Server.CertAlgorithm); a != "" {
		algo := utils.KeyAlgorithm(a)
		if err := validateKeyAlgorithm(algo); err != nil {
			return "", "", fmt.Errorf("server.certAlgorithm: %w", err)
		}
		return algo, "config", nil
	}
	// Prefer existing cert's algorithm when present
	if fileExists(certFile) {
		algo, err := utils.DetectCertAlgorithm(certFile)
		if err == nil {
			return algo, "existing", nil
		}
	}
	return utils.KeyEd25519, "default", nil
}

func validateKeyAlgorithm(algo utils.KeyAlgorithm) error {
	switch algo {
	case utils.KeyEd25519, utils.KeyECDSA256, utils.KeyRSA2048, utils.KeyRSA4096:
		return nil
	default:
		return fmt.Errorf("unsupported key algorithm %q (want ed25519, ecdsa256, rsa2048, rsa4096)", algo)
	}
}

func missingSANs(have, want []string) []string {
	set := make(map[string]struct{}, len(have))
	for _, h := range have {
		set[strings.ToLower(h)] = struct{}{}
	}
	var missing []string
	for _, w := range want {
		if _, ok := set[strings.ToLower(w)]; !ok {
			missing = append(missing, w)
		}
	}
	return missing
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func chownIfPossible(path, userGroup string) error {
	if _, err := exec.LookPath("chown"); err != nil {
		return err
	}
	return exec.Command("chown", userGroup, path).Run()
}
