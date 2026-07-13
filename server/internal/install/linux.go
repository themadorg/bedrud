package install

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"bedrud/internal/livekit"
	"bedrud/internal/utils"

	"gopkg.in/yaml.v3"
)

const (
	loopbackIPv4 = "127.0.0.1"
	loopbackIPv6 = "::1"
)

type installConfigYAML struct {
	Server struct {
		Port           string   `yaml:"port"`
		Host           string   `yaml:"host"`
		EnableTLS      bool     `yaml:"enableTLS"`
		CertFile       string   `yaml:"certFile"`
		KeyFile        string   `yaml:"keyFile"`
		Domain         string   `yaml:"domain"`
		Email          string   `yaml:"email"`
		UseACME        bool     `yaml:"useACME"`
		BehindProxy    bool     `yaml:"behindProxy,omitempty"`
		TrustedProxies []string `yaml:"trustedProxies,omitempty"`
		ProxyHeader    string   `yaml:"proxyHeader,omitempty"`
	} `yaml:"server"`
	Database struct {
		Type string `yaml:"type"`
		Path string `yaml:"path"`
	} `yaml:"database"`
	LiveKit struct {
		Host          string `yaml:"host"`
		InternalHost  string `yaml:"internalHost"`
		APIKey        string `yaml:"apiKey"`
		APISecret     string `yaml:"apiSecret"`
		ConfigPath    string `yaml:"configPath,omitempty"`
		SkipTLSVerify bool   `yaml:"skipTLSVerify"`
		External      bool   `yaml:"external"`
	} `yaml:"livekit"`
	Auth struct {
		JWTSecret     string `yaml:"jwtSecret"`
		SessionSecret string `yaml:"sessionSecret"`
		TokenDuration int    `yaml:"tokenDuration"`
	} `yaml:"auth"`
	Logger struct {
		Level      string `yaml:"level"`
		OutputPath string `yaml:"outputPath"`
	} `yaml:"logger"`
	CORS struct {
		AllowedOrigins   string `yaml:"allowedOrigins"`
		AllowCredentials bool   `yaml:"allowCredentials"`
	} `yaml:"cors"`
	Webxdc struct {
		Enabled      bool   `yaml:"enabled"`
		BaseDomain   string `yaml:"baseDomain,omitempty"`
		UploadPolicy string `yaml:"uploadPolicy,omitempty"`
	} `yaml:"webxdc"`
}

