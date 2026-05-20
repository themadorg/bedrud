package main

import "bedrud/internal/cli"

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "--version", "-v":
			fmt.Println("bedrud " + version)
			return
		case "--livekit":
			lkCmd := flag.NewFlagSet("livekit", flag.ExitOnError)
			configPath := lkCmd.String("config", "", "Path to LiveKit config file")
			_ = lkCmd.Parse(os.Args[i+1:])
			if err := livekit.RunLiveKit(*configPath); err != nil {
				fmt.Fprintf(os.Stderr, "LiveKit error: %v\n", err)
				os.Exit(1)
			}
			return
		case "--run":
			runCmd := flag.NewFlagSet("run", flag.ExitOnError)
			configPath := runCmd.String("config", "", "Path to Bedrud config file")
			skipMigrate := runCmd.Bool("skip-migrate", false, "Skip database migrations on startup")
			_ = runCmd.Parse(os.Args[i+1:])
			path := *configPath
			if path == "" {
				path = os.Getenv("CONFIG_PATH")
				if path == "" {
					path = "config.yaml"
				}
			}
			if *skipMigrate {
				os.Setenv("BEDRUD_SKIP_MIGRATE", "1")
			}
			if err := server.Run(path, version); err != nil {
				fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	command := os.Args[1]
	switch command {
	case "server", "run":
		serverCmd := flag.NewFlagSet("server", flag.ExitOnError)
		configPath := serverCmd.String("config", "", "Path to config file")
		skipMigrate := serverCmd.Bool("skip-migrate", false, "Skip database migrations on startup")
		_ = serverCmd.Parse(os.Args[2:])

		path := *configPath
		if path == "" {
			path = os.Getenv("CONFIG_PATH")
			if path == "" {
				path = "config.yaml"
			}
		}

		if *skipMigrate {
			os.Setenv("BEDRUD_SKIP_MIGRATE", "1")
		}

		if err := server.Run(path, version); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}

	case "install":
		installCmd := flag.NewFlagSet("install", flag.ExitOnError)
		enableTLS := installCmd.Bool("tls", false, "Enable HTTPS with self-signed certificate (same as --self-signed)")
		selfSigned := installCmd.Bool("self-signed", false, "Generate and use a self-signed TLS certificate")
		noTLS := installCmd.Bool("no-tls", false, "Disable TLS entirely (plain HTTP, overrides --tls/--self-signed)")
		ipOverride := installCmd.String("ip", "", "Override detected IP address")
		domainFlag := installCmd.String("domain", "", "Domain for Let's Encrypt")
		emailFlag := installCmd.String("email", "", "Email for Let's Encrypt")
		portFlag := installCmd.String("port", "", "Override default port (default 443 for TLS, 8090 for HTTP)")
		certFlag := installCmd.String("cert", "", "Path to existing certificate file")
		keyFlag := installCmd.String("key", "", "Path to existing private key file")
		lkPortFlag := installCmd.String("lk-port", "", "Override LiveKit API port (default 7880)")
		lkTcpPortFlag := installCmd.String("lk-tcp-port", "", "Override LiveKit RTC TCP port (default 7881)")
		lkUdpPortFlag := installCmd.String("lk-udp-port", "", "Override LiveKit RTC UDP port (default 7882)")
		freshFlag := installCmd.Bool("fresh", false, "Remove existing installation before installing")
		behindProxyFlag := installCmd.Bool("behind-proxy", false, "Running behind a CDN/reverse-proxy (Cloudflare, nginx, etc.)")
		externalLKFlag := installCmd.String("external-livekit", "", "URL of a fully external LiveKit server (different machine, e.g. https://lk.example.com)")
		lkDomainFlag := installCmd.String("livekit-domain", "", "Separate domain for the local LiveKit server (e.g. lk.example.com, bypasses CDN)")
		lkIPFlag := installCmd.String("lk-ip", "", "Separate IP for LiveKit NodeIP (when server behind CDN, LiveKit needs direct-reachable IP)")
		lkUDPPortRangeFlag := installCmd.String("lk-udp-range", "", "UDP port range for WebRTC media, e.g. 50000-60000 (default 50000-60000)")
		certAlgoFlag := installCmd.String("cert-algorithm", "", "Key algorithm for self-signed cert: ed25519 (default), ecdsa256, rsa2048, rsa4096")
		_ = installCmd.Parse(os.Args[2:])

		lkUDPPortRangeStart := ""
		lkUDPPortRangeEnd := ""
		if *lkUDPPortRangeFlag != "" {
			parts := strings.SplitN(*lkUDPPortRangeFlag, "-", 2)
			if len(parts) == 2 {
				lkUDPPortRangeStart = parts[0]
				lkUDPPortRangeEnd = parts[1]
			} else {
				fmt.Fprintf(os.Stderr, "Invalid --lk-udp-range format: %s (expected start-end, e.g. 50000-60000)\n", *lkUDPPortRangeFlag)
				os.Exit(1)
			}
		}

		cfg := install.InstallConfig{
			EnableTLS:           (*enableTLS || *selfSigned) && !*noTLS,
			DisableTLS:          *noTLS,
			SelfSigned:          *selfSigned && !*noTLS,
			OverrideIP:          *ipOverride,
			Domain:              *domainFlag,
			Email:               *emailFlag,
			Port:                *portFlag,
			CertPath:            *certFlag,
			KeyPath:             *keyFlag,
			LKPort:              *lkPortFlag,
			LKTcpPort:           *lkTcpPortFlag,
			LKUdpPort:           *lkUdpPortFlag,
			LKUDPPortRangeStart: lkUDPPortRangeStart,
			LKUDPPortRangeEnd:   lkUDPPortRangeEnd,
			Fresh:               *freshFlag,
			BehindProxy:         *behindProxyFlag,
			ExternalLKURL:       *externalLKFlag,
			LKDomain:            *lkDomainFlag,
			LKIP:                *lkIPFlag,
			CertAlgorithm:       *certAlgoFlag,
		}

		if err := install.LinuxInstall(&cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Installation error: %v\n", err)
			os.Exit(1)
		}

	case "uninstall":
		if err := install.LinuxUninstall(); err != nil {
			fmt.Fprintf(os.Stderr, "Uninstallation error: %v\n", err)
			os.Exit(1)
		}

	case "user":
		userCmd := flag.NewFlagSet("user", flag.ExitOnError)
		configPath := userCmd.String("config", "/etc/bedrud/config.yaml", "Path to Bedrud config file")
		_ = userCmd.Parse(os.Args[2:])

		if len(userCmd.Args()) == 0 {
			fmt.Println("Usage: bedrud user [--config <path>] <subcommand> [flags]")
			fmt.Println("  create  --email <email> --password <password> --name <name>")
			fmt.Println("  delete  --email <email>")
			fmt.Println("  promote --email <email> [--role <role>]")
			fmt.Println("          Roles: superadmin (default), admin, moderator, user, guest")
			fmt.Println("  demote  --email <email> [--role <role>]")
			fmt.Println("          Roles: superadmin (default), admin, moderator")
			os.Exit(1)
		}
		sub := userCmd.Args()[0]
		subCmd := flag.NewFlagSet(sub, flag.ExitOnError)
		emailFlag := subCmd.String("email", "", "User email address")
		passwordFlag := subCmd.String("password", "", "User password")
		nameFlag := subCmd.String("name", "", "User name")
		roleFlag := subCmd.String("role", "", "User role (superadmin, admin, moderator, user, guest)")
		_ = subCmd.Parse(userCmd.Args()[1:])

		if *emailFlag == "" {
			fmt.Fprintf(os.Stderr, "Error: --email is required\n")
			os.Exit(1)
		}
		switch sub {
		case "create":
			if *passwordFlag == "" || *nameFlag == "" {
				fmt.Fprintf(os.Stderr, "Error: --password and --name are required for create\n")
				os.Exit(1)
			}
			if err := usercli.CreateUser(*configPath, *emailFlag, *passwordFlag, *nameFlag); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		case "delete":
			if err := usercli.DeleteUser(*configPath, *emailFlag); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		case "promote":
			role := *roleFlag
			if role == "" {
				role = "superadmin"
			}
			if err := usercli.PromoteUser(*configPath, *emailFlag, role); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		case "demote":
			role := *roleFlag
			if role == "" {
				role = "superadmin"
			}
			if err := usercli.DemoteUser(*configPath, *emailFlag, role); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		default:
			fmt.Fprintf(os.Stderr, "Unknown user subcommand: %s\n", sub)
			os.Exit(1)
		}

	case "cert":
		certCmd := flag.NewFlagSet("cert", flag.ExitOnError)
		certConfigPath := certCmd.String("config", "", "Path to Bedrud config file")
		_ = certCmd.Parse(os.Args[2:])

		if len(certCmd.Args()) == 0 {
			fmt.Println("Usage: bedrud cert [--config <path>] <subcommand>")
			fmt.Println("  renew   Renew the self-signed TLS certificate")
			fmt.Println("  info    Show TLS certificate status")
			os.Exit(1)
		}

		certSub := certCmd.Args()[0]
		path := *certConfigPath
		if path == "" {
			path = os.Getenv("CONFIG_PATH")
			if path == "" {
				path = "/etc/bedrud/config.yaml"
			}
		}

		switch certSub {
		case "renew":
			renewFlags := flag.NewFlagSet("renew", flag.ExitOnError)
			renewAlgo := renewFlags.String("algo", "", "Key algorithm: ed25519, ecdsa256, rsa2048, rsa4096 (default: detect from existing cert)")
			renewFlags.Parse(certCmd.Args()[1:])
			runCertRenew(path, *renewAlgo)
		case "info":
			runCertInfo(path)
		default:
			fmt.Fprintf(os.Stderr, "Unknown cert subcommand: %s\n", certSub)
			os.Exit(1)
		}

	case "version":
		fmt.Println("bedrud " + version)

	case "help":
		printUsage()

	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func runCertRenew(configPath, algoStr string) {
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
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
			fmt.Fprintf(os.Stderr, "Error renewing certificate: %v\n", err)
			os.Exit(1)
		}
	} else {
		if err := utils.RenewSelfSignedCert(certFile, keyFile, hosts...); err != nil {
			fmt.Fprintf(os.Stderr, "Error renewing certificate: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Println("Self-signed TLS certificate renewed successfully")
	fmt.Printf("  Cert: %s\n", certFile)
	fmt.Printf("  Key:  %s\n", keyFile)
	fmt.Printf("  SANs: %v\n", hosts)
	fmt.Printf("  Valid for %d days\n", utils.SelfSignedCertDays)
}

func runCertInfo(configPath string) {
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if !cfg.Server.EnableTLS || cfg.Server.DisableTLS {
		fmt.Println("TLS: not enabled")
		return
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
		fmt.Fprintf(os.Stderr, "TLS certificate error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Subject:        %s\n", info.Subject)
	fmt.Printf("Issuer:         %s\n", info.Issuer)
	fmt.Printf("Not Before:     %s\n", info.NotBefore.Format("2006-01-02 15:04:05 MST"))
	fmt.Printf("Not After:      %s\n", info.NotAfter.Format("2006-01-02 15:04:05 MST"))
	fmt.Printf("Days Remaining: %d\n", info.DaysRemaining)
	fmt.Printf("Status:         %s\n", info.Status)
	fmt.Printf("SANs:           %v\n", info.SANs)
}

func printUsage() {
	fmt.Println("Bedrud - Open Source Video Meetings (All-in-One Binary)")
	fmt.Println("\nUsage:")
	fmt.Println("  bedrud <command> [arguments]")
	fmt.Println("  bedrud --livekit --config <path>")
	fmt.Println("  bedrud --run --config <path>")
	fmt.Println("  bedrud --version")
	fmt.Println("\nCommands:")
	fmt.Println("  run       Start the meeting server")
	fmt.Println("  livekit   Start the embedded LiveKit server")
	fmt.Println("  install   Install Bedrud on a Debian/Linux system")
	fmt.Println("            Flags: --tls / --self-signed, --no-tls, --domain, --email,")
	fmt.Println("                   --ip, --port, --cert, --key,")
	fmt.Println("                   --lk-port, --lk-tcp-port, --lk-udp-port,")
	fmt.Println("                   --lk-ip <ip>            (separate NodeIP when behind CDN)")
	fmt.Println("                   --lk-udp-range <s-e>    (UDP port range, e.g. 50000-60000)")
	fmt.Println("                   --fresh, --behind-proxy,")
	fmt.Println("                   --livekit-domain <domain>  (local LK on its own domain)")
	fmt.Println("                   --external-livekit <url>   (fully separate LK machine)")
	fmt.Println("  uninstall Uninstall Bedrud from the system")
	fmt.Println("  cert      Manage TLS certificates")
	fmt.Println("            renew [--config <path>]   Renew self-signed certificate")
	fmt.Println("            info  [--config <path>]   Show certificate status")
	fmt.Println("  user      Manage users")
	fmt.Println("            create  --email <email> --password <password> --name <name>")
	fmt.Println("            delete  --email <email>")
	fmt.Println("            promote --email <email>  Grant superadmin access")
	fmt.Println("            demote  --email <email>  Remove superadmin access")
	fmt.Println("  version   Print version")
	fmt.Println("  help      Show this help message")
}
