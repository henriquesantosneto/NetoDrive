#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/server"
go test ./...
cd "$ROOT/clients/desktop"
go test ./...
echo "OK"
