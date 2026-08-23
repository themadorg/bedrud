package main

import "testing"

func TestParseInitArgs(t *testing.T) {
	f, err := parseInitArgs([]string{
		"--linode-token", "lt",
		"--cloudflare-token", "ct",
		"--cloudflare-domain", "example.com",
		"--cloudflare-zone", "zid",
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.LinodeToken != "lt" || f.CFToken != "ct" || f.CFDomain != "example.com" || f.CFZoneID != "zid" {
		t.Fatalf("%+v", f)
	}
}

func TestParseCreateArgs(t *testing.T) {
	f, err := parseCreateArgs([]string{"--prefix", "Meet.example.com", "-dry-run"}, "example.com")
	if err != nil {
		t.Fatal(err)
	}
	if f.Prefix != "meet" || !f.DryRun {
		t.Fatalf("%+v", f)
	}
	if _, err := parseCreateArgs([]string{"-prefix"}, "example.com"); err == nil {
		t.Fatal("expected missing prefix")
	}
	if _, err := parseCreateArgs([]string{"-nope"}, "example.com"); err == nil {
		t.Fatal("expected unknown")
	}
}

func TestParseDeleteArgs(t *testing.T) {
	f, err := parseDeleteArgs([]string{"meet.example.com", "-yes", "--dry-run"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Host != "meet.example.com" || !f.Yes || !f.DryRun {
		t.Fatalf("%+v", f)
	}
	if _, err := parseDeleteArgs(nil); err == nil {
		t.Fatal("expected host required")
	}
	if _, err := parseDeleteArgs([]string{"meet", "-x"}); err == nil {
		t.Fatal("expected unknown")
	}
}

func TestParseRecordArgs(t *testing.T) {
	f, err := parseRecordArgs([]string{
		"meet.example.com",
		"--ipv4", "203.0.113.10",
		"--linode-id", "99",
		"--linode-label", "meet-example-com",
		"--admin-name", "admin-meet",
		"--admin-email", "admin@meet.example.com",
		"--admin-password", "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.Host != "meet.example.com" || f.IPv4 != "203.0.113.10" || f.LinodeID != 99 {
		t.Fatalf("%+v", f)
	}
	if f.AdminPassword != "secret" {
		t.Fatalf("%+v", f)
	}
	if _, err := parseRecordArgs(nil); err == nil {
		t.Fatal("expected host")
	}
	if _, err := parseRecordArgs([]string{"h", "--linode-id", "x"}); err == nil {
		t.Fatal("expected number error")
	}
}
