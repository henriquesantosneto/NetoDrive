# NetoDrive

Sincronize arquivos entre **Windows**, **Android** e a **Web**, com servidor em **Linux**.  
Interfaces no estilo **OneDrive**; o servidor roda como **serviço systemd** (sem Docker).

## Componentes

| Pasta | Função |
|---|---|
| `server/` | API de sync + storage (blobs SHA-256) + JWT |
| `web/` | UI estilo OneDrive (lista, sidebar, abrir remoto) |
| `clients/desktop/` | Sync de pastas no Windows/Linux |
| `clients/android/` | Galeria + modo cache LRU + abrir remoto |
| `deploy/` | Unit systemd + env de exemplo |
| `scripts/install-service.sh` | Instala o serviço no Linux |

## Servidor Linux (serviço — recomendado)

No servidor Ubuntu/Debian (precisa de Go e Node só na instalação):

```bash
sudo ./scripts/install-service.sh
```

Isso:
1. Compila o binário e a UI web  
2. Instala em `/opt/netodrive`  
3. Guarda dados em `/var/lib/netodrive`  
4. Cria `/etc/netodrive/netodrive.env`  
5. Habilita e inicia `netodrive.service`

```bash
sudo systemctl status netodrive
sudo journalctl -u netodrive -f
sudo nano /etc/netodrive/netodrive.env   # usuário/senha/JWT
sudo systemctl restart netodrive
```

Acesse: `http://IP-DO-SERVIDOR:8080`  
Login padrão (altere em produção): `admin` / `admin123`

### Desenvolvimento local (sem instalar serviço)

```bash
./scripts/run-server.sh
```

## Cliente Windows

```bash
cd clients/desktop
go run ./cmd/netodrive-sync -init -config netodrive.json
# edite server_url (http://IP:8080), local_folder, remote_prefix (ex.: PC)
go run ./cmd/netodrive-sync -config netodrive.json
```

Para gerar `.exe`: `./scripts/build.sh` → `dist/netodrive-sync.exe`

## Cliente Web

Já é servida pelo próprio serviço. Em dev:

```bash
cd web && npm install && npm run dev
```

## Cliente Android

Abra `clients/android` no Android Studio.

- Emulador → `http://10.0.2.2:8080`
- Aparelho → `http://IP-DO-SERVIDOR:8080`

**Galeria + cache:** sincroniza fotos para o servidor; com modo cache ligado, mídias são baixadas sob demanda e removidas do aparelho quando o orçamento (MB) estoura. **Abrir remoto** usa streaming/cache via `ContentProvider`.

## API (resumo)

| Método | Rota | Uso |
|---|---|---|
| POST | `/api/auth/login` | JWT |
| GET | `/api/files?path=` | Listar (UI OneDrive) |
| PUT | `/api/sync/upload` | Upload |
| GET | `/api/sync/download/...` | Download |
| GET | `/api/open/...` | Abrir/stream remoto |
| GET | `/api/gallery` | Galeria |
| PUT | `/api/gallery/sync` | Upload da galeria Android |

## Testes

```bash
./scripts/test.sh
```

Docker (`docker compose`) é opcional e **não** é o caminho principal — use o serviço systemd.
