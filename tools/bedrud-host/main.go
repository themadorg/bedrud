package main

import (
	"fmt"
	"os"
)

func usage() {
	fmt.Fprintf(os.Stderr, `bedrud-host — create/delete a Bedrud VM (Linode + Cloudflare DNS)

Env (required):
  LINODE_TOKEN
  CLOUDFLARE_API_TOKEN
  CLOUDFLARE_ZONE          apex zone, e.g. example.com

Optional:
  LINODE_REGION            default de-fra-2
  LINODE_TYPE              default g6-standard-1
  LINODE_IMAGE             default linode/debian13
  LINODE_API_URL           default https://api.linode.com/v4
  CLOUDFLARE_API_URL       default https://api.cloudflare.com/client/v4
  SSH_IDENTITY             default ~/.ssh/id_rsa
  SSH_PUBLIC_KEY           default ~/.ssh/id_rsa.pub
  BEDRUD_HOST_CONFIG_DIR   default ~/.config/bedrud-host

The AES key is generated on first run and appended to this binary.
Rebuilding the binary creates a new key; the old encrypted db will not open.

Usage:
  bedrud-host init [-linode-token T] [-cloudflare-token T]
                   [-cloudflare-email E] [-cloudflare-api-key K]
                   [-cloudflare-domain example.com] [-cloudflare-zone ZONEID]
                   (missing values are prompted)
  bedrud-host create [-prefix NAME] [-dry-run]
  bedrud-host delete HOST|ID [-yes] [-dry-run]
  bedrud-host status [-local]
  bedrud-host reset [-yes]
  bedrud-host list
  bedrud-host view HOST
  bedrud-host admin HOST
  bedrud-host record HOST [-ipv4 IP] [-linode-id ID] [-linode-label L]
                         [-admin-name N] [-admin-email E] [-admin-password P]
`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	needAPI := cmd == "create" || cmd == "delete"
	c, err := loadConfigMode(needAPI)
	if err != nil && cmd != "help" && cmd != "-h" && cmd != "--help" {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	switch cmd {
	case "help", "-h", "--help":
		usage()
	case "init":
		ia, err := parseInitArgs(args)
		if err != nil {
			fatal(err.Error())
		}
		if err := cmdInit(c, ia); err != nil {
			fatal(err.Error())
		}
	case "create":
		f, err := parseCreateArgs(args, c.Zone)
		if err != nil {
			fatal(err.Error())
		}
		if err := cmdCreate(c, f.Prefix, f.DryRun); err != nil {
			fatal(err.Error())
		}
	case "delete":
		f, err := parseDeleteArgs(args)
		if err != nil {
			fatal(err.Error())
		}
		if err := cmdDelete(c, f.Host, f.Yes, f.DryRun); err != nil {
			fatal(err.Error())
		}
	case "status":
		local := false
		for _, a := range args {
			if a == "-local" || a == "--local" {
				local = true
			}
		}
		if err := cmdStatus(c, local); err != nil {
			fatal(err.Error())
		}
	case "reset":
		yes := false
		for _, a := range args {
			if a == "-yes" || a == "--yes" {
				yes = true
			}
		}
		if err := cmdReset(c, yes); err != nil {
			fatal(err.Error())
		}
	case "list":
		if err := cmdList(c); err != nil {
			fatal(err.Error())
		}
	case "view":
		if len(args) < 1 {
			fatal("view requires HOST")
		}
		if err := cmdView(c, args[0]); err != nil {
			fatal(err.Error())
		}
	case "admin":
		if len(args) < 1 {
			fatal("admin requires HOST")
		}
		if err := cmdAdmin(c, args[0]); err != nil {
			fatal(err.Error())
		}
	case "record":
		f, err := parseRecordArgs(args)
		if err != nil {
			fatal(err.Error())
		}
		if err := cmdRecord(c, f); err != nil {
			fatal(err.Error())
		}
	default:
		usage()
		os.Exit(2)
	}
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
