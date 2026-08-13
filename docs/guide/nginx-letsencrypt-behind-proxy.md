# nginx + Let's Encrypt in Front of Bedrud

Run Bedrud as plain HTTP on `localhost` and terminate TLS at nginx using Certbot. Do **not** configure `bedrud install` to issue or manage certificates.

The examples in this guide use the reserved domain `meet.example.com` and IP `203.0.113.10`. Replace them with your host's values.

---

## Why This Layout

| Component | Owner |
|-----------|-------|
| Website HTTPS | nginx + Let's Encrypt (`/etc/letsencrypt/live/meet.example.com/`) |
| Bedrud API + UI | `127.0.0.1:8090` HTTP only (`enableTLS: false`) |
| LiveKit signaling (WebSocket) | Same origin: `https://meet.example.com/livekit` → Bedrud → `127.0.0.1:7880` |
| WebRTC media (UDP) | Direct to the server IP (`node_ip`), bypassing nginx |

Do not instruct Bedrud to create certificates (`--tls`, `--self-signed`, or `--domain` + `--email` ACME). Let's Encrypt management remains strictly on the proxy.

Re-running `bedrud install` without `--fresh` leaves `/etc/bedrud/config.yaml` and `livekit.yaml` unchanged and will not overwrite Let's Encrypt files. Running `install --tls` on an existing installation can write an unused self-signed certificate pair to `/etc/bedrud/cert.pem`; this pair is ignored as long as `enableTLS` remains `false`.

---

## 1. DNS

```text
meet.example.com.  A  203.0.113.10
```

Port 80 must reach this host so Certbot can complete HTTP-01 challenges.

---

## 2. Install Bedrud Without TLS

```bash
sudo bedrud install \
  --no-tls \
  --behind-proxy \
  --domain meet.example.com \
  --ip 203.0.113.10 \
  --port 8090
```

Do not pass `--tls`, `--self-signed`, `--cert`, `--key`, or `--email`.

Edit `/etc/bedrud/config.yaml`:

```yaml
server:
  port: "8090"
  host: "127.0.0.1"          # Do not bind to public IP; keep 8090 off the internet
  enableTLS: false
  useACME: false
  domain: meet.example.com
  behindProxy: true
  trustedProxies:
    - "127.0.0.1"
    - "::1"
  proxyHeader: X-Forwarded-For

livekit:
  host: "https://meet.example.com/livekit"   # Must use https on 443, not :8090
  internalHost: "http://127.0.0.1:7880"
  skipTLSVerify: true
  external: false

auth:
  frontendURL: "https://meet.example.com"

cors:
  allowedOrigins: "https://meet.example.com"
  allowCredentials: true
```

Write `/etc/bedrud/livekit.yaml` to match this layout (`bedrud install --no-tls` produces the same shape). `keys` must match `livekit.apiKey` / `livekit.apiSecret` in `config.yaml`. `rtc.node_ip` is the **public** ICE address (`203.0.113.10` here). Do not point TURN `cert_file` / `key_file` at Let's Encrypt; TLS stays on nginx.

```yaml
# /etc/bedrud/livekit.yaml
port: 7880
bind_addresses:
  - 127.0.0.1
keys:
  CHANGE_ME_LIVEKIT_API_KEY: CHANGE_ME_LIVEKIT_API_SECRET
rtc:
  tcp_port: 7881
  port_range_start: 50000
  port_range_end: 60000
  use_external_ip: false
  node_ip: 203.0.113.10
turn:
  enabled: true
  domain: meet.example.com
  udp_port: 3478
  # No tls_port / cert_file / key_file: HTTPS is terminated at nginx.
webhook:
  urls: []
  api_key: ""
logging:
  json: true
  level: info
```

Restart the services after editing:

```bash
sudo systemctl restart bedrud livekit
```

---

## 3. Configure nginx + Certbot

Set up a temporary HTTP site block so Certbot can issue a certificate, then switch to HTTPS.

