package livekit

import (
	"bedrud/internal/utils"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// ExportBinary writes the embedded LiveKit server binary to the specified path
func ExportBinary(destPath string) error {
	binData, err := Bin.ReadFile(lkBinKey)
	if err != nil {
		return fmt.Errorf("failed to read embedded LiveKit binary: %w", err)
	}
	// Unlink before writing — on Linux you cannot overwrite a file that is
	// currently mapped as an executable (ETXTBSY).  Removing the path lets
	// the running process keep its inode while we create a fresh one.
	_ = os.Remove(destPath)
	if err := os.WriteFile(destPath, binData, 0o755); err != nil {
		return fmt.Errorf("failed to write LiveKit binary to %s: %w", destPath, err)
	}
	return nil
}

// resolveLiveKitPath tries to export the embedded LiveKit binary to a series
// of candidate directories, returning the first successful path.
// Falls back to bare executable name (PATH lookup) if all exports fail.
func resolveLiveKitPath() string {
	candidates := []func() string{
		tempDirPath,
		userCachePath,
		exeDirPath,
		cwdPath,
	}

	for _, fn := range candidates {
		p := fn()
		if p == "" {
			continue
		}
		// Ensure parent directory exists.
		dir := filepath.Dir(p)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Debug().Err(err).Str("dir", dir).Msg("LiveKit: cannot create parent dir, skipping candidate")
			continue
		}
		if err := ExportBinary(p); err != nil {
			log.Debug().Err(err).Str("path", p).Msg("LiveKit: export failed, trying next candidate")
			continue
		}
		if err := os.Chmod(p, 0o755); err != nil {
			log.Warn().Err(err).Str("path", p).Msg("Failed to chmod LiveKit binary")
		}
		log.Info().Str("path", p).Msg("LiveKit: binary exported successfully")
		return p
	}

	// Last resort: bare name, relies on $PATH containing livekit-server
	log.Warn().Msg("LiveKit: all export candidates failed, falling back to PATH lookup")
	return lkExeName
}

func tempDirPath() string {
	return filepath.Join(os.TempDir(), lkExeName)
}

func userCachePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "bedrud", lkExeName)
}

func exeDirPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), lkExeName)
}

func cwdPath() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, lkExeName)
}

// RunLiveKit starts the embedded LiveKit server directly with the provided config
func RunLiveKit(configPath string) error {
	lkPath := resolveLiveKitPath()

	args := []string{}
	if configPath != "" {
		args = append(args, "--config", configPath)
	}

	cmd := exec.Command(lkPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Info().Str("path", lkPath).Str("config", configPath).Msg("➜ Running embedded LiveKit server")
	return cmd.Run()
}

// StartInternalServer starts a LiveKit server using the provided config file
func generateTempConfig(apiKey, apiSecret string, port int, nodeIP, certFile, keyFile, serverHost, httpPort string) (string, error) {
	cfg := ConfigYAML{}
	cfg.Port = port
	cfg.Keys = map[string]string{apiKey: apiSecret}

	if nodeIP != "" {
		if net.ParseIP(nodeIP) == nil {
			return "", fmt.Errorf("invalid NodeIP %q: not a valid IP address", nodeIP)
		}
		cfg.RTC.UseExternalIP = false
		cfg.RTC.NodeIP = nodeIP
		log.Info().Str("nodeIP", nodeIP).Msg("LiveKit: using explicit NodeIP (STUN disabled)")
	} else {
		cfg.RTC.UseExternalIP = true
		log.Warn().Msg("LiveKit: no NodeIP configured, using STUN-based IP detection (may fail on air-gapped/firewalled networks)")
	}

	if certFile != "" && keyFile != "" {
		cfg.TURN.Enabled = true
		cfg.TURN.TLSPort = 5349
		cfg.TURN.UDPPort = 3478
		cfg.TURN.CertFile = certFile
		cfg.TURN.KeyFile = keyFile
		if serverHost != "" {
			cfg.TURN.Domain = serverHost
		}
	}

	// Auto-configure webhook to send events back to our API
	// (only when no external config is provided)
	if httpPort != "" {
		webhookURL := fmt.Sprintf("http://localhost:%s/api/livekit/webhook", httpPort)
		cfg.Webhook.URLs = []string{webhookURL}
		cfg.Webhook.APIKey = apiKey
		log.Info().Str("url", webhookURL).Msg("LiveKit: configured webhook to local API")
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal temp LiveKit config: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "bedrud-livekit-*.yaml")
	if err != nil {
		return "", fmt.Errorf("failed to create temp LiveKit config: %w", err)
	}

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write temp LiveKit config: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to close temp LiveKit config: %w", err)
	}

	return tmpFile.Name(), nil
}

func ResolveNodeIP(explicitIP, serverHost string) string {
	if explicitIP != "" {
		return explicitIP
	}
	if serverHost != "" {
		if ip := net.ParseIP(serverHost); ip != nil && !ip.IsLoopback() && !ip.IsUnspecified() {
			return ip.String()
		}
	}
	if ip := utils.OutboundIP(); ip != nil && !ip.IsLoopback() && !ip.IsUnspecified() {
		return ip.String()
	}
	return ""
}

func StartInternalServer(ctx context.Context, apiKey, apiSecret string, port int, certFile, keyFile, externalConfigPath, nodeIP, serverHost, httpPort string) error {
	if os.Getenv("LIVEKIT_MANAGED") == "true" {
		log.Info().Msg("➜ Skipping internal LiveKit management (managed by system service)")
		return nil
	}

	lkPath := resolveLiveKitPath()

	args := []string{}
	configPath := externalConfigPath
	cleanupTemp := ""

	if configPath == "" {
		tmpPath, err := generateTempConfig(apiKey, apiSecret, port, nodeIP, certFile, keyFile, serverHost, httpPort)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to generate temp LiveKit config, falling back to inline args")
		} else {
			configPath = tmpPath
			cleanupTemp = tmpPath
		}
	}

	if configPath != "" {
		args = append(args, "--config", configPath)
	} else {
		args = append(args, "--port", fmt.Sprintf("%d", port), "--keys", fmt.Sprintf("%s: %s", apiKey, apiSecret))
	}

	cmd := exec.CommandContext(ctx, lkPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	go func() {
		log.Info().Str("path", lkPath).Msg("➜ Starting internal LiveKit process")
		if err := cmd.Run(); err != nil {
			log.Error().Err(err).Msg("LiveKit process exited")
		}
		if cleanupTemp != "" {
			_ = os.Remove(cleanupTemp)
		}
	}()

	time.Sleep(3 * time.Second)
	return nil
}
