package main

import (
	"fmt"
	"net/url"
	"strings"
)

type cfEnvelope[T any] struct {
	Success bool    `json:"success"`
	Errors  []cfErr `json:"errors"`
	Result  T       `json:"result"`
}

type cfErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type cfZone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type cfRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

func cfCheck[T any](env cfEnvelope[T]) error {
	if env.Success {
		return nil
	}
	if len(env.Errors) == 0 {
		return fmt.Errorf("cloudflare: unsuccessful")
	}
	return fmt.Errorf("cloudflare: %s", env.Errors[0].Message)
}

func pingCloudflare(c Config) error {
	u := c.CloudflareBase + "/zones?per_page=1"
	if strings.TrimSpace(c.Zone) != "" {
		u = c.CloudflareBase + "/zones?name=" + url.QueryEscape(c.Zone)
	}
	var env cfEnvelope[[]cfZone]
	if err := doJSON("GET", u, c.cfHeaders(), nil, &env); err != nil {
		return err
	}
	return cfCheck(env)
}

func lookupZoneID(c Config) (string, error) {
	if strings.TrimSpace(c.ZoneID) != "" {
		return c.ZoneID, nil
	}
	u := c.CloudflareBase + "/zones?name=" + url.QueryEscape(c.Zone)
	var env cfEnvelope[[]cfZone]
	if err := doJSON("GET", u, c.cfHeaders(), nil, &env); err != nil {
		return "", err
	}
	if err := cfCheck(env); err != nil {
		return "", err
	}
	if len(env.Result) == 0 {
		return "", fmt.Errorf("cloudflare zone %q not found", c.Zone)
	}
	return env.Result[0].ID, nil
}

func listDNS(c Config, zoneID string) ([]cfRecord, error) {
	u := c.CloudflareBase + "/zones/" + zoneID + "/dns_records?per_page=100"
	var env cfEnvelope[[]cfRecord]
	if err := doJSON("GET", u, c.cfHeaders(), nil, &env); err != nil {
		return nil, err
	}
	if err := cfCheck(env); err != nil {
		return nil, err
	}
	return env.Result, nil
}

func createADNS(c Config, zoneID, fqdn, ipv4 string) (cfRecord, error) {
	body := map[string]any{
		"type":    "A",
		"name":    fqdn,
		"content": ipv4,
		"ttl":     300,
		"proxied": false,
	}
	var env cfEnvelope[cfRecord]
	err := doJSON("POST", c.CloudflareBase+"/zones/"+zoneID+"/dns_records", c.cfHeaders(), body, &env)
	if err != nil {
		return cfRecord{}, err
	}
	if err := cfCheck(env); err != nil {
		return cfRecord{}, err
	}
	return env.Result, nil
}

func deleteDNS(c Config, zoneID, recordID string) error {
	var env cfEnvelope[any]
	err := doJSON("DELETE", c.CloudflareBase+"/zones/"+zoneID+"/dns_records/"+recordID, c.cfHeaders(), nil, &env)
	if err != nil {
		return err
	}
	return cfCheck(env)
}

func recordsForHost(recs []cfRecord, host, zone string) []cfRecord {
	want := strings.ToLower(fqdn(host, zone))
	wild := "*." + want
	var out []cfRecord
	for _, r := range recs {
		n := strings.ToLower(r.Name)
		if n == want || n == wild {
			out = append(out, r)
		}
	}
	return out
}

func takenHosts(recs []cfRecord, zone string) map[string]bool {
	z := strings.ToLower(strings.Trim(zone, "."))
	m := map[string]bool{}
	suf := "." + z
	for _, r := range recs {
		n := strings.ToLower(r.Name)
		n = strings.TrimSuffix(n, suf)
		if n != "" {
			m[n] = true
		}
	}
	return m
}
