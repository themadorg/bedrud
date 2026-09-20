package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func readPubKey(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return "", fmt.Errorf("empty public key %s", path)
	}
	return s, nil
}

func remoteInstallScript() string {
	return strings.TrimSpace(`
set -euxo pipefail
HOST="$1"
IP="$2"
ADMIN_EMAIL="$3"
ADMIN_NAME="$4"
ADMIN_PASS="$5"

hostnamectl set-hostname "$HOST" || true
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y nginx certbot python3-certbot-nginx ufw curl xz-utils ca-certificates

ufw --force reset || true
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw allow 7881/tcp
ufw allow 3478/udp
ufw allow 50000:60000/udp
ufw --force enable

cd /tmp
curl -fsSL -o bedrud_linux_amd64.tar.xz \
  https://github.com/themadorg/bedrud/releases/latest/download/bedrud_linux_amd64.tar.xz
tar -xJf bedrud_linux_amd64.tar.xz
install -m 0755 bedrud /usr/local/bin/bedrud
bedrud install --no-tls --behind-proxy --domain "$HOST" --ip "$IP" --port 8090

python3 - "$HOST" << 'PY'
import re, sys
from pathlib import Path
host = sys.argv[1]
p = Path("/etc/bedrud/config.yaml")
text = p.read_text()
text = re.sub(r"(?m)^(\s+host:\s).*$", r'\1"127.0.0.1"', text, count=1)
text = re.sub(r"(?m)^(\s+host:\s)http://.+$", r'\1"https://%s/livekit"' % host, text, count=1)
text = re.sub(r"(?m)^(\s+frontendURL:\s).*$", r'\1"https://%s"' % host, text, count=1)
text = re.sub(r"(?m)^(\s+allowedOrigins:\s).*$", r'\1"https://%s"' % host, text, count=1)
p.write_text(text)
PY

cat >/etc/nginx/sites-available/"$HOST" << NGINX
server {
    listen 80;
    listen [::]:80;
    server_name $HOST;
    location / {
        proxy_pass http://127.0.0.1:8090;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }
}
NGINX
rm -f /etc/nginx/sites-enabled/default
ln -sfn /etc/nginx/sites-available/"$HOST" /etc/nginx/sites-enabled/"$HOST"
nginx -t && systemctl reload nginx
systemctl restart bedrud livekit || true
certbot --nginx -d "$HOST" --agree-tos --redirect --non-interactive --register-unsafely-without-email

cat >/etc/nginx/sites-available/"$HOST" << NGINX
server {
    listen 80;
    listen [::]:80;
    server_name $HOST;
    return 301 https://\$host\$request_uri;
}
server {
    listen 443 ssl;
    listen [::]:443 ssl;
    server_name $HOST;
    ssl_certificate     /etc/letsencrypt/live/$HOST/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/$HOST/privkey.pem;
    include /etc/letsencrypt/options-ssl-nginx.conf;
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;
    location /livekit/ {
        proxy_pass http://127.0.0.1:7880/;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host 127.0.0.1:7880;
        proxy_read_timeout 86400;
    }
    location / {
        proxy_pass http://127.0.0.1:8090;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host \$host;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }
}
NGINX
nginx -t && systemctl reload nginx
bedrud user create --email "$ADMIN_EMAIL" --password "$ADMIN_PASS" --name "$ADMIN_NAME" --admin
`) + "\n"
}

var waitSSHFn = waitSSH
var sshRunFn = sshRun

func sshRun(identity, ip, script string, args []string, timeout time.Duration) error {
	cmdArgs := []string{
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10",
		"-i", identity,
		"root@" + ip,
		"bash", "-s", "--",
	}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command("ssh", cmdArgs...)
	cmd.Stdin = strings.NewReader(script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if timeout > 0 {
		timer := time.AfterFunc(timeout, func() { _ = cmd.Process.Kill() })
		defer timer.Stop()
	}
	return cmd.Run()
}

func waitSSH(identity, ip string, tries int) error {
	_ = exec.Command("ssh-keygen", "-R", ip).Run()
	for i := 0; i < tries; i++ {
		cmd := exec.Command("ssh",
			"-o", "StrictHostKeyChecking=accept-new",
			"-o", "ConnectTimeout=8",
			"-i", identity,
			"root@"+ip, "echo", "UP")
		out, err := cmd.CombinedOutput()
		if err == nil && strings.Contains(string(out), "UP") {
			return nil
		}
		fmt.Printf("  ssh wait %d/%d\n", i+1, tries)
		time.Sleep(8 * time.Second)
	}
	return fmt.Errorf("ssh never came up on %s", ip)
}
