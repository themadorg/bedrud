package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config is filled only from the environment. No credentials live in source.
type Config struct {
	LinodeToken     string
	LinodeBase      string
	LinodeRegion    string
	LinodeType      string
	LinodeImage     string
	CloudflareTok   string
	CloudflareEmail string
	CloudflareKey   string
	CloudflareBase  string
	Zone            string
	ZoneID          string
	SSHIdentity     string
	SSHPubKeyPath   string
	ConfigDir       string
}

func loadConfig() (Config, error) {
	return loadConfigMode(true)
}

func loadConfigMode(requireAPI bool) (Config, error) {
	home, _ := os.UserHomeDir()
	c := Config{
		LinodeToken:     os.Getenv("LINODE_TOKEN"),
		LinodeBase:      envOr("LINODE_API_URL", "https://api.linode.com/v4"),
		LinodeRegion:    envOr("LINODE_REGION", "de-fra-2"),
		LinodeType:      envOr("LINODE_TYPE", "g6-standard-1"),
		LinodeImage:     envOr("LINODE_IMAGE", "linode/debian13"),
		CloudflareTok:   os.Getenv("CLOUDFLARE_API_TOKEN"),
		CloudflareEmail: os.Getenv("CLOUDFLARE_EMAIL"),
		CloudflareKey:   os.Getenv("CLOUDFLARE_API_KEY"),
		CloudflareBase:  envOr("CLOUDFLARE_API_URL", "https://api.cloudflare.com/client/v4"),
		Zone:            os.Getenv("CLOUDFLARE_ZONE"),
		SSHIdentity:     envOr("SSH_IDENTITY", filepath.Join(home, ".ssh", "id_rsa")),
		SSHPubKeyPath:   envOr("SSH_PUBLIC_KEY", filepath.Join(home, ".ssh", "id_rsa.pub")),
		ConfigDir:       defaultConfigDir(home),
	}
	applyStoredCreds(&c)
	if requireAPI {
		var missing []string
		if c.LinodeToken == "" {
			missing = append(missing, "LINODE_TOKEN")
		}
		if c.CloudflareTok == "" && (c.CloudflareEmail == "" || c.CloudflareKey == "") {
			missing = append(missing, "CLOUDFLARE_API_TOKEN or CLOUDFLARE_EMAIL+CLOUDFLARE_API_KEY")
		}
		if c.Zone == "" {
			missing = append(missing, "CLOUDFLARE_ZONE")
		}
		if len(missing) > 0 {
			return c, errNeedInit(missing)
		}
	}
	c.LinodeBase = strings.TrimRight(c.LinodeBase, "/")
	c.CloudflareBase = strings.TrimRight(c.CloudflareBase, "/")
	return c, nil
}

func defaultConfigDir(home string) string {
	if d := strings.TrimSpace(os.Getenv("BEDRUD_HOST_CONFIG_DIR")); d != "" {
		return d
	}
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "bedrud-host")
	}
	if home == "" {
		home = "."
	}
	return filepath.Join(home, ".config", "bedrud-host")
}

func errNeedInit(missing []string) error {
	msg := "not initialized — run:  bedrud-host init"
	if len(missing) > 0 {
		msg += "\n(need Linode token, Cloudflare token or email+API key, and domain"
		msg += "; missing: " + strings.Join(missing, ", ") + ")"
	}
	return fmt.Errorf("%s", msg)
}

func requireInit(c Config) error {
	var missing []string
	if c.LinodeToken == "" {
		missing = append(missing, "linode token")
	}
	if c.CloudflareTok == "" && (c.CloudflareEmail == "" || c.CloudflareKey == "") {
		missing = append(missing, "cloudflare credentials")
	}
	if c.Zone == "" {
		missing = append(missing, "cloudflare domain")
	}
	if len(missing) > 0 {
		return errNeedInit(missing)
	}
	return nil
}

func envOr(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}

func applyStoredCreds(c *Config) {
	st, err := openStore(c.ConfigDir)
	if err != nil {
		return
	}
	defer st.Close()
	if c.LinodeToken == "" {
		if v, ok, _ := st.GetSetting(setLinodeToken); ok {
			c.LinodeToken = v
		}
	}
	if c.CloudflareTok == "" {
		if v, ok, _ := st.GetSetting(setCFToken); ok {
			c.CloudflareTok = v
		}
	}
	if c.CloudflareEmail == "" {
		if v, ok, _ := st.GetSetting(setCFEmail); ok {
			c.CloudflareEmail = v
		}
	}
	if c.CloudflareKey == "" {
		if v, ok, _ := st.GetSetting(setCFKey); ok {
			c.CloudflareKey = v
		}
	}
	if c.Zone == "" {
		if v, ok, _ := st.GetSetting(setCFDomain); ok {
			c.Zone = v
		}
	}
	if c.ZoneID == "" {
		if v, ok, _ := st.GetSetting(setCFZoneID); ok {
			c.ZoneID = v
		}
	}
}

func (c Config) linodeHeaders() map[string]string {
	return map[string]string{"Authorization": "Bearer " + c.LinodeToken}
}

func (c Config) cfHeaders() map[string]string {
	if strings.TrimSpace(c.CloudflareTok) != "" {
		return map[string]string{"Authorization": "Bearer " + c.CloudflareTok}
	}
	return map[string]string{
		"X-Auth-Email": c.CloudflareEmail,
		"X-Auth-Key":   c.CloudflareKey,
	}
}