```nginx
# /etc/nginx/sites-available/meet.example.com
server {
    listen 80;
    listen [::]:80;
    server_name meet.example.com;

    location / {
        proxy_pass http://127.0.0.1:8090;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Enable the site and obtain the certificate:

```bash
sudo ln -sfn /etc/nginx/sites-available/meet.example.com /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
sudo apt-get install -y certbot python3-certbot-nginx
sudo certbot --nginx -d meet.example.com --agree-tos --redirect
```

Then update `/etc/nginx/sites-available/meet.example.com` to use this HTTPS configuration (keep any `ssl_certificate` directives added by Certbot):

```nginx
server {
    listen 80;
    listen [::]:80;
    server_name meet.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    listen [::]:443 ssl;
    server_name meet.example.com;

    ssl_certificate     /etc/letsencrypt/live/meet.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/meet.example.com/privkey.pem;
    include /etc/letsencrypt/options-ssl-nginx.conf;
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;

    # Signaling: WebSocket upgrade. Bedrud strips /livekit and proxies to :7880.
    location /livekit {
        proxy_pass http://127.0.0.1:8090;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 86400;
        proxy_send_timeout 86400;
    }

    location / {
        proxy_pass http://127.0.0.1:8090;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Test and reload nginx:

```bash
sudo nginx -t && sudo systemctl reload nginx
```

nginx uses the Let's Encrypt files directly. Bedrud does not need `certFile` or `keyFile` set to these paths.

---

## 4. Firewall Configuration

Signaling is proxied through port 443. WebRTC media traffic does **not** pass through nginx.

| Port | Protocol | Purpose |
|------|----------|---------|
| 80, 443 | TCP | HTTP challenge + HTTPS site |
| 7881 | TCP | WebRTC TCP fallback |
| 50000–60000 | UDP | WebRTC media (or your configured `livekit.yaml` range) |
| 3478 | UDP | TURN |

Do **not** publish ports `8090` or `7880` on the public interface.

---

## 5. Troubleshooting: `wss://meet.example.com:8090` + `ERR_SSL_PROTOCOL_ERROR`

If the meeting page attempts to connect to:

```text
wss://meet.example.com:8090/livekit/rtc/v1?...
```

the browser is sending TLS traffic to Bedrud's **plain HTTP** port. This occurs when `livekit.host` is set to an explicit HTTP URL with port 8090:

```yaml
livekit:
  host: "http://meet.example.com:8090/livekit"
```

Because the web page loaded over HTTPS, the client upgrades the WebSocket scheme to `wss` while retaining port 8090. Since no certificate is served on port 8090, the browser throws `ERR_SSL_PROTOCOL_ERROR` and fails to join the room.

**Fix:** Set `livekit.host` to `https://meet.example.com/livekit` (omitting the port or specifying `:443`). Verify in DevTools that the socket URL is `wss://meet.example.com/livekit/...`. Perform a hard refresh after modifying the configuration to avoid using cached join URLs.

*Note:* A `POST /api/auth/refresh` returning HTTP **400** for a guest without a refresh cookie is expected behavior and unrelated to LiveKit connection errors.

---

## 6. Verification

Verify API health via HTTPS:

```bash
curl -fsS https://meet.example.com/api/health
# {"status":"healthy",...}
```

*Note:* A `GET /livekit/rtc/v1` request without a WebSocket upgrade header often returns `404`, which is normal. Confirm that TLS on port 443 presents the Let's Encrypt certificate rather than a self-signed Bedrud certificate.

Verify active listeners:

```bash
sudo ss -lntp | grep -E '8090|7880|443'
# 443 → nginx
# 127.0.0.1:8090 → bedrud
# 127.0.0.1:7880 → livekit
```

Compare file hashes of `/etc/letsencrypt/live/meet.example.com/*.pem` before and after running `bedrud install` (without `--fresh`) to ensure they remain untouched.

---

## Related Documentation

- [CLI Install Guide](../server/cli/install.md) — `--fresh` vs. preserving existing config
- [Behind Proxy Guide](https://bedrud.org/en/docs/guides/behind-proxy)
