#!/usr/bin/env bash
# Instala o NetoDrive como serviço systemd no Linux (sem Docker).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PREFIX="${PREFIX:-/opt/netodrive}"
DATA_DIR="${DATA_DIR:-/var/lib/netodrive}"
ENV_DIR="${ENV_DIR:-/etc/netodrive}"

if [[ "$(id -u)" -ne 0 ]]; then
  echo "Execute como root: sudo $0" >&2
  exit 1
fi

echo "==> Compilando servidor e interface web"
mkdir -p "$ROOT/dist"
(cd "$ROOT/server" && go build -o "$ROOT/dist/netodrive" ./cmd/netodrive)
if [[ ! -d "$ROOT/web/dist" ]]; then
  (cd "$ROOT/web" && npm ci && npm run build)
elif [[ ! -f "$ROOT/web/dist/index.html" ]]; then
  (cd "$ROOT/web" && npm ci && npm run build)
fi

echo "==> Criando usuário e pastas"
id netodrive &>/dev/null || useradd --system --home "$DATA_DIR" --shell /usr/sbin/nologin netodrive
mkdir -p "$PREFIX/bin" "$PREFIX/web" "$DATA_DIR" "$ENV_DIR"
cp "$ROOT/dist/netodrive" "$PREFIX/bin/netodrive"
rm -rf "$PREFIX/web/dist"
cp -a "$ROOT/web/dist" "$PREFIX/web/dist"
chown -R netodrive:netodrive "$DATA_DIR"
chown -R root:root "$PREFIX"
chmod 755 "$PREFIX/bin/netodrive"

if [[ ! -f "$ENV_DIR/netodrive.env" ]]; then
  cp "$ROOT/deploy/netodrive.env.example" "$ENV_DIR/netodrive.env"
  # gera secret aleatório
  if command -v openssl >/dev/null; then
    SECRET="$(openssl rand -hex 32)"
    sed -i "s/CHANGE-ME-to-a-long-random-secret/$SECRET/" "$ENV_DIR/netodrive.env"
  fi
  chmod 640 "$ENV_DIR/netodrive.env"
  chown root:netodrive "$ENV_DIR/netodrive.env"
  echo "Criado $ENV_DIR/netodrive.env — edite a senha admin antes de produção."
fi

echo "==> Instalando unit systemd"
cp "$ROOT/deploy/netodrive.service" /etc/systemd/system/netodrive.service
# WorkingDirectory aponta para /opt/netodrive; o binário encontra web/dist relativo
systemctl daemon-reload
systemctl enable netodrive.service
systemctl restart netodrive.service

echo
echo "NetoDrive instalado como serviço."
echo "  status:  systemctl status netodrive"
echo "  logs:    journalctl -u netodrive -f"
echo "  config:  $ENV_DIR/netodrive.env"
echo "  dados:   $DATA_DIR"
echo "  URL:     http://$(hostname -I 2>/dev/null | awk '{print $1}'):8080"
