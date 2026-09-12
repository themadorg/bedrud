package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func cmdDelete(c Config, target string, yes, dryRun bool) error {
	if err := requireInit(c); err != nil {
		return err
	}
	st, err := openStore(c.ConfigDir)
	if err != nil {
		return fmt.Errorf("config db: %w", err)
	}
	defer st.Close()

	host, zone, name := "", c.Zone, ""
	if id, conv := strconv.Atoi(target); conv == nil && strconv.Itoa(id) == target {
		row, ok, err := st.GetByID(id)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("not in local db: id %d", id)
		}
		host, zone, name = row.Host, row.Zone, row.FQDN
		if c.Zone == "" {
			c.Zone = zone
		}
	} else {
		var err error
		host, zone, name, err = resolveName(target, c.Zone)
		if err != nil {
			return err
		}
	}
	label := linodeLabel(host, zone)

	zoneID, err := lookupZoneID(c)
	if err != nil {
		return err
	}
	recs, err := listDNS(c, zoneID)
	if err != nil {
		return err
	}
	match := recordsForHost(recs, host, zone)
	var ipv4 string
	for _, r := range match {
		if r.Type == "A" && r.Content != "" {
			ipv4 = r.Content
			break
		}
	}
	inst, found, err := findLinodeByLabelOrIP(c, label, ipv4)
	if err != nil {
		return err
	}

	row, inDB, err := st.Get(name)
	if err != nil {
		return err
	}
	if !found && inDB && row.LinodeID != 0 {
		inst = linodeInstance{ID: row.LinodeID, Label: row.LinodeLabel, IPv4: []string{row.IPv4}}
		found = true
	}

	fmt.Printf("delete %s\n", name)
	fmt.Printf("  dns records: %d\n", len(match))
	for _, r := range match {
		fmt.Printf("    %s %s → %s (%s)\n", r.Type, r.Name, r.Content, r.ID)
	}
	if found {
		fmt.Printf("  linode id=%d label=%s ipv4=%v\n", inst.ID, inst.Label, inst.IPv4)
	} else {
		fmt.Println("  linode: not found")
	}
	if len(match) == 0 && !found && !inDB {
		return fmt.Errorf("nothing to delete for %s", name)
	}
	if dryRun {
		fmt.Println("dry-run; not deleting")
		return nil
	}
	if !yes {
		if !shouldPrompt() {
			return fmt.Errorf("refusing to delete without -yes (or a TTY to confirm)")
		}
		ans, err := readLine(fmt.Sprintf("Delete %s (DNS + Linode)? type yes: ", name))
		if err != nil {
			return err
		}
		switch strings.ToLower(strings.TrimSpace(ans)) {
		case "yes", "y":
		default:
			fmt.Println("cancelled")
			return nil
		}
	}
	for _, r := range match {
		fmt.Printf("delete dns %s\n", r.ID)
		if err := deleteDNS(c, zoneID, r.ID); err != nil && !isHTTPStatus(err, 404) {
			return err
		}
	}
	if err := waitDNSGone(c, zoneID, host, zone, 12); err != nil {
		return err
	}
	if found {
		fmt.Printf("delete linode %d\n", inst.ID)
		if err := deleteLinode(c, inst.ID); err != nil {
			return err
		}
		if err := waitLinodeGone(c, inst.ID, 48); err != nil {
			return err
		}
	}
	if err := st.Delete(name); err != nil {
		return err
	}
	fmt.Printf("deleted %s\n", name)
	return nil
}

func waitDNSGone(c Config, zoneID, host, zone string, tries int) error {
	for i := 0; i < tries; i++ {
		recs, err := listDNS(c, zoneID)
		if err != nil {
			return err
		}
		left := recordsForHost(recs, host, zone)
		if len(left) == 0 {
			fmt.Println("dns gone")
			return nil
		}
		fmt.Printf("dns wait %d/%d records=%d\n", i+1, tries, len(left))
		for _, r := range left {
			_ = deleteDNS(c, zoneID, r.ID)
		}
		time.Sleep(linodePoll)
	}
	return fmt.Errorf("dns records still present for %s", host)
}
