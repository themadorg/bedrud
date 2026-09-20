package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func okStr(ok bool, detail string) string {
	if ok {
		if detail != "" {
			return "ok  " + detail
		}
		return "ok"
	}
	if detail != "" {
		return "missing  " + detail
	}
	return "missing"
}

func cmdStatus(c Config, local bool) error {
	failed := false
	line := func(name, result string, ok bool) {
		if !ok {
			failed = true
		}
		fmt.Printf("%-22s %s\n", name, result)
	}

	exe, err := selfPath()
	if err != nil {
		line("binary", err.Error(), false)
		exe = ""
	} else {
		line("binary", exe, true)
	}
	if exe != "" {
		if _, ok, err := readKeyTrailer(exe); err != nil {
			line("binary-key", err.Error(), false)
		} else if !ok {
			line("binary-key", "no trailer (run init)", false)
		} else {
			line("binary-key", "ok", true)
		}
	}

	if c.ConfigDir == "" {
		line("config-dir", "not set", false)
	} else {
		st, err := os.Stat(c.ConfigDir)
		if err != nil || !st.IsDir() {
			line("config-dir", c.ConfigDir+" (absent)", false)
		} else {
			line("config-dir", c.ConfigDir, true)
		}
	}
	enc := filepath.Join(c.ConfigDir, "hosts.sqlite.enc")
	if _, err := os.Stat(enc); err != nil {
		line("encrypted-db", enc+" (absent)", false)
	} else {
		line("encrypted-db", enc, true)
	}

	if c.LinodeToken == "" {
		line("linode-token", "not set", false)
	} else {
		line("linode-token", maskSecret(c.LinodeToken), true)
	}
	cfOK := c.CloudflareTok != "" || (c.CloudflareEmail != "" && c.CloudflareKey != "")
	if c.CloudflareTok != "" {
		line("cloudflare-token", maskSecret(c.CloudflareTok), true)
	} else {
		line("cloudflare-token", "not set", cfOK)
	}
	if c.CloudflareEmail != "" {
		line("cloudflare-email", c.CloudflareEmail, true)
	} else {
		line("cloudflare-email", "not set", c.CloudflareTok != "")
	}
	if c.CloudflareKey != "" {
		line("cloudflare-api-key", maskSecret(c.CloudflareKey), true)
	} else {
		line("cloudflare-api-key", "not set", c.CloudflareTok != "")
	}
	if c.Zone == "" {
		line("cloudflare-domain", "not set", false)
	} else {
		line("cloudflare-domain", c.Zone, true)
	}
	if c.ZoneID == "" {
		line("cloudflare-zone-id", "not set", c.Zone != "")
	} else {
		line("cloudflare-zone-id", c.ZoneID, true)
	}

	hosts := 0
	if st, err := openStore(c.ConfigDir); err != nil {
		line("local-hosts", err.Error(), false)
	} else {
		rows, err := st.List()
		_ = st.Close()
		if err != nil {
			line("local-hosts", err.Error(), false)
		} else {
			hosts = len(rows)
			line("local-hosts", fmt.Sprintf("%d", hosts), true)
		}
	}
	if st, err := openStore(c.ConfigDir); err == nil {
		if avg, n, err := st.createStats(); err == nil && n > 0 {
			line("create-avg", fmt.Sprintf("%s (%d runs)", fmtDuration(avg), n), true)
		} else {
			line("create-avg", "no samples yet", true)
		}
		_ = st.Close()
	}

	if local {
		line("linode-api", "skipped (local)", true)
		line("cloudflare-api", "skipped (local)", true)
	} else if c.LinodeToken != "" && c.LinodeBase != "" {
		if err := pingLinode(c); err != nil {
			line("linode-api", err.Error(), false)
		} else {
			line("linode-api", "reachable", true)
		}
	} else {
		line("linode-api", "skipped", false)
	}
	if !local {
		if cfOK && c.CloudflareBase != "" {
			if err := pingCloudflare(c); err != nil {
				line("cloudflare-api", err.Error(), false)
			} else {
				line("cloudflare-api", "reachable", true)
			}
		} else {
			line("cloudflare-api", "skipped", false)
		}
	}

	if failed {
		return fmt.Errorf("status: some checks failed")
	}
	return nil
}
