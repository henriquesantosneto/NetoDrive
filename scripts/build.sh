#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

mkdir -p "$ROOT/dist"
(cd "$ROOT/server" && go build -o "$ROOT/dist/netodrive" ./cmd/netodrive)
(cd "$ROOT/clients/desktop" && go build -o "$ROOT/dist/netodrive-sync" ./cmd/netodrive-sync)
(cd "$ROOT/clients/desktop" && GOOS=windows GOARCH=amd64 go build -o "$ROOT/dist/netodrive-sync.exe" ./cmd/netodrive-sync)

if command -v npm >/dev/null; then
  (cd "$ROOT/web" && npm install && npm run build)
fi

echo "Binaries in $ROOT/dist"
