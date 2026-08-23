package main

import (
	"fmt"
	"os"
	"time"
)

func cmdCreate(c Config, prefix string, dryRun bool) error {
	if err := requireInit(c); err != nil {
		return err
	}
	zoneID, err := lookupZoneID(c)
	if err != nil {
		return err
	}
	recs, err := listDNS(c, zoneID)
	if err != nil {
		return err
	}
	taken := takenHosts(recs, c.Zone)

	host := prefix
	if host == "" {
		for i := 0; i < 40; i++ {
			h, err := randomLabel(5)
			if err != nil {
				return err
			}
			if !taken[h] {
				host = h
				break
			}
		}
		if host == "" {
			return fmt.Errorf("could not pick a free subdomain")
		}
	} else if taken[host] {
		return fmt.Errorf("subdomain %s already has DNS", host)
	}

	name := fqdn(host, c.Zone)
	label := linodeLabel(host, c.Zone)
	rootPass, err := randomPassword(20)
	if err != nil {
		return err
	}
	adminPass, err := randomPassword(16)
	if err != nil {
		return err
	}
	adminName := "admin-" + host
	email := adminEmail(host, c.Zone)

	fmt.Printf("plan: linode %s in %s, host %s\n", c.LinodeType, c.LinodeRegion, name)
	if dryRun {
		fmt.Println("dry-run; not creating")
		return nil
	}

	st, err := openStore(c.ConfigDir)
	if err != nil {
		return fmt.Errorf("config db: %w", err)
	}
	defer st.Close()
	if avg, n, err := st.createStats(); err != nil {
		return err
	} else if n > 0 {
		fmt.Printf("average create time: %s (%d runs)\n", fmtDuration(avg), n)
	}

	started := time.Now()

	pub, err := readPubKey(c.SSHPubKeyPath)
	if err != nil {
		return err
	}

	inst, err := createLinode(c, label, rootPass, []string{pub})
	if err != nil {
		return err
	}
	fmt.Printf("linode id=%d status=%s\n", inst.ID, inst.Status)
	ipv4 := ""
	if len(inst.IPv4) > 0 {
		ipv4 = inst.IPv4[0]
	}
	if ipv4 == "" {
		ipv4, err = waitLinodeIP(c, inst.ID, 24)
		if err != nil {
			return err
		}
	}
	fmt.Printf("ipv4 %s\n", ipv4)

	rec, err := createADNS(c, zoneID, name, ipv4)
	if err != nil {
		return err
	}
	fmt.Printf("dns %s → %s (%s)\n", rec.Name, rec.Content, rec.ID)

	if err := waitSSHFn(c.SSHIdentity, ipv4, 24); err != nil {
		return err
	}
	fmt.Println("installing bedrud…")
	args := []string{name, ipv4, email, adminName, adminPass}
	if err := sshRunFn(c.SSHIdentity, ipv4, remoteInstallScript(), args, 12*time.Minute); err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, "")
	fmt.Println("========== bedrud ready ==========")
	fmt.Printf("url:            https://%s\n", name)
	fmt.Printf("ssh:            ssh -i %s root@%s\n", c.SSHIdentity, ipv4)
	fmt.Printf("admin name:     %s\n", adminName)
	fmt.Printf("admin email:    %s\n", email)
	fmt.Printf("admin password: %s\n", adminPass)
	hourly, monthly, perr := linodeTypePrice(c, c.LinodeType)
	if perr != nil {
		fmt.Printf("hourly:         unknown (%v)\n", perr)
	} else {
		fmt.Printf("hourly:         %s\n", fmtUSDRate(hourly, monthly, c.LinodeType))
	}
	took := time.Since(started)
	avg, n, err := st.recordCreateTime(took)
	if err != nil {
		return fmt.Errorf("save timing: %w", err)
	}

	fmt.Println("=================================")
	if err := st.Upsert(HostRow{
		FQDN: name, Host: host, Zone: c.Zone, IPv4: ipv4,
		LinodeID: inst.ID, LinodeLabel: label,
		AdminName: adminName, AdminEmail: email, AdminPassword: adminPass,
		CreateMS: took.Milliseconds(), HourlyUSD: hourly, LinodeType: c.LinodeType,
	}); err != nil {
		return fmt.Errorf("save host: %w", err)
	}
	fmt.Printf("saved in %s\n", c.ConfigDir)
	fmt.Printf("took:           %s\n", fmtDuration(took))
	fmt.Printf("average create: %s (%d runs)\n", fmtDuration(avg), n)
	return nil
}