func LinuxInstall(cfg *InstallConfig) error {
	if cfg.Fresh {
		fmt.Println("➜ Fresh install: removing previous deployment...")
		if err := LinuxUninstall(); err != nil {
			return fmt.Errorf("failed to uninstall previous deployment: %w", err)
		}
		fmt.Println()
	}
	if runtime.GOOS != "linux" {
		return fmt.Errorf("only linux is supported")
	}

	isTerm := true
	if stat, err := os.Stdin.Stat(); err != nil {
		isTerm = false
	} else if (stat.Mode() & os.ModeCharDevice) == 0 {
		isTerm = false
	}

	if isTerm {
		promptConfig(os.Stdin, os.Stdout, cfg)
	}

	cfg.SetDefaults()
	if cfg.OverrideIP == "" || cfg.OverrideIP == "0.0.0.0" {
		cfg.OverrideIP = utils.OutboundIP().String()
	}

	fmt.Println("➜ Preparing Bedrud installation...")
	fmt.Println("➜ Using IP:", cfg.OverrideIP)
	if cfg.Domain != "" {
		fmt.Println("➜ Using Domain:", cfg.Domain)
	}

	if err := os.MkdirAll(etcDir, 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", etcDir, err)
	}
	if err := os.MkdirAll(varLibDir, 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", varLibDir, err)
	}
	if err := os.MkdirAll(varLibDir+"/certs", 0o750); err != nil {
		return fmt.Errorf("failed to create %s/certs: %w", varLibDir, err)
	}
	if err := os.MkdirAll(varLogDir, 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", varLogDir, err)
	}

	if err := createBedrudUser(); err != nil {
		fmt.Printf("⚠ Warning: could not create 'bedrud' user: %v\n", err)
	}

	// 1. Stop existing services and remove standalone binary to avoid ETXTBSY
	fmt.Println("➜ Stopping existing services...")
	stopAllInitSystems([]string{"bedrud", "livekit"})
	_ = os.Remove(binaryLocalPath)

	// Chown directories to bedrud:bedrud
	for _, dir := range []string{etcDir, varLibDir, varLogDir} {
		if err := runChownR("bedrud:bedrud", dir); err != nil {
			return err
		}
	}

	// 2. Install Bedrud Binary
	// Prefer package path (/usr/bin) when dpkg/rpm already placed the binary there
	// so apt/dnf upgrades keep working and we don't maintain a second copy.
	bedrudBin := binaryLocalPath
	if _, err := os.Stat(binaryPackagePath); err == nil {
		bedrudBin = binaryPackagePath
		fmt.Println("➜ Using package binary at", bedrudBin)
	} else {
		if err := installSelfBinary(bedrudBin); err != nil {
			return err
		}
		fmt.Println("➜ Installed binary to", bedrudBin)
	}

	apiKey := generateSecret(32)
	apiSecret := generateSecret(48)
	jwtSecret := generateSecret(32)
	sessionSecret := generateSecret(32)

	protocol := "http"
	if cfg.EnableTLS {
		protocol = "https"
	}

	certFile := cfg.CertPath
	if certFile == "" {
		certFile = etcDir + "/cert.pem"
	}
	keyFile := cfg.KeyPath
	if keyFile == "" {
		keyFile = etcDir + "/key.pem"
	}

	hostForLK := cfg.OverrideIP
	if cfg.Domain != "" {
		hostForLK = cfg.Domain
	}

	livekitPublicHost := fmt.Sprintf("%s://%s:%s/livekit", protocol, hostForLK, cfg.Port)
	if cfg.Port == "443" {
		livekitPublicHost = fmt.Sprintf("https://%s/livekit", hostForLK)
	}

	corsOrigins := "*"
	corsCredentials := false
	if cfg.Domain != "" {
		corsOrigins = fmt.Sprintf("%s://%s", protocol, cfg.Domain)
		corsCredentials = true
	}

	isExternalLK := cfg.ExternalLKURL != ""
	hasSeparateLKDomain := cfg.LKDomain != "" && !isExternalLK

	if isExternalLK {
		livekitPublicHost = cfg.ExternalLKURL
	} else if hasSeparateLKDomain {
		livekitPublicHost = fmt.Sprintf("https://%s", cfg.LKDomain)
	}

	lkNodeIP := cfg.OverrideIP
	if cfg.LKIP != "" {
		lkNodeIP = cfg.LKIP
	}

	// Build config.yaml
	var configYAML installConfigYAML
	configYAML.Server.Port = cfg.Port
	configYAML.Server.Host = cfg.OverrideIP
	configYAML.Server.EnableTLS = cfg.EnableTLS
	configYAML.Server.CertFile = certFile
	configYAML.Server.KeyFile = keyFile
	configYAML.Server.Domain = cfg.Domain
	configYAML.Server.Email = cfg.Email
	configYAML.Server.UseACME = (cfg.Email != "" && !cfg.DisableTLS && cfg.Domain != "")

	if cfg.BehindProxy || (!cfg.EnableTLS && cfg.Domain != "") {
		configYAML.Server.BehindProxy = true
		configYAML.Server.TrustedProxies = []string{loopbackIPv4, loopbackIPv6}
		configYAML.Server.ProxyHeader = "X-Forwarded-For"
		if !cfg.BehindProxy {
			fmt.Println("⚠ Warning: behindProxy enabled because domain is set without TLS. Defaulting trustedProxies to localhost.")
		}
	}

	configYAML.Database.Type = "sqlite"
	configYAML.Database.Path = varLibDir + "/bedrud.db"

	configYAML.LiveKit.Host = livekitPublicHost
	configYAML.LiveKit.InternalHost = fmt.Sprintf("http://%s:%s", loopbackIPv4, cfg.LKPort)
	if isExternalLK {
		configYAML.LiveKit.InternalHost = livekitPublicHost
	}
	configYAML.LiveKit.APIKey = apiKey
	configYAML.LiveKit.APISecret = apiSecret
	if !isExternalLK {
		configYAML.LiveKit.ConfigPath = etcLivekitPath
	}
	configYAML.LiveKit.SkipTLSVerify = true
	configYAML.LiveKit.External = isExternalLK || hasSeparateLKDomain

	configYAML.Auth.JWTSecret = jwtSecret
	configYAML.Auth.SessionSecret = sessionSecret
	configYAML.Auth.TokenDuration = 24

	configYAML.Logger.Level = "debug"
	configYAML.Logger.OutputPath = varLogDir + "/bedrud.log"

	configYAML.CORS.AllowedOrigins = corsOrigins
	configYAML.CORS.AllowCredentials = corsCredentials

	if cfg.EnableWebxdc && cfg.Domain != "" && cfg.WebxdcBaseDomain != "" && cfg.WebxdcDNSAck {
		configYAML.Webxdc.Enabled = true
		configYAML.Webxdc.BaseDomain = cfg.WebxdcBaseDomain
		configYAML.Webxdc.UploadPolicy = "owner_mod"
	} else {
		configYAML.Webxdc.Enabled = false
	}

	configData, err := yaml.Marshal(&configYAML)
	if err != nil {
		return fmt.Errorf("failed to marshal config.yaml: %w", err)
	}
	if err := os.WriteFile(etcConfigPath, configData, 0o600); err != nil {
		return fmt.Errorf("failed to write config.yaml: %w", err)
	}
	_ = runChown("bedrud:bedrud", etcConfigPath)

	// 3. Create LiveKit Config
	if !isExternalLK {
		turnDomain := hostForLK
		if hasSeparateLKDomain {
			turnDomain = cfg.LKDomain
		}

		lkBindAddr := loopbackIPv4
		if hasSeparateLKDomain {
			lkBindAddr = "0.0.0.0"
		}

		var lkYAML livekit.ConfigYAML
		lkPort, err := strconv.Atoi(cfg.LKPort)
		if err != nil || lkPort < 1 || lkPort > 65535 {
			return fmt.Errorf("invalid livekit port %q: must be 1-65535", cfg.LKPort)
		}
		lkYAML.Port = lkPort
		lkYAML.BindAddresses = []string{lkBindAddr}
		lkYAML.Keys = map[string]string{apiKey: apiSecret}
		lkTcpPort, err := strconv.Atoi(cfg.LKTcpPort)
		if err != nil || lkTcpPort < 1 || lkTcpPort > 65535 {
			return fmt.Errorf("invalid livekit TCP port %q: must be 1-65535", cfg.LKTcpPort)
		}
		lkYAML.RTC.TCPPort = lkTcpPort
		lkYAML.RTC.UseExternalIP = false
		lkYAML.RTC.NodeIP = lkNodeIP

		if cfg.LKUDPPortRangeStart != "" && cfg.LKUDPPortRangeEnd != "" {
			prStart, err := strconv.Atoi(cfg.LKUDPPortRangeStart)
			if err != nil || prStart < 1 || prStart > 65535 {
				return fmt.Errorf("invalid livekit UDP port range start %q: must be 1-65535", cfg.LKUDPPortRangeStart)
			}
			prEnd, err := strconv.Atoi(cfg.LKUDPPortRangeEnd)
			if err != nil || prEnd < 1 || prEnd > 65535 {
				return fmt.Errorf("invalid livekit UDP port range end %q: must be 1-65535", cfg.LKUDPPortRangeEnd)
			}
			lkYAML.RTC.PortRangeStart = prStart
			lkYAML.RTC.PortRangeEnd = prEnd
		} else {
			lkUdpPort, err := strconv.Atoi(cfg.LKUdpPort)
			if err != nil || lkUdpPort < 1 || lkUdpPort > 65535 {
				return fmt.Errorf("invalid livekit UDP port %q: must be 1-65535", cfg.LKUdpPort)
			}
			lkYAML.RTC.UDPPort = lkUdpPort
		}
		lkYAML.TURN.Enabled = true
		lkYAML.TURN.Domain = turnDomain
		lkYAML.TURN.UDPPort = 3478
		if cfg.EnableTLS {
			lkYAML.TURN.TLSPort = 5349
			lkYAML.TURN.CertFile = certFile
			lkYAML.TURN.KeyFile = keyFile
		}
		lkYAML.Logging.JSON = true
		lkYAML.Logging.Level = "debug"

		lkData, err := yaml.Marshal(&lkYAML)
		if err != nil {
			return fmt.Errorf("failed to marshal livekit.yaml: %w", err)
		}
		if err := os.WriteFile(etcLivekitPath, lkData, 0o600); err != nil {
			return fmt.Errorf("failed to write livekit.yaml: %w", err)
		}
		_ = runChown("bedrud:bedrud", etcLivekitPath)
	}

	if cfg.EnableTLS && cfg.CertPath == "" && cfg.KeyPath == "" {
		cp, kp := etcDir+"/cert.pem", etcDir+"/key.pem"
		if _, err := os.Stat(cp); os.IsNotExist(err) {
			hosts := []string{"localhost", loopbackIPv4, loopbackIPv6}
			if cfg.OverrideIP != "" && cfg.OverrideIP != loopbackIPv4 && cfg.OverrideIP != "localhost" {
				hosts = append(hosts, cfg.OverrideIP)
			}
			if cfg.Domain != "" {
				hosts = append(hosts, cfg.Domain)
			}
			algo := utils.KeyAlgorithm(cfg.CertAlgorithm)
			if algo == "" {
				algo = utils.KeyEd25519
			}
			if err := utils.GenerateSelfSignedCertWithAlgo(cp, kp, algo, hosts...); err != nil {
				return fmt.Errorf("failed to generate self-signed cert: %w", err)
			}
		}
	}

	// 4. Detect init system and install services
	if err := refreshServices(bedrudBin, isExternalLK); err != nil {
		return err
	}

	// Record installed version when known (CLI may set InstallConfig.Version).
	if cfg.Version != "" {
		if err := writeInstalledVersion(cfg.Version); err != nil {
			fmt.Printf("⚠ Warning: could not write version file: %v\n", err)
		}
	}

	fmt.Println("\n✓ Installation complete!")
	fmt.Println("--------------------------------------------------")
	fmt.Println("Sensitive credentials were generated and written to configuration files.")
	fmt.Println("For security, secrets are not displayed in console output.")
	fmt.Println("--------------------------------------------------")

	accessURL := fmt.Sprintf("%s://%s:%s", protocol, cfg.OverrideIP, cfg.Port)
	if cfg.Port == "443" || cfg.Port == "80" {
		accessURL = fmt.Sprintf("%s://%s", protocol, cfg.OverrideIP)
	}
	fmt.Println("  Access URL: ", accessURL)
	if cfg.Domain != "" {
		fmt.Println("  Domain URL: ", fmt.Sprintf("%s://%s", protocol, cfg.Domain))
	}
	if cfg.EnableWebxdc && configYAML.Webxdc.Enabled {
		fmt.Println("  WebXDC:          enabled (experimental)")
		fmt.Println("  WebXDC base:     ", cfg.WebxdcBaseDomain)
		fmt.Println("  WebXDC DNS:      *."+cfg.WebxdcBaseDomain, "→ MUST point to this server / reverse proxy")
		fmt.Println("  WebXDC TLS:      cert MUST cover *." + cfg.WebxdcBaseDomain)
	} else {
		fmt.Println("  WebXDC:          disabled")
	}
	fmt.Println("  LiveKit Host:", livekitPublicHost)
	if !isExternalLK {
		displayNodeIP := lkNodeIP
		if displayNodeIP == "" {
			displayNodeIP = cfg.OverrideIP
		}
		if displayNodeIP != cfg.OverrideIP {
			fmt.Println("  LiveKit NodeIP:", displayNodeIP, "(different from server — set via --lk-ip)")
		}
		if cfg.LKUDPPortRangeStart != "" && cfg.LKUDPPortRangeEnd != "" {
			fmt.Println("  LiveKit UDP range:", cfg.LKUDPPortRangeStart+"-"+cfg.LKUDPPortRangeEnd)
		}
	}
	return nil
}

