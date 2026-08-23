# bedrud-host

Tiny Go CLI (standard library only) that creates or deletes a Bedrud VM using the public **Linode** and **Cloudflare** APIs.

Tokens come from the environment. Nothing is hardcoded.

```bash
export LINODE_TOKEN=...
export CLOUDFLARE_API_TOKEN=...
export CLOUDFLARE_ZONE=example.com   # your DNS zone

make test
make build
make init
# or: make init ARGS='--linode-token T --cloudflare-token T --cloudflare-domain example.com'

./bedrud-host init                   # prompts for Linode + Cloudflare credentials
./bedrud-host init --linode-token T --cloudflare-token T --cloudflare-domain example.com
./bedrud-host create                 # random subdomain of CLOUDFLARE_ZONE
./bedrud-host create -prefix meet    # meet.example.com
./bedrud-host create -dry-run

./bedrud-host delete meet -dry-run
./bedrud-host delete meet.example.com -yes
./bedrud-host status
./bedrud-host list
./bedrud-host view meet.example.com
./bedrud-host admin meet.example.com
./bedrud-host record meet.example.com -ipv4 203.0.113.10 -linode-id 1
```

`create` boots a Debian Linode, adds a DNS A record, installs latest Bedrud behind nginx + Let's Encrypt, and prints a random admin name/email/password.

Local state:

```
~/.config/bedrud-host/hosts.sqlite.enc   # AES-256-GCM wrapped SQLite
```

The 32-byte key is **appended to the bedrud-host binary** on first run (trailer `BHKEY1`). It is not stored next to the db. Rebuilding the binary drops that trailer, so the old db will not decrypt.

`bedrud-host list` reads the db. `delete` also removes the row.
