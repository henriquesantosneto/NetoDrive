# NetoDrive

Sincronize arquivos entre **Windows**, **Android** e a **Web**, com servidor em **Linux**.

## O que este projeto faz

| Componente | Função |
|---|---|
| `server/` | API de sync + storage em disco (blobs SHA-256) + JWT |
| `web/` | Interface web para navegar, enviar e **abrir arquivos remotos** |
| `clients/desktop/` | Cliente de pasta (Windows/Linux/macOS) — sync bidirecional |
| `clients/android/` | App Kotlin: sync da **galeria**, **modo cache** e abertura remota |

### Android — galeria + modo cache

1. **Sincronizar galeria**: envia fotos do `MediaStore` para `Gallery/` no servidor (com `gallery_key` estável).
2. **Modo cache**: miniaturas/arquivos são baixados sob demanda para um cache LRU em disco. Quando o orçamento (ex.: 512 MB) estoura, os itens menos usados são apagados — o original continua no servidor.
3. **Abrir remoto**: `RemoteFileProvider` baixa (ou reutiliza o cache) e abre com o visualizador do sistema.

### Abrir arquivos remotos

- **Web**: botão *Abrir* faz streaming via `/api/open/...` (Range/HTTP).
- **Android**: content provider + intent `ACTION_VIEW`.
- **Desktop**: após sync local, abra no explorador; ou use a URL `/api/open/<path>?token=...`.

## Subir o servidor (Linux)

```bash
# Opção A — direto
cd server
go run ./cmd/netodrive

# Opção B — Docker
docker compose up --build -d
```

Variáveis úteis:

| Variável | Padrão |
|---|---|
| `NETODRIVE_ADDR` | `:8080` |
| `NETODRIVE_DATA` | `./data` |
| `NETODRIVE_ADMIN_USER` | `admin` |
| `NETODRIVE_ADMIN_PASS` | `admin123` |
| `NETODRIVE_JWT_SECRET` | *(troque em produção)* |

Login web: `http://SERVIDOR:8080` → `admin` / `admin123`

## Cliente Windows (e Linux/macOS)

```bash
cd clients/desktop
go run ./cmd/netodrive-sync -init -config netodrive.json
# edite server_url, local_folder, remote_prefix
go run ./cmd/netodrive-sync -config netodrive.json
# ou um ciclo só:
go run ./cmd/netodrive-sync -config netodrive.json -once
```

Binário Windows: `scripts/build.sh` gera `dist/netodrive-sync.exe`.

Coloque a pasta a sincronizar em `local_folder` (ex.: `D:\NetoDrive`) e o prefixo remoto em `remote_prefix` (ex.: `PC`).

## Cliente Web (dev)

```bash
cd web
npm install
npm run dev
```

Proxy automático para `http://127.0.0.1:8080`.

## Cliente Android

Abra `clients/android` no **Android Studio**, configure o SDK e rode no aparelho/emulador.

- Emulador → servidor na máquina host: `http://10.0.2.2:8080`
- Aparelho físico → IP da LAN do servidor Linux, ex.: `http://192.168.0.10:8080`

Fluxo sugerido:

1. Entrar com usuário/senha  
2. Conceder permissão da galeria  
3. **Sincronizar galeria** (upload)  
4. Manter **Modo cache** ligado e ajustar o orçamento em MB  
5. Tocar em uma foto → **Abrir remoto** (baixa sob demanda)

## API (resumo)

| Método | Rota | Descrição |
|---|---|---|
| POST | `/api/auth/login` | JWT |
| GET | `/api/files?path=` | Lista pasta |
| PUT | `/api/sync/upload` | Upload (`X-File-Path`, body = bytes) |
| GET | `/api/sync/download/...` | Download |
| GET | `/api/open/...` | Abrir/stream remoto |
| GET | `/api/sync/manifest` | Manifesto para sync |
| GET | `/api/sync/changes?since=` | Feed de mudanças |
| GET | `/api/gallery` | Itens da galeria |
| PUT | `/api/gallery/sync` | Upload de mídia da galeria |

## Build / testes

```bash
./scripts/test.sh
./scripts/build.sh
```

## Arquitetura

```
Windows/Android/Web  --HTTPS/HTTP-->  NetoDrive (Linux)
                                      ├─ SQLite (metadados)
                                      └─ blobs/ (conteúdo por hash)
```

Sync por hash SHA-256: só sobe/baixa o que mudou. Galeria Android usa chaves `img-<mediaStoreId>` para não duplicar.