func promptConfig(r io.Reader, w io.Writer, cfg *InstallConfig) {
	fmt.Fprintln(w, "\n--- Bedrud Configuration ---")

	if cfg.OverrideIP == "" {
		detectedIP := getLocalIP()
		fmt.Fprintf(w, "➜ Detect IP address [%s]: ", detectedIP)
		var inputIP string
		_, _ = fmt.Fscanln(r, &inputIP)

		if inputIP != "" {
			cfg.OverrideIP = inputIP
		} else {
			cfg.OverrideIP = detectedIP
		}
	}

	if cfg.Domain == "" {
		fmt.Fprintf(w, "➜ Enter Domain (leave empty for IP-only): ")
		_, _ = fmt.Fscanln(r, &cfg.Domain)
	}

	if cfg.Domain != "" {
		if cfg.Email == "" {
			fmt.Fprintf(w, "➜ Enter Email for Let's Encrypt: ")
			_, _ = fmt.Fscanln(r, &cfg.Email)
		}
		if cfg.Email != "" && !cfg.DisableTLS {
			cfg.EnableTLS = true
		}
	}

	if cfg.CertPath != "" && cfg.KeyPath != "" && !cfg.DisableTLS {
		cfg.EnableTLS = true
	}

	if cfg.SelfSigned && !cfg.DisableTLS {
		cfg.EnableTLS = true
	}

	if !cfg.EnableTLS && cfg.Email == "" && !cfg.DisableTLS && !cfg.SelfSigned {
		fmt.Fprintf(w, "➜ Enable Self-Signed TLS? [Y/n]: ")
		var secure string
		_, _ = fmt.Fscanln(r, &secure)
		if secure == "" || secure == "y" || secure == "Y" {
			cfg.EnableTLS = true
		}
	}

	// Experimental WebXDC — domain required (not IP-only)
	if cfg.Domain == "" {
		fmt.Fprintln(w, "  WebXDC: skipped (requires a domain name; not available on IP-only installs)")
		cfg.EnableWebxdc = false
	} else if !cfg.EnableWebxdc {
		fmt.Fprintf(w, "➜ Enable experimental WebXDC mini-apps in meetings? [y/N]:\n")
		fmt.Fprintf(w, "  (Requires DNS *.<baseDomain> → this server/proxy, and a wildcard TLS cert)\n  ")
		var yn string
		_, _ = fmt.Fscanln(r, &yn)
		if yn == "y" || yn == "Y" || yn == "yes" {
			cfg.EnableWebxdc = true
		}
	}
	if cfg.EnableWebxdc && cfg.Domain != "" {
		if cfg.WebxdcBaseDomain == "" {
			cfg.WebxdcBaseDomain = "wx." + cfg.Domain
			fmt.Fprintf(w, "➜ WebXDC base domain [%s]: ", cfg.WebxdcBaseDomain)
			var bd string
			_, _ = fmt.Fscanln(r, &bd)
			if strings.TrimSpace(bd) != "" {
				cfg.WebxdcBaseDomain = strings.TrimSpace(bd)
			}
		}
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "╔══════════════════════════════════════════════════════════════════╗")
		fmt.Fprintln(w, "║  WebXDC — REQUIRED DNS (do this now)                             ║")
		fmt.Fprintln(w, "╠══════════════════════════════════════════════════════════════════╣")
		fmt.Fprintf(w, "║  Add wildcard DNS:  *.%s\n", cfg.WebxdcBaseDomain)
		fmt.Fprintln(w, "║  → same IP / target as your main Bedrud domain                   ║")
		fmt.Fprintf(w, "║  TLS must cover:    *.%s\n", cfg.WebxdcBaseDomain)
		fmt.Fprintln(w, "║  Without this, mini-apps will not load.                          ║")
		fmt.Fprintln(w, "╚══════════════════════════════════════════════════════════════════╝")
		if !cfg.WebxdcDNSAck {
			fmt.Fprintf(w, "➜ Confirm: I will (or already did) point *.%s at this server/proxy [y/N]: ", cfg.WebxdcBaseDomain)
			var ack string
			_, _ = fmt.Fscanln(r, &ack)
			if ack != "y" && ack != "Y" && ack != "yes" {
				fmt.Fprintln(w, "  WebXDC left disabled (DNS not acknowledged).")
				cfg.EnableWebxdc = false
			} else {
				cfg.WebxdcDNSAck = true
			}
		}
	}
}

