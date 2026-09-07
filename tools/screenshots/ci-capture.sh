#!/usr/bin/env bash
# Build the app the way a release does, serve it from the Go binary, and capture
# product screenshots (CI).
#
# The app and the API share one origin here because that is what a deployment
# looks like: the Go server serves the embedded bundle from server/frontend and
# answers /api on the same port. Capturing `vite dev` instead adds a TanStack
# Start SSR pass whose dehydrated route loaders never re-run on the client, so
# every dashboard page renders signed out — see #129.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

EMAIL="${BEDRUD_SCREENSHOT_EMAIL:-screenshots@bedrud.local}"
PASSWORD="${BEDRUD_SCREENSHOT_PASSWORD:-Screenshot1!}"

# One origin for the app and the API — the Go server serves both.
APP_URL="http://127.0.0.1:7071"

export PATH="$HOME/.local/bin:$PATH"

echo "➜ installing web + screenshot deps"
(cd apps/web && bun install --frozen-lockfile)
(cd tools/screenshots && bun install --frozen-lockfile)
(cd server && go mod download)

# Gitignored local configs — CI never has these unless we seed them.
if [ ! -f server/config.yaml ]; then
  cp server/config.local.yaml.example server/config.yaml
  echo "➜ wrote server/config.yaml from config.local.yaml.example"
fi
if [ ! -f server/livekit.yaml ]; then
  cp server/livekit.yaml.example server/livekit.yaml
  echo "➜ wrote server/livekit.yaml from livekit.yaml.example"
fi
mkdir -p server/internal/livekit/bin
touch server/internal/livekit/bin/livekit-server

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

# Must precede the server build: server/frontend is embedded at compile time.
# VITE_API_URL stays unset on purpose — an empty base is what makes the bundle
# same-origin, and that is what a deployment ships.
echo "➜ building the web bundle into server/frontend"
(cd apps/web && bun run build:embed)

echo "➜ starting stack"
make dev-api > /tmp/bedrud-api.log 2>&1 &
make dev-livekit > /tmp/bedrud-lk.log 2>&1 &

http_up() {
  curl -sf -o /dev/null "$1" || curl -sf -o /dev/null --connect-timeout 2 "${1/127.0.0.1/localhost}"
}

wait_port() {
  local url="$1"
  local log="$2"
  local n=0
  until http_up "$url" || [ "$n" -ge 90 ]; do
    n=$((n + 1))
    sleep 2
  done
  if [ "$n" -ge 90 ]; then
    echo "timed out waiting for $url" >&2
    tail -n 80 "$log" >&2 || true
    exit 1
  fi
}

# The API health endpoint and the app itself come up together — one process.
wait_port "$APP_URL/api/health" /tmp/bedrud-api.log
wait_port "$APP_URL/" /tmp/bedrud-api.log
wait_port "http://127.0.0.1:7072/" /tmp/bedrud-lk.log

echo "➜ ensuring screenshot admin user"
(cd server && go run ./cmd/bedrud --config config.yaml user create \
  --email "$EMAIL" --password "$PASSWORD" --name "Shahram Farhadi" --admin) || true

echo "➜ capturing screenshots"
(cd tools/screenshots && \
  BEDRUD_SCREENSHOT_EMAIL="$EMAIL" \
  BEDRUD_SCREENSHOT_PASSWORD="$PASSWORD" \
  BEDRUD_BASE_URL="$APP_URL" \
  BEDRUD_API_URL="$APP_URL" \
  BEDRUD_SCREENSHOT_NO_START=1 \
  node screenshot.js --no-start --timeout 120000)

echo "➜ publishing into site public/"
bash tools/screenshots/publish-to-site.sh
