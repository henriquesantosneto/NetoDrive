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

```powershell
# No PC Windows (com Go) ou copie dist/netodrive-sync.exe
cd clients\desktop\windows
.\Install-NetoDrive.ps1
# ou rode o painel:
.\Start-NetoDrive.bat
```

O cliente abre um **painel local estilo OneDrive** (`-ui`): sincronizar agora, abrir pasta e abrir a web.  
Pasta padrão: `%USERPROFILE%\NetoDrive` · Config: `%APPDATA%\NetoDrive\netodrive.json`

Abrir um arquivo remoto sem sync completo:
```bat
netodrive-sync.exe -config %APPDATA%\NetoDrive\netodrive.json -open PC\docs\arquivo.pdf
```

Para gerar o `.exe` a partir do Linux/Mac: `./scripts/build.sh` → `dist/netodrive-sync.exe`

## Cliente Web

Já é servida pelo próprio serviço. Em dev:

```bash
cd web && npm install && npm run dev
```

## Cliente Android

Abra `clients/android` no **Android Studio**.

- Emulador → `http://10.0.2.2:8080`
- Aparelho → `http://IP-DO-SERVIDOR:8080`

App com navegação inferior estilo OneDrive:

1. **Arquivos** — navegar pastas remotas e **abrir** (baixa sob demanda)
2. **Galeria** — sincronizar fotos do aparelho + abrir remoto
3. **Cache** — modo cache LRU e orçamento em MB (libera espaço automaticamente)

## Testes

```bash
./scripts/test.sh
./scripts/e2e.sh
```

Docker (`docker compose`) é opcional e **não** é o caminho principal — use o serviço systemd.
