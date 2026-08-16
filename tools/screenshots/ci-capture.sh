#!/usr/bin/env bash
# Start local web + API + LiveKit and capture product screenshots (CI).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

EMAIL="${BEDRUD_SCREENSHOT_EMAIL:-screenshots@bedrud.local}"
PASSWORD="${BEDRUD_SCREENSHOT_PASSWORD:-Screenshot1!}"

export PATH="$HOME/.local/bin:$PATH"

echo "➜ installing web + screenshot deps"
(cd apps/web && bun install --frozen-lockfile)
(cd tools/screenshots && bun install --frozen-lockfile)

if ! command -v livekit-server >/dev/null 2>&1; then
  echo "➜ installing LiveKit server"
  LK_VER="${LIVEKIT_VERSION:-1.10.1}"
  curl -fsSL "https://github.com/livekit/livekit/releases/download/v${LK_VER}/livekit_${LK_VER}_linux_amd64.tar.gz" \
    -o /tmp/livekit.tar.gz
  tar -xzf /tmp/livekit.tar.gz -C /tmp livekit-server
  mkdir -p "$HOME/.local/bin"
  mv /tmp/livekit-server "$HOME/.local/bin/livekit-server"
  chmod +x "$HOME/.local/bin/livekit-server"
fi

echo "➜ starting stack"
make dev-web > /tmp/bedrud-web.log 2>&1 &
make dev-api > /tmp/bedrud-api.log 2>&1 &
make dev-livekit > /tmp/bedrud-lk.log 2>&1 &

wait_port() {
  local url="$1"
  local n=0
  until curl -sf -o /dev/null "$url" || [ "$n" -ge 90 ]; do
    n=$((n + 1))
    sleep 2
  done
  if [ "$n" -ge 90 ]; then
    echo "timed out waiting for $url" >&2
    tail -n 80 /tmp/bedrud-web.log /tmp/bedrud-api.log /tmp/bedrud-lk.log >&2 || true
    exit 1
  fi
}

wait_port "http://127.0.0.1:7070/"
# API health
n=0
until curl -sf -o /dev/null "http://127.0.0.1:7071/api/health" || [ "$n" -ge 90 ]; do
  n=$((n + 1))
  sleep 2
done
if [ "$n" -ge 90 ]; then
  echo "timed out waiting for API" >&2
  tail -n 80 /tmp/bedrud-api.log >&2 || true
  exit 1
fi
wait_port "http://127.0.0.1:7072/"

echo "➜ ensuring screenshot admin user"
(cd server && go run ./cmd/bedrud --config config.yaml user create \
  --email "$EMAIL" --password "$PASSWORD" --name "Shahram Farhadi" --admin) || true

echo "➜ capturing screenshots"
(cd tools/screenshots && \
  BEDRUD_SCREENSHOT_EMAIL="$EMAIL" \
  BEDRUD_SCREENSHOT_PASSWORD="$PASSWORD" \
  BEDRUD_SCREENSHOT_NO_START=1 \
  node screenshot.js --no-start --timeout 120000)

echo "➜ publishing into site public/"
bash tools/screenshots/publish-to-site.sh
