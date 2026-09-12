package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"
)

func shouldPrompt() bool {
	f, ok := promptIn.(*os.File)
	if !ok || f != os.Stdin {
		return true
	}
	return term.IsTerminal(int(f.Fd()))
}

func cmdInit(c Config, a initArgs) error {
	if a.LinodeToken == "" {
		if !shouldPrompt() {
			return fmt.Errorf("linode token, cloudflare token, and cloudflare domain are required")
		}
		v, err := readSecret("Linode API token: ")
		if err != nil {
			return err
		}
		a.LinodeToken = v
	}
	if a.CFToken == "" && a.CFKey == "" {
		if !shouldPrompt() {
			return fmt.Errorf("linode token, cloudflare token or api key, and cloudflare domain are required")
		}
		v, err := readSecret("Cloudflare API token (blank to use global API key): ")
		if err != nil {
			return err
		}
		a.CFToken = v
		if a.CFToken == "" {
			v, err = readLine("Cloudflare account email: ")
			if err != nil {
				return err
			}
			a.CFEmail = v
			v, err = readSecret("Cloudflare global API key: ")
			if err != nil {
				return err
			}
			a.CFKey = v
		}
	}
	if a.CFKey != "" && a.CFEmail == "" && shouldPrompt() {
		v, err := readLine("Cloudflare account email: ")
		if err != nil {
			return err
		}
		a.CFEmail = v
	}
	if a.CFDomain == "" {
		if !shouldPrompt() {
			return fmt.Errorf("linode token, cloudflare token, and cloudflare domain are required")
		}
		v, err := readLine("Cloudflare domain (zone name): ")
		if err != nil {
			return err
		}
		a.CFDomain = v
	}
	if a.CFZoneID == "" && shouldPrompt() {
		v, err := readLine("Cloudflare zone ID (blank to look up): ")
		if err != nil {
			return err
		}
		a.CFZoneID = v
	}
	a.LinodeToken = strings.TrimSpace(a.LinodeToken)
	a.CFToken = strings.TrimSpace(a.CFToken)
	a.CFEmail = strings.TrimSpace(a.CFEmail)
	a.CFKey = strings.TrimSpace(a.CFKey)
	a.CFDomain = strings.ToLower(strings.Trim(strings.TrimSpace(a.CFDomain), "."))
	a.CFZoneID = strings.TrimSpace(a.CFZoneID)
	cfOK := a.CFToken != "" || (a.CFEmail != "" && a.CFKey != "")
	if a.LinodeToken == "" || !cfOK || a.CFDomain == "" {
		return fmt.Errorf("linode token, cloudflare token or email+api-key, and cloudflare domain are required")
	}

	st, err := openStore(c.ConfigDir)
	if err != nil {
		return err
	}
	defer st.Close()

	c.CloudflareTok = a.CFToken
	c.CloudflareEmail = a.CFEmail
	c.CloudflareKey = a.CFKey
	c.Zone = a.CFDomain
	c.ZoneID = a.CFZoneID
	if c.CloudflareBase == "" {
		c.CloudflareBase = "https://api.cloudflare.com/client/v4"
	}
	if a.CFZoneID == "" {
		zid, err := lookupZoneID(c)
		if err != nil {
			return fmt.Errorf("look up zone id: %w", err)
		}
		a.CFZoneID = zid
		c.ZoneID = zid
	}

	if err := st.SetSetting(setLinodeToken, a.LinodeToken); err != nil {
		return err
	}
	if err := st.SetSetting(setCFToken, a.CFToken); err != nil {
		return err
	}
	if err := st.SetSetting(setCFEmail, a.CFEmail); err != nil {
		return err
	}
	if err := st.SetSetting(setCFKey, a.CFKey); err != nil {
		return err
	}
	if err := st.SetSetting(setCFDomain, a.CFDomain); err != nil {
		return err
	}
	if err := st.SetSetting(setCFZoneID, a.CFZoneID); err != nil {
		return err
	}

	exe, err := selfPath()
	if err != nil {
		return err
	}
	fmt.Printf("config:             %s\n", c.ConfigDir)
	fmt.Printf("database:           %s/hosts.sqlite.enc\n", c.ConfigDir)
	fmt.Printf("key:                appended to %s\n", exe)
	fmt.Printf("linode token:       %s\n", maskSecret(a.LinodeToken))
	fmt.Printf("cloudflare token:   %s\n", maskSecret(a.CFToken))
	fmt.Printf("cloudflare email:   %s\n", a.CFEmail)
	fmt.Printf("cloudflare api key: %s\n", maskSecret(a.CFKey))
	fmt.Printf("cloudflare domain:  %s\n", a.CFDomain)
	fmt.Printf("cloudflare zone id: %s\n", a.CFZoneID)
	return nil
}

