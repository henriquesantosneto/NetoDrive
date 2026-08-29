#!/usr/bin/env bash
# Roda o servidor localmente (desenvolvimento), sem Docker e sem systemd.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export NETODRIVE_ADDR="${NETODRIVE_ADDR:-0.0.0.0:8080}"
export NETODRIVE_DATA="${NETODRIVE_DATA:-$ROOT/data}"
export NETODRIVE_DB="${NETODRIVE_DB:-$ROOT/data/netodrive.db}"
export NETODRIVE_ADMIN_USER="${NETODRIVE_ADMIN_USER:-admin}"
export NETODRIVE_ADMIN_PASS="${NETODRIVE_ADMIN_PASS:-admin123}"

mkdir -p "$NETODRIVE_DATA"
if [[ ! -d "$ROOT/web/dist" ]]; then
  (cd "$ROOT/web" && npm ci && npm run build)
fi
cd "$ROOT"
exec go run -C "$ROOT/server" ./cmd/netodrive