func createBedrudUser() error {
	// Check if user exists
	if _, err := exec.Command("getent", "passwd", "bedrud").Output(); err == nil {
		return nil // Already exists
	}

	// Create system user: no-login, home at /var/lib/bedrud
	cmd := exec.Command("useradd", "-r", "-s", "/usr/sbin/nologin", "-d", varLibDir, "bedrud")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create bedrud user: %s %w", string(out), err)
	}
	return nil
}

func getLocalIP() string {
	// 1. Try to get public IP first
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Get("https://ifconfig.me")
	if err == nil {
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			ip := strings.TrimSpace(string(body))
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	// 2. Fallback to local interface
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return loopbackIPv4
}

func LinuxUninstall() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("only linux is supported")
	}

	fmt.Println("\n--- Bedrud Uninstallation ---")
	fmt.Println("➜ Stopping and disabling services...")

	svcs := []string{"bedrud", "livekit"}

	stopAllInitSystems(svcs)
	disableAllInitSystems(svcs)

	fmt.Println("➜ Removing service files...")
	cleanupAllServiceFiles()

	// Remove binaries
	fmt.Println("➜ Removing binaries...")
	var errs []error
	if err := os.Remove(binaryLocalPath); err != nil && !os.IsNotExist(err) {
		errs = append(errs, fmt.Errorf("failed to remove %s: %w", binaryLocalPath, err))
	}
	if err := os.Remove("/tmp/bedrud"); err != nil && !os.IsNotExist(err) {
		errs = append(errs, fmt.Errorf("failed to remove /tmp/bedrud: %w", err))
	}
	if err := os.Remove("/tmp/bedrud-livekit-server"); err != nil && !os.IsNotExist(err) {
		errs = append(errs, fmt.Errorf("failed to remove /tmp/bedrud-livekit-server: %w", err))
	}

	// Remove PID files
	for _, svc := range svcs {
		pidFile := fmt.Sprintf("/var/run/%s.pid", svc)
		if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("failed to remove %s: %w", pidFile, err))
		}
	}

	// Remove config and data
	fmt.Println("➜ Removing configurations and data...")
	if err := os.RemoveAll(etcDir); err != nil {
		errs = append(errs, fmt.Errorf("failed to remove %s: %w", etcDir, err))
	}
	if err := os.RemoveAll(varLibDir); err != nil {
		errs = append(errs, fmt.Errorf("failed to remove %s: %w", varLibDir, err))
	}
	if err := os.RemoveAll(varLogDir); err != nil {
		errs = append(errs, fmt.Errorf("failed to remove %s: %w", varLogDir, err))
	}

	if _, err := exec.Command("getent", "passwd", "bedrud").Output(); err == nil {
		fmt.Println("➜ Removing bedrud system user...")
		if out, err := exec.Command("userdel", "-r", "bedrud").CombinedOutput(); err != nil {
			fmt.Printf("⚠ Warning: failed to remove bedrud user: %s %v\n", string(out), err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	fmt.Println("✓ Uninstallation complete!")
	return nil
}