func cmdList(c Config) error {
	st, err := openStore(c.ConfigDir)
	if err != nil {
		return err
	}
	defer st.Close()
	rows, err := st.List()
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		fmt.Printf("no hosts in %s\n", c.ConfigDir)
		return nil
	}
	for _, h := range rows {
		took := ""
		if h.CreateMS > 0 {
			took = "  took=" + fmtDuration(time.Duration(h.CreateMS)*time.Millisecond)
		}
		rate := ""
		if h.HourlyUSD > 0 {
			rate = fmt.Sprintf("  $%.4f/hr", h.HourlyUSD)
		}
		fmt.Printf("%d  %s  ipv4=%s  linode=%d  admin=%s  created=%s%s%s\n",
			h.ID, h.FQDN, h.IPv4, h.LinodeID, h.AdminEmail, h.CreatedAt, took, rate)
	}
	return nil
}

func lookupRow(c Config, arg string) (HostRow, error) {
	st, err := openStore(c.ConfigDir)
	if err != nil {
		return HostRow{}, err
	}
	defer st.Close()
	if id, err := strconv.Atoi(arg); err == nil && strconv.Itoa(id) == arg {
		h, ok, err := st.GetByID(id)
		if err != nil {
			return HostRow{}, err
		}
		if !ok {
			return HostRow{}, fmt.Errorf("not in local db: id %d", id)
		}
		return h, nil
	}
	_, _, name, err := resolveName(arg, c.Zone)
	if err != nil {
		return HostRow{}, err
	}
	h, ok, err := st.Get(name)
	if err != nil {
		return HostRow{}, err
	}
	if !ok {
		return HostRow{}, fmt.Errorf("not in local db: %s", name)
	}
	return h, nil
}

func cmdView(c Config, arg string) error {
	h, err := lookupRow(c, arg)
	if err != nil {
		return err
	}
	fmt.Printf("id:            %d\n", h.ID)
	fmt.Printf("fqdn:          %s\n", h.FQDN)
	fmt.Printf("host:          %s\n", h.Host)
	fmt.Printf("zone:          %s\n", h.Zone)
	fmt.Printf("ipv4:          %s\n", h.IPv4)
	fmt.Printf("linode-id:     %d\n", h.LinodeID)
	fmt.Printf("linode-label:  %s\n", h.LinodeLabel)
	fmt.Printf("admin-name:    %s\n", h.AdminName)
	fmt.Printf("admin-email:   %s\n", h.AdminEmail)
	fmt.Printf("created:       %s\n", h.CreatedAt)
	if h.CreateMS > 0 {
		fmt.Printf("create-time:   %s\n", fmtDuration(time.Duration(h.CreateMS)*time.Millisecond))
	}
	if h.HourlyUSD > 0 {
		fmt.Printf("hourly:        $%.4f/hr (%s)\n", h.HourlyUSD, h.LinodeType)
	}
	return nil
}

func cmdAdmin(c Config, arg string) error {
	h, err := lookupRow(c, arg)
	if err != nil {
		return err
	}
	if h.AdminName == "" && h.AdminEmail == "" && h.AdminPassword == "" {
		return fmt.Errorf("no admin credentials stored for %s", h.FQDN)
	}
	fmt.Printf("name:     %s\n", h.AdminName)
	fmt.Printf("email:    %s\n", h.AdminEmail)
	fmt.Printf("password: %s\n", h.AdminPassword)
	return nil
}

func cmdRecord(c Config, a recordArgs) error {
	host, zone, name, err := resolveName(a.Host, c.Zone)
	if err != nil {
		return err
	}
	st, err := openStore(c.ConfigDir)
	if err != nil {
		return err
	}
	defer st.Close()
	row := HostRow{
		FQDN: name, Host: host, Zone: zone,
		IPv4: a.IPv4, LinodeID: a.LinodeID, LinodeLabel: a.LinodeLabel,
		AdminName: a.AdminName, AdminEmail: a.AdminEmail, AdminPassword: a.AdminPassword,
	}
	if prev, ok, err := st.Get(name); err != nil {
		return err
	} else if ok {
		if row.IPv4 == "" {
			row.IPv4 = prev.IPv4
		}
		if row.LinodeID == 0 {
			row.LinodeID = prev.LinodeID
		}
		if row.LinodeLabel == "" {
			row.LinodeLabel = prev.LinodeLabel
		}
		if row.AdminName == "" {
			row.AdminName = prev.AdminName
		}
		if row.AdminEmail == "" {
			row.AdminEmail = prev.AdminEmail
		}
		if row.AdminPassword == "" {
			row.AdminPassword = prev.AdminPassword
		}
		row.CreatedAt = prev.CreatedAt
	}
	if err := st.Upsert(row); err != nil {
		return err
	}
	fmt.Printf("recorded %s\n", name)
	return nil
}
