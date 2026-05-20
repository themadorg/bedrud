package cli

import (
	"bedrud/config"
	"bedrud/internal/utils"
	"fmt"
	"net"

	"github.com/spf13/cobra"
)

const defaultEtcConfig = "/etc/bedrud/config.yaml"

func newCertCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cert",
		Short: "Manage TLS certificates",
	}
	cmd.AddCommand(newCertRenewCmd(), newCertInfoCmd())
	return cmd
}

func newCertRenewCmd() *cobra.Command {
	var algo string
	cmd := &cobra.Command{
		Use:   "renew",
		Short: "Renew the self-signed TLS certificate",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCertRenew(resolveConfigPath(defaultEtcConfig), algo)
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

func runCertRenew(configPath, algoStr string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	certFile := cfg.Server.CertFile
	keyFile := cfg.Server.KeyFile
	if certFile == "" {
		certFile = "/etc/bedrud/cert.pem"
	}
	if keyFile == "" {
		keyFile = "/etc/bedrud/key.pem"
	}

	var hosts []string
	if cfg.Server.Domain != "" {
		hosts = append(hosts, cfg.Server.Domain)
	}
	if ip := net.ParseIP(cfg.Server.Host); ip != nil && !ip.IsLoopback() && !ip.IsUnspecified() {
		hosts = append(hosts, cfg.Server.Host)
	}
	if outIP := utils.OutboundIP(); outIP != nil && !outIP.IsLoopback() && !outIP.IsUnspecified() {
		found := false
		for _, h := range hosts {
			if h == outIP.String() {
				found = true
				break
			}
		}
		if !found {
			hosts = append(hosts, outIP.String())
		}
	}
	hosts = append(hosts, "localhost", "127.0.0.1", "::1")

	if algoStr != "" {
		if err := utils.RenewSelfSignedCertWithAlgo(certFile, keyFile, utils.KeyAlgorithm(algoStr), hosts...); err != nil {
			return fmt.Errorf("renewing certificate: %w", err)
		}
	} else {
		if err := utils.RenewSelfSignedCert(certFile, keyFile, hosts...); err != nil {
			return fmt.Errorf("renewing certificate: %w", err)
		}
	}
	fmt.Println("Self-signed TLS certificate renewed successfully")
	fmt.Printf("  Cert: %s\n", certFile)
	fmt.Printf("  Key:  %s\n", keyFile)
	fmt.Printf("  SANs: %v\n", hosts)
	fmt.Printf("  Valid for %d days\n", utils.SelfSignedCertDays)
	return nil
}

func runCertInfo(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if !cfg.Server.EnableTLS || cfg.Server.DisableTLS {
		fmt.Println("TLS: not enabled")
		return nil
	}
	certFile := cfg.Server.CertFile
	keyFile := cfg.Server.KeyFile
	if certFile == "" {
		certFile = "/etc/bedrud/cert.pem"
	}
	if keyFile == "" {
		keyFile = "/etc/bedrud/key.pem"
	}
	info, err := utils.ValidateTLSCertPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("TLS certificate: %w", err)
	}
	fmt.Printf("Subject:        %s\n", info.Subject)
	fmt.Printf("Issuer:         %s\n", info.Issuer)
	fmt.Printf("Not Before:     %s\n", info.NotBefore.Format("2006-01-02 15:04:05 MST"))
	fmt.Printf("Not After:      %s\n", info.NotAfter.Format("2006-01-02 15:04:05 MST"))
	fmt.Printf("Days Remaining: %d\n", info.DaysRemaining)
	fmt.Printf("Status:         %s\n", info.Status)
	fmt.Printf("SANs:           %v\n", info.SANs)
	return nil
}
