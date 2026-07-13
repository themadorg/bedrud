package install

import (
	"fmt"
	"os"
)

// refreshServices rewrites init unit files for the current binary path and
// LiveKit topology, then enables and starts services.
func refreshServices(bedrudBin string, isExternalLK bool) error {
	initSystem := detectInitSystem()
	fmt.Println("➜ Detected init system:", initSystem)

	if initSystem == InitSystemNone {
		printContainerInstructions()
		return nil
	}

	serviceCfg := buildServiceConfig(isExternalLK)
	cleanupStaleServiceFiles(initSystem)

	lkManagedEnv := ""
	bedrudAfter := "network.target"
	if !isExternalLK {
		lkManagedEnv = "\nEnvironment=LIVEKIT_MANAGED=true"
		bedrudAfter = "network.target livekit.service"
	}

	lkService := fmt.Sprintf(`[Unit]
Description=Bedrud LiveKit Media Server (embedded)
Documentation=https://docs.bedrud.com
After=network.target network-online.target
Wants=network-online.target

[Service]
User=bedrud
Group=bedrud
Type=simple
ExecStart=%s --livekit --config /etc/bedrud/livekit.yaml
Restart=on-failure
RestartSec=5s
WorkingDirectory=/etc/bedrud
StandardOutput=journal
StandardError=journal
SyslogIdentifier=livekit

NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=/var/lib/bedrud /var/log/bedrud /etc/bedrud /tmp

[Install]
WantedBy=multi-user.target
`, bedrudBin)

	serviceContent := fmt.Sprintf(`[Unit]
Description=Bedrud Video Meeting Server
Documentation=https://docs.bedrud.com
After=%s network-online.target
Wants=network-online.target

[Service]
User=bedrud
Group=bedrud
Type=simple
ExecStart=%s run --config /etc/bedrud/config.yaml
Restart=on-failure
RestartSec=5s
Environment=CONFIG_PATH=/etc/bedrud/config.yaml%s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=bedrud

NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=/var/lib/bedrud /var/log/bedrud /etc/bedrud

[Install]
WantedBy=multi-user.target
`, bedrudAfter, bedrudBin, lkManagedEnv)

	if err := writeServiceFiles(initSystem, &serviceCfg, bedrudBin, bedrudAfter, lkManagedEnv, lkService, serviceContent); err != nil {
		return fmt.Errorf("failed to write service files: %w", err)
	}

	fmt.Println("➜ Enabling and starting services...")
	if err := enableAndStartServices(initSystem, &serviceCfg); err != nil {
		return fmt.Errorf("failed to enable/start services: %w", err)
	}
	return nil
}

// isExternalLiveKitFromConfig inspects config.yaml for livekit.external without
// loading the full config package (avoids singleton side effects when possible).
func isExternalLiveKitFromConfig(configPath string) (bool, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false, err
	}
	// Minimal parse: look for "external: true" under livekit by using yaml via existing type.
	var cfg struct {
		LiveKit struct {
			External bool `yaml:"external"`
		} `yaml:"livekit"`
	}
	if err := yamlUnmarshal(data, &cfg); err != nil {
		return false, err
	}
	// Also treat missing livekit.yaml + external as external-only installs
	if cfg.LiveKit.External {
		return true, nil
	}
	if _, err := os.Stat(etcLivekitPath); os.IsNotExist(err) {
		// No local livekit.yaml → external or not yet configured
		return true, nil
	}
	return false, nil
}
