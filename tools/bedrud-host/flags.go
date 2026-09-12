package main

import (
	"fmt"
	"strconv"
	"strings"
)

type initArgs struct {
	LinodeToken string
	CFToken     string
	CFEmail     string
	CFKey       string
	CFDomain    string
	CFZoneID    string
}

func parseInitArgs(args []string) (initArgs, error) {
	var out initArgs
	i := 0
	for i < len(args) {
		flag := args[i]
		i++
		take := func() (string, error) {
			if i >= len(args) {
				return "", fmt.Errorf("missing %s value", flag)
			}
			v := args[i]
			i++
			return v, nil
		}
		switch flag {
		case "-linode-token", "--linode-token":
			v, err := take()
			if err != nil {
				return out, err
			}
			out.LinodeToken = v
		case "-cloudflare-token", "--cloudflare-token":
			v, err := take()
			if err != nil {
				return out, err
			}
			out.CFToken = v
		case "-cloudflare-email", "--cloudflare-email":
			v, err := take()
			if err != nil {
				return out, err
			}
			out.CFEmail = v
		case "-cloudflare-api-key", "--cloudflare-api-key":
			v, err := take()
			if err != nil {
				return out, err
			}
			out.CFKey = v
		case "-cloudflare-domain", "--cloudflare-domain":
			v, err := take()
			if err != nil {
				return out, err
			}
			out.CFDomain = v
		case "-cloudflare-zone", "--cloudflare-zone":
			v, err := take()
			if err != nil {
				return out, err
			}
			out.CFZoneID = v
		default:
			return out, fmt.Errorf("unknown flag %s", flag)
		}
	}
	return out, nil
}

type createArgs struct {
	Prefix string
	DryRun bool
}

type deleteArgs struct {
	Host   string
	Yes    bool
	DryRun bool
}

func parseCreateArgs(args []string, zone string) (createArgs, error) {
	var out createArgs
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-prefix", "--prefix":
			i++
			if i >= len(args) {
				return out, fmt.Errorf("missing -prefix value")
			}
			out.Prefix = hostFromArg(args[i], zone)
		case "-dry-run", "--dry-run":
			out.DryRun = true
		default:
			return out, fmt.Errorf("unknown flag %s", args[i])
		}
	}
	return out, nil
}

func parseDeleteArgs(args []string) (deleteArgs, error) {
	var out deleteArgs
	if len(args) < 1 {
		return out, fmt.Errorf("delete requires HOST")
	}
	out.Host = args[0]
	for _, a := range args[1:] {
		switch a {
		case "-yes", "--yes":
			out.Yes = true
		case "-dry-run", "--dry-run":
			out.DryRun = true
		default:
			return out, fmt.Errorf("unknown flag %s", a)
		}
	}
	return out, nil
}

type recordArgs struct {
	Host          string
	IPv4          string
	LinodeID      int
	LinodeLabel   string
	AdminName     string
	AdminEmail    string
	AdminPassword string
}

func parseRecordArgs(args []string) (recordArgs, error) {
	var out recordArgs
	if len(args) < 1 || strings.HasPrefix(args[0], "-") {
		return out, fmt.Errorf("record requires HOST")
	}
	out.Host = args[0]
	i := 1
	for i < len(args) {
		flag := args[i]
		i++
		take := func() (string, error) {
			if i >= len(args) {
				return "", fmt.Errorf("missing %s value", flag)
			}
			v := args[i]
			i++
			return v, nil
		}
		switch flag {
		case "-ipv4", "--ipv4":
			v, err := take()
			if err != nil {
				return out, err
			}
			out.IPv4 = v
		case "-linode-id", "--linode-id":
			v, err := take()
			if err != nil {
				return out, err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return out, fmt.Errorf("linode-id: %w", err)
			}
			out.LinodeID = n
		case "-linode-label", "--linode-label":
			v, err := take()
			if err != nil {
				return out, err
			}
			out.LinodeLabel = v
		case "-admin-name", "--admin-name":
			v, err := take()
			if err != nil {
				return out, err
			}
			out.AdminName = v
		case "-admin-email", "--admin-email":
			v, err := take()
			if err != nil {
				return out, err
			}
			out.AdminEmail = v
		case "-admin-password", "--admin-password":
			v, err := take()
			if err != nil {
				return out, err
			}
			out.AdminPassword = v
		default:
			return out, fmt.Errorf("unknown flag %s", flag)
		}
	}
	return out, nil
}
